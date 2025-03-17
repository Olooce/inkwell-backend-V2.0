package service

import (
	"time"

	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type StoryService interface {
	GetStories() ([]model.Story, error)
	CreateStory(userID uint, title string) (*model.Story, error)
	AddSentence(storyID uint, sentence string) (*model.Sentence, error)
	CompleteStory(storyID uint) error
	GetProgress(userID uint) (map[string]interface{}, error)
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

func (s *storyService) AddSentence(storyID uint, sentence string) (*model.Sentence, error) {
	newSentence := &model.Sentence{
		StoryID:       storyID,
		OriginalText:  sentence,
		CorrectedText: sentence,
		Feedback:      "Nice sentence!",
		ImageURL:      "default.png",
		CreatedAt:     time.Now(),
	}
	err := s.storyRepo.CreateSentence(newSentence)
	if err != nil {
		return nil, err
	}
	return newSentence, nil
}

func (s *storyService) CompleteStory(storyID uint) error {
	return s.storyRepo.CompleteStory(storyID)
}

func (s *storyService) GetProgress(userID uint) (map[string]interface{}, error) {
	story, err := s.storyRepo.GetCurrentStoryByUser(userID)
	if err != nil {
		return nil, err
	}
	count, err := s.storyRepo.GetSentenceCount(story.ID)
	if err != nil {
		return nil, err
	}
	// Build a progress response.
	progress := map[string]interface{}{
		"current_sentence_count": count,
		"max_sentences":          5,
		"story_status":           "in_progress",
	}
	return progress, nil
}
