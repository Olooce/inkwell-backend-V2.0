package service

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type AssessmentService interface {
	CreateAssessment(c *gin.Context, topic string) (*model.Assessment, []model.Question, error)
	GetAssessments() ([]model.Assessment, error)
	GetAssessmentBySessionID(sessionID string) (*model.Assessment, error)
	SaveAnswer(answer *model.Answer) (*model.AnswerResponse, error)
}

type assessmentService struct {
	assessmentRepo repository.AssessmentRepository
	ollamaClient   *llm.OllamaClient
}

func NewAssessmentService(assessmentRepo repository.AssessmentRepository, ollamaClient *llm.OllamaClient) AssessmentService {
	return &assessmentService{
		assessmentRepo: assessmentRepo,
		ollamaClient:   ollamaClient,
	}
}

func (s *assessmentService) CreateAssessment(c *gin.Context, topic string) (*model.Assessment, []model.Question, error) {
	sessionID := uuid.New().String()

	// Retrieve user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		return nil, nil, fmt.Errorf("user ID not found in token")
	}

	// Convert userID to the expected type (uint)
	userIDUint, ok := userID.(uint)
	if !ok {
		return nil, nil, fmt.Errorf("invalid user ID format")
	}

	// Fetch questions based on topic
	questions, err := s.assessmentRepo.GetRandomQuestions(topic, 5)
	if err != nil {
		return nil, nil, err
	}

	// Create assessment object
	assessment := model.Assessment{
		UserID:      userIDUint, // Assign user ID
		SessionID:   sessionID,
		Title:       fmt.Sprintf("%s Assessment", topic),
		Description: fmt.Sprintf("Assessment on %s", topic),
		Status:      "ongoing",
		Category:    topic,
		Questions:   questions,
	}

	// Save assessment and questions relation in DB
	err = s.assessmentRepo.CreateAssessment(&assessment)
	if err != nil {
		return nil, nil, err
	}

	return &assessment, questions, nil
}

// GetAssessments - Fetch all assessments
func (s *assessmentService) GetAssessments() ([]model.Assessment, error) {
	return s.assessmentRepo.GetAssessments()
}

// GetAssessmentBySessionID - Fetch a specific assessment by session ID
func (s *assessmentService) GetAssessmentBySessionID(sessionID string) (*model.Assessment, error) {
	return s.assessmentRepo.GetAssessmentBySessionID(sessionID)
}

func (s *assessmentService) SaveAnswer(answer *model.Answer) (*model.AnswerResponse, error) {
	// Fetch the assessment
	assessment, err := s.assessmentRepo.GetAssessmentBySessionID(answer.SessionID)
	if err != nil {
		return nil, fmt.Errorf("assessment not found")
	}

	// Fetch the question
	question, err := s.assessmentRepo.GetQuestionByID(answer.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("question not found")
	}

	// Extract question text based on type
	var questionText string
	if question.QuestionType == "masked" {
		questionText = question.MaskedSentence
	} else if question.QuestionType == "error_correction" {
		questionText = question.ErrorSentence
	} else {
		return nil, fmt.Errorf("unknown question type: %s", question.QuestionType)
	}

	// Evaluate the answer using LLM
	isCorrect, feedback, err := s.ollamaClient.EvaluateAnswer(questionText, answer.Answer, question.CorrectAnswer)
	if err != nil {
		return nil, err
	}

	// Save the answer result
	answer.IsCorrect = isCorrect
	answer.Feedback = feedback
	err = s.assessmentRepo.SaveAnswer(answer)
	if err != nil {
		return nil, err
	}

	// Update assessment score
	if isCorrect {
		assessment.Score += 1 // Increment score for correct answers
	}

	// Check if all questions have been answered
	answeredCount, err := s.assessmentRepo.CountAnswersByAssessmentID(assessment.ID)
	if err != nil {
		return nil, err
	}

	// If all questions are answered, mark assessment as completed
	if answeredCount >= len(assessment.Questions) {
		assessment.Status = "completed"

		// Mark user as having completed assessment
		err = s.assessmentRepo.MarkUserAssessmentCompleted(assessment.UserID)
		if err != nil {
			return nil, err
		}
	}

	// Save updated assessment
	err = s.assessmentRepo.UpdateAssessment(assessment)
	if err != nil {
		return nil, err
	}

	return &model.AnswerResponse{
		IsCorrect: isCorrect,
		Feedback:  feedback,
	}, nil
}
