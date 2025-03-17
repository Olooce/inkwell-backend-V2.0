package llm

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

// DeepFloydWrapper handles DeepFloyd image generation
type DeepFloydWrapper struct{}

// GenerateImage calls the Python script to create an image from a text prompt
func (d *DeepFloydWrapper) GenerateImage(prompt string) (string, error) {
	// Ensure the script path is absolute
	scriptPath, err := filepath.Abs("internal/llm/deepFloyd.py")
	if err != nil {
		return "", fmt.Errorf("failed to determine script path: %s", err)
	}

	cmd := exec.Command("python3", scriptPath, prompt)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute script: %s\nOutput: %s", err, output)
	}

	// Parse JSON response
	var result map[string]string
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("invalid JSON response: %s", output)
	}

	if result["status"] == "error" {
		return "", fmt.Errorf("image generation failed: %s", result["message"])
	}

	return result["path"], nil
}
