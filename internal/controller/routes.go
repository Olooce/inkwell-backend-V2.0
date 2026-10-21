package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/service"
	"inkwell-backend-V2.0/internal/speech"
)

func RegisterRoutes(
	r *gin.Engine,
	authService service.AuthService,
	userService service.UserService,
	assessmentService service.AssessmentService,
	storyService service.StoryService,
	ollamaClient *llm.OllamaClient,
	speechEngine *speech.Engine, // nil disables the speech endpoints
) {
	// Auth routes.
	authCtrl := NewAuthController(authService)
	authRoutes := r.Group("/auth")
	{
		authRoutes.POST("/register", authCtrl.Register)
		authRoutes.POST("/login", authCtrl.Login)
		authRoutes.POST("/refresh", authCtrl.Refresh)
	}

	// User routes.
	userCtrl := NewUserController(userService)
	r.GET("/user", userCtrl.GetCurrentUser)

	// Assessment routes.
	assessmentCtrl := NewAssessmentController(assessmentService)
	assessRoutes := r.Group("/assessments")
	{
		assessRoutes.POST("/start", assessmentCtrl.StartAssessment)
		assessRoutes.POST("/submit", assessmentCtrl.SubmitAssessment)
		assessRoutes.GET("/:session_id", assessmentCtrl.GetAssessment)
	}

	// Story routes.
	storyCtrl := NewStoryController(storyService)
	storyRoutes := r.Group("/stories")
	{
		storyRoutes.GET("/", storyCtrl.GetStories)
		storyRoutes.POST("/start_story", storyCtrl.StartStory)
		storyRoutes.POST("/:id/add_sentence", storyCtrl.AddSentence)
		storyRoutes.POST("/:id/complete_story", storyCtrl.CompleteStory)
		storyRoutes.GET("/progress", storyCtrl.GetProgress)
		storyRoutes.GET("/comics", storyCtrl.GetComics)
	}

	// Analysis routes.
	analysisCtrl := NewAnalysisController()
	analysisRoutes := r.Group("/writing-skills/analysis")
	{
		analysisRoutes.GET("/", analysisCtrl.GetCompletedStories)
		analysisRoutes.GET("/overview", analysisCtrl.GetOverview)
		analysisRoutes.GET("/download_report", analysisCtrl.DownloadReport)
	}

	// Chat routes - NEW
	chatCtrl := NewChatController(ollamaClient, speechEngine)
	chatRoutes := r.Group("/chat")
	{
		chatRoutes.POST("/stream", chatCtrl.StreamChat)
		chatRoutes.GET("/writing-tip", chatCtrl.GetWritingTip)
		chatRoutes.GET("/story-idea", chatCtrl.GetStoryIdea)
		chatRoutes.POST("/improve-text", chatCtrl.ImproveText)
		chatRoutes.POST("/speech-to-text", chatCtrl.SpeechToText)
		chatRoutes.POST("/text-to-speech", chatCtrl.TextToSpeech)
		chatRoutes.GET("/health", chatCtrl.ChatHealth)
	}

	// Static routes.
	staticCtrl := NewStaticController()
	r.StaticFS("/static", http.Dir("./working"))
	r.GET("/download/comics/:filename", staticCtrl.DownloadComic)
}
