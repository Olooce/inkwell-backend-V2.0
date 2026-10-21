package service

import (
	"context"
	"fmt"
	"log"

	"inkwell-backend-V2.0/internal/imagegen"
	llm2 "inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/pkg/event_bus"

	"time"
)

type StoryService interface {
	GetStories(userID uint) ([]model.Story, error)
	GetStory(storyID uint) (*model.Story, error)
	CreateStory(userID uint, title string) (*model.Story, error)
	AddSentence(storyID uint, sentence string) (*model.Sentence, error)
	CompleteStory(storyID uint) error
	GetProgress(userID uint) (map[string]interface{}, error)
	GetComicsByUser(userID uint) ([]ComicResponse, error)
}

type storyService struct {
	storyRepo repository.StoryRepository
	llmClient *llm2.OllamaClient
	images    imagegen.Generator // nil when no image backend is available
}

func NewStoryService(storyRepo repository.StoryRepository, llmClient *llm2.OllamaClient, images imagegen.Generator) StoryService {
	return &storyService{
		storyRepo: storyRepo,
		llmClient: llmClient,
		images:    images,
	}
}

func (s *storyService) GetStory(storyID uint) (*model.Story, error) {
	return s.storyRepo.GetStoryByID(storyID)
}

func (s *storyService) GetStories(userID uint) ([]model.Story, error) {
	return s.storyRepo.GetStoriesByUser(userID)
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

// AddSentence corrects the sentence with the LLM, illustrates the scene the
// LLM describes, and saves the result. Illustration waits for the review so
// the picture can use the disambiguated scene description.
func (s *storyService) AddSentence(storyID uint, sentence string) (*model.Sentence, error) {
	newSentence := &model.Sentence{
		StoryID:       storyID,
		OriginalText:  sentence,
		CorrectedText: sentence,
		Feedback:      "Could not generate feedback",
		CreatedAt:     time.Now(),
	}

	title := ""
	if story, err := s.storyRepo.GetStoryByID(storyID); err == nil {
		title = story.Title
	}
	var previous []string
	if earlier, err := s.storyRepo.GetSentencesByStory(storyID); err == nil {
		for _, e := range earlier {
			previous = append(previous, e.CorrectedText)
		}
	}

	imagePrompt := imagegen.StoryPrompt(title, sentence)
	if review, err := s.llmClient.ReviewSentence(title, previous, sentence); err != nil {
		log.Printf("Sentence review failed: %v", err)
	} else {
		newSentence.CorrectedText = review.Corrected
		if review.Feedback != "" {
			newSentence.Feedback = review.Feedback
		}
		if review.Scene != "" {
			imagePrompt = imagegen.ScenePrompt(review.Scene)
		}
	}

	if s.images == nil {
		log.Println("Warning: no image generator configured")
	} else if path, err := s.images.Generate(context.Background(), imagePrompt); err != nil {
		log.Printf("Warning: Failed to generate image: %v", err)
	} else {
		newSentence.ImageURL = path
	}

	// Save the sentence record.
	err := s.storyRepo.CreateSentence(newSentence)
	if err != nil {
		return nil, err
	}
	return newSentence, nil
}

func (s *storyService) CompleteStory(storyID uint) error {
	err := s.storyRepo.CompleteStory(storyID)
	if err != nil {
		return err
	}
	// Publish event for comic generation
	event_bus.GlobalEventBus.Publish("story_completed", storyID)

	return nil
}

func (s *storyService) GetProgress(userID uint) (map[string]interface{}, error) {
	story, err := s.storyRepo.GetCurrentStoryByUser(userID)
	if err != nil {
		return nil, err
	}

	if story.Status == "completed" {
		return map[string]interface{}{
			"message": "No active story",
		}, nil
	}

	count, err := s.storyRepo.GetSentenceCount(story.ID)
	if err != nil {
		return nil, err
	}

	const maxSentencesAllowed = 5
	sentencesLeft := maxSentencesAllowed - count

	progress := map[string]interface{}{
		"current_sentence_count": count,
		"max_sentences":          sentencesLeft,
		"story_status":           story.Status,
		"story_id":               story.ID,
		"title":                  story.Title,
	}
	return progress, nil
}

type ComicResponse struct {
	ID          uint   `json:"id"`
	UserID      uint   `json:"user_id"`
	StoryID     uint   `json:"story_id"`
	Title       string `json:"title"`
	Thumbnail   string `json:"thumbnail"`
	ViewURL     string `json:"view_url"`
	DownloadURL string `json:"download_url"`
	DoneOn      string `json:"done_on"`
}

func (s *storyService) GetComicsByUser(userID uint) ([]ComicResponse, error) {
	comics, err := s.storyRepo.GetComicsByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comics: %w", err)
	}

	var response []ComicResponse
	for _, comic := range comics {
		response = append(response, ComicResponse{
			ID:          comic.ID,
			UserID:      comic.UserID,
			StoryID:     comic.StoryID,
			Title:       comic.Title,
			Thumbnail:   comic.Thumbnail,
			ViewURL:     comic.ViewURL,
			DownloadURL: comic.DownloadURL,
			DoneOn:      comic.DoneOn.Format("2006-01-02 15:04:05"),
		})
	}

	return response, nil
}
