package service

import (
	"fmt"

	llm2 "inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/pkg/event_bus"

	"time"
)

type StoryService interface {
	GetStories() ([]model.Story, error)
	CreateStory(userID uint, title string) (*model.Story, error)
	AddSentence(storyID uint, sentence string) (*model.Sentence, error)
	CompleteStory(storyID uint) error
	GetProgress(userID uint) (map[string]interface{}, error)
	GetComicsByUser(userID uint) ([]ComicResponse, error)
}

type storyService struct {
	storyRepo       repository.StoryRepository
	llmClient       *llm2.OllamaClient
	diffusionClient *llm2.StableDiffusionWrapper
}

func NewStoryService(storyRepo repository.StoryRepository, llmClient *llm2.OllamaClient, diffusionClient *llm2.StableDiffusionWrapper) StoryService {
	return &storyService{
		storyRepo:       storyRepo,
		llmClient:       llmClient,
		diffusionClient: diffusionClient,
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

// result types for each asynchronous call
type llmResult struct {
	corrected string
	feedback  string
	err       error
}

type imageResult struct {
	path string
	err  error
}

// AddSentence creates a new sentence record, corrects the sentence using the LLM client,
// generates an image using the diffusion client, and saves the result.
func (s *storyService) AddSentence(storyID uint, sentence string) (*model.Sentence, error) {
	// Create a new sentence record with the original text.
	newSentence := &model.Sentence{
		StoryID:      storyID,
		OriginalText: sentence,
		CreatedAt:    time.Now(),
	}

	// Channels to receive results.
	llmCh := make(chan llmResult)
	imageCh := make(chan imageResult)

	// Run LLM correction concurrently.
	go func() {
		corrected, feedback, err := s.llmClient.CorrectSentence(sentence)
		llmCh <- llmResult{corrected: corrected, feedback: feedback, err: err}
	}()

	// Run image generation concurrently.
	go func() {
		comicPrompt := "Comic-style illustration with bold outlines, vibrant colors, and dynamic poses. Scene: " + sentence + ". " +
			"Expressive characters and engaging composition like a graphic novel. Use strong lighting and shading for depth."

		path, err := s.diffusionClient.GenerateImage(comicPrompt)
		imageCh <- imageResult{path: path, err: err}
	}()

	// Wait for both operations to complete.
	llmRes := <-llmCh
	imgRes := <-imageCh

	// Set corrected text and feedback.
	if llmRes.err != nil {
		newSentence.CorrectedText = sentence
		newSentence.Feedback = "Could not generate feedback"
	} else {
		newSentence.CorrectedText = llmRes.corrected
		newSentence.Feedback = llmRes.feedback
	}

	// Set image URL.
	if imgRes.err != nil {
		fmt.Printf("Warning: Failed to generate image: %v\n", imgRes.err)
		newSentence.ImageURL = ""
	} else {
		fmt.Println("Generated image at:", imgRes.path)
		newSentence.ImageURL = imgRes.path
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
