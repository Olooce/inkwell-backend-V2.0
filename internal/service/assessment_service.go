package service

import (
	"fmt"
	"github.com/google/uuid"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type AssessmentService interface {
	CreateAssessment(topic string) (*model.Assessment, []model.Question, error)
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

// CreateAssessment - Generates an assessment using either DB questions or AI-generated questions
func (s *assessmentService) CreateAssessment(topic string) (*model.Assessment, []model.Question, error) {
	sessionID := uuid.New().String()

	// Fetch questions based on category/topic
	questions, err := s.assessmentRepo.GetRandomQuestions(topic, 5)
	if err != nil {
		return nil, nil, err
	}

	// Create assessment without directly storing questions
	assessment := model.Assessment{
		UserID:      0, // Default
		SessionID:   sessionID,
		Title:       fmt.Sprintf("%s Assessment", topic),
		Description: fmt.Sprintf("Assessment on %s", topic),
		Status:      "ongoing",
		Category:    topic, // Used to fetch related questions
	}

	err = s.assessmentRepo.CreateAssessment(&assessment)
	if err != nil {
		return nil, nil, err
	}

	// Return both the assessment and the questions
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

// SaveAnswer - Stores an answer and evaluates it using the LLM module
func (s *assessmentService) SaveAnswer(answer *model.Answer) (*model.AnswerResponse, error) {
	// Fetch the question directly by ID
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

	// Use the LLM to evaluate the answer
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

	return &model.AnswerResponse{
		IsCorrect: isCorrect,
		Feedback:  feedback,
	}, nil

}
