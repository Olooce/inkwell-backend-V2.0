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
	"strconv"
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
	// Load XML configuration from file.
	cfg, err := config.LoadConfig("config.xml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	printStartUpBanner()
	// Initialize DB using the loaded config.
	db.InitDBFromConfig(cfg)

	//err = llm.AuthenticateHuggingFace(cfg)
	//if err != nil {
	//	log.Fatalf("Hugging Face authentication failed: %v", err)
	//}
	diffussionClient = &llm.StableDiffusionWrapper{AccessToken: cfg.THIRD_PARTY.HFToken}

	//imagePath, err := diffussionClient.GenerateImage("A house")
	//if err != nil {
	//	fmt.Printf("Warning: Failed to generate image: %v\n", err)
	//} else {
	//	fmt.Println("Generated image at:", imagePath)
	//}

	startOllama()

	// Wait until Ollama is responsive before proceeding
	waitForOllama()

	// Initialize Ollama Client
	ollamaClient = llm.NewOllamaClient("http://localhost:11434/api/generate")

	// Preload the model
	preloadModel("mistral")

	// Run migrations.
	err = db.GetDB().AutoMigrate(&model.User{}, &model.Assessment{}, &model.Question{}, &model.Answer{}, &model.Story{}, &model.Sentence{})
	if err != nil {
		log.Fatalf("AutoMigration Error: %v", err)
		return
	}

	// Create repositories.
	userRepo := repository.NewUserRepository()
	assessmentRepo := repository.NewAssessmentRepository()
	storyRepo := repository.NewStoryRepository()

	// Create services.
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	assessmentService := service.NewAssessmentService(assessmentRepo, ollamaClient)

	storyService := service.NewStoryService(storyRepo, ollamaClient, diffussionClient)

	// Initialize Gin router.
	r := gin.Default()

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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create story"})
				return
			}

			// Return the structure expected by the frontend:
			// { story_id, guidance, max_sentences }
			c.JSON(http.StatusCreated, gin.H{
				"story_id":      story.ID,
				"guidance":      "Begin with an exciting sentence!", // example guidance
				"max_sentences": 5,                                  // default maximum sentence count
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

	// Serve static files from "working" directory
	r.StaticFS("/static", http.Dir("./working"))

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
		log.Println("Received termination signal. Shutting down gracefully...")

		stopOllama()

		log.Println("Application shut down successfully.")
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
		log.Fatalf("Failed to start Ollama: %v", err)
	}

	// Process standard output logs
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			log.Println(scanner.Text()) // Normal logs
		}
	}()

	// Process error output logs separately
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			log.Println("[OLLAMA WARNING]", scanner.Text()) // Prefix error logs
		}
	}()

	log.Println("Ollama started successfully")
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
			log.Printf("Failed to close body: %v", err)
		}
	}(resp.Body)
	return resp.StatusCode == http.StatusOK
}

// Wait until Ollama is ready
func waitForOllama() {
	for i := 0; i < 10; i++ { // Try 10 times before failing
		if isOllamaRunning() {
			log.Println("Ollama is now ready.")
			return
		}
		log.Println("Waiting for Ollama to start...")
		time.Sleep(2 * time.Second)
	}
	log.Fatal("Ollama did not start in time.")
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Model '%s' preloaded successfully.", modelName)
	} else {
		log.Printf("Failed to preload model '%s', status: %d", modelName, resp.StatusCode)
	}
}

// Stop Ollama on shutdown
func stopOllama() {
	if ollamaCmd != nil {
		log.Println("Stopping Ollama...")
		err := ollamaCmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Printf("Failed to stop Ollama: %v", err)
		}
	}
}
