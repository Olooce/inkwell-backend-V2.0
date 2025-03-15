package service

import (
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"

	"github.com/google/uuid"
)

type AssessmentService interface {
	CreateAssessment() (*model.Assessment, error)
	GetAssessments() ([]model.Assessment, error)
	GetAssessmentBySessionID(sessionID string) (*model.Assessment, error)
	SaveAnswer(answer *model.Answer) error
}

type assessmentService struct {
	assessmentRepo repository.AssessmentRepository
}

func NewAssessmentService(assessmentRepo repository.AssessmentRepository) AssessmentService {
	return &assessmentService{assessmentRepo: assessmentRepo}
}

// CreateAssessment - Starts a new assessment without requiring request input
func (s *assessmentService) CreateAssessment() (*model.Assessment, error) {
	sessionID := uuid.New().String()

	assessment := model.Assessment{
		UserID:      0, // Default or anonymous user ID
		SessionID:   sessionID,
		Title:       "New Assessment",
		Description: "Auto-generated assessment",
		Status:      "ongoing",
		Questions:   []model.Question{}, // No predefined questions
	}

	err := s.assessmentRepo.CreateAssessment(&assessment)
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
