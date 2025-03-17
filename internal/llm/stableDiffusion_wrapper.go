package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StableDiffusionWrapper handles Stable Diffusion image generation using the Hugging Face Inference API.
type StableDiffusionWrapper struct {
	AccessToken string
}

// GenerateImage sends a prompt to the Stable Diffusion 2 model on Hugging Face and saves the resulting image.
// It returns the path to the generated image or an error.
func (s *StableDiffusionWrapper) GenerateImage(prompt string) (string, error) {
	if s.AccessToken == "" {
		return "", fmt.Errorf("missing Hugging Face API token")
	}

	// Prepare the payload.
	payload := map[string]interface{}{
		"inputs": prompt,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Stable Diffusion 2 API endpoint.
	apiURL := "https://api-inference.huggingface.co/models/stabilityai/stable-diffusion-2"

	// Create the HTTP request.
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response is an image by verifying the content type.
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode == http.StatusOK && len(contentType) >= 5 && contentType[:5] == "image" {
		// Ensure the directory exists.
		dir := "working/storyImages"
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}

		// Generate a unique file name using a timestamp.
		uniqueName := fmt.Sprintf("storyImage_%d.png", time.Now().UnixNano())
		imagePath := filepath.Join(dir, uniqueName)

		file, err := os.Create(imagePath)
		if err != nil {
			return "", fmt.Errorf("failed to create image file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to save image: %w", err)
		}

		return strings.TrimPrefix(imagePath, "working/"), nil

	}

	// Otherwise, read the error response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read error response: %w", err)
	}
	return "", fmt.Errorf("image generation failed: %s", string(body))
}
