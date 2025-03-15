package service

import (
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"

	"github.com/google/uuid"
)

type AssessmentService interface {
	CreateAssessment(userID uint, title string, description string, questions []model.Question) (*model.Assessment, error)
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

func (s *assessmentService) CreateAssessment(userID uint, title string, description string, questions []model.Question) (*model.Assessment, error) {
	sessionID := uuid.New().String()

	assessment := model.Assessment{
		UserID:      userID,
		SessionID:   sessionID,
		Title:       title,
		Description: description,
		Status:      "ongoing",
		Questions:   questions,
	}

	err := s.assessmentRepo.CreateAssessment(&assessment)
	if err != nil {
		return nil, err
	}

	return &assessment, nil
}

func (s *assessmentService) GetAssessments() ([]model.Assessment, error) {
	return s.assessmentRepo.GetAssessments()
}

func (s *assessmentService) GetAssessmentBySessionID(sessionID string) (*model.Assessment, error) {
	return s.assessmentRepo.GetAssessmentBySessionID(sessionID)
}

func (s *assessmentService) SaveAnswer(answer *model.Answer) error {
	return s.assessmentRepo.SaveAnswer(answer)
}
