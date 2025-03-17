package llm

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

// DeepFloydWrapper handles DeepFloyd image generation
type DeepFloydWrapper struct {
	AccessToken string
}

// GenerateImage generates an image from a prompt using DeepFloyd
func (d *DeepFloydWrapper) GenerateImage(prompt string) (string, error) {
	// Get absolute paths for script and virtual environment
	scriptPath, err := filepath.Abs("internal/llm/deepFloyd.py")
	if err != nil {
		return "", fmt.Errorf("failed to determine script path: %s", err)
	}

	venvPath, err := filepath.Abs("internal/llm/deepfloyd_env/bin/python")
	if err != nil {
		return "", fmt.Errorf("failed to determine virtual environment path: %s", err)
	}

	// Ensure access token is set
	if d.AccessToken == "" {
		return "", fmt.Errorf("missing DeepFloyd access token")
	}

	// Execute Python script with access token and prompt
	cmd := exec.Command(venvPath, scriptPath, d.AccessToken, prompt)
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
