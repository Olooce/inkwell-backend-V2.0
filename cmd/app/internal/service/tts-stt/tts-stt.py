import os
import subprocess
import tempfile
import uuid
import logging

from fastapi import FastAPI, UploadFile, HTTPException, Form
from fastapi.responses import FileResponse
from fastapi.middleware.cors import CORSMiddleware
from faster_whisper import WhisperModel

# -----------------------------------
# Logging setup
# -----------------------------------
logging.basicConfig(
    level=logging.DEBUG,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
logger = logging.getLogger(__name__)

app = FastAPI()

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Load Whisper model
logger.info("Loading Whisper model...")
model = WhisperModel("base", device="cpu", compute_type="int8")
logger.info("Whisper model loaded successfully.")


# -----------------------------------
# Speech to Text
# -----------------------------------
@app.post("/stt")
async def speech_to_text(file: UploadFile):
    logger.debug(f"Received STT request: filename={file.filename}, content_type={file.content_type}")

    if not file.filename:
        raise HTTPException(status_code=400, detail="No file uploaded")

    # temp file
    file_ext = file.filename.split('.')[-1] if '.' in file.filename else 'webm'
    with tempfile.NamedTemporaryFile(delete=False, suffix=f".{file_ext}") as tmp:
        content = await file.read()
        logger.debug(f"Uploaded file size: {len(content)} bytes")
        tmp.write(content)
        tmp_path = tmp.name

    try:
        # Convert to wav if not already
        if file_ext != 'wav':
            wav_path = tmp_path + ".wav"
            logger.debug(f"Converting {tmp_path} -> {wav_path} using ffmpeg")
            process = subprocess.run(
                [
                    "ffmpeg", "-i", tmp_path, "-acodec", "pcm_s16le",
                    "-ar", "16000", "-ac", "1", wav_path, "-y"
                ],
                capture_output=True,
                text=True
            )
            if process.returncode != 0:
                logger.error(f"ffmpeg failed: {process.stderr}")
                raise HTTPException(status_code=500, detail="FFmpeg conversion failed")

            os.unlink(tmp_path)
            tmp_path = wav_path
            logger.debug("FFmpeg conversion successful")

        # Transcribe
        logger.info(f"Transcribing {tmp_path}...")
        segments, info = model.transcribe(tmp_path, beam_size=5)
        text = " ".join([segment.text for segment in segments])
        logger.info(f"Transcription done: {text[:50]}... (lang={info.language})")

        return {"text": text, "language": info.language}

    except Exception as e:
        logger.exception("Transcription error")
        raise HTTPException(status_code=500, detail=f"Transcription error: {str(e)}")

    finally:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)
            logger.debug(f"Deleted temp file {tmp_path}")


# -----------------------------------
# Text to Speech
# -----------------------------------
@app.post("/tts")
async def text_to_speech(text: str = Form(...)):
    if not text.strip():
        raise HTTPException(status_code=400, detail="Text cannot be empty")

    # temp files
    out_wav = f"/tmp/{uuid.uuid4()}.wav"
    out_mp3 = out_wav.replace(".wav", ".mp3")

    try:
        subprocess.run(
            ["piper", "--model", "en_US-amy-low.onnx", "--output_file", out_wav],
            input=text.encode("utf-8"),
            check=True,
        )

        # Convert wav → mp3 for browser compatibility
        subprocess.run(
            ["ffmpeg", "-i", out_wav, "-codec:a", "libmp3lame", out_mp3, "-y"],
            check=True
        )

        return FileResponse(
            out_mp3,
            media_type="audio/mpeg",
            filename="speech.mp3",
            headers={"Content-Disposition": "inline; filename=speech.mp3"}
        )

    except subprocess.CalledProcessError:
        raise HTTPException(status_code=500, detail="TTS generation failed")
    finally:
        if os.path.exists(out_wav):
            os.remove(out_wav)


# -----------------------------------
# Run server
# -----------------------------------
if __name__ == "__main__":
    import uvicorn
    logger.info("Starting FastAPI server...")
    uvicorn.run(app, host="0.0.0.0", port=8001)
