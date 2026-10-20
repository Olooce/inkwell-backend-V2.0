package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// decodeAudio uses ffmpeg to convert any audio file to mono float32 samples
// at the given sample rate. ffmpeg is killed if ctx is cancelled.
func decodeAudio(ctx context.Context, path string, sampleRate int) ([]float32, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", path,
		"-f", "f32le", "-acodec", "pcm_f32le",
		"-ac", "1", "-ar", strconv.Itoa(sampleRate),
		"pipe:1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	raw := stdout.Bytes()
	samples := make([]float32, len(raw)/4)
	for i := range samples {
		samples[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return samples, nil
}

// encodeWAV encodes normalised [-1, 1] mono samples as a 16-bit PCM WAV file.
func encodeWAV(samples []float32, sampleRate int) []byte {
	const (
		channels      = 1
		bitsPerSample = 16
	)
	dataSize := len(samples) * bitsPerSample / 8
	byteRate := sampleRate * channels * bitsPerSample / 8

	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	le := binary.LittleEndian

	buf.WriteString("RIFF")
	_ = binary.Write(buf, le, uint32(36+dataSize))
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	_ = binary.Write(buf, le, uint32(16)) // fmt chunk size
	_ = binary.Write(buf, le, uint16(1))  // PCM
	_ = binary.Write(buf, le, uint16(channels))
	_ = binary.Write(buf, le, uint32(sampleRate))
	_ = binary.Write(buf, le, uint32(byteRate))
	_ = binary.Write(buf, le, uint16(channels*bitsPerSample/8)) // block align
	_ = binary.Write(buf, le, uint16(bitsPerSample))

	buf.WriteString("data")
	_ = binary.Write(buf, le, uint32(dataSize))
	pcm := make([]byte, dataSize)
	for i, s := range samples {
		s = max(-1, min(1, s))
		le.PutUint16(pcm[i*2:], uint16(int16(s*math.MaxInt16)))
	}
	buf.Write(pcm)

	return buf.Bytes()
}
