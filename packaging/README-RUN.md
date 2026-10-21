# Running the Inkwell backend bundle

This folder is self-contained: the server binary, its native libraries, the
speech and image models, and the image generation tool are all included.

## Requirements on this machine
- Linux x86-64 with a recent glibc (Ubuntu 24.04 or newer, or equivalent).
- **PostgreSQL** - the database configured in `config.xml`.
- **Ollama** with the configured model pulled, e.g. `ollama pull qwen2.5:3b`.
  If Ollama is installed but not running, Inkwell starts it.
- **ffmpeg** on the PATH (used to decode recorded audio for speech-to-text).
- Optional: a Vulkan-capable GPU driver (NVIDIA, AMD or Intel). Without one,
  story pictures are generated on the CPU, which is slower.

## Quick start
```sh
cp config-example.xml config.xml   # then edit DB, auth secrets, port, Ollama
./inkwell
```
The server can be started from any directory; it uses the folder it lives in
for `config.xml`, `models/`, `tools/`, `working/` (generated images and comics)
and `logs/`.

## Checking the bundle
```sh
sha256sum -c SHA256SUMS
```

See `THIRD_PARTY_NOTICES.md` and `licenses/` for bundled components and their licences.
