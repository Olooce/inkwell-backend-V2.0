package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/cmd/app/internal/llm"
	service2 "inkwell-backend-V2.0/cmd/app/internal/service"
)

func RegisterRoutes(
	r *gin.Engine,
	authService service2.AuthService,
	userService service2.UserService,
	assessmentService service2.AssessmentService,
	storyService service2.StoryService,
	ollamaClient *llm.OllamaClient, // Add ollama client parameter
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
	r.GET("/user", userCtrl.GetAllUsers)

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
	chatCtrl := NewChatController(ollamaClient)
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

	//// API routes for frontend compatibility
	//apiRoutes := r.Group("/api")
	//{
	//	// Mirror chat routes under /api for frontend
	//	apiChatRoutes := apiRoutes.Group("/chat")
	//	{
	//		apiChatRoutes.POST("/stream", chatCtrl.StreamChat)
	//		apiChatRoutes.GET("/writing-tip", chatCtrl.GetWritingTip)
	//		apiChatRoutes.GET("/story-idea", chatCtrl.GetStoryIdea)
	//		apiChatRoutes.POST("/improve-text", chatCtrl.ImproveText)
	//		chatRoutes.POST("/speech-to-text", chatCtrl.SpeechToText)
	//		chatRoutes.POST("/text-to-speech", chatCtrl.TextToSpeech)
	//		apiChatRoutes.GET("/health", chatCtrl.ChatHealth)
	//	}
	//}

	// Static routes.
	staticCtrl := NewStaticController()
	r.StaticFS("/static", http.Dir("./working"))
	r.GET("/download/comics/:filename", staticCtrl.DownloadComic)
}
