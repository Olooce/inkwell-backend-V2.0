package service

import (
	"fmt"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/utilities"
	"log"
	"time"
)

// AnalysisService defines methods to analyze a story.
type AnalysisService interface {
	AnalyzeStory(story model.Story) (map[string]interface{}, error)
}

type analysisService struct {
	llmClient *llm.OllamaClient
}

// NewAnalysisService creates a new AnalysisService.
func NewAnalysisService(llmClient *llm.OllamaClient) AnalysisService {
	return &analysisService{
		llmClient: llmClient,
	}
}

// InitAnalysisEventListeners subscribes to the "story_completed" event.
func (a *analysisService) InitAnalysisEventListeners(storyRepo repository.StoryRepository) {
	utilities.GlobalEventBus.Subscribe("story_completed", func(data interface{}) {
		storyID, ok := data.(uint)
		if !ok {
			fmt.Println("Invalid story ID received for analysis")
			return
		}

		log.Printf("[Event] Story completed: Running analysis for story ID %d", storyID)

		story, err := storyRepo.GetStoryByID(storyID)
		if err != nil {
			log.Printf("Failed to fetch story: %v", err)
			return
		}

		analysisResult, err := a.AnalyzeStory(*story)
		if err != nil {
			log.Printf("Failed to analyze story: %v", err)
			return
		}

		// Extract analysis and tips from the result.
		analysisText, ok := analysisResult["analysis"].(string)
		if !ok {
			log.Println("Analysis text missing or not a string")
			return
		}
		tips, ok := analysisResult["tips"].([]string)
		if !ok {
			log.Println("Tips missing or not of type []string")
			return
		}

		// Update the story with the analysis.
		err = storyRepo.UpdateStoryAnalysis(storyID, analysisText, tips)
		if err != nil {
			log.Printf("Failed to update story analysis: %v", err)
			return
		}

		log.Printf("Successfully updated story with analysis for story ID %d", storyID)
	})
}

// AnalyzeStory generates a prompt from the story content, calls the LLM,
// and returns a structured analysis with writing tips.
func (a *analysisService) AnalyzeStory(story model.Story) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(
		`Please analyze the following story for structure, style, and common errors.
Return your response as JSON in the following format:
{
	"analysis": "Your analysis text",
	"tips": ["Tip 1", "Tip 2", ...]
}
Story Content:
%s`, story.Content)

	//// (Optionally add a context with timeout)
	//ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	//defer cancel()

	analysisResp, err := a.llmClient.AnalyzeText(prompt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"analysis":  analysisResp.Analysis,
		"tips":      analysisResp.Tips,
		"timestamp": time.Now(),
	}
	return result, nil
}
