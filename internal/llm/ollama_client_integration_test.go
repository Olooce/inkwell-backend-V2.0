package llm

import (
	"os"
	"strings"
	"testing"
)

// Integration tests against a running Ollama. Run with:
//
//	OLLAMA_TEST=1 OLLAMA_MODEL=qwen2.5:3b go test -run Integration -v ./internal/llm/
func integrationClient(t *testing.T) *OllamaClient {
	if os.Getenv("OLLAMA_TEST") == "" {
		t.Skip("set OLLAMA_TEST=1 to run against a local Ollama")
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "mistral"
	}
	return NewOllamaClient("http://localhost:11434/api/generate", model)
}

func TestIntegrationReviewSentence(t *testing.T) {
	c := integrationClient(t)
	r, err := c.ReviewSentence("The Lost Kite", []string{"A red kite got stuck in a tall green tree."}, "A kind girl climb up and save it.")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("corrected: %s\nfeedback: %s\nscene: %s", r.Corrected, r.Feedback, r.Scene)
	if !strings.Contains(strings.ToLower(r.Corrected), "climbed") {
		t.Errorf("expected 'climb' to be corrected to 'climbed': %q", r.Corrected)
	}
	if r.Scene == "" {
		t.Error("empty scene")
	}
}

func TestIntegrationJudgeCorrection(t *testing.T) {
	c := integrationClient(t)
	cases := []struct {
		original, expected, answer string
		want                       bool
	}{
		{"He was given a gift by me.", "I gave him a gift.", "I gave a gift to him.", true},
		{"The dog bark at the stranger.", "The dog barks at the stranger.", "The dog barked at the stranger.", false},
		{"She go to school every day.", "She goes to school every day.", "She go to school every day!", false},
	}
	for _, tc := range cases {
		ok, why, err := c.JudgeCorrection(tc.original, tc.expected, tc.answer)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%q -> %v (%s)", tc.answer, ok, why)
		if ok != tc.want {
			t.Errorf("%q: acceptable=%v, want %v", tc.answer, ok, tc.want)
		}
	}
}
