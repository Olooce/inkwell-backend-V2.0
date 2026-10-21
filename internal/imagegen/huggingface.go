package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// HuggingFaceConfig configures the hosted Hugging Face Inference Providers backend.
type HuggingFaceConfig struct {
	Token string
	// Model id. Default "stabilityai/stable-diffusion-3-medium-diffusers".
	Model string
	// Provider: "hf-inference" (default) or an OpenAI-compatible images provider
	// such as "nscale". Hosted providers need Hugging Face credits.
	Provider string
	// BaseURL of the router; overridable for tests.
	BaseURL string
}

// HuggingFace generates images through router.huggingface.co.
type HuggingFace struct {
	cfg     HuggingFaceConfig
	workDir string
	client  *http.Client
}

func NewHuggingFace(cfg HuggingFaceConfig, workDir string) (*HuggingFace, error) {
	if cfg.Token == "" {
		return nil, errors.New("no Hugging Face token configured")
	}
	if cfg.Model == "" {
		cfg.Model = "stabilityai/stable-diffusion-3-medium-diffusers"
	}
	if cfg.Provider == "" {
		cfg.Provider = "hf-inference"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://router.huggingface.co"
	}
	return &HuggingFace{cfg: cfg, workDir: workDir, client: &http.Client{Timeout: 2 * time.Minute}}, nil
}

func (h *HuggingFace) Name() string { return "huggingface" }

func (h *HuggingFace) Generate(ctx context.Context, prompt string) (string, error) {
	var url string
	var body any
	if h.cfg.Provider == "hf-inference" {
		url = fmt.Sprintf("%s/hf-inference/models/%s", h.cfg.BaseURL, h.cfg.Model)
		body = map[string]any{"inputs": prompt}
	} else {
		url = fmt.Sprintf("%s/%s/v1/images/generations", h.cfg.BaseURL, h.cfg.Provider)
		body = map[string]any{"model": h.cfg.Model, "prompt": prompt, "response_format": "b64_json"}
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+h.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, apiError(data))
	}

	image := data
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		var openAI struct {
			Data []struct {
				B64JSON string `json:"b64_json"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &openAI); err != nil || len(openAI.Data) == 0 {
			return "", fmt.Errorf("unexpected response: %s", apiError(data))
		}
		if image, err = base64.StdEncoding.DecodeString(openAI.Data[0].B64JSON); err != nil {
			return "", fmt.Errorf("invalid image data: %w", err)
		}
	}

	abs, rel, err := newImagePath(h.workDir)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, image, 0o644); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}
	return rel, nil
}

// apiError extracts {"error": "..."} from an API response, or a short raw excerpt.
func apiError(data []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error != "" {
		return e.Error
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
