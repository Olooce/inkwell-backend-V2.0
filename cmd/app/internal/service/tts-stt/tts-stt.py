import os
import subprocess
import tempfile
import uuid
import logging
import asyncio
from concurrent.futures import ThreadPoolExecutor

from fastapi import FastAPI, UploadFile, HTTPException, Form, BackgroundTasks
from fastapi.responses import FileResponse, StreamingResponse
from fastapi.middleware.cors import CORSMiddleware
from faster_whisper import WhisperModel
from TTS.api import TTS
import torch

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

# try:
#     model = WhisperModel("base", device="cuda", compute_type="float16")
#     logger.info("Whisper model loaded on GPU.")
# except Exception as e:
#     logger.warning(f"GPU failed: {e}. Falling back to CPU.")
#     model = WhisperModel("base", device="cpu", compute_type="int8")

# Initialize  TTS models
logger.info("Loading TTS models...")

try:
    tts_fast = TTS("tts_models/en/ljspeech/vits")
    tts_fast.to("cuda")
    logger.info("VITS model loaded successfully")
except:
    # Fallback to FastSpeech2 if VITS not available
    try:
        tts_fast = TTS("tts_models/en/ljspeech/fastspeech2")
        tts_fast.to("cuda")
        logger.info("FastSpeech2 model loaded successfully")
    except:
        # Last resort Tacotron2
        tts_fast = TTS("tts_models/en/ljspeech/tacotron2-DDC")
        tts_fast.to("cuda")
        logger.info("Using Tacotron2 (slower option)")

# Thread pool for CPU-bound tasks
executor = ThreadPoolExecutor(max_workers=2)

# -----------------------------------
# Text preprocessing for faster inference
# -----------------------------------
def split_text_smartly(text: str, max_length: int = 100) -> list:
    sentences = text.split('. ')
    chunks = []
    current_chunk = ""

    for sentence in sentences:
        if len(current_chunk) + len(sentence) < max_length:
            current_chunk += sentence + ". "
        else:
            if current_chunk:
                chunks.append(current_chunk.strip())
            current_chunk = sentence + ". "

    if current_chunk:
        chunks.append(current_chunk.strip())

    return chunks

# -----------------------------------
# Speech to Text
# -----------------------------------
@app.post("/stt")
async def speech_to_text(file: UploadFile):
    logger.debug(f"Received STT request: filename={file.filename}, content_type={file.content_type}")

    if not file.filename:
        raise HTTPException(status_code=400, detail="No file uploaded")

    file_ext = file.filename.split('.')[-1] if '.' in file.filename else 'webm'
    with tempfile.NamedTemporaryFile(delete=False, suffix=f".{file_ext}") as tmp:
        content = await file.read()
        logger.debug(f"Uploaded file size: {len(content)} bytes")
        tmp.write(content)
        tmp_path = tmp.name

    try:
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
def generate_audio_chunk(text_chunk: str, output_path: str):
    """Generate audio for a text chunk"""
    try:
        tts_fast.tts_to_file(text=text_chunk, file_path=output_path)
        return True
    except Exception as e:
        logger.error(f"TTS generation failed for chunk: {e}")
        return False

@app.post("/tts")
async def text_to_speech(
        background_tasks: BackgroundTasks,
        text: str = Form(...)
):
    start_time = asyncio.get_event_loop().time()

    # For very short texts, process directly
    if len(text) < 50:
        out_wav = f"/tmp/{uuid.uuid4()}.wav"
        out_mp3 = out_wav.replace(".wav", ".mp3")

        loop = asyncio.get_event_loop()
        success = await loop.run_in_executor(
            executor,
            generate_audio_chunk,
            text,
            out_wav
        )

        if not success:
            raise HTTPException(status_code=500, detail="TTS generation failed")

        await loop.run_in_executor(
            executor,
            lambda: subprocess.run(
                ["ffmpeg", "-i", out_wav, "-codec:a", "libmp3lame", out_mp3, "-y"],
                check=True,
                capture_output=True
            )
        )

        processing_time = asyncio.get_event_loop().time() - start_time
        logger.info(f"TTS processing time: {processing_time:.2f}s")

        def cleanup():
            for f in [out_wav, out_mp3]:
                if os.path.exists(f):
                    os.remove(f)

        background_tasks.add_task(cleanup)
        return FileResponse(out_mp3, media_type="audio/mpeg", filename="speech.mp3")

    # For longer texts, split and process in parallel
    text_chunks = split_text_smartly(text, max_length=80)
    logger.info(f"Split text into {len(text_chunks)} chunks")

    tasks = []
    temp_files = []

    for i, chunk in enumerate(text_chunks):
        chunk_wav = f"/tmp/{uuid.uuid4()}_chunk_{i}.wav"
        temp_files.append(chunk_wav)

        task = asyncio.get_event_loop().run_in_executor(
            executor,
            generate_audio_chunk,
            chunk,
            chunk_wav
        )
        tasks.append(task)

    results = await asyncio.gather(*tasks)
    if not all(results):
        for temp_file in temp_files:
            if os.path.exists(temp_file):
                os.remove(temp_file)
        raise HTTPException(status_code=500, detail="Some TTS chunks failed")

    final_wav = f"/tmp/{uuid.uuid4()}_final.wav"
    final_mp3 = final_wav.replace(".wav", ".mp3")
    concat_list = f"/tmp/{uuid.uuid4()}_list.txt"

    with open(concat_list, 'w') as f:
        for temp_file in temp_files:
            f.write(f"file '{temp_file}'\n")

    await asyncio.get_event_loop().run_in_executor(
        executor,
        lambda: subprocess.run([
            "ffmpeg", "-f", "concat", "-safe", "0", "-i", concat_list,
            "-c", "copy", final_wav, "-y"
        ], check=True, capture_output=True)
    )

    await asyncio.get_event_loop().run_in_executor(
        executor,
        lambda: subprocess.run([
            "ffmpeg", "-i", final_wav, "-codec:a", "libmp3lame", final_mp3, "-y"
        ], check=True, capture_output=True)
    )

    processing_time = asyncio.get_event_loop().time() - start_time
    logger.info(f"Total TTS processing time: {processing_time:.2f}s")

    def cleanup():
        for temp_file in temp_files + [final_wav, final_mp3, concat_list]:
            if os.path.exists(temp_file):
                os.remove(temp_file)

    background_tasks.add_task(cleanup)
    return FileResponse(final_mp3, media_type="audio/mpeg", filename="speech.mp3")


# -----------------------------------
# Streaming TTS endpoint (for real-time feel)
# -----------------------------------
@app.post("/tts-stream")
async def text_to_speech_stream(text: str = Form(...)):
    """Stream audio as it's generated for better perceived performance"""

    async def audio_generator():
        text_chunks = split_text_smartly(text, max_length=50)

        for chunk in text_chunks:
            chunk_wav = f"/tmp/{uuid.uuid4()}_stream.wav"
            chunk_mp3 = chunk_wav.replace(".wav", ".mp3")

            try:
                # Generate audio chunk
                loop = asyncio.get_event_loop()
                await loop.run_in_executor(
                    executor,
                    generate_audio_chunk,
                    chunk,
                    chunk_wav
                )

                # Convert to MP3
                await loop.run_in_executor(
                    executor,
                    lambda: subprocess.run([
                        "ffmpeg", "-i", chunk_wav, "-codec:a", "libmp3lame", chunk_mp3, "-y"
                    ], check=True, capture_output=True)
                )

                # Stream the chunk
                with open(chunk_mp3, 'rb') as f:
                    chunk_data = f.read()
                    yield chunk_data

                # Cleanup
                os.remove(chunk_wav)
                os.remove(chunk_mp3)

            except Exception as e:
                logger.error(f"Error generating chunk: {e}")
                break

    return StreamingResponse(
        audio_generator(),
        media_type="audio/mpeg"
    )

# -----------------------------------
# Run server
# -----------------------------------
if __name__ == "__main__":
    import uvicorn
    logger.info("Starting FastAPI server...")
    uvicorn.run(app, host="0.0.0.0", port=8001)