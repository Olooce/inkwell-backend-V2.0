package main

import (
	"fmt"
	"log"
	"net/http"
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

func main() {
	printStartUpBanner()

	// Load XML configuration from file.
	cfg, err := config.LoadConfig("config.xml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize DB using the loaded config.
	db.InitDBFromConfig(cfg)
	// Run migrations.
	db.GetDB().AutoMigrate(&model.User{}, &model.Assessment{}, &model.Story{})

	// Create repositories.
	userRepo := repository.NewUserRepository()
	assessmentRepo := repository.NewAssessmentRepository()
	storyRepo := repository.NewStoryRepository()

	// Create services.
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	assessmentService := service.NewAssessmentService(assessmentRepo)
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
	r.POST("/assessments", func(c *gin.Context) {
		var assessment model.Assessment
		if err := c.ShouldBindJSON(&assessment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}
		if err := assessmentService.CreateAssessment(&assessment); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, assessment)
	})
	r.GET("/assessments", func(c *gin.Context) {
		assessments, err := assessmentService.GetAssessments()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, assessments)
	})

	// Story routes.
	r.GET("/stories", func(c *gin.Context) {
		stories, err := storyService.GetStories()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stories)
	})

	// Start server on the host and port specified in the XML config.
	addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
	r.Run(addr)
}

func printStartUpBanner() {
	myFigure := figure.NewFigure("INKWELL", "", true)
	myFigure.Print()

	fmt.Println("======================================================")
	fmt.Printf("INKWELL API (v%s)\n\n", "2.0.0-StoryScape")
}
