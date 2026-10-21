package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"inkwell-backend-V2.0/pkg/event_bus"
)

// AnalysisService defines methods to analyze a story.
type AnalysisService interface {
	AnalyzeStory(story model.Story, sentences []model.Sentence) (map[string]interface{}, error)
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

// errNoSentences means a story has nothing to analyse yet.
var errNoSentences = errors.New("story has no sentences")

// InitAnalysisEventListeners subscribes to the "story_completed" event.
func InitAnalysisEventListeners(storyRepo repository.StoryRepository, ollamaClient *llm.OllamaClient) {
	event_bus.GlobalEventBus.Subscribe("story_completed", func(data interface{}) {
		storyID, ok := data.(uint)
		if !ok {
			log.Println("Invalid story ID received for analysis")
			return
		}
		log.Printf("[Event] Story completed: Running analysis for story ID %d", storyID)

		story, err := storyRepo.GetStoryByID(storyID)
		if err != nil {
			log.Printf("Failed to fetch story: %v", err)
			return
		}
		if err := analyzeAndSave(storyRepo, NewAnalysisService(ollamaClient), *story); err != nil {
			log.Printf("Failed to analyze story ID %d: %v", storyID, err)
			return
		}
		log.Printf("Successfully updated story with analysis for story ID %d", storyID)
	})
}

// CreateAnalysisForAllStoriesWithoutIt fetches all stories that lack analysis,
// runs analysis via the LLM, and updates the story with analysis, tips, and performance score.
func CreateAnalysisForAllStoriesWithoutIt(storyRepo repository.StoryRepository, ollamaClient *llm.OllamaClient) error {
	stories, err := storyRepo.GetStoriesWithoutAnalysis()
	if err != nil {
		return err
	}
	analysisSvc := NewAnalysisService(ollamaClient)
	for _, story := range stories {
		log.Printf("Analyzing story ID %d", story.ID)
		if err := analyzeAndSave(storyRepo, analysisSvc, story); err != nil {
			log.Printf("Failed to analyze story ID %d: %v", story.ID, err)
			continue
		}
		log.Printf("Successfully updated analysis for story ID %d", story.ID)
	}
	return nil
}

// analyzeAndSave analyses one story from its sentences and stores the result.
func analyzeAndSave(storyRepo repository.StoryRepository, svc AnalysisService, story model.Story) error {
	sentences, err := storyRepo.GetSentencesByStory(story.ID)
	if err != nil {
		return fmt.Errorf("failed to load sentences: %w", err)
	}
	result, err := svc.AnalyzeStory(story, sentences)
	if err != nil {
		return err
	}
	return storyRepo.UpdateStoryAnalysis(story.ID,
		result["analysis"].(string), result["tips"].([]string), result["performance_score"].(int))
}

// AnalyzeStory asks the LLM for feedback on a finished story. The story text is
// built from its sentences (Story.Content is never populated), and each
// sentence is shown as written and as corrected so the feedback can address the
// child's actual mistakes.
func (a *analysisService) AnalyzeStory(story model.Story, sentences []model.Sentence) (map[string]interface{}, error) {
	if len(sentences) == 0 {
		return nil, errNoSentences
	}

	var b strings.Builder
	for i, s := range sentences {
		fmt.Fprintf(&b, "%d. Child wrote: %s\n", i+1, s.OriginalText)
		if s.CorrectedText != "" && s.CorrectedText != s.OriginalText {
			fmt.Fprintf(&b, "   Corrected:   %s\n", s.CorrectedText)
		}
	}

	prompt := fmt.Sprintf(`You are a warm, encouraging writing teacher for children aged 7-12.
A child wrote this short story, one sentence at a time.

Title: %s
%s
Give feedback on the story's ideas, structure and the grammar mistakes the child made.
- "analysis": 2-4 short sentences, addressed to the child ("you"), starting with something they did well.
- "tips": 3 specific, practical tips for their next story, each one short sentence, based on their actual mistakes.
- "performance_score": an integer from 0 to 100 for writing quality, considering the child's age.`, story.Title, b.String())

	resp, err := a.llmClient.AnalyzeText(prompt)
	if err != nil {
		return nil, err
	}

	var tips []string
	for _, tip := range resp.Tips {
		if tip = strings.TrimSpace(tip); tip != "" {
			tips = append(tips, tip)
		}
	}
	if strings.TrimSpace(resp.Analysis) == "" && len(tips) == 0 {
		return nil, errors.New("model returned empty feedback")
	}

	return map[string]interface{}{
		"analysis":          strings.TrimSpace(resp.Analysis),
		"tips":              tips,
		"performance_score": min(max(resp.PerformanceScore, 0), 100),
		"timestamp":         time.Now(),
	}, nil
}
