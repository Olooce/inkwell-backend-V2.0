package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type OllamaClient struct {
	ollamaURL string
}

func NewOllamaClient(url string) *OllamaClient {
	return &OllamaClient{ollamaURL: url}
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
	prompt := fmt.Sprintf("Question: %s\nUser Answer: %s\nCorrect Answer: %s\nIs the answer correct? Explain why.", question, userAnswer, correctAnswer)
	response, err := o.callOllama(prompt)
	if err != nil {
		return false, "", err
	}

	isCorrect := determineCorrectness(response)
	return isCorrect, response, nil
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if responseText, ok := result["response"].(string); ok {
		return responseText, nil
	}

	return "", errors.New("invalid response from Ollama")
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
