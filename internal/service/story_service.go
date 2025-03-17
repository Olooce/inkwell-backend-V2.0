package service

import (
	"strings"
	"time"

	"inkwell-backend-V2.0/internal/llm"
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
	llmClient *llm.OllamaClient
}

func NewStoryService(storyRepo repository.StoryRepository, llmClient *llm.OllamaClient) StoryService {
	return &storyService{
		storyRepo: storyRepo,
		llmClient: llmClient,
	}
}

func (s *storyService) GetStories() ([]model.Story, error) {
	return s.storyRepo.GetStories()
}

func (s *storyService) CreateStory(userID uint, title string) (*model.Story, error) {
	story := &model.Story{
		UserID:  userID,
		Title:   title,
		Content: "",
		Status:  "in_progress",
	}
	err := s.storyRepo.CreateStory(story)
	if err != nil {
		return nil, err
	}
	return story, nil
}

func (s *storyService) AddSentence(storyID uint, sentence string) (*model.Sentence, error) {
	newSentence := &model.Sentence{
		StoryID:      storyID,
		OriginalText: sentence,
	}

	// Use the LLM to correct the sentence and generate feedback.
	prompt := "Please correct the following sentence if needed and provide feedback in the format 'Corrected: <corrected sentence> Feedback: <feedback message>': " + sentence
	correctedResponse, err := s.llmClient.CallOllama(prompt)
	if err != nil {
		// Fall back to original sentence if there's an error.
		newSentence.CorrectedText = sentence
		newSentence.Feedback = "Could not generate feedback"
	} else {
		// Parse the response. Expecting: "Corrected: ... Feedback: ..."
		parts := strings.Split(correctedResponse, "Feedback:")
		var correctedText, feedback string
		if len(parts) >= 2 {
			correctedPart := strings.TrimSpace(parts[0])
			if strings.HasPrefix(correctedPart, "Corrected:") {
				correctedText = strings.TrimSpace(strings.TrimPrefix(correctedPart, "Corrected:"))
			} else {
				correctedText = sentence
			}
			feedback = strings.TrimSpace(parts[1])
		} else {
			correctedText = sentence
			feedback = "No feedback provided"
		}
		newSentence.CorrectedText = correctedText
		newSentence.Feedback = feedback
	}

	newSentence.ImageURL = "default.png"
	newSentence.CreatedAt = time.Now()

	err = s.storyRepo.CreateSentence(newSentence)
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
	progress := map[string]interface{}{
		"current_sentence_count": count,
		"max_sentences":          5,
		"story_status":           story.Status,
	}
	return progress, nil
}
