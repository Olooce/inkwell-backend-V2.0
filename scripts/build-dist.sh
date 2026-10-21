#!/usr/bin/env bash
# Builds a self-contained release bundle: the server binary, its native
# libraries, the speech and image models and the image tool, as a folder and a
# .tar.gz under dist/.
# Usage: scripts/build-dist.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
NAME="inkwell-$VERSION-linux-amd64"
DIST="$REPO_ROOT/dist"
OUT="$DIST/$NAME"

# 1. Models and tools (both scripts skip anything already present).
scripts/download-speech-models.sh
scripts/download-image-model.sh

rm -rf "$OUT" "$DIST/$NAME.tar.gz" "$DIST/$NAME.tar.gz.sha256"
mkdir -p "$OUT/lib" "$OUT/models" "$OUT/tools" "$OUT/working/storyImages" "$OUT/working/comics" "$OUT/licenses"

# 2. Binary, looking for its shared libraries in ./lib next to itself.
echo "Building $NAME..."
CGO_ENABLED=1 CGO_LDFLAGS='-Wl,-rpath,$ORIGIN/lib' \
  go build -trimpath -o "$OUT/inkwell" ./cmd/app

SHERPA_DIR="$(go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-linux)"
cp "$SHERPA_DIR/lib/x86_64-unknown-linux-gnu/libsherpa-onnx-c-api.so" \
   "$SHERPA_DIR/lib/x86_64-unknown-linux-gnu/libonnxruntime.so" "$OUT/lib/"

if command -v patchelf >/dev/null; then
  # Drop the rpath into this machine's Go module cache; only ./lib remains.
  patchelf --set-rpath '$ORIGIN/lib' "$OUT/inkwell"
else
  echo "note: patchelf not installed; the binary also keeps an rpath into this machine's Go module cache (harmless elsewhere)."
fi

# 3. Models: only the files the code loads.
W=sherpa-onnx-whisper-base.en
mkdir -p "$OUT/models/$W" "$OUT/models/sd-turbo"
cp models/$W/base.en-encoder.int8.onnx models/$W/base.en-decoder.int8.onnx models/$W/base.en-tokens.txt "$OUT/models/$W/"
cp -r models/vits-piper-en_US-lessac-medium "$OUT/models/"
cp models/sd-turbo/sd_turbo-q8_0.gguf "$OUT/models/sd-turbo/"
cp -r tools/stable-diffusion.cpp "$OUT/tools/"

# 4. Docs and licences.
cp config-example.xml THIRD_PARTY_NOTICES.md "$OUT/"
cp packaging/README-RUN.md "$OUT/README.md"
cp LICENSE "$OUT/licenses/inkwell-LICENSE.txt"
cp "$SHERPA_DIR/LICENSE" "$OUT/licenses/sherpa-onnx-LICENSE.txt"
cp tools/stable-diffusion.cpp/stable-diffusion.cpp.txt "$OUT/licenses/stable-diffusion.cpp-LICENSE.txt"
cp tools/stable-diffusion.cpp/ggml.txt "$OUT/licenses/ggml-LICENSE.txt"
cp models/sd-turbo/LICENSE.md "$OUT/licenses/sd-turbo-LICENSE.md"
cp models/vits-piper-en_US-lessac-medium/MODEL_CARD "$OUT/licenses/piper-lessac-MODEL_CARD.txt"

# 5. Checksums and archive.
(cd "$OUT" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)
echo "Packing $NAME.tar.gz..."
tar -C "$DIST" -czf "$DIST/$NAME.tar.gz" "$NAME"
(cd "$DIST" && sha256sum "$NAME.tar.gz" > "$NAME.tar.gz.sha256")

du -sh "$OUT" "$DIST/$NAME.tar.gz"
echo "Bundle ready: $OUT"
