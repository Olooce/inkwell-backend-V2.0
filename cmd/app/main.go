package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/context"
	"inkwell-backend-V2.0/internal/config"
	"inkwell-backend-V2.0/internal/controller"
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/internal/service"
	"inkwell-backend-V2.0/internal/speech"
	"inkwell-backend-V2.0/internal/utilities"
	Log "inkwell-backend-V2.0/pkg/logging"
	"inkwell-backend-V2.0/pkg/middleware"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
	"golang.org/x/term"
)

var (
	ollamaCmd       *exec.Cmd // Ollama process, only set if this app started it
	diffusionClient *llm.StableDiffusionWrapper
	ollamaClient    *llm.OllamaClient
	speechEngine    *speech.Engine
	wg              = &sync.WaitGroup{}
	cleanupOnce     sync.Once
)

func main() {
	// Make sure child processes don't outlive a panic in main.
	defer func() {
		if r := recover(); r != nil {
			cleanup()
			panic(r)
		}
	}()

	printStartUpBanner()

	cfg := loadConfig("config.xml")

	debugMode := cfg.Context.Mode != gin.ReleaseMode

	if cfg.Logging.MaxSizeMB <= 0 {
		log.Printf("Invalid MAX_SIZE_MB in config, must be > 0")
		os.Exit(1)
	}
	if cfg.Logging.MaxBackups < 0 || cfg.Logging.MaxAgeDays < 0 {
		log.Printf("Invalid MAX_BACKUPS or MAX_AGE_DAYS in config, must be >= 0")
		os.Exit(1)
	}

	Log.SetupLogging(Log.LoggingOptions{
		LogDir: struct {
			Path     string
			Relative bool
		}(cfg.Logging.LogDir),
		EnableDebug:  debugMode,
		MaxSizeMB:    cfg.Logging.MaxSizeMB,
		MaxBackups:   cfg.Logging.MaxBackups,
		MaxAgeDays:   cfg.Logging.MaxAgeDays,
		CompressLogs: cfg.Logging.CompressLogs,
	})

	initDatabase(cfg)
	initAuth(cfg)
	initThirdPartyClients(cfg)
	runMigrations()
	seedQuestions()

	// Create repositories and register event listeners.
	userRepo, assessmentRepo, storyRepo := createRepositories()
	registerEventListeners(storyRepo)

	// Run background tasks.
	runBackgroundTasks(storyRepo)

	// Create services.
	authService, userService, assessmentService, storyService := createServices(userRepo, assessmentRepo, storyRepo)

	// Initialize and configure Gin router.
	r := initRouter(cfg)

	// Register API routes.
	controller.RegisterRoutes(r, authService, userService, assessmentService, storyService, ollamaClient, speechEngine)

	// Start server and listen for termination signals.
	runServer(cfg, r)
}

//
// CONFIGURATION & INITIALIZATION FUNCTIONS
//

func loadConfig(path string) *config.APIConfig {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		log.Printf("failed to load config: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
	return cfg

}

func initDatabase(cfg *config.APIConfig) {
	db.InitDBFromConfig(cfg)
}

func initAuth(cfg *config.APIConfig) {
	utilities.InitAuthConfig(cfg)
}

func initThirdPartyClients(cfg *config.APIConfig) {
	// Load in-process STT/TTS models. Missing models only disable the speech endpoints.
	engine, err := speech.NewEngine(speech.Config{ModelDir: cfg.ThirdParty.SpeechModelDir})
	if err != nil {
		Log.Warn("Speech engine unavailable, STT/TTS endpoints will return 503: %v", err)
	} else {
		speechEngine = engine
	}

	// Initialize Stable Diffusion wrapper.
	diffusionClient = &llm.StableDiffusionWrapper{AccessToken: cfg.ThirdParty.HFToken}

	// Determine Ollama host.
	ollamaHost := cfg.ThirdParty.OllamaHost
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	ollamaModel := cfg.ThirdParty.OllamaModel
	if ollamaModel == "" {
		ollamaModel = "mistral"
	}

	// Start Ollama if installed and not already running (e.g. as a system service).
	if isOllamaInstalled() {
		if isOllamaRunning() {
			Log.Info("Ollama is already running; using the existing instance.")
		} else {
			startOllama()
			waitForOllama()
		}
	} else {
		Log.Warn("Ollama not found locally. Using configured remote Ollama host: %s", ollamaHost)
	}
	// Initialize Ollama Client.
	ollamaClient = llm.NewOllamaClient(ollamaHost + "/api/generate")
	// Preload model if using local Ollama.
	if isOllamaInstalled() {
		preloadModel(ollamaModel)
	}
}

func runMigrations() {
	err := db.GetDB().AutoMigrate(&model.User{}, &model.Assessment{}, &model.Question{}, &model.Answer{},
		&model.Story{}, &model.Sentence{}, &model.Comic{})
	if err != nil {
		Log.Error("AutoMigration Error: %v", err)
		exitWithCleanup(1)
	}
}

//
// REPOSITORIES & EVENT REGISTRATION
//

func createRepositories() (repository.UserRepository, repository.AssessmentRepository, repository.StoryRepository) {
	userRepo := repository.NewUserRepository()
	assessmentRepo := repository.NewAssessmentRepository()
	storyRepo := repository.NewStoryRepository()
	return userRepo, assessmentRepo, storyRepo
}

func registerEventListeners(storyRepo repository.StoryRepository) {
	service.InitComicEventListeners(storyRepo)
	service.InitAnalysisEventListeners(storyRepo, ollamaClient)
}

//
// BACKGROUND TASKS
//

func runBackgroundTasks(storyRepo repository.StoryRepository) {
	wg.Add(2)
	go func() {
		defer wg.Done()
		service.GenerateMissingComics(storyRepo)
	}()
	go func() {
		defer wg.Done()
		if err := service.CreateAnalysisForAllStoriesWithoutIt(storyRepo, ollamaClient); err != nil {
			Log.Error("Error creating analysis for stories: %v", err)
		}
	}()
}

//
// SERVICES & ROUTER INIT
//

func createServices(userRepo repository.UserRepository, assessmentRepo repository.AssessmentRepository, storyRepo repository.StoryRepository) (service.AuthService, service.UserService, service.AssessmentService, service.StoryService) {
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	assessmentService := service.NewAssessmentService(assessmentRepo, ollamaClient)
	storyService := service.NewStoryService(storyRepo, ollamaClient, diffusionClient)
	return authService, userService, assessmentService, storyService
}

func initRouter(cfg *config.APIConfig) *gin.Engine {
	gin.SetMode(cfg.Context.Mode)
	router := gin.New()
	if err := router.SetTrustedProxies(cfg.Context.TrustedProxies.Proxies); err != nil {
		Log.Error("Failed to set trusted proxies: %v", err)
	}
	// Register global middleware.
	middlewares := []gin.HandlerFunc{
		middleware.CORSMiddleware(),
		middleware.AuthMiddleware(),
		middleware.RateLimitMiddleware(),
		gin.Recovery(),
	}

	if cfg.RequestDump {
		middlewares = append(middlewares, middleware.RequestDumpMiddleware())
	}

	if cfg.Context.Mode != gin.ReleaseMode {
		middlewares = append(middlewares, gin.Logger())
	}

	router.Use(middlewares...)
	return router
}

func runServer(cfg *config.APIConfig, router *gin.Engine) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Log.Error("Server failed: %v", err)
			exitWithCleanup(1)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	<-signalChan
	Log.Info("Received termination signal. Shutting down gracefully...")

	cancel()

	// Stop accepting requests first so in-flight ones can still reach Ollama.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		Log.Warn("HTTP server shutdown error: %v", err)
	}

	//wait for goroutines, but force‑exit after 5s
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		Log.Info("All workers exited gracefully.")
	case <-time.After(5 * time.Second):
		Log.Warn("Timeout waiting for workers; forcing exit.")
	}

	cleanup()

	Log.Info("Application shut down.")
	os.Exit(0)
}

// cleanup stops child processes and releases native resources. It is safe to
// call more than once and from any exit path.
func cleanup() {
	cleanupOnce.Do(func() {
		stopOllama()
		if speechEngine != nil {
			speechEngine.Close()
		}
	})
}

// exitWithCleanup must be used instead of os.Exit once child processes may be
// running, because os.Exit skips deferred calls.
func exitWithCleanup(code int) {
	cleanup()
	os.Exit(code)
}

func printStartUpBanner() {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 60 {
		width = 80
	}

	myFigure := figure.NewFigure("INKWELL", "slant", true)
	lines := strings.Split(myFigure.String(), "\n")

	blue := "\033[34m"
	reset := "\033[0m"

	for _, line := range lines {
		spaces := (width - len(line)) / 2
		if spaces < 0 {
			spaces = 0
		}
		fmt.Printf("%s%s%s\n", strings.Repeat(" ", spaces), blue, line)
	}
	fmt.Print(reset)

	sep := strings.Repeat("=", width)
	fmt.Println(sep)

	banner := fmt.Sprintf("INKWELL API (v%s)\n\n", "2.0.0-StoryScape")
	spaces := (width - len(banner)) / 2
	if spaces < 0 {
		spaces = 0
	}
	fmt.Printf("%s%s\n\n", strings.Repeat(" ", spaces), banner)
}

// Start Ollama if not already running
func startOllama() {
	// Run "ollama serve" directly on every OS. Going through "cmd /C start" on
	// Windows detached the server, so it could never be stopped.
	cmd := exec.Command("ollama", "serve")
	configureChildProcess(cmd)

	// Create pipes for standard output and error.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		Log.Error("Failed to create Ollama stdout pipe: %v", err)
		exitWithCleanup(1)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		Log.Error("Failed to create Ollama stderr pipe: %v", err)
		exitWithCleanup(1)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		Log.Error("Failed to start Ollama: %v", err)
		exitWithCleanup(1)
	}
	ollamaCmd = cmd

	// Process standard output logs.
	go processLogs(stdoutPipe, "[OLLAMA INFO]")

	// Process error output logs with classification.
	go processLogs(stderrPipe, "[OLLAMA ERROR]")

	log.Println("Ollama started successfully")
}

// Handles log output
func processLogs(pipe io.Reader, prefix string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		text := scanner.Text()

		// Classify log levels
		if strings.Contains(text, "level=INFO") {
			log.Println("[OLLAMA INFO]", text)
		} else if strings.Contains(text, "level=WARN") {
			log.Println("[OLLAMA WARNING]", text)
		} else if strings.Contains(text, "level=ERROR") {
			log.Println("[OLLAMA ERROR]", text)
		} else {
			log.Println(prefix, text)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("%s Log reading error: %v", prefix, err)
	}
}

// Check if Ollama is already running
func isOllamaRunning() bool {
	resp, err := http.Get("http://localhost:11434")
	if err != nil {
		return false
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			Log.Error("Failed to close body: %v", err)
		}
	}(resp.Body)
	return resp.StatusCode == http.StatusOK
}

// Wait until Ollama is ready
func waitForOllama() {
	for i := 0; i < 10; i++ { // Try 10 times before failing
		if isOllamaRunning() {
			Log.Info("Ollama is now ready.")
			return
		}
		Log.Info("Waiting for Ollama to start...")
		time.Sleep(2 * time.Second)
	}
	Log.Error("Ollama did not start in time.")
}

// Preload Ollama model
func preloadModel(modelName string) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": modelName,
	})

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		Log.Error("Failed to preload model %s: %v", modelName, err)
		exitWithCleanup(1)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK {
		Log.Info("Model '%s' preloaded successfully.", modelName)
	} else {
		Log.Warn("Failed to preload model '%s', status: %d", modelName, resp.StatusCode)
	}
}

// stopOllama stops the Ollama server if this application started it. An
// Ollama instance that was already running (e.g. a system service) is left alone.
func stopOllama() {
	stopChildProcess("Ollama", ollamaCmd)
	ollamaCmd = nil
}

// stopChildProcess asks a child process (and its process group) to exit,
// force-killing it if it is still running after 5 seconds.
func stopChildProcess(name string, cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := terminateChild(cmd); err != nil {
		Log.Warn("Failed to terminate %s: %v", name, err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			Log.Info("%s exited with: %v", name, err)
		} else {
			Log.Info("%s exited gracefully.", name)
		}
	case <-time.After(5 * time.Second):
		if err := killChild(cmd); err != nil {
			Log.Warn("Failed to force-kill %s: %v", name, err)
		}
		<-done
		Log.Info("%s force-killed.", name)
	}
}

func isOllamaInstalled() bool {
	cmd := exec.Command("ollama", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
