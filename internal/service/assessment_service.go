package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-skynet/go-llama.cpp"
	"github.com/google/uuid"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type AssessmentService interface {
	CreateAssessment() (*model.Assessment, error)
	GetAssessments() ([]model.Assessment, error)
	GetAssessmentBySessionID(sessionID string) (*model.Assessment, error)
	SaveAnswer(answer *model.Answer) error
}

type assessmentService struct {
	assessmentRepo repository.AssessmentRepository
	llmModel       *llama.LLama
}

func NewAssessmentService(assessmentRepo repository.AssessmentRepository, modelPath string) AssessmentService {
	// Load LLaMA model
	llm, err := llama.New(modelPath, llama.SetContext(512))
	if err != nil {
		log.Fatalf("Failed to load LLaMA model: %v", err)
	}

	return &assessmentService{
		assessmentRepo: assessmentRepo,
		llmModel:       llm,
	}
}

// GenerateQuestions - Uses LLaMA to generate assessment questions
func (s *assessmentService) GenerateQuestions(topic string) ([]model.Question, error) {
	prompt := fmt.Sprintf("Generate 5 multiple-choice questions on %s.", topic)

	response, err := s.llmModel.Predict(prompt, llama.Debug)
	if err != nil {
		return nil, fmt.Errorf("error generating questions: %w", err)
	}

	// Parse response into question list
	questions := parseQuestions(response)

	return questions, nil
}

// CreateAssessment - Generates an assessment with AI-generated questions
func (s *assessmentService) CreateAssessment() (*model.Assessment, error) {
	sessionID := uuid.New().String()

	// Generate questions using LLaMA
	questions, err := s.GenerateQuestions("General Knowledge")
	if err != nil {
		return nil, err
	}

	assessment := model.Assessment{
		UserID:      0, // Default
		SessionID:   sessionID,
		Title:       "AI-Generated Assessment",
		Description: "This assessment was generated using LLaMA.",
		Status:      "ongoing",
		Questions:   questions,
	}

	err = s.assessmentRepo.CreateAssessment(&assessment)
	if err != nil {
		return nil, err
	}

	return &assessment, nil
}

// GetAssessments - Fetch all assessments
func (s *assessmentService) GetAssessments() ([]model.Assessment, error) {
	return s.assessmentRepo.GetAssessments()
}

// GetAssessmentBySessionID - Fetch a specific assessment by session ID
func (s *assessmentService) GetAssessmentBySessionID(sessionID string) (*model.Assessment, error) {
	return s.assessmentRepo.GetAssessmentBySessionID(sessionID)
}

// SaveAnswer - Stores an answer for a given assessment question
func (s *assessmentService) SaveAnswer(answer *model.Answer) error {
	return s.assessmentRepo.SaveAnswer(answer)
}

// parseQuestions - Parses AI-generated text into structured questions
func parseQuestions(response string) []model.Question {
	var questions []model.Question
	// Simple parsing logic
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if line != "" {
			questions = append(questions, model.Question{Text: line})
		}
	}
	return questions
}
