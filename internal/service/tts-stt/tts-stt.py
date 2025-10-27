from concurrent.futures import ThreadPoolExecutor

import asyncio
import logging
import os
import subprocess
import tempfile
import torch
from TTS.api import TTS
from fastapi import FastAPI, UploadFile, HTTPException, Form, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, StreamingResponse
from faster_whisper import WhisperModel

logging.basicConfig(
    level=logging.DEBUG,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
logger = logging.getLogger(__name__)

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

model = None
tts_fast = None
executor = ThreadPoolExecutor(max_workers=2)

async def load_models():
    global model, tts_fast
    if model is None:
        model = WhisperModel("base", device="cpu", compute_type="int8")
    if tts_fast is None:
        try:
            tts_fast = TTS("tts_models/en/ljspeech/vits")
            tts_fast.to("cuda")
        except Exception:
            try:
                tts_fast = TTS("tts_models/en/ljspeech/fastspeech2")
                tts_fast.to("cuda")
            except Exception:
                tts_fast = TTS("tts_models/en/ljspeech/tacotron2-DDC")
                tts_fast.to("cuda")

def split_text_smartly(text: str, max_length: int = 100) -> list:
    import re
    sentences = re.split(r'(?<=[.!?]) +', text)
    chunks = []
    current_chunk = ""
    for sentence in sentences:
        if len(current_chunk) + len(sentence) < max_length:
            current_chunk += sentence + " "
        else:
            if current_chunk:
                chunks.append(current_chunk.strip())
            current_chunk = sentence + " "
    if current_chunk:
        chunks.append(current_chunk.strip())
    return chunks

@app.post("/stt")
async def speech_to_text(file: UploadFile):
    await load_models()
    if not file.filename:
        raise HTTPException(status_code=400, detail="No file uploaded")
    file_ext = file.filename.split('.')[-1] if '.' in file.filename else 'webm'
    with tempfile.NamedTemporaryFile(delete=False, suffix=f".{file_ext}") as tmp:
        content = await file.read()
        tmp.write(content)
        tmp_path = tmp.name
    try:
        if file_ext != 'wav':
            wav_path = tmp_path + ".wav"
            process = subprocess.run(
                [
                    "ffmpeg", "-i", tmp_path, "-acodec", "pcm_s16le",
                    "-ar", "16000", "-ac", "1", wav_path, "-y"
                ],
                capture_output=True,
                text=True,
                check=True
            )
            os.unlink(tmp_path)
            tmp_path = wav_path
        segments, info = model.transcribe(tmp_path, beam_size=5)
        text = " ".join([segment.text for segment in segments])
        return {"text": text, "language": info.language}
    except subprocess.CalledProcessError as e:
        raise HTTPException(status_code=500, detail=f"FFmpeg conversion failed: {e.stderr}")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Transcription error: {str(e)}")
    finally:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)

def generate_audio_chunk(text_chunk: str, output_path: str):
    tts_fast.tts_to_file(text=text_chunk, file_path=output_path)

@app.post("/tts")
async def text_to_speech(background_tasks: BackgroundTasks, text: str = Form(...)):
    await load_models()
    start_time = asyncio.get_running_loop().time()
    out_wav = None
    out_mp3 = None
    if len(text) < 50:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as tmp_wav:
            out_wav = tmp_wav.name
        out_mp3 = out_wav.replace(".wav", ".mp3")
        loop = asyncio.get_running_loop()
        await loop.run_in_executor(executor, generate_audio_chunk, text, out_wav)
        await loop.run_in_executor(
            executor,
            lambda: subprocess.run(
                ["ffmpeg", "-i", out_wav, "-codec:a", "libmp3lame", out_mp3, "-y"],
                check=True, capture_output=True
            )
        )
        def cleanup():
            for f in [out_wav, out_mp3]:
                if os.path.exists(f):
                    os.remove(f)
        background_tasks.add_task(cleanup)
        return FileResponse(out_mp3, media_type="audio/mpeg", filename="speech.mp3")
    text_chunks = split_text_smartly(text, max_length=80)
    temp_files = []
    tasks = []
    loop = asyncio.get_running_loop()
    for i, chunk in enumerate(text_chunks):
        tmp_file = tempfile.NamedTemporaryFile(delete=False, suffix=f"_chunk_{i}.wav")
        temp_files.append(tmp_file.name)
        tmp_file.close()
        tasks.append(loop.run_in_executor(executor, generate_audio_chunk, chunk, temp_files[-1]))
    await asyncio.gather(*tasks)
    final_wav_file = tempfile.NamedTemporaryFile(delete=False, suffix="_final.wav")
    final_wav = final_wav_file.name
    final_wav_file.close()
    final_mp3 = final_wav.replace(".wav", ".mp3")
    concat_list_file = tempfile.NamedTemporaryFile(delete=False, suffix="_list.txt")
    concat_list = concat_list_file.name
    concat_list_file.close()
    with open(concat_list, 'w') as f:
        for temp_file in temp_files:
            f.write(f"file '{temp_file}'\n")
    await loop.run_in_executor(
        executor,
        lambda: subprocess.run([
            "ffmpeg", "-f", "concat", "-safe", "0", "-i", concat_list,
            "-c", "copy", final_wav, "-y"
        ], check=True, capture_output=True)
    )
    await loop.run_in_executor(
        executor,
        lambda: subprocess.run([
            "ffmpeg", "-i", final_wav, "-codec:a", "libmp3lame", final_mp3, "-y"
        ], check=True, capture_output=True)
    )
    def cleanup():
        for f in temp_files + [final_wav, final_mp3, concat_list]:
            if os.path.exists(f):
                os.remove(f)
    background_tasks.add_task(cleanup)
    return FileResponse(final_mp3, media_type="audio/mpeg", filename="speech.mp3")

@app.post("/tts-stream")
async def text_to_speech_stream(text: str = Form(...)):
    await load_models()
    async def audio_generator():
        text_chunks = split_text_smartly(text, max_length=50)
        loop = asyncio.get_running_loop()
        for chunk in text_chunks:
            with tempfile.NamedTemporaryFile(delete=False, suffix="_stream.wav") as tmp_wav:
                chunk_wav = tmp_wav.name
            chunk_mp3 = chunk_wav.replace(".wav", ".mp3")
            await loop.run_in_executor(executor, generate_audio_chunk, chunk, chunk_wav)
            await loop.run_in_executor(
                executor,
                lambda: subprocess.run([
                    "ffmpeg", "-i", chunk_wav, "-codec:a", "libmp3lame", chunk_mp3, "-y"
                ], check=True, capture_output=True)
            )
            with open(chunk_mp3, 'rb') as f:
                yield f.read()
            os.remove(chunk_wav)
            os.remove(chunk_mp3)
    return StreamingResponse(audio_generator(), media_type="audio/mpeg")

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
