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
	model     string
	client    *http.Client
}

func NewOllamaClient(url, model string) *OllamaClient {
	return &OllamaClient{
		ollamaURL: url,
		model:     model,
		client: &http.Client{
			Timeout: 600 * time.Second,
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
		"model":  o.model,
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
		"model":  o.model,
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

// judgeSchema constrains JudgeCorrection's output (Ollama structured outputs).
// Fields are generated in order, so the model checks each rule before it
// commits to a verdict; asked for the verdict first it decides, then rationalises.
var judgeSchema = orderedSchema{
	{"fixes_mistake", map[string]interface{}{"type": "boolean"}},
	{"same_meaning_and_tense", map[string]interface{}{"type": "boolean"}},
	{"grammatical", map[string]interface{}{"type": "boolean"}},
	{"explanation", map[string]interface{}{"type": "string"}},
}

type schemaField struct {
	name string
	def  map[string]interface{}
}

// orderedSchema marshals as a JSON object schema whose properties keep their
// declared order (Go maps would sort them alphabetically).
type orderedSchema []schemaField

func (o orderedSchema) MarshalJSON() ([]byte, error) {
	var props bytes.Buffer
	required := make([]string, len(o))
	props.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			props.WriteByte(',')
		}
		name, _ := json.Marshal(f.name)
		def, err := json.Marshal(f.def)
		if err != nil {
			return nil, err
		}
		props.Write(name)
		props.WriteByte(':')
		props.Write(def)
		required[i] = f.name
	}
	props.WriteByte('}')
	req, _ := json.Marshal(required)
	return []byte(fmt.Sprintf(`{"type":"object","properties":%s,"required":%s}`, props.String(), req)), nil
}

// JudgeCorrection decides whether a student's correction of a sentence is an
// acceptable alternative to the expected answer. It implements grading.Judge
// and is only called for answers already close to the expected one.
func (o *OllamaClient) JudgeCorrection(original, expected, answer string) (bool, string, error) {
	prompt := fmt.Sprintf(`You are a strict but kind English teacher grading a child's grammar exercise.

The child was asked to fix the mistake in this sentence:
ORIGINAL: %s
A correct answer is:
EXPECTED: %s
The child wrote:
ANSWER: %s

Compare the ANSWER with the ORIGINAL and the EXPECTED answer word by word, then decide:
- "fixes_mistake": does the ANSWER correct the mistake that the EXPECTED answer corrects? If the mistaken word from the ORIGINAL is still there, this is false.
- "same_meaning_and_tense": same people, actions and verb tense as the EXPECTED answer? A different tense (e.g. "barked" instead of "barks") is false.
- "grammatical": is it a complete, grammatically correct sentence with correct capitals and punctuation?
Different wording is fine when all three are true. If you are unsure, answer false.

"explanation": one short, friendly sentence to the child that matches your decisions.`, original, expected, answer)

	var result struct {
		FixesMistake bool   `json:"fixes_mistake"`
		SameMeaning  bool   `json:"same_meaning_and_tense"`
		Grammatical  bool   `json:"grammatical"`
		Explanation  string `json:"explanation"`
	}
	if err := o.generateJSON(prompt, judgeSchema, &result); err != nil {
		return false, "", err
	}
	// Accept only if every rule passes.
	ok := result.FixesMistake && result.SameMeaning && result.Grammatical
	return ok, strings.TrimSpace(result.Explanation), nil
}

// generateJSON runs a single non-streaming, deterministic generation whose
// output is constrained to schema, and decodes it into out.
func (o *OllamaClient) generateJSON(prompt string, schema interface{}, out interface{}) error {
	requestBody, err := json.Marshal(map[string]interface{}{
		"model":   o.model,
		"prompt":  prompt,
		"stream":  false,
		"format":  schema,
		"options": map[string]interface{}{"temperature": 0},
	})
	if err != nil {
		return err
	}
	resp, err := o.client.Post(o.ollamaURL, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("failed to decode Ollama response: %w", err)
	}
	if err := json.Unmarshal([]byte(envelope.Response), out); err != nil {
		return fmt.Errorf("failed to parse model JSON %q: %w", envelope.Response, err)
	}
	return nil
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

// SentenceReview is the LLM's review of one story sentence.
type SentenceReview struct {
	Corrected string `json:"corrected"`
	Feedback  string `json:"feedback"`
	// Scene is a short, concrete visual description for the illustrator,
	// resolving ambiguity from the story context.
	Scene string `json:"scene"`
}

var sentenceReviewSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"corrected": map[string]interface{}{"type": "string"},
		"feedback":  map[string]interface{}{"type": "string"},
		"scene":     map[string]interface{}{"type": "string"},
	},
	"required": []string{"corrected", "feedback", "scene"},
}

// ReviewSentence corrects a child's story sentence, explains the correction,
// and describes the scene for illustration. title and previous (earlier
// corrected sentences) give the context needed to keep pictures consistent.
func (o *OllamaClient) ReviewSentence(title string, previous []string, sentence string) (*SentenceReview, error) {
	context := "(this is the first sentence)"
	if len(previous) > 0 {
		context = strings.Join(previous, " ")
	}
	prompt := fmt.Sprintf(`You are helping a child (aged 7-12) write a story, one sentence at a time.

Story title: %s
Story so far: %s
New sentence: %s

Return:
- "corrected": the new sentence with spelling, grammar and punctuation fixed. Change as little as possible and keep the child's words and meaning. If it is already correct, return it unchanged.
- "feedback": one short, encouraging sentence to the child. If you changed something, name the rule simply (e.g. "Use 'went' for the past tense of 'go'."); otherwise praise something specific.
- "scene": a concrete visual description of what the new sentence shows, for an image generator, at most 25 words. Words with two meanings confuse the image generator, so always name the object type: "a red diamond-shaped toy kite" (never just "red kite", which is also a bird), "a wooden baseball bat", "a computer mouse". Describe recurring characters the same way each time.`, title, context, sentence)

	var review SentenceReview
	if err := o.generateJSON(prompt, sentenceReviewSchema, &review); err != nil {
		return nil, err
	}
	review.Corrected = strings.TrimSpace(review.Corrected)
	review.Feedback = strings.TrimSpace(review.Feedback)
	review.Scene = strings.TrimSpace(review.Scene)
	if review.Corrected == "" {
		return nil, errors.New("model returned an empty correction")
	}
	return &review, nil
}

type AnalysisResponse struct {
	Analysis         string   `json:"analysis"`
	Tips             []string `json:"tips"`
	PerformanceScore int      `json:"performance_score"`
}

var analysisSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"analysis":          map[string]interface{}{"type": "string"},
		"tips":              map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		"performance_score": map[string]interface{}{"type": "integer"},
	},
	"required": []string{"analysis", "tips", "performance_score"},
}

// AnalyzeText runs prompt with output constrained to the AnalysisResponse schema.
func (o *OllamaClient) AnalyzeText(prompt string) (*AnalysisResponse, error) {
	var analysisResp AnalysisResponse
	if err := o.generateJSON(prompt, analysisSchema, &analysisResp); err != nil {
		return nil, fmt.Errorf("failed to analyse text: %w", err)
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
