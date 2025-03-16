package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"inkwell-backend-V2.0/internal/model"
	"net/http"
)

// LLMClient defines the interface for interacting with LLM services
type LLMClient interface {
	GenerateResponse(prompt string) (string, error)
}

// ollamaClient implements LLMClient for Ollama
type ollamaClient struct {
	url string
}

// NewLLMClient initializes an LLM client
func NewLLMClient(url string) LLMClient {
	return &ollamaClient{url: url}
}

// GenerateResponse - Calls LLM API and streams AI responses
func (c *ollamaClient) GenerateResponse(prompt string) (string, error) {
	requestBody, _ := json.Marshal(map[string]interface{}{
		"model":  "mistral",
		"prompt": prompt,
		"stream": true,
	})

	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read response stream
	var fullResponse string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fullResponse += scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return fullResponse, nil
}

// ParseQuestions - Converts LLM response to structured questions
func ParseQuestions(response string) []model.Question {
	var parsed struct {
		Questions []model.Question `json:"questions"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return nil
	}

	return parsed.Questions
}
