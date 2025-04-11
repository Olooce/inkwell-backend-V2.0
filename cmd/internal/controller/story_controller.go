package controller

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/cmd/internal/service"
)

type StoryController struct {
	StoryService service.StoryService
}

func NewStoryController(storyService service.StoryService) *StoryController {
	return &StoryController{StoryService: storyService}
}

// GetStories handles GET /stories/
func (sc *StoryController) GetStories(c *gin.Context) {
	stories, err := sc.StoryService.GetStories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stories)
}

// StartStory handles POST /stories/start_story
func (sc *StoryController) StartStory(c *gin.Context) {
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
	progress, err := sc.StoryService.GetProgress(uid)
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
	story, err := sc.StoryService.CreateStory(uid, req.Title)
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
}

// AddSentence handles POST /stories/:id/add_sentence
func (sc *StoryController) AddSentence(c *gin.Context) {
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
	sentenceObj, err := sc.StoryService.AddSentence(uint(storyIDUint), req.Sentence)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add sentence"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sentence": sentenceObj})
}

// CompleteStory handles POST /stories/:id/complete_story
func (sc *StoryController) CompleteStory(c *gin.Context) {
	storyIDParam := c.Param("id")
	storyIDUint, err := strconv.ParseUint(storyIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid story ID"})
		return
	}
	if err := sc.StoryService.CompleteStory(uint(storyIDUint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete story"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Story completed successfully"})
}

// GetProgress handles GET /stories/progress
func (sc *StoryController) GetProgress(c *gin.Context) {
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
	progress, err := sc.StoryService.GetProgress(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get progress"})
		return
	}
	c.JSON(http.StatusOK, progress)
}

// GetComics handles GET /stories/comics
func (sc *StoryController) GetComics(c *gin.Context) {
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
	comics, err := sc.StoryService.GetComicsByUser(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve comics"})
		return
	}
	c.JSON(http.StatusOK, comics)
}
