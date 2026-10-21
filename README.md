<div align="center">

# Inkwell Backend

![image](https://github.com/user-attachments/assets/24b39c64-7bcf-4b6c-a8f6-9b8b76ff0988)

</div>


## Overview
Inkwell is a writing companion for children. A child takes a
short grammar quiz, then writes stories one sentence at a time: each sentence is
checked and corrected, illustrated, and the finished story becomes a downloadable
comic with personalised writing tips.

This repository is the Go backend. It serves the REST API used by the
[Inkwell frontend](https://github.com/Olooce/inkwell-V2.0) and runs the AI
features. The heavy models run locally, so children's writing doesn't have to
leave the server.

## Features
- **Accounts:** registration, login and JWT access/refresh tokens.
- **Grammar quiz:** a five-question placement quiz, which can be retaken to
  measure progress. Answers are graded by deterministic rules; only close,
  reworded answers are referred to the LLM, as a strict judge.
- **Story writing:** each sentence is corrected with a short explanation and
  illustrated. Finished stories are turned into PDF comics.
- **Writing feedback:** an analysis, tips and a score for every finished story,
  generated in the background.
- **Progress:** quiz scores over time, writing statistics and accuracy.
- **Writing helper chat:** streaming LLM chat, with speech-to-text input and
  text-to-speech read-aloud.
- **Release bundle:** `make dist` builds a self-contained package with all the
  models included.

## How the AI parts fit together
| Feature | Runs on | Implementation |
|---|---|---|
| Quiz grading | Rules, plus the LLM for close calls | `internal/grading` |
| Sentence correction, feedback, story analysis, chat | Local LLM via [Ollama](https://ollama.com); set `OLLAMA_MODEL` (default `mistral`) | `internal/llm` |
| Story pictures | [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) running SD-Turbo locally (GPU via Vulkan, CPU fallback); Hugging Face optional | `internal/imagegen` |
| Speech-to-text and text-to-speech | [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) in-process (Whisper base.en, Piper voice) | `internal/speech` |

LLM output is requested as schema-constrained JSON, so it is parsed reliably.
If Ollama is installed but not running, the server starts it and stops it again
on shutdown.

## Tech stack
Go 1.25 (cgo), Gin, GORM with PostgreSQL, Ollama, sherpa-onnx with ONNX Runtime,
stable-diffusion.cpp, gofpdf.

## Project structure
```
.
├── cmd/app/
│   ├── main.go                # Entry point: config, services, routes, child processes
│   ├── seed.go                # Seeds the quiz questions on first start
│   └── process_*.go           # Per-OS handling of child processes (Ollama)
├── internal/
│   ├── config/                # XML configuration loader
│   ├── controller/            # HTTP handlers and route registration
│   ├── db/                    # Database connection pool
│   ├── grading/               # Rule-based quiz grading (+ LLM judge interface)
│   ├── imagegen/              # Image generation: local stable-diffusion.cpp, Hugging Face
│   ├── llm/                   # Ollama client: chat, corrections, judging, analysis
│   ├── model/                 # GORM data models
│   ├── repository/            # Data access
│   ├── service/               # Business logic (auth, quiz, stories, comics, analysis, progress)
│   ├── speech/                # In-process speech-to-text / text-to-speech
│   └── utilities/             # JWT helpers
├── pkg/
│   ├── event_bus/             # In-process events (e.g. story completed -> comic + analysis)
│   ├── logging/               # Rotating log files
│   └── middleware/            # Auth, CORS, rate limiting, request dumps
├── scripts/                   # Model downloads and release bundle build
├── packaging/                 # Files shipped inside the release bundle
├── models/  tools/            # Downloaded models and stable-diffusion.cpp (git-ignored)
├── working/                   # Generated story images and comic PDFs (served under /static)
├── config-example.xml         # Configuration template
└── Makefile
```

## Getting started

### Prerequisites
- **Go 1.25+** and a C compiler (cgo is required for the speech engine).
- **PostgreSQL.**
- **Ollama** with the model you configure, e.g. `ollama pull qwen2.5:3b`.
- **ffmpeg** on the PATH (decodes recorded audio for speech-to-text).
- Optional: a **Vulkan-capable GPU driver** for faster pictures (about 5 s each
  on a GTX 1650; slower on the CPU).
- Optional: a **Hugging Face token** with credits, if you want hosted image
  generation as a fallback.

### Setup
```bash
git clone https://github.com/Olooce/inkwell-backend-V2.0.git
cd inkwell-backend-V2.0
cp config-example.xml config.xml   # then edit it, see below
make models                        # downloads the speech and image models (~2.4 GB, once)
```

### Configuration
All settings live in `config.xml` (start from `config-example.xml`). If the file
is missing, the server reads the same XML from the `CONFIG_XML` environment
variable, which can also be set in a `.env` file.

| Section | What it controls |
|---|---|
| `CONTEXT` | Host, port, mode (`debug` / `release`), trusted proxies |
| `AUTHENTICATION` | Token lifetimes and the access/refresh signing secrets |
| `DB` | PostgreSQL connection and pool settings |
| `THIRD_PARTY` | `OLLAMA_HOST`, `OLLAMA_MODEL`, `SPEECH_MODEL_DIR`, `HF_TOKEN` |
| `IMAGE_GENERATION` | `PROVIDER` (`local` or `huggingface`), `FALLBACK`, model paths, steps |
| `LOGGING` | Log directory, rotation size, backups and retention |

Keep `config.xml` out of version control; it holds secrets and is already
git-ignored.

### Running
```bash
go run ./cmd/app      # development; quiz questions are seeded on first start
make build && ./inkwell
```
If the speech or image models are missing, the server still starts: the speech
endpoints return 503, and stories are saved without pictures.

### Tests
```bash
make test             # go vet + unit tests
OLLAMA_TEST=1 OLLAMA_MODEL=qwen2.5:3b go test ./internal/llm/   # against a local Ollama
IMAGEGEN_TEST=1 go test -run TestLocalGenerate ./internal/imagegen/   # renders a real image
SPEECH_MODEL_DIR=$PWD/models go test ./internal/speech/              # speech round trip
```

## Release bundle
`make dist` builds a self-contained package in `dist/`: a folder and a `.tar.gz`
(about 2.5 GB) containing the server binary, its native libraries, the speech and
image models and stable-diffusion.cpp, plus `SHA256SUMS`. Unpack it on any Linux
x86-64 machine, put a `config.xml` next to the binary and run `./inkwell`. It
can be started from any directory and needs no Go toolchain or downloads.

The target machine still needs PostgreSQL, Ollama (with the model pulled),
ffmpeg, and a glibc at least as new as Ubuntu 24.04's. A Vulkan GPU driver is
optional.

## License
Inkwell is licensed under the **MIT License**. The release bundle also contains
third-party components under their own licences, including the SD-Turbo model
(Stability AI Community License); see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

---
<a href="https://next.ossinsight.io/widgets/official/compose-contributors?repo_id=949105136&limit=200" target="_blank" style="display: block" align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://next.ossinsight.io/widgets/official/compose-contributors/thumbnail.png?repo_id=949105136&limit=200&image_size=auto&color_scheme=dark" width="655" height="auto">
    <img alt="Contributors to Olooce/inkwell-backend-V2.0" src="https://next.ossinsight.io/widgets/official/compose-contributors/thumbnail.png?repo_id=949105136&limit=200&image_size=auto&color_scheme=light" width="655" height="auto">
  </picture>
</a>

