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

// GenerateImage sends a prompt to the Stable Diffusion 2.1 model on Hugging Face and saves the resulting image.
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

	// Stable Diffusion 2.1 API endpoint (corrected model slug).
	apiURL := "https://api-inference.huggingface.co/models/stabilityai/stable-diffusion-2-1"

	// Create the HTTP request.
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wait-For-Model", "true") // Wait for model to load instead of getting 503.

	fmt.Printf("DEBUG: firing POST to [%s] with token prefix [%.15s...]\n", apiURL, s.AccessToken)

	// Execute the request with a generous timeout — SD can be slow on cold starts.
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	contentType := resp.Header.Get("Content-Type")

	// Happy path — response is an image.
	if resp.StatusCode == http.StatusOK && strings.HasPrefix(contentType, "image/") {
		// Ensure the output directory exists.
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
		defer func() {
			_ = file.Close()
		}()

		if _, err = io.Copy(file, resp.Body); err != nil {
			return "", fmt.Errorf("failed to save image: %w", err)
		}

		return strings.TrimPrefix(imagePath, "working/"), nil
	}

	// Model is still loading despite X-Wait-For-Model (shouldn't normally happen, but guard anyway).
	if resp.StatusCode == http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("model is loading, please retry shortly: %s", string(body))
	}

	// Any other non-OK response — read and surface the error body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unexpected status %d and failed to read error response: %w", resp.StatusCode, err)
	}
	return "", fmt.Errorf("image generation failed (status %d): %s", resp.StatusCode, string(body))
}
