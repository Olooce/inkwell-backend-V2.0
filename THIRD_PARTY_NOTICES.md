# Third-party components

Release bundles (`make dist`) ship the following components next to the Inkwell
binary. Full licence texts are in `licenses/` inside the bundle.

| Component | Used for | Licence | Source |
|---|---|---|---|
| sherpa-onnx (C API) | Speech-to-text and text-to-speech runtime | Apache-2.0 | https://github.com/k2-fsa/sherpa-onnx |
| ONNX Runtime | Neural network runtime used by sherpa-onnx | MIT | https://github.com/microsoft/onnxruntime |
| Whisper base.en (sherpa-onnx export) | Speech-to-text model | MIT | https://github.com/openai/whisper |
| Piper voice en_US-lessac-medium | Text-to-speech voice | See `MODEL_CARD`; trained on the Blizzard 2013 Lessac dataset, whose licence applies - check it before commercial use | https://github.com/rhasspy/piper |
| stable-diffusion.cpp + ggml | Local image generation runtime | MIT | https://github.com/leejet/stable-diffusion.cpp |
| SD-Turbo (8-bit GGUF conversion) | Image generation model | Stability AI Community License: free for non-commercial use and for organisations under USD 1M annual revenue; larger commercial use needs a Stability AI licence | https://huggingface.co/stabilityai/sd-turbo |

