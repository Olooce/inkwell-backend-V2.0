package main

import (
	"fmt"
	"inkwell-backend-V2.0/internal/llm"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"inkwell-backend-V2.0/internal/config"
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/internal/service"
)

var ollamaCmd *exec.Cmd // Store the Ollama process

func main() {
	printStartUpBanner()
	startOllama()

	// Load XML configuration from file.
	cfg, err := config.LoadConfig("config.xml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize DB using the loaded config.
	db.InitDBFromConfig(cfg)
	// Run migrations.
	db.GetDB().AutoMigrate(&model.User{}, &model.Assessment{}, &model.Question{}, &model.Story{})

	// Create repositories.
	userRepo := repository.NewUserRepository()
	assessmentRepo := repository.NewAssessmentRepository()
	storyRepo := repository.NewStoryRepository()

	// Create services.
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	ollamaClient := llm.NewOllamaClient("http://localhost:11434")
	assessmentService := service.NewAssessmentService(assessmentRepo, ollamaClient)

	storyService := service.NewStoryService(storyRepo)

	// Initialize Gin router.
	r := gin.Default()

	// CORS configuration.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

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

			rand.Seed(time.Now().UnixNano()) // Ensure randomness
			selectedTopic := grammarTopics[rand.Intn(len(grammarTopics))]

			assessment, questions, err := assessmentService.CreateAssessment(selectedTopic)
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

			// Fetch the question directly by ID instead of looping through assessment.Questions
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
	r.GET("/stories", func(c *gin.Context) {
		stories, err := storyService.GetStories()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stories)
	})
	// **Graceful shutdown handling in a separate goroutine**
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		log.Println("Received termination signal. Shutting down gracefully...")

		stopOllama() // Stop Ollama

		log.Println("Application shut down successfully.")
		os.Exit(0)
	}()

	// Start the server
	addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func printStartUpBanner() {
	myFigure := figure.NewFigure("INKWELL", "", true)
	myFigure.Print()

	fmt.Println("======================================================")
	fmt.Printf("INKWELL API (v%s)\n\n", "2.0.0-StoryScape")
}

// Start Ollama when the app starts
func startOllama() {
	ollamaCmd = exec.Command("ollama", "serve")

	// Redirect Ollama logs to the terminal
	ollamaCmd.Stdout = os.Stdout
	ollamaCmd.Stderr = os.Stderr

	// Start Ollama
	err := ollamaCmd.Start()
	if err != nil {
		log.Fatalf("Failed to start Ollama: %v", err)
	}
	log.Println("Ollama started successfully")
}

// Stop Ollama when the app exits
func stopOllama() {
	if ollamaCmd != nil {
		log.Println("Stopping Ollama...")
		err := ollamaCmd.Process.Signal(syscall.SIGTERM) // Gracefully stop Ollama
		if err != nil {
			log.Printf("Failed to stop Ollama: %v", err)
		}
	}
}
