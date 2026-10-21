package imagegen

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Generator produces one image for a prompt and returns its path relative to
// the working directory root
type Generator interface {
	Name() string
	Generate(ctx context.Context, prompt string) (string, error)
}

// Config selects and configures the generators.
type Config struct {
	// Provider is the preferred backend: "local" (default) or "huggingface".
	Provider string
	// Fallback is tried when the preferred backend fails: "huggingface",
	// "local" or "none". Empty means "huggingface" if a token is set, else "none".
	Fallback string

	// WorkDir is the directory images are served from; images go in
	// WorkDir/storyImages. Defaults to "working".
	WorkDir string

	Local       LocalConfig
	HuggingFace HuggingFaceConfig
}

// New builds the configured generator chain.
func New(cfg Config) (Generator, error) {
	if cfg.WorkDir == "" {
		cfg.WorkDir = "working"
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "local"
	}
	fallback := strings.ToLower(strings.TrimSpace(cfg.Fallback))
	if fallback == "" {
		fallback = "none"
		if cfg.HuggingFace.Token != "" && provider != "huggingface" {
			fallback = "huggingface"
		}
	}

	// Returns a plain nil interface on error, never a typed nil pointer, so
	// callers can safely check generator != nil.
	build := func(name string) (Generator, error) {
		switch name {
		case "local":
			g, err := NewLocal(cfg.Local, cfg.WorkDir)
			if err != nil {
				return nil, err
			}
			return g, nil
		case "huggingface":
			g, err := NewHuggingFace(cfg.HuggingFace, cfg.WorkDir)
			if err != nil {
				return nil, err
			}
			return g, nil
		default:
			return nil, fmt.Errorf("unknown image provider %q (use local or huggingface)", name)
		}
	}

	primary, primaryErr := build(provider)
	if fallback == "none" || fallback == provider {
		return primary, primaryErr
	}
	secondary, secondaryErr := build(fallback)

	switch {
	case primaryErr == nil && secondaryErr == nil:
		return Chain{primary, secondary}, nil
	case primaryErr == nil:
		return primary, nil
	case secondaryErr == nil:
		// The preferred backend isn't usable (e.g. models not downloaded): run on the fallback.
		return secondary, fmt.Errorf("%s image generation unavailable, using %s: %w", provider, fallback, primaryErr)
	default:
		return nil, errors.Join(primaryErr, secondaryErr)
	}
}

// Chain tries each generator in order until one succeeds.
type Chain []Generator

func (c Chain) Name() string {
	names := make([]string, len(c))
	for i, g := range c {
		names[i] = g.Name()
	}
	return strings.Join(names, " → ")
}

func (c Chain) Generate(ctx context.Context, prompt string) (string, error) {
	var errs []error
	for _, g := range c {
		path, err := g.Generate(ctx, prompt)
		if err == nil {
			return path, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", g.Name(), err))
		if ctx.Err() != nil {
			break
		}
	}
	return "", errors.Join(errs...)
}

// StoryPrompt builds a text-to-image prompt for a story sentence. The scene
// comes first because CLIP-based models only read the first ~75 tokens; the
// story title follows to disambiguate words like "kite" (toy or bird).
func StoryPrompt(title, sentence string) string {
	scene := truncate(strings.TrimSpace(sentence), 200)
	if title = truncate(strings.TrimSpace(title), 60); title != "" {
		scene += fmt.Sprintf(" (from a story called %q)", title)
	}
	return scene + styleSuffix
}

// ScenePrompt builds a prompt from an LLM-written scene description, which is
// already concrete and disambiguated.
func ScenePrompt(scene string) string {
	return truncate(strings.TrimSpace(scene), 220) + styleSuffix
}

const styleSuffix = ", children's comic book illustration, bold clean outlines, bright cheerful colours, expressive characters, simple background"

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// newImagePath returns a fresh absolute output path and its served relative path.
func newImagePath(workDir string) (abs, rel string, err error) {
	dir := filepath.Join(workDir, "storyImages")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("failed to create image directory: %w", err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := fmt.Sprintf("%s_%s.png", time.Now().Format("20060102_150405"), hex.EncodeToString(b))
	return filepath.Join(dir, name), filepath.ToSlash(filepath.Join("storyImages", name)), nil
}
