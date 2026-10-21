package imagegen

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeGen struct {
	name string
	path string
	err  error
	n    int
}

func (f *fakeGen) Name() string { return f.name }
func (f *fakeGen) Generate(context.Context, string) (string, error) {
	f.n++
	return f.path, f.err
}

func TestChainPrefersFirstAndFallsBack(t *testing.T) {
	local := &fakeGen{name: "local", path: "storyImages/a.png"}
	hosted := &fakeGen{name: "huggingface", path: "storyImages/b.png"}
	if p, err := (Chain{local, hosted}).Generate(context.Background(), "x"); err != nil || p != "storyImages/a.png" || hosted.n != 0 {
		t.Fatalf("got %q, %v; hosted called %d times", p, err, hosted.n)
	}

	local.err = errors.New("out of memory")
	if p, err := (Chain{local, hosted}).Generate(context.Background(), "x"); err != nil || p != "storyImages/b.png" {
		t.Fatalf("fallback: got %q, %v", p, err)
	}

	hosted.err = errors.New("402")
	_, err := (Chain{local, hosted}).Generate(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "local: out of memory") || !strings.Contains(err.Error(), "huggingface: 402") {
		t.Fatalf("combined error = %v", err)
	}
}

func TestNewSelection(t *testing.T) {
	missing := LocalConfig{Binary: "/nonexistent/sd-cli", Model: "/nonexistent/model.gguf"}

	// No token, local missing: error, no generator.
	if g, err := New(Config{Local: missing}); g != nil || err == nil {
		t.Errorf("expected no generator, got %v, %v", g, err)
	}

	// Local missing but a token is configured: falls back to Hugging Face, with a warning.
	g, err := New(Config{Local: missing, HuggingFace: HuggingFaceConfig{Token: "t"}})
	if g == nil || g.Name() != "huggingface" || err == nil {
		t.Errorf("expected huggingface with warning, got %v, %v", g, err)
	}

	// Explicit huggingface provider, no fallback chain.
	g, err = New(Config{Provider: "huggingface", HuggingFace: HuggingFaceConfig{Token: "t"}})
	if err != nil || g.Name() != "huggingface" {
		t.Errorf("got %v, %v", g, err)
	}

	// Both available: local first, then Hugging Face.
	dir := t.TempDir()
	bin, model := filepath.Join(dir, "sd-cli"), filepath.Join(dir, "m.gguf")
	_ = os.WriteFile(bin, nil, 0o755)
	_ = os.WriteFile(model, nil, 0o644)
	g, err = New(Config{Local: LocalConfig{Binary: bin, Model: model}, HuggingFace: HuggingFaceConfig{Token: "t"}})
	if err != nil || g.Name() != "local → huggingface" {
		t.Errorf("got %v, %v", g, err)
	}

	if _, err := New(Config{Provider: "dalle"}); err == nil {
		t.Error("unknown provider accepted")
	}
}

func TestHuggingFaceBackends(t *testing.T) {
	png := []byte("\x89PNG fake")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/hf-inference/models/"):
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		case r.URL.Path == "/nscale/v1/images/generations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(png) + `"}]}`))
		default:
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"error":"You have no remaining credits."}`))
		}
	}))
	defer srv.Close()

	for _, provider := range []string{"hf-inference", "nscale"} {
		work := t.TempDir()
		h, _ := NewHuggingFace(HuggingFaceConfig{Token: "tok", Provider: provider, BaseURL: srv.URL}, work)
		rel, err := h.Generate(context.Background(), "a kite")
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if got, _ := os.ReadFile(filepath.Join(work, rel)); string(got) != string(png) {
			t.Errorf("%s: saved %q", provider, got)
		}
	}

	h, _ := NewHuggingFace(HuggingFaceConfig{Token: "tok", Provider: "fal-ai", BaseURL: srv.URL}, t.TempDir())
	if _, err := h.Generate(context.Background(), "a kite"); err == nil || !strings.Contains(err.Error(), "no remaining credits") {
		t.Errorf("402 error not surfaced: %v", err)
	}
}

func TestStoryPromptPutsSceneFirst(t *testing.T) {
	p := StoryPrompt("The Lost Kite", "  A kite got stuck in a tall tree.  ")
	if !strings.HasPrefix(p, `A kite got stuck in a tall tree. (from a story called "The Lost Kite"),`) {
		t.Errorf("prompt = %q", p)
	}
	if p := StoryPrompt("", "A dog ran."); !strings.HasPrefix(p, "A dog ran.,") {
		t.Errorf("untitled prompt = %q", p)
	}
}

// TestLocalGenerate renders a real image. Set IMAGEGEN_TEST=1 with the model downloaded.
func TestLocalGenerate(t *testing.T) {
	if os.Getenv("IMAGEGEN_TEST") == "" {
		t.Skip("set IMAGEGEN_TEST=1 to run (needs tools/stable-diffusion.cpp and models/sd-turbo)")
	}
	root, _ := filepath.Abs("../..")
	work := t.TempDir()
	l, err := NewLocal(LocalConfig{
		Binary: filepath.Join(root, "tools/stable-diffusion.cpp/sd-cli"),
		Model:  filepath.Join(root, "models/sd-turbo/sd_turbo-q8_0.gguf"),
	}, work)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	rel, err := l.Generate(context.Background(), StoryPrompt("The Lost Kite", "A red kite got stuck in a tall green tree."))
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(work, rel))
	t.Logf("generated %s (%d bytes) in %s", rel, info.Size(), time.Since(start).Round(time.Millisecond))
	if out := os.Getenv("IMAGEGEN_KEEP"); out != "" {
		data, _ := os.ReadFile(filepath.Join(work, rel))
		_ = os.WriteFile(out, data, 0o644)
	}
}
