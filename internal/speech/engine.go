// Package speech provides in-process speech-to-text (Whisper) and
// text-to-speech (Piper/VITS) using sherpa-onnx, replacing the former
// Python STT/TTS sidecar service.
package speech

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const (
	whisperDir = "sherpa-onnx-whisper-base.en"
	piperDir   = "vits-piper-en_US-lessac-medium"

	// sttSampleRate is the sample rate Whisper expects.
	sttSampleRate = 16000
	// whisperWindow is how many samples Whisper can transcribe at once; it
	// only handles up to 30s, so longer audio is split into windows.
	whisperWindow = 28 * sttSampleRate

	// MaxTTSChars caps synthesis input to keep request latency bounded.
	MaxTTSChars = 5000
)

// ErrEmptyText is returned when Synthesize is called with blank text.
var ErrEmptyText = errors.New("text is empty")

// Config controls where models are loaded from.
type Config struct {
	// ModelDir contains the whisper and piper model directories.
	// Defaults to "models".
	ModelDir string
	// NumThreads used by each model. Defaults to half the CPUs, capped at 4.
	NumThreads int
}

// Engine holds the loaded STT and TTS models. It is safe for concurrent use.
type Engine struct {
	recognizer *sherpa.OfflineRecognizer
	tts        *sherpa.OfflineTts

	// The models are CPU-bound and their thread-safety isn't documented, so
	// requests are serialised per model.
	sttMu sync.Mutex
	ttsMu sync.Mutex
}

// NewEngine loads the speech models. It returns an error naming any missing
// file so the caller can disable speech features without crashing.
func NewEngine(cfg Config) (*Engine, error) {
	if cfg.ModelDir == "" {
		cfg.ModelDir = "models"
	}
	if cfg.NumThreads <= 0 {
		cfg.NumThreads = min(max(runtime.NumCPU()/2, 1), 4)
	}

	w := filepath.Join(cfg.ModelDir, whisperDir)
	p := filepath.Join(cfg.ModelDir, piperDir)
	files := map[string]string{
		"whisper encoder": filepath.Join(w, "base.en-encoder.int8.onnx"),
		"whisper decoder": filepath.Join(w, "base.en-decoder.int8.onnx"),
		"whisper tokens":  filepath.Join(w, "base.en-tokens.txt"),
		"piper model":     filepath.Join(p, "en_US-lessac-medium.onnx"),
		"piper tokens":    filepath.Join(p, "tokens.txt"),
		"espeak-ng data":  filepath.Join(p, "espeak-ng-data"),
	}
	for name, path := range files {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%s not found at %s (run scripts/download-speech-models.sh): %w", name, path, err)
		}
	}

	recognizer := sherpa.NewOfflineRecognizer(&sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{SampleRate: sttSampleRate, FeatureDim: 80},
		ModelConfig: sherpa.OfflineModelConfig{
			Whisper: sherpa.OfflineWhisperModelConfig{
				Encoder:      files["whisper encoder"],
				Decoder:      files["whisper decoder"],
				Language:     "en",
				Task:         "transcribe",
				TailPaddings: -1,
			},
			Tokens:     files["whisper tokens"],
			NumThreads: cfg.NumThreads,
			Provider:   "cpu",
		},
		DecodingMethod: "greedy_search",
	})
	if recognizer == nil {
		return nil, errors.New("failed to load whisper model")
	}

	tts := sherpa.NewOfflineTts(&sherpa.OfflineTtsConfig{
		Model: sherpa.OfflineTtsModelConfig{
			Vits: sherpa.OfflineTtsVitsModelConfig{
				Model:       files["piper model"],
				Tokens:      files["piper tokens"],
				DataDir:     files["espeak-ng data"],
				NoiseScale:  0.667,
				NoiseScaleW: 0.8,
				LengthScale: 1.0,
			},
			NumThreads: cfg.NumThreads,
			Provider:   "cpu",
		},
		MaxNumSentences: 1,
	})
	if tts == nil {
		sherpa.DeleteOfflineRecognizer(recognizer)
		return nil, errors.New("failed to load piper model")
	}

	return &Engine{recognizer: recognizer, tts: tts}, nil
}

// Close frees the native model memory. The Engine must not be used afterwards.
func (e *Engine) Close() {
	e.sttMu.Lock()
	defer e.sttMu.Unlock()
	e.ttsMu.Lock()
	defer e.ttsMu.Unlock()

	if e.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(e.recognizer)
		e.recognizer = nil
	}
	if e.tts != nil {
		sherpa.DeleteOfflineTts(e.tts)
		e.tts = nil
	}
}

// TranscribeFile converts an audio file in any format ffmpeg understands to text.
func (e *Engine) TranscribeFile(ctx context.Context, path string) (string, error) {
	samples, err := decodeAudio(ctx, path, sttSampleRate)
	if err != nil {
		return "", err
	}
	if len(samples) == 0 {
		return "", nil
	}

	e.sttMu.Lock()
	defer e.sttMu.Unlock()

	var parts []string
	for start := 0; start < len(samples); start += whisperWindow {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		end := min(start+whisperWindow, len(samples))

		stream := sherpa.NewOfflineStream(e.recognizer)
		stream.AcceptWaveform(sttSampleRate, samples[start:end])
		e.recognizer.Decode(stream)
		text := strings.TrimSpace(stream.GetResult().Text)
		sherpa.DeleteOfflineStream(stream)

		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " "), nil
}

// Synthesize converts text to speech and returns a 16-bit PCM WAV file.
func (e *Engine) Synthesize(text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmptyText
	}
	if len(text) > MaxTTSChars {
		return nil, fmt.Errorf("text exceeds %d characters", MaxTTSChars)
	}

	e.ttsMu.Lock()
	audio := e.tts.Generate(text, 0, 1.0)
	e.ttsMu.Unlock()

	if audio == nil || len(audio.Samples) == 0 {
		return nil, errors.New("speech synthesis produced no audio")
	}
	return encodeWAV(audio.Samples, audio.SampleRate), nil
}
