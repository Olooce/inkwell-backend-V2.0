package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"inkwell-backend-V2.0/cmd/internal/config"
	"inkwell-backend-V2.0/cmd/internal/db"
	"inkwell-backend-V2.0/cmd/internal/llm"
	"inkwell-backend-V2.0/cmd/internal/model"
	"inkwell-backend-V2.0/cmd/internal/repository"
	"inkwell-backend-V2.0/cmd/internal/service"
	"inkwell-backend-V2.0/cmd/utilities"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
)

var (
	ollamaCmd        *exec.Cmd // Store the Ollama process
	diffussionClient *llm.StableDiffusionWrapper
	ollamaClient     *llm.OllamaClient
	wg               = &sync.WaitGroup{}
	storyRepo        repository.StoryRepository
)

func main() {
	utilities.SetupLogging("logs")

	cfg := loadConfig("config.xml")
	printStartUpBanner()

	initDatabase(cfg)
	initAuth(cfg)
	initThirdPartyClients(cfg)
	runMigrations()

	// Create repositories and register event listeners.
	userRepo, assessmentRepo, storyRepoLocal := createRepositories()
	storyRepo = storyRepoLocal // for use in event listener below
	registerEventListeners(storyRepo)

	// Run background tasks.
	runBackgroundTasks(storyRepo)

	// Create services.
	authService, userService, assessmentService, storyService := createServices(userRepo, assessmentRepo, storyRepo)

	// Initialize and configure Gin router.
	r := initRouter(cfg)

	// Register API routes.
	registerRoutes(r, authService, userService, assessmentService, storyService)

	// Start server and listen for termination signals.
	runServer(cfg, r)
}

//
// CONFIGURATION & INITIALIZATION FUNCTIONS
//

func loadConfig(path string) *config.APIConfig {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		utilities.Error("failed to load config: %v", err)
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
		utilities.Warn("Ollama not found locally. Using configured remote Ollama host: %s", ollamaHost)
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
		utilities.Error("AutoMigration Error: %v", err)
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
			utilities.Error("Error creating analysis for stories: %v", err)
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
	router := gin.Default()
	if err := router.SetTrustedProxies(cfg.Context.TrustedProxies.Proxies); err != nil {
		utilities.Error("Failed to set trusted proxies: %v", err)
	}
	// Register global middleware.
	router.Use(utilities.CORSMiddleware(), utilities.AuthMiddleware(), utilities.RateLimitMiddleware())
	return router
}

//
// ROUTE REGISTRATION
//

func registerRoutes(r *gin.Engine, authService service.AuthService, userService service.UserService, assessmentService service.AssessmentService, storyService service.StoryService) {
	// Auth routes.
	auth := r.Group("/auth")
	{
		auth.POST("/register", func(c *gin.Context) {
			var user model.User
			if err := c.ShouldBindJSON(&user); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
				return
			}
			if err := authService.Register(&user); err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
		})
		auth.POST("/login", func(c *gin.Context) {
			var creds struct {
				Email    string `json:"email"`
				AuthHash string `json:"authhash"`
			}
			if err := c.ShouldBindJSON(&creds); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
				return
			}
			user, err := authService.Login(creds.Email, creds.AuthHash)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, user)
		})
		auth.POST("/refresh", func(c *gin.Context) {
			var req struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
				return
			}
			newTokens, err := authService.RefreshTokens(req.RefreshToken)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, newTokens)
		})
	}

	// User routes.
	r.GET("/user", func(c *gin.Context) {
		users, err := userService.GetAllUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	})

	// Assessment routes.
	assessmentRoutes := r.Group("/assessments")
	{
		assessmentRoutes.POST("/start", func(c *gin.Context) {
			grammarTopics := []string{
				"Tenses", "Subject-Verb Agreement", "Active and Passive Voice",
				"Direct and Indirect Speech", "Punctuation Rules",
			}
			src := rand.NewSource(time.Now().UnixNano())
			ra := rand.New(src)
			selectedTopic := grammarTopics[ra.Intn(len(grammarTopics))]
			assessment, questions, err := assessmentService.CreateAssessment(c, selectedTopic)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"session_id": assessment.SessionID,
				"topic":      selectedTopic,
				"questions":  questions,
			})
		})

		assessmentRoutes.POST("/submit", func(c *gin.Context) {
			var req struct {
				SessionID  string `json:"session_id" binding:"required"`
				QuestionID uint   `json:"question_id" binding:"required"`
				Answer     string `json:"answer" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				log.Printf("Received payload: %+v", req)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: missing required fields"})
				return
			}
			assessment, err := assessmentService.GetAssessmentBySessionID(req.SessionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
				return
			}
			question, err := repository.NewAssessmentRepository().GetQuestionByID(req.QuestionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Question not found"})
				return
			}
			var belongs bool
			for _, q := range assessment.Questions {
				if q.ID == question.ID {
					belongs = true
					break
				}
			}
			if !belongs {
				log.Printf("Assessment Questions: %+v", assessment.Questions)
				log.Printf("Submitted Question ID: %d", req.QuestionID)
				c.JSON(http.StatusForbidden, gin.H{"error": "Question does not belong to this assessment"})
				return
			}
			isCorrect := question.CorrectAnswer == req.Answer
			feedback := "Incorrect"
			if isCorrect {
				feedback = "Correct"
			}
			answer := model.Answer{
				AssessmentID: assessment.ID,
				SessionID:    req.SessionID,
				QuestionID:   req.QuestionID,
				UserID:       assessment.UserID,
				Answer:       req.Answer,
				IsCorrect:    isCorrect,
				Feedback:     feedback,
			}
			answerResponse, err := assessmentService.SaveAnswer(&answer)
			if err != nil {
				log.Printf("Failed to save answer: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save answer"})
				return
			}
			c.JSON(http.StatusOK, answerResponse)
		})

		assessmentRoutes.GET("/:session_id", func(c *gin.Context) {
			sessionID := c.Param("session_id")
			assessment, err := assessmentService.GetAssessmentBySessionID(sessionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
				return
			}
			c.JSON(http.StatusOK, assessment)
		})
	}

	// Story routes.
	storyRoutes := r.Group("/stories")
	{
		storyRoutes.GET("/", func(c *gin.Context) {
			stories, err := storyService.GetStories()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, stories)
		})
		storyRoutes.POST("/start_story", func(c *gin.Context) {
			var req struct {
				Title string `json:"title" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
				return
			}
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
				return
			}
			uid, ok := userID.(uint)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
				return
			}
			// Check for an unfinished story.
			progress, err := storyService.GetProgress(uid)
			if err == nil && progress["story_status"] == "in_progress" {
				c.JSON(http.StatusOK, gin.H{
					"message":                "You have an unfinished story",
					"story_id":               progress["story_id"],
					"title":                  progress["title"],
					"guidance":               "Continue building on the story!",
					"current_sentence_count": progress["current_sentence_count"],
					"max_sentences":          progress["max_sentences"],
					"story_status":           progress["story_status"],
				})
				return
			}
			story, err := storyService.CreateStory(uid, req.Title)
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create story"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{
				"message":                "New story started",
				"story_id":               story.ID,
				"guidance":               "Begin with an exciting sentence!",
				"current_sentence_count": 0,
				"max_sentences":          5,
				"story_status":           "in_progress",
			})
		})
		storyRoutes.POST("/:id/add_sentence", func(c *gin.Context) {
			storyIDParam := c.Param("id")
			storyIDUint, err := strconv.ParseUint(storyIDParam, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid story ID"})
				return
			}
			var req struct {
				Sentence string `json:"sentence" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
				return
			}
			sentenceObj, err := storyService.AddSentence(uint(storyIDUint), req.Sentence)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add sentence"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"sentence": sentenceObj})
		})
		storyRoutes.POST("/:id/complete_story", func(c *gin.Context) {
			storyIDParam := c.Param("id")
			storyIDUint, err := strconv.ParseUint(storyIDParam, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid story ID"})
				return
			}
			if err := storyService.CompleteStory(uint(storyIDUint)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete story"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Story completed successfully"})
		})
		storyRoutes.GET("/progress", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
				return
			}
			uid, ok := userID.(uint)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
				return
			}
			progress, err := storyService.GetProgress(uid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get progress"})
				return
			}
			c.JSON(http.StatusOK, progress)
		})
		storyRoutes.GET("/comics", func(c *gin.Context) {
			userID, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
				return
			}
			uid, ok := userID.(uint)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
				return
			}
			comics, err := storyService.GetComicsByUser(uid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve comics"})
				return
			}
			c.JSON(http.StatusOK, comics)
		})
	}

	// Analysis routes.
	analysisRoutes := r.Group("/writing-skills/analysis")
	{
		analysisRoutes.GET("/", func(c *gin.Context) {
			userIDVal, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
				return
			}
			uid, ok := userIDVal.(uint)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
				return
			}
			stories, err := storyRepo.GetCompletedStoriesWithAnalysis(uid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch completed stories"})
				return
			}
			var analyzedStories []map[string]interface{}
			for _, story := range stories {
				analyzedStories = append(analyzedStories, map[string]interface{}{
					"story_id": story.ID,
					"title":    story.Title,
					"analysis": story.Analysis,
					"tips":     strings.Split(story.Tips, "\n"),
				})
			}
			c.JSON(http.StatusOK, gin.H{"stories": analyzedStories})
		})
		analysisRoutes.GET("/overview", func(c *gin.Context) {
			userIDVal, exists := c.Get("user_id")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
				return
			}
			uid, ok := userIDVal.(uint)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
				return
			}
			progressData, err := service.GenerateProgressData(db.GetDB(), uid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"initial_progress": progressData.InitialProgress,
				"current_progress": progressData.CurrentProgress,
			})
		})
		analysisRoutes.GET("/download_report", func(c *gin.Context) {
			reportType := c.Query("type")
			var filename string
			if reportType == "initial" {
				filename = "initial_progress_report.pdf"
			} else if reportType == "current" {
				filename = "progress_report.pdf"
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid report type"})
				return
			}
			pdfContent := []byte("%PDF-1.4 dummy pdf content")
			c.Header("Content-Disposition", "attachment; filename="+filename)
			c.Data(http.StatusOK, "application/pdf", pdfContent)
		})
	}

	// Static file serving.
	r.StaticFS("/static", http.Dir("./working"))
	r.GET("/download/comics/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := "./working/comics/" + filename
		if filepath.Ext(filename) == ".pdf" {
			c.Header("Content-Disposition", "attachment; filename="+filename)
			c.Header("Content-Type", "application/pdf")
		}
		c.File(filePath)
	})
}

//
// SERVER RUN & GRACEFUL SHUTDOWN
//

func runServer(cfg *config.APIConfig, router *gin.Engine) {
	// Set up channel for termination signals.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Run server in a goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
		if err := router.Run(addr); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for termination signal.
	<-signalChan
	utilities.Info("Received termination signal. Shutting down gracefully...")
	stopOllama()
	wg.Wait()
	utilities.Info("Application shut down successfully.")
	utilities.FlushLogs()
	os.Exit(0)
}

func printStartUpBanner() {
	myFigure := figure.NewFigure("INKWELL", "", true)
	myFigure.Print()

	fmt.Println("======================================================")
	fmt.Printf("INKWELL API (v%s)\n\n", "2.0.0-StoryScape")
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
			utilities.Error("Failed to close body: %v", err)
		}
	}(resp.Body)
	return resp.StatusCode == http.StatusOK
}

// Wait until Ollama is ready
func waitForOllama() {
	for i := 0; i < 10; i++ { // Try 10 times before failing
		if isOllamaRunning() {
			utilities.Info("Ollama is now ready.")
			return
		}
		utilities.Info("Waiting for Ollama to start...")
		time.Sleep(2 * time.Second)
	}
	utilities.Error("Ollama did not start in time.")
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
		utilities.Info("Model '%s' preloaded successfully.", modelName)
	} else {
		utilities.Warn("Failed to preload model '%s', status: %d", modelName, resp.StatusCode)
	}
}

// Stop Ollama on shutdown
func stopOllama() {
	if ollamaCmd == nil {
		log.Println("Ollama is not running.")
		return
	}

	var command string
	var args []string

	switch runtime.GOOS {
	case "windows":
		command = "taskkill"
		args = []string{"/F", "/IM", "ollama.exe"}
	case "darwin", "linux":
		command = "pkill"
		args = []string{"-f", "ollama"}
	default:
		log.Println("Unsupported OS for stopping Ollama")
		return
	}

	stopCmd := exec.Command(command, args...)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr

	err := stopCmd.Run()
	if err != nil {
		log.Printf("Failed to stop Ollama: %v", err)
		return
	}

	ollamaCmd = nil
	log.Println("Ollama stopped successfully")
}

func isOllamaInstalled() bool {
	cmd := exec.Command("ollama", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
