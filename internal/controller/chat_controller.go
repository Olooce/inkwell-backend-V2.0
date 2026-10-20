package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"inkwell-backend-V2.0/internal/llm"
	"inkwell-backend-V2.0/internal/speech"
)

// ChatController handles chat-related endpoints
type ChatController struct {
	ollamaClient *llm.OllamaClient
	speech       *speech.Engine // nil when the speech models aren't installed
}

// NewChatController creates a new chat controller
func NewChatController(ollamaClient *llm.OllamaClient, speechEngine *speech.Engine) *ChatController {
	return &ChatController{
		ollamaClient: ollamaClient,
		speech:       speechEngine,
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

// maxAudioUploadBytes caps speech-to-text uploads (~25 MB).
const maxAudioUploadBytes = 25 << 20

// SpeechToText handles audio file upload and converts to text
func (cc *ChatController) SpeechToText(c *gin.Context) {
	if cc.speech == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "speech-to-text is not available"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAudioUploadBytes)
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or oversized audio file"})
		return
	}

	// Save to a server-chosen temp name; the client filename is only used for its extension.
	tmp, err := os.CreateTemp("", "stt-*"+filepath.Ext(filepath.Base(file.Filename)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp file"})
		return
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	text, err := cc.speech.TranscribeFile(ctx, path)
	if err != nil {
		log.Printf("STT failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "STT failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"text": text})
}

// TextToSpeech converts text to speech and returns audio
func (cc *ChatController) TextToSpeech(c *gin.Context) {
	if cc.speech == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "text-to-speech is not available"})
		return
	}

	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if len(req.Text) > speech.MaxTTSChars {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("text exceeds %d characters", speech.MaxTTSChars)})
		return
	}

	wav, err := cc.speech.Synthesize(req.Text)
	if errors.Is(err, speech.ErrEmptyText) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is empty"})
		return
	}
	if err != nil {
		log.Printf("TTS failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS failed"})
		return
	}

	c.Data(http.StatusOK, "audio/wav", wav)
}
