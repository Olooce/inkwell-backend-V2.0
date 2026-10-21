.PHONY: build models dist test clean

# Dev build; models and tools are read from ./models and ./tools.
build:
	go build -o inkwell ./cmd/app

# Download the speech and image models (skips anything already present).
models:
	scripts/download-speech-models.sh
	scripts/download-image-model.sh

# Self-contained release bundle in dist/ (folder + .tar.gz).
dist:
	scripts/build-dist.sh

test:
	go vet ./...
	go test ./...

clean:
	rm -rf dist inkwell
