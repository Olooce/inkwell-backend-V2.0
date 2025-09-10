package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/cmd/app/internal/llm"
)

// ChatController handles chat-related endpoints
type ChatController struct {
	ollamaClient *llm.OllamaClient
}

// NewChatController creates a new chat controller
func NewChatController(ollamaClient *llm.OllamaClient) *ChatController {
	return &ChatController{
		ollamaClient: ollamaClient,
	}
}

// ChatRequest represents the incoming chat message request
type ChatRequest struct {
	Message      string                 `json:"message" binding:"required"`
	Conversation []llm.ChatMessage      `json:"conversation,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// StreamChatResponse represents a streaming response chunk
type StreamChatResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// StreamChat handles streaming chat responses
func (cc *ChatController) StreamChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Set headers for Server-Sent Events
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Build conversation history
	conversation := req.Conversation
	conversation = append(conversation, llm.ChatMessage{
		Role:    "user",
		Content: req.Message,
	})

	// Stream the response
	err := cc.ollamaClient.StreamChatWithConversation(ctx, conversation, func(response string, done bool) error {
		// Create response chunk
		chunk := StreamChatResponse{
			Response: response,
			Done:     done,
		}

		// Marshal to JSON
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}

		// Send as Server-Sent Event
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()

		return nil
	})

	if err != nil {
		log.Printf("Streaming error: %v", err)
		// Send error message
		errorChunk := StreamChatResponse{
			Error: "Failed to generate response",
			Done:  true,
		}
		data, _ := json.Marshal(errorChunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	// Send completion signal
	_, err = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	if err != nil {
		return
	}
	c.Writer.Flush()
}

// GetWritingTip handles writing tip requests
func (cc *ChatController) GetWritingTip(c *gin.Context) {
	topic := c.Query("topic")
	if topic == "" {
		topic = "general writing"
	}

	tip, err := cc.ollamaClient.GenerateWritingTip(topic)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate writing tip"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tip":   tip,
		"topic": topic,
	})
}

// GetStoryIdea handles story idea generation requests
func (cc *ChatController) GetStoryIdea(c *gin.Context) {
	genre := c.Query("genre")
	theme := c.Query("theme")

	if genre == "" {
		genre = "fantasy"
	}
	if theme == "" {
		theme = "adventure"
	}

	idea, err := cc.ollamaClient.GenerateStoryIdea(genre, theme)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate story idea"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"idea":  idea,
		"genre": genre,
		"theme": theme,
	})
}

// ImproveText handles text improvement requests
func (cc *ChatController) ImproveText(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	improvement, err := cc.ollamaClient.ImproveWriting(req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to improve text"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"original_text": req.Text,
		"suggestions":   improvement,
	})
}

// ChatHealth checks if the chat service is available
func (cc *ChatController) ChatHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "chat",
		"timestamp": time.Now().Unix(),
	})
}

// SpeechToText handles audio file upload and converts to text
func (cc *ChatController) SpeechToText(c *gin.Context) {
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing audio file"})
		return
	}

	// Create working directory if it doesn't exist
	if err := os.MkdirAll("./working", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
		return
	}

	path := "./working/" + file.Filename
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	defer os.Remove(path) // Clean up after processing

	text, err := SendAudioToSTT(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "STT failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"text": text})
}

// TextToSpeech converts text to speech and returns audio
func (cc *ChatController) TextToSpeech(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	// Create working directory if it doesn't exist
	if err := os.MkdirAll("./working", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
		return
	}

	outFile := fmt.Sprintf("./working/tts_%d.wav", time.Now().Unix())
	defer os.Remove(outFile) // Clean up after sending

	if err := GetSpeechFromTTS(req.Text, outFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS failed: " + err.Error()})
		return
	}

	c.File(outFile)
}
func SendAudioToSTT(audioFile string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(audioFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(audioFile))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest("POST", "http://localhost:8001/stt", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("STT service returned status: %s", resp.Status)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["text"], nil
}

func GetSpeechFromTTS(text string, outFile string) error {
	formData := url.Values{}
	formData.Set("text", text)

	resp, err := http.PostForm("http://localhost:8001/tts", formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS service returned status: %s, body: %s", resp.Status, string(body))
	}

	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
