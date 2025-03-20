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
func InitAnalysisEventListeners(storyRepo repository.StoryRepository, ollamaClient *llm.OllamaClient) {
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

		analysisService := NewAnalysisService(ollamaClient)

		analysisResult, err := analysisService.AnalyzeStory(*story)
		if err != nil {
			log.Printf("Failed to analyze story: %v", err)
			return
		}

		// Extract analysis, tips, and performance score.
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
		perfScore, ok := analysisResult["performance_score"].(int)
		if !ok {
			// If the LLM returns a number as float64, you might need to convert:
			if scoreFloat, ok := analysisResult["performance_score"].(float64); ok {
				perfScore = int(scoreFloat)
			} else {
				log.Println("Performance score missing or invalid")
				return
			}
		}

		// Update the story with the analysis.
		err = storyRepo.UpdateStoryAnalysis(storyID, analysisText, tips, perfScore)
		if err != nil {
			log.Printf("Failed to update story analysis: %v", err)
			return
		}

		log.Printf("Successfully updated story with analysis for story ID %d", storyID)
	})
}

// / AnalyzeStory generates a prompt from the story content, calls the LLM,
// and returns a structured analysis with writing tips and a performance score.
func (a *analysisService) AnalyzeStory(story model.Story) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(
		`Please analyze the following story for structure, style, and common errors.
Return your response as JSON in the following format:
{
	"analysis": "Your analysis text",
	"tips": ["Tip 1", "Tip 2", ...],
	"performance_score": 85
}
Story Content:
%s`, story.Content)

	analysisResp, err := a.llmClient.AnalyzeText(prompt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"analysis":          analysisResp.Analysis,
		"tips":              analysisResp.Tips,
		"performance_score": analysisResp.PerformanceScore,
		"timestamp":         time.Now(),
	}
	return result, nil
}

// CreateAnalysisForAllStoriesWithoutIt fetches all stories that lack analysis,
// runs analysis via the LLM, and updates the story with analysis, tips, and performance score.
func CreateAnalysisForAllStoriesWithoutIt(storyRepo repository.StoryRepository, ollamaClient *llm.OllamaClient) error {
	// Instantiate the analysis service.
	analysisSvc := NewAnalysisService(ollamaClient)

	// Retrieve stories that do not have analysis yet.
	stories, err := storyRepo.GetStoriesWithoutAnalysis()
	if err != nil {
		return err
	}

	// Iterate over each story and generate analysis.
	for _, story := range stories {
		log.Printf("Analyzing story ID %d", story.ID)

		analysisResult, err := analysisSvc.AnalyzeStory(story)
		if err != nil {
			log.Printf("Failed to analyze story ID %d: %v", story.ID, err)
			continue
		}

		// Extract the analysis, tips, and performance score.
		analysisText, ok := analysisResult["analysis"].(string)
		if !ok {
			log.Printf("Analysis text missing or invalid for story ID %d", story.ID)
			continue
		}

		tips, ok := analysisResult["tips"].([]string)
		if !ok {
			log.Printf("Tips missing or invalid for story ID %d", story.ID)
			continue
		}

		perfScore, ok := analysisResult["performance_score"].(int)
		if !ok {
			// If the LLM returned a float, convert it to int.
			if scoreFloat, ok := analysisResult["performance_score"].(float64); ok {
				perfScore = int(scoreFloat)
			} else {
				log.Printf("Performance score missing or invalid for story ID %d", story.ID)
				continue
			}
		}

		// Update the story with the analysis data.
		if err := storyRepo.UpdateStoryAnalysis(story.ID, analysisText, tips, perfScore); err != nil {
			log.Printf("Failed to update analysis for story ID %d: %v", story.ID, err)
			continue
		}
		log.Printf("Successfully updated analysis for story ID %d", story.ID)
	}
	return nil
}
