#!/usr/bin/env bash
# Downloads the speech-to-text (Whisper base.en) and text-to-speech (Piper
# en_US-lessac-medium) models used by internal/speech.
# Usage: scripts/download-speech-models.sh [model_dir]   (default: models)
set -euo pipefail

MODEL_DIR="${1:-models}"
BASE="https://github.com/k2-fsa/sherpa-onnx/releases/download"
MODELS=(
  "asr-models/sherpa-onnx-whisper-base.en"
  "tts-models/vits-piper-en_US-lessac-medium"
)

mkdir -p "$MODEL_DIR"
for m in "${MODELS[@]}"; do
  name="$(basename "$m")"
  if [ -d "$MODEL_DIR/$name" ]; then
    echo "$name already present, skipping."
    continue
  fi
  echo "Downloading $name..."
  curl -fSL "$BASE/$m.tar.bz2" | tar -xj -C "$MODEL_DIR"
done

# Only the int8 Whisper weights are used; drop the full-precision ones (~290 MB).
rm -f "$MODEL_DIR"/sherpa-onnx-whisper-base.en/base.en-{encoder,decoder}.onnx
rm -rf "$MODEL_DIR"/sherpa-onnx-whisper-base.en/test_wavs

echo "Speech models ready in $MODEL_DIR"
