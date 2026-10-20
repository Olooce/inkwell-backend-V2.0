package speech

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeWAVHeader(t *testing.T) {
	wav := encodeWAV([]float32{0, 1, -1, 2}, 22050)

	if len(wav) != 44+4*2 {
		t.Fatalf("len = %d, want %d", len(wav), 44+8)
	}
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || string(wav[36:40]) != "data" {
		t.Fatalf("bad header: %q", wav[:44])
	}
	if got := binary.LittleEndian.Uint32(wav[24:]); got != 22050 {
		t.Errorf("sample rate = %d, want 22050", got)
	}
	// Out-of-range samples are clipped to full scale.
	if got := int16(binary.LittleEndian.Uint16(wav[50:])); got != 32767 {
		t.Errorf("clipped sample = %d, want 32767", got)
	}
}

// TestRoundTrip synthesises speech and transcribes it back. It needs the
// models from scripts/download-speech-models.sh and ffmpeg; set
// SPEECH_MODEL_DIR to point at them.
func TestRoundTrip(t *testing.T) {
	dir := os.Getenv("SPEECH_MODEL_DIR")
	if dir == "" {
		t.Skip("SPEECH_MODEL_DIR not set")
	}

	engine, err := NewEngine(Config{ModelDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	wav, err := engine.Synthesize("The quick brown fox jumps over the lazy dog.")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "speech.wav")
	if err := os.WriteFile(path, wav, 0o644); err != nil {
		t.Fatal(err)
	}

	text, err := engine.TranscribeFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	text = strings.ToLower(text)
	for _, word := range []string{"quick", "brown", "fox", "lazy", "dog"} {
		if !strings.Contains(text, word) {
			t.Errorf("transcript %q missing %q", text, word)
		}
	}
}
