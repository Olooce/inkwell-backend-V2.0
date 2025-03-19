package service

import (
	"fmt"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/model"
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
