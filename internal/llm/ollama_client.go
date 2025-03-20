package llm

import (
	"bytes"
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

type LLMResponseChunk struct {
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
			var chunk LLMResponseChunk
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

func determineCorrectness(response string) bool {
	return !strings.Contains(strings.ToLower(response), "incorrect")
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
