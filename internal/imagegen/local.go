package imagegen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LocalConfig configures the local stable-diffusion.cpp backend.
type LocalConfig struct {
	// Binary is the stable-diffusion.cpp CLI. Default "tools/stable-diffusion.cpp/sd-cli".
	Binary string
	// Model is the model file. Default "models/sd-turbo/sd_turbo-q8_0.gguf".
	Model string
	// Steps per image. SD-Turbo is distilled for 1–4 steps; default 2.
	Steps int
	// Width and Height in pixels; default 512 (SD-Turbo's native size).
	Width, Height int
	// Timeout per image; default 3 minutes (CPU-only machines are slow).
	Timeout time.Duration
	// LowVRAM keeps the text encoder on the CPU (it costs ~0.4s there) so the
	// diffusion model and VAE fit on small GPUs next to Ollama. Default true.
	LowVRAM *bool
}

// Local generates images by running the stable-diffusion.cpp CLI. The model is
// loaded per image so GPU memory is released for Ollama between pictures.
type Local struct {
	cfg     LocalConfig
	workDir string
	mu      sync.Mutex // one image at a time: a single GPU, and VRAM is tight
}

// NewLocal checks the binary and model exist and returns a Local generator.
func NewLocal(cfg LocalConfig, workDir string) (*Local, error) {
	if cfg.Binary == "" {
		cfg.Binary = "tools/stable-diffusion.cpp/sd-cli"
	}
	if cfg.Model == "" {
		cfg.Model = "models/sd-turbo/sd_turbo-q8_0.gguf"
	}
	if cfg.Steps <= 0 {
		cfg.Steps = 2
	}
	if cfg.Width <= 0 {
		cfg.Width = 512
	}
	if cfg.Height <= 0 {
		cfg.Height = 512
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Minute
	}
	if cfg.LowVRAM == nil {
		lowVRAM := true
		cfg.LowVRAM = &lowVRAM
	}
	if _, err := os.Stat(cfg.Binary); err != nil {
		return nil, fmt.Errorf("stable-diffusion.cpp not found at %s (run scripts/download-image-model.sh): %w", cfg.Binary, err)
	}
	if _, err := os.Stat(cfg.Model); err != nil {
		return nil, fmt.Errorf("image model not found at %s (run scripts/download-image-model.sh): %w", cfg.Model, err)
	}
	return &Local{cfg: cfg, workDir: workDir}, nil
}

func (l *Local) Name() string { return "local" }

func (l *Local) Generate(ctx context.Context, prompt string) (string, error) {
	abs, rel, err := newImagePath(l.workDir)
	if err != nil {
		return "", err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, l.cfg.Timeout)
	defer cancel()

	args := []string{
		"-m", l.cfg.Model,
		"-p", prompt,
		"-o", abs,
		"--steps", strconv.Itoa(l.cfg.Steps),
		"--cfg-scale", "1.0", // turbo models are trained without classifier-free guidance
		"--sampling-method", "euler_a",
		"-W", strconv.Itoa(l.cfg.Width),
		"-H", strconv.Itoa(l.cfg.Height),
		"--seed", "-1",
	}
	// sd-cli's default --auto-fit moves the rest to RAM/CPU if VRAM is short.
	if *l.cfg.LowVRAM {
		args = append(args, "--backend", "te=cpu")
	}

	cmd := exec.CommandContext(ctx, l.cfg.Binary, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		_ = os.Remove(abs)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("timed out after %s", l.cfg.Timeout)
		}
		return "", fmt.Errorf("%w: %s", err, lastLines(output.String(), 5))
	}
	if info, err := os.Stat(abs); err != nil || info.Size() == 0 {
		return "", fmt.Errorf("no image was written: %s", lastLines(output.String(), 5))
	}
	return rel, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}
