#!/usr/bin/env bash
# Sets up local story-image generation (internal/imagegen):
#   - stable-diffusion.cpp CLI, Vulkan build (uses NVIDIA/AMD/Intel GPUs, falls back to CPU)
#   - Stability AI's SD-Turbo, converted to an 8-bit GGUF (~2 GB instead of 5.2 GB)
# SD-Turbo is under the Stability AI Community License (free for non-commercial use
# and for organisations under $1M annual revenue).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SD_RELEASE="master-945-a1ded76"
SD_ZIP="sd-master-a1ded76-bin-Linux-Ubuntu-24.04-x86_64-vulkan.zip"
SD_DIR="$REPO_ROOT/tools/stable-diffusion.cpp"
MODEL_DIR="$REPO_ROOT/models/sd-turbo"
MODEL="$MODEL_DIR/sd_turbo-q8_0.gguf"

if [ ! -x "$SD_DIR/sd-cli" ]; then
  echo "Downloading stable-diffusion.cpp $SD_RELEASE..."
  mkdir -p "$SD_DIR"
  tmp="$(mktemp -d)"
  curl -fSL -o "$tmp/sd.zip" "https://github.com/leejet/stable-diffusion.cpp/releases/download/$SD_RELEASE/$SD_ZIP"
  unzip -oq "$tmp/sd.zip" -d "$SD_DIR"
  rm -rf "$tmp"
fi

if [ ! -f "$MODEL" ]; then
  mkdir -p "$MODEL_DIR"
  echo "Downloading SD-Turbo weights from stabilityai/sd-turbo (5.2 GB)..."
  curl -fSL -C - -o "$MODEL_DIR/sd_turbo.safetensors" \
    "https://huggingface.co/stabilityai/sd-turbo/resolve/main/sd_turbo.safetensors"
  # Going via f16 first also shrinks the tensors q8_0 can't quantize (2.1 GB vs 3.4 GB).
  echo "Converting to 8-bit GGUF..."
  "$SD_DIR/sd-cli" -M convert -m "$MODEL_DIR/sd_turbo.safetensors" -o "$MODEL_DIR/sd_turbo-f16.gguf" --type f16
  "$SD_DIR/sd-cli" -M convert -m "$MODEL_DIR/sd_turbo-f16.gguf" -o "$MODEL" --type q8_0
  rm -f "$MODEL_DIR/sd_turbo.safetensors" "$MODEL_DIR/sd_turbo-f16.gguf"
fi

"$SD_DIR/sd-cli" --list-devices 2>/dev/null | grep -v '^load_backend\|^ggml_' || true
echo "Image generation ready: $MODEL"
