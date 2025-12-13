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
	"runtime"
	"runtime/debug"
	"strconv"
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
	"inkwell-backend-V2.0/internal/utilities"
	Log "inkwell-backend-V2.0/pkg/logging"
	"inkwell-backend-V2.0/pkg/middleware"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
	"golang.org/x/term"
)

var (
	ollamaCmd        *exec.Cmd // Store the Ollama process
	sttTtsCmd        *exec.Cmd
	diffussionClient *llm.StableDiffusionWrapper
	ollamaClient     *llm.OllamaClient
	wg               = &sync.WaitGroup{}
)

func main() {
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
	controller.RegisterRoutes(r, authService, userService, assessmentService, storyService, ollamaClient)

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
	err := startSTTTTS(cfg)
	if err != nil {
		Log.Error("Failed to start STT/TTS service: %v", err)
		os.Exit(1)
	}
	waitForSTTTTS()

	// Initialize Stable Diffusion wrapper.
	diffussionClient = &llm.StableDiffusionWrapper{AccessToken: cfg.ThirdParty.HFToken}

	// Determine Ollama host.
	ollamaHost := cfg.ThirdParty.OllamaHost
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	// Start Ollama if installed.
	if isOllamaInstalled() {
		startOllama()
		waitForOllama()
	} else {
		Log.Warn("Ollama not found locally. Using configured remote Ollama host: %s", ollamaHost)
	}
	// Initialize Ollama Client.
	ollamaClient = llm.NewOllamaClient(ollamaHost + "/api/generate")
	// Preload model if using local Ollama.
	if isOllamaInstalled() {
		preloadModel("mistral")
	}
}

func runMigrations() {
	err := db.GetDB().AutoMigrate(&model.User{}, &model.Assessment{}, &model.Question{}, &model.Answer{},
		&model.Story{}, &model.Sentence{}, &model.Comic{})
	if err != nil {
		Log.Error("AutoMigration Error: %v", err)
		os.Exit(1)
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
	storyService := service.NewStoryService(storyRepo, ollamaClient, diffussionClient)
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
			os.Exit(1)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan
	Log.Info("Received termination signal. Shutting down gracefully...")

	//cancel any background work
	cancel()
	stopSTTTTS()
	stopOllama()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		Log.Warn("HTTP server shutdown error: %v", err)
	}

	//wait for your goroutines, but force‑exit after 5s
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

	Log.Info("Application shut down.")
	os.Exit(0)
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
	var command string
	var args []string

	switch runtime.GOOS {
	case "windows":
		command = "cmd"
		args = []string{"/C", "start", "ollama", "serve"}
	case "darwin", "linux":
		command = "ollama"
		args = []string{"serve"}
	default:
		log.Println("Unsupported OS for starting Ollama")
		return
	}

	ollamaCmd = exec.Command(command, args...)

	// Create pipes for standard output and error.
	stdoutPipe, err := ollamaCmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe: %v", err)
	}
	stderrPipe, err := ollamaCmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to create stderr pipe: %v", err)
	}

	// Start the process
	err = ollamaCmd.Start()
	if err != nil {
		log.Fatalf("Failed to start Ollama: %v", err)
	}

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
		log.Fatalf("Failed to preload model %s: %v", modelName, err)
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

// Stop Ollama on shutdown
func stopOllama() {
	// 1) Graceful shutdown of child process
	if ollamaCmd != nil && ollamaCmd.Process != nil {
		// send interrupt
		_ = ollamaCmd.Process.Signal(os.Interrupt)

		// wait up to 5s
		done := make(chan error, 1)
		go func() { done <- ollamaCmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				log.Printf("Ollama exited with error: %v", err)
			} else {
				log.Println("Ollama exited gracefully.")
			}
		case <-time.After(5 * time.Second):
			if err := ollamaCmd.Process.Kill(); err != nil {
				log.Printf("Failed to kill Ollama gracefully: %v", err)
			} else {
				log.Println("Ollama force‑killed.")
			}
		}
		ollamaCmd = nil
	}

	// 2) Fallback: find any remaining Ollama PIDs and kill them
	pids, err := listOllama()
	if err != nil {
		log.Printf("Could not list Ollama processes: %v", err)
		return
	}
	if len(pids) == 0 {
		log.Println("No stray Ollama processes found.")
		return
	}
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			log.Printf("FindProcess(%d) failed: %v", pid, err)
			continue
		}
		if err := proc.Kill(); err != nil {
			log.Printf("Failed to kill PID %d: %v", pid, err)
		} else {
			log.Printf("Killed stray Ollama PID %d", pid)
		}
	}
}

// listOllama returns PIDs of any running ‘ollama’ processes.
func listOllama() ([]int, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq ollama.exe", "/NH", "/FO", "CSV")
	case "darwin", "linux":
		cmd = exec.Command("pgrep", "-f", "ollama")
	default:
		return nil, fmt.Errorf("unsupported OS")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		// On Unix, pgrep returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		// On Windows, tasklist returns non-zero if no matches; treat similarly
		if runtime.GOOS == "windows" && len(out) == 0 {
			return nil, nil
		}
		return nil, err
	}

	var pids []int
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if runtime.GOOS == "windows" {
			parts := strings.Split(line, "\",\"")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], `"`)
				if pid, err := strconv.Atoi(pidStr); err == nil {
					pids = append(pids, pid)
				}
			}
		} else {
			if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids, scanner.Err()
}

func isOllamaInstalled() bool {
	cmd := exec.Command("ollama", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func startSTTTTS(cfg *config.APIConfig) error {
	cmd := exec.Command(cfg.Context.PythonVenv, "internal/service/tts-stt/tts-stt.py")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	sttTtsCmd = cmd
	Log.Info("STT/TTS service started")
	return nil
}

func waitForSTTTTS() {
	for i := 0; i < 10; i++ {
		resp, err := http.Get("http://localhost:8001/health")
		if err == nil {
			err := resp.Body.Close()
			if err != nil {
				return
			}
			if resp.StatusCode == 200 {
				Log.Info("STT/TTS is now ready.")
				return
			}
		}
		Log.Info("Waiting for STT/TTS to start...")
		time.Sleep(1 * time.Second)
	}
	Log.Error("STT/TTS did not start in time.")
}

func stopSTTTTS() {
	if sttTtsCmd != nil && sttTtsCmd.Process != nil {
		_ = sttTtsCmd.Process.Signal(os.Interrupt)
		
		// Wait up to 5s
		done := make(chan error, 1)
		go func() { done <- sttTtsCmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				Log.Warn("STT/TTS exited with error: %v", err)
			} else {
				Log.Info("STT/TTS exited gracefully.")
			}
		case <-time.After(5 * time.Second):
			if err := sttTtsCmd.Process.Kill(); err != nil {
				Log.Warn("Failed to kill STT/TTS gracefully: %v", err)
			} else {
				Log.Info("STT/TTS force-killed.")
			}
		}
		sttTtsCmd = nil
	}
}
