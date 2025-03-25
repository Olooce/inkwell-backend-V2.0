package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"inkwell-backend-V2.0/internal/llm"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"

	"inkwell-backend-V2.0/internal/config"
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/internal/service"
	"inkwell-backend-V2.0/utilities"
)

var ollamaCmd *exec.Cmd // Store the Ollama process
var ollamaClient *llm.OllamaClient
var diffussionClient *llm.StableDiffusionWrapper

func main() {
	utilities.SetupLogging("logs")

	// Load XML configuration from file.
	cfg, err := config.LoadConfig("config.xml")
	if err != nil {
		utilities.Error("failed to load config: %v", err)
	}

	printStartUpBanner()
	// Initialize DB using the loaded config.
	db.InitDBFromConfig(cfg)

	utilities.InitAuthConfig(cfg)

	//err = llm.AuthenticateHuggingFace(cfg)
	//if err != nil {
	//	log.Fatalf("Hugging Face authentication failed: %v", err)
	//}
	diffussionClient = &llm.StableDiffusionWrapper{AccessToken: cfg.ThirdParty.HFToken}

	//imagePath, err := diffussionClient.GenerateImage("A house")
	//if err != nil {
	//	fmt.Printf("Warning: Failed to generate image: %v\n", err)
	//} else {
	//	fmt.Println("Generated image at:", imagePath)
	//}

	// Attempt to start Ollama
	ollamaHost := cfg.ThirdParty.OllamaHost
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434" // Default if not set
	}

	if isOllamaInstalled() {
		startOllama()
		waitForOllama()
	} else {
		utilities.Warn("Ollama not found locally. Using configured remote Ollama host:", ollamaHost)
	}

	// Initialize Ollama Client
	ollamaClient = llm.NewOllamaClient(ollamaHost + "/api/generate")

	// Preload model only if using local Ollama
	if isOllamaInstalled() {
		preloadModel("mistral")
	}

	// Run migrations.
	err = db.GetDB().AutoMigrate(&model.User{}, &model.Assessment{}, &model.Question{}, &model.Answer{}, &model.Story{},
		&model.Sentence{}, &model.Comic{})
	if err != nil {
		utilities.Error("AutoMigration Error: %v", err)
		return
	}

	// Create repositories.
	userRepo := repository.NewUserRepository()
	assessmentRepo := repository.NewAssessmentRepository()
	storyRepo := repository.NewStoryRepository()

	// Register event listeners
	service.InitComicEventListeners(storyRepo)
	service.InitAnalysisEventListeners(storyRepo, ollamaClient)

	// Fire-and-forget: run GenerateMissingComics in the background.
	go func() {
		service.GenerateMissingComics(storyRepo)
	}()

	// Fire-and-forget: run CreateAnalysisForAllStoriesWithoutIt in the background.
	go func() {
		if err := service.CreateAnalysisForAllStoriesWithoutIt(storyRepo, ollamaClient); err != nil {
			utilities.Error("Error creating analysis for stories: %v", err)
		}
	}()

	// Create services.
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	assessmentService := service.NewAssessmentService(assessmentRepo, ollamaClient)
	storyService := service.NewStoryService(storyRepo, ollamaClient, diffussionClient)

	// Initialize Gin router.
	gin.SetMode(cfg.Context.Mode)
	r := gin.Default()

	// Set trusted proxies from config.
	if err := r.SetTrustedProxies(cfg.Context.TrustedProxies.Proxies); err != nil {
		utilities.Error("Failed to set trusted proxies: %v", err)
	}

	// CORS configuration.
	r.Use(utilities.CORSMiddleware())

	//Authentication middleware
	r.Use(utilities.AuthMiddleware())

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
			var request struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
				return
			}

			newTokens, err := authService.RefreshTokens(request.RefreshToken)
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
				"Tenses",
				"Subject-Verb Agreement",
				"Active and Passive Voice",
				"Direct and Indirect Speech",
				"Punctuation Rules",
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

		// Submit an answer
		assessmentRoutes.POST("/submit", func(c *gin.Context) {
			var req struct {
				SessionID  string `json:"session_id" binding:"required"`
				QuestionID uint   `json:"question_id" binding:"required"`
				Answer     string `json:"answer" binding:"required"`
			}

			// Validate JSON input
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: missing required fields"})
				return
			}

			// Fetch the assessment
			assessment, err := assessmentService.GetAssessmentBySessionID(req.SessionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
				return
			}

			// Fetch the question by ID
			question, err := assessmentRepo.GetQuestionByID(req.QuestionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Question not found"})
				return
			}

			// Ensure the question belongs to the current assessment
			var questionBelongsToAssessment bool
			for _, q := range assessment.Questions {
				if q.ID == question.ID {
					questionBelongsToAssessment = true
					break
				}
			}

			if !questionBelongsToAssessment {
				log.Printf("Assessment Questions: %+v", assessment.Questions)
				log.Printf("Submitted Question ID: %d", req.QuestionID)

				c.JSON(http.StatusForbidden, gin.H{"error": "Question does not belong to this assessment"})
				return
			}

			// Evaluate the answer
			isCorrect := question.CorrectAnswer == req.Answer
			feedback := "Incorrect"
			if isCorrect {
				feedback = "Correct"
			}

			// Save the answer
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

			// Return a successful response
			c.JSON(http.StatusOK, answerResponse)
		})

		// Get a specific assessment
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
	storiesGroup := r.Group("/stories")
	{
		// GET /stories: Get all stories
		storiesGroup.GET("/", func(c *gin.Context) {
			stories, err := storyService.GetStories()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, stories)
		})

		// POST /stories/start_story: Start a new story
		storiesGroup.POST("/start_story", func(c *gin.Context) {
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

			story, err := storyService.CreateStory(uid, req.Title)
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create story"})
				return
			}

			// Return the structure expected by the frontend:
			// { story_id, guidance, max_sentences }
			c.JSON(http.StatusCreated, gin.H{
				"story_id":      story.ID,
				"guidance":      "Begin with an exciting sentence!",
				"max_sentences": 5,
			})
		})

		// POST /stories/:id/add_sentence: Add a sentence to a story
		storiesGroup.POST("/:id/add_sentence", func(c *gin.Context) {
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
			// Return the sentence in the expected structure:
			// { sentence: { original_text, corrected_text, feedback, image_url } }
			c.JSON(http.StatusOK, gin.H{"sentence": sentenceObj})
		})

		// POST /stories/:id/complete_story: Mark a story as complete
		storiesGroup.POST("/:id/complete_story", func(c *gin.Context) {
			storyIDParam := c.Param("id")
			storyIDUint, err := strconv.ParseUint(storyIDParam, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid story ID"})
				return
			}

			err = storyService.CompleteStory(uint(storyIDUint))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete story"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Story completed successfully"})
		})

		// GET /stories/progress: Get progress of the current user's in-progress story
		storiesGroup.GET("/progress", func(c *gin.Context) {
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

		// GET /stories/comics: Get all generated comics for the user
		storiesGroup.GET("/comics", func(c *gin.Context) {
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

			// Fetch all completed stories with analysis for the user
			stories, err := storyRepo.GetCompletedStoriesWithAnalysis(uid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch completed stories"})
				return
			}

			// Transform data into a JSON-friendly response format
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

		// GET /writing-skills/analysis/overview
		analysisRoutes.GET("/overview", func(c *gin.Context) {
			// Retrieve the authenticated user's ID from context.
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

			// Generate progress data from the database.
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

		// GET /writing-skills/analysis/download_report/?type=initial or type=current
		analysisRoutes.GET("/download_report", func(c *gin.Context) {
			reportType := c.Query("type")
			var filename string

			// Determine the file name based on the report type.
			if reportType == "initial" {
				filename = "initial_progress_report.pdf"
			} else if reportType == "current" {
				filename = "progress_report.pdf"
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid report type"})
				return
			}

			pdfContent := []byte("%PDF-1.4 dummy pdf content")

			// Set headers so the browser downloads the file.
			c.Header("Content-Disposition", "attachment; filename="+filename)
			c.Data(http.StatusOK, "application/pdf", pdfContent)
		})
	}

	// Serve static files from "working" directory
	r.StaticFS("/static", http.Dir("./working"))

	// Serve PDFs with download headers
	r.GET("/download/comics/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := "./working/comics/" + filename

		// Check if it's a PDF
		if filepath.Ext(filename) == ".pdf" {
			c.Header("Content-Disposition", "attachment; filename="+filename)
			c.Header("Content-Type", "application/pdf")
		}

		c.File(filePath)
	})

	// Start the server
	addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

	// **Graceful shutdown handling in a separate goroutine**
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		utilities.Info("Received termination signal. Shutting down gracefully...")

		stopOllama()

		utilities.Info("Application shut down successfully.")
		os.Exit(0)
	}()

}

func printStartUpBanner() {
	myFigure := figure.NewFigure("INKWELL", "", true)
	myFigure.Print()

	fmt.Println("======================================================")
	fmt.Printf("INKWELL API (v%s)\n\n", "2.0.0-StoryScape")
}

// Start Ollama if not already running
func startOllama() {
	ollamaCmd = exec.Command("ollama", "serve")

	// Create a standard output pipe to filter logs
	stdoutPipe, _ := ollamaCmd.StdoutPipe()
	stderrPipe, _ := ollamaCmd.StderrPipe()

	// Start Ollama
	err := ollamaCmd.Start()
	if err != nil {
		utilities.Error("Failed to start Ollama: %v", err)
	}

	// Process standard output logs
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			utilities.Info(scanner.Text()) // Normal logs
		}
	}()

	// Process error output logs separately
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			utilities.Error("[OLLAMA WARNING]", scanner.Text()) // Prefix error logs
		}
	}()

	utilities.Info("Ollama started successfully")
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
	if ollamaCmd != nil {
		utilities.Info("Stopping Ollama...")
		err := ollamaCmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			utilities.Error("Failed to stop Ollama: %v", err)
		}
	}
}

func isOllamaInstalled() bool {
	cmd := exec.Command("ollama", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
