package controller

import (
	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/cmd/internal/service"
)

// RegisterRoutes registers all route groups and their endpoints.
func RegisterRoutes(r *gin.Engine,
	authService service.AuthService,
	userService service.UserService,
	assessmentService service.AssessmentService,
	storyService service.StoryService,
	progressService service.ProgressService, // if required by analysis endpoints
) {
	registerAuthRoutes(r, authService)
	registerUserRoutes(r, userService)
	registerAssessmentRoutes(r, assessmentService)

	// Story routes via StoryController.
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

	// Analysis routes via AnalysisController.
	analysisCtrl := NewAnalysisController()
	analysisRoutes := r.Group("/writing-skills/analysis")
	{
		analysisRoutes.GET("/", analysisCtrl.GetCompletedStories)
		analysisRoutes.GET("/overview", analysisCtrl.GetOverview)
		analysisRoutes.GET("/download_report", analysisCtrl.DownloadReport)
	}

	registerStaticRoutes(r)
}
