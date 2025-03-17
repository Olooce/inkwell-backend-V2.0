package service

import (
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type StoryService interface {
	GetStories() ([]model.Story, error)
	CreateStory(userID uint, title string) (*model.Story, error)
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

func (s *storyService) CreateStory(userID uint, title string) (*model.Story, error) {
	story := &model.Story{
		UserID: userID,
		Title:  title,
	}

	err := s.storyRepo.CreateStory(story)
	if err != nil {
		return nil, err
	}

	return story, nil
}
