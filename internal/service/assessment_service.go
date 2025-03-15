package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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
	ollamaURL      string
}

func NewAssessmentService(assessmentRepo repository.AssessmentRepository) AssessmentService {
	return &assessmentService{
		assessmentRepo: assessmentRepo,
		ollamaURL:      "http://localhost:11434/api/generate", // Local Ollama instance
	}
}

// GenerateQuestions - Uses Ollama to generate assessment questions
func (s *assessmentService) GenerateQuestions(topic string) ([]model.Question, error) {
	prompt := fmt.Sprintf("Generate 5 multiple-choice questions on %s.", topic)

	response, err := s.callOllama(prompt)
	if err != nil {
		return nil, fmt.Errorf("error generating questions: %w", err)
	}

	questions := parseQuestions(response)

	return questions, nil
}

// CreateAssessment - Generates an assessment with AI-generated questions
func (s *assessmentService) CreateAssessment() (*model.Assessment, error) {
	sessionID := uuid.New().String()

	// Generate questions using Ollama
	questions, err := s.GenerateQuestions("General Knowledge")
	if err != nil {
		return nil, err
	}

	assessment := model.Assessment{
		UserID:      0, // Default
		SessionID:   sessionID,
		Title:       "AI-Generated Assessment",
		Description: "This assessment was generated using Ollama.",
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

// callOllama - Calls the Ollama API
func (s *assessmentService) callOllama(prompt string) (string, error) {
	requestBody, _ := json.Marshal(map[string]interface{}{
		"model":  "mistral", // Change to "llama2" or any available model
		"prompt": prompt,
	})

	req, err := http.NewRequest("POST", s.ollamaURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if responseText, ok := result["response"].(string); ok {
		return responseText, nil
	}

	return "", fmt.Errorf("invalid response from Ollama")
}

// parseQuestions - Parses AI-generated text into structured questions
func parseQuestions(response string) []model.Question {
	var questions []model.Question
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if line != "" {
			questions = append(questions, model.Question{Text: line})
		}
	}
	return questions
}
