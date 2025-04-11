package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/cmd/internal/db"
	"inkwell-backend-V2.0/cmd/internal/repository"
	"inkwell-backend-V2.0/cmd/internal/service"
)

type AnalysisController struct {
}

func NewAnalysisController() *AnalysisController {
	return &AnalysisController{}
}

// GetCompletedStories handles GET /writing-skills/analysis/
func (ac *AnalysisController) GetCompletedStories(c *gin.Context) {
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
	stories, err := repository.NewAssessmentRepository().GetCompletedStoriesWithAnalysis(uid)
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
}

// GetOverview handles GET /writing-skills/analysis/overview
func (ac *AnalysisController) GetOverview(c *gin.Context) {
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
}

// DownloadReport handles GET /writing-skills/analysis/download_report
func (ac *AnalysisController) DownloadReport(c *gin.Context) {
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
}
