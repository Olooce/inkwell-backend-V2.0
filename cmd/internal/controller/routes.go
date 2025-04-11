package controller

import (
	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/cmd/internal/service"
	"net/http"
)

func RegisterRoutes(
	r *gin.Engine,
	authService service.AuthService,
	userService service.UserService,
	assessmentService service.AssessmentService,
	storyService service.StoryService,
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

	// Static routes.
	staticCtrl := NewStaticController()
	r.StaticFS("/static", http.Dir("./working"))
	r.GET("/download/comics/:filename", staticCtrl.DownloadComic)
}
