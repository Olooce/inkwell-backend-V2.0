package service

import (
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type StoryService interface {
	GetStories() ([]model.Story, error)
}

type storyService struct {
	storyRepo repository.StoryRepository
}

func NewStoryService(storyRepo repository.StoryRepository) StoryService {
	return &storyService{storyRepo: storyRepo}
}

func (s *storyService) GetStories() ([]model.Story, error) {
	return s.storyRepo.GetStories()
}
