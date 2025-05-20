package controller

import (
	"inkwell-backend-V2.0/cmd/app/internal/model"
	"inkwell-backend-V2.0/cmd/app/internal/repository"
	"inkwell-backend-V2.0/cmd/app/internal/service"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"math/rand"
)

type AssessmentController struct {
	AssessmentService service.AssessmentService
}

func NewAssessmentController(assessmentService service.AssessmentService) *AssessmentController {
	return &AssessmentController{AssessmentService: assessmentService}
}

func (ac *AssessmentController) StartAssessment(c *gin.Context) {
	grammarTopics := []string{
		"Tenses", "Subject-Verb Agreement", "Active and Passive Voice",
		"Direct and Indirect Speech", "Punctuation Rules",
	}
	src := rand.NewSource(time.Now().UnixNano())
	ra := rand.New(src)
	selectedTopic := grammarTopics[ra.Intn(len(grammarTopics))]
	assessment, questions, err := ac.AssessmentService.CreateAssessment(c, selectedTopic)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"session_id": assessment.SessionID,
		"topic":      selectedTopic,
		"questions":  questions,
	})
}

func (ac *AssessmentController) SubmitAssessment(c *gin.Context) {
	var req struct {
		SessionID  string `json:"session_id" binding:"required"`
		QuestionID uint   `json:"question_id" binding:"required"`
		Answer     string `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Received payload: %+v", req)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: missing required fields"})
		return
	}
	assessment, err := ac.AssessmentService.GetAssessmentBySessionID(req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	question, err := repository.NewAssessmentRepository().GetQuestionByID(req.QuestionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Question not found"})
		return
	}
	var belongs bool
	for _, q := range assessment.Questions {
		if q.ID == question.ID {
			belongs = true
			break
		}
	}
	if !belongs {
		log.Printf("Assessment Questions: %+v", assessment.Questions)
		log.Printf("Submitted Question ID: %d", req.QuestionID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Question does not belong to this assessment"})
		return
	}
	isCorrect := question.CorrectAnswer == req.Answer
	feedback := "Incorrect"
	if isCorrect {
		feedback = "Correct"
	}
	answer := model.Answer{
		AssessmentID: assessment.ID,
		SessionID:    req.SessionID,
		QuestionID:   req.QuestionID,
		UserID:       assessment.UserID,
		Answer:       req.Answer,
		IsCorrect:    isCorrect,
		Feedback:     feedback,
	}
	answerResponse, err := ac.AssessmentService.SaveAnswer(&answer)
	if err != nil {
		log.Printf("Failed to save answer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save answer"})
		return
	}
	c.JSON(http.StatusOK, answerResponse)
}

func (ac *AssessmentController) GetAssessment(c *gin.Context) {
	sessionID := c.Param("session_id")
	assessment, err := ac.AssessmentService.GetAssessmentBySessionID(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Assessment not found"})
		return
	}
	c.JSON(http.StatusOK, assessment)
}
