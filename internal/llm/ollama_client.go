package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type OllamaClient struct {
	ollamaURL string
	client    *http.Client
}

func NewOllamaClient(url string) *OllamaClient {
	return &OllamaClient{
		ollamaURL: url,
		client: &http.Client{
			Timeout: 600 * time.Second, // Set a timeout to avoid hanging requests
		},
	}
}

// StreamResponse represents a streaming response chunk from Ollama
type StreamResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	Context   []int  `json:"context,omitempty"`
}

// StreamCallback defines the callback function type for streaming responses
type StreamCallback func(response string, done bool) error

// StreamChat sends a prompt to Ollama and streams the response via callback
func (o *OllamaClient) StreamChat(ctx context.Context, prompt string, callback StreamCallback) error {
	requestBody, err := json.Marshal(map[string]interface{}{
		"model":  "mistral",
		"prompt": prompt,
		"stream": true, // Enable streaming
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.ollamaURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
			log.Printf("Failed to unmarshal stream response: %v", err)
			continue
		}

		// Call the callback with the response chunk
		if err := callback(streamResp.Response, streamResp.Done); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}

		if streamResp.Done {
			break
		}
	}

	return scanner.Err()
}

// StreamChatWithConversation maintains conversation context for better responses
func (o *OllamaClient) StreamChatWithConversation(ctx context.Context, messages []ChatMessage, callback StreamCallback) error {
	// Build conversation prompt
	var conversationBuilder strings.Builder
	conversationBuilder.WriteString("You are a helpful AI writing assistant for a creative writing application called Inkwell. ")
	conversationBuilder.WriteString("You help users with writing tips, grammar, story ideas, and creative writing. ")
	conversationBuilder.WriteString("Be friendly, encouraging, and provide practical advice.\n\n")

	for _, msg := range messages {
		if msg.Role == "user" {
			conversationBuilder.WriteString("User: " + msg.Content + "\n")
		} else if msg.Role == "assistant" {
			conversationBuilder.WriteString("Assistant: " + msg.Content + "\n")
		}
	}
	conversationBuilder.WriteString("Assistant: ")

	return o.StreamChat(ctx, conversationBuilder.String(), callback)
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

func (o *OllamaClient) callOllama(prompt string) (string, error) {
	requestBody, _ := json.Marshal(map[string]interface{}{
		"model":  "mistral",
		"prompt": prompt,
	})

	req, err := http.NewRequest("POST", o.ollamaURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	// Read the full response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fullBody := string(bodyBytes)
	// Log the full response for debugging
	//log.Println("Full LLM response body:", fullBody)

	// If the response is streamed as multiple JSON objects (separated by newlines),
	// aggregate them using our standalone function.
	if strings.Contains(fullBody, "\n") {
		aggregated := AggregateStreamedResponse(fullBody)
		return aggregated, nil
	}

	// Otherwise, attempt to decode a single JSON object.
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}
	if responseText, ok := result["response"].(string); ok {
		return responseText, nil
	}

	return "", errors.New("invalid response from Ollama")
}

type ResponseChunk struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// AggregateStreamedResponse takes the full raw response body (a string with multiple JSON objects separated by newlines)
// and concatenates the "response" fields into one final string.
func AggregateStreamedResponse(body string) string {
	lines := strings.Split(body, "\n")
	var builder strings.Builder
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			var chunk ResponseChunk
			if err := json.Unmarshal([]byte(trimmed), &chunk); err != nil {
				log.Println("Error unmarshaling chunk:", err)
				continue
			}
			builder.WriteString(chunk.Response)
		}
	}
	return builder.String()
}

func (o *OllamaClient) GenerateQuestions(topic string, limit int) ([]string, error) {
	prompt := fmt.Sprintf("Generate %d multiple-choice questions on %s.", limit, topic)
	response, err := o.callOllama(prompt)
	if err != nil {
		return nil, err
	}
	return parseQuestions(response), nil
}

func (o *OllamaClient) EvaluateAnswer(question, userAnswer, correctAnswer string) (bool, string, error) {
	// Instruct the model to output a minimal JSON response
	prompt := fmt.Sprintf(
		"Question: %s\nUser Answer: %s\nCorrect Answer: %s\n"+
			"Evaluate the answer. Output minimal JSON with keys 'correct' (boolean) and 'feedback' (string).",
		question, userAnswer, correctAnswer,
	)

	response, err := o.callOllama(prompt)
	if err != nil {
		return false, "", err
	}

	// Expected JSON response structure:
	// {"correct": true, "feedback": "Short feedback message"}
	var result struct {
		Correct  bool   `json:"correct"`
		Feedback string `json:"feedback"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return false, "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result.Correct, result.Feedback, nil
}

func parseQuestions(response string) []string {
	var questions []string
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if line != "" {
			questions = append(questions, line)
		}
	}
	return questions
}

func (o *OllamaClient) CorrectSentence(sentence string) (string, string, error) {
	prompt := "Please correct the following sentence if needed and provide feedback in the format 'Corrected: <corrected sentence> Feedback: <feedback message>': " + sentence
	response, err := o.callOllama(prompt)
	if err != nil {
		log.Println("Error calling Ollama:", err)
		return sentence, "Could not generate feedback", err
	}
	//log.Println("LLM response:", response)
	// Parse response expecting: "Corrected: <corrected sentence> Feedback: <feedback message>"
	parts := strings.Split(response, "Feedback:")
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
	return correctedText, feedback, nil
}

type AnalysisResponse struct {
	Analysis         string   `json:"analysis"`
	Tips             []string `json:"tips"`
	PerformanceScore int      `json:"performance_score"`
}

// AnalyzeText sends the prompt to Ollama and attempts to parse the response as JSON.
func (o *OllamaClient) AnalyzeText(prompt string) (*AnalysisResponse, error) {
	response, err := o.callOllama(prompt)
	if err != nil {
		return nil, err
	}

	var analysisResp AnalysisResponse
	if err := json.Unmarshal([]byte(response), &analysisResp); err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}
	return &analysisResp, nil
}

// GenerateWritingTip generates a writing tip based on user's request
func (o *OllamaClient) GenerateWritingTip(topic string) (string, error) {
	prompt := fmt.Sprintf("Provide a helpful writing tip about %s. Keep it concise and actionable.", topic)
	return o.callOllama(prompt)
}

// GenerateStoryIdea generates creative story ideas
func (o *OllamaClient) GenerateStoryIdea(genre, theme string) (string, error) {
	prompt := fmt.Sprintf("Generate a creative story idea for the %s genre with the theme of %s. Include a brief plot outline.", genre, theme)
	return o.callOllama(prompt)
}

// ImproveWriting provides suggestions to improve a piece of writing
func (o *OllamaClient) ImproveWriting(text string) (string, error) {
	prompt := fmt.Sprintf("Review the following text and provide constructive feedback on how to improve it:\n\n%s", text)
	return o.callOllama(prompt)
}
