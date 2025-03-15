package service

import (
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type AssessmentService interface {
	CreateAssessment(assessment *model.Assessment) error
	GetAssessments() ([]model.Assessment, error)
}

type assessmentService struct {
	assessmentRepo repository.AssessmentRepository
}

func NewAssessmentService(assessmentRepo repository.AssessmentRepository) AssessmentService {
	return &assessmentService{assessmentRepo: assessmentRepo}
}

func (s *assessmentService) CreateAssessment(assessment *model.Assessment) error {
	return s.assessmentRepo.CreateAssessment(assessment)
}

func (s *assessmentService) GetAssessments() ([]model.Assessment, error) {
	return s.assessmentRepo.GetAssessments()
}
