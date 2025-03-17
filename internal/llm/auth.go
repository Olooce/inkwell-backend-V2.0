package llm

import (
	"fmt"
	"inkwell-backend-V2.0/internal/config"
	"log"
	"net/http"
)

type Config struct {
	HFToken string `json:"hf_token"`
}

// AuthenticateHuggingFace verifies the token by making a test request
func AuthenticateHuggingFace(cfg *config.APIConfig) error {
	url := "https://huggingface.co/api/whoami"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set the Authorization header
	req.Header.Set("Authorization", "Bearer "+cfg.THIRD_PARTY.HFToken)
	log.Println("Hugging Face Token:", cfg.THIRD_PARTY.HFToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to authenticate with Hugging Face API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: received status code %d", resp.StatusCode)
	}

	log.Println("Successfully authenticated with Hugging Face API.")
	return nil
}
