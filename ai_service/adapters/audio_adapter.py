"""
Audio Processing Adapter for LexiAssist AI Infrastructure.
Provides Speech-to-Text (STT) and Text-to-Speech (TTS) capabilities.
"""
import os
import io
import uuid
import base64
import logging
from typing import Optional, Dict, Any

logger = logging.getLogger(__name__)


class AudioAdapter:
    """Adapter for speech recognition and synthesis."""

    def __init__(self):
        self.temp_dir = os.path.join(os.path.dirname(__file__), "temp_audio")
        os.makedirs(self.temp_dir, exist_ok=True)

    async def transcribe_audio(
        self,
        audio_bytes: bytes,
        filename: str = "audio.wav",
        language: str = "en-US"
    ) -> Dict[str, Any]:
        """
        Transcribe audio bytes to text using SpeechRecognition and pydub.
        """
        try:
            import speech_recognition as sr
            from pydub import AudioSegment
            
            recognizer = sr.Recognizer()
            file_ext = os.path.splitext(filename)[1].lower() or ".wav"
            
            temp_id = str(uuid.uuid4())
            input_path = os.path.join(self.temp_dir, f"in_{temp_id}{file_ext}")
            wav_path = os.path.join(self.temp_dir, f"out_{temp_id}.wav")

            with open(input_path, "wb") as f:
                f.write(audio_bytes)

            try:
                # Convert to WAV if needed
                if file_ext != ".wav":
                    audio = AudioSegment.from_file(input_path)
                    audio.export(wav_path, format="wav")
                    process_path = wav_path
                else:
                    process_path = input_path

                with sr.AudioFile(process_path) as source:
                    audio_data = recognizer.record(source)
                    text = recognizer.recognize_google(audio_data, language=language)

                return {
                    "text": text,
                    "language": language,
                    "confidence": 0.95,
                }
            finally:
                for p in (input_path, wav_path):
                    if os.path.exists(p):
                        try:
                            os.remove(p)
                        except OSError:
                            pass

        except Exception as e:
            logger.warning(f"STT transcription failed (fallback to empty transcript): {e}")
            return {
                "text": "",
                "language": language,
                "error": str(e),
                "confidence": 0.0,
            }

    async def synthesize_speech(
        self,
        text: str,
        voice: str = "default",
        lang: str = "en"
    ) -> Dict[str, Any]:
        """
        Convert text to speech and return base64 encoded audio.
        """
        try:
            from gtts import gTTS
            tts = gTTS(text=text, lang=lang, slow=False)
            buffer = io.BytesIO()
            tts.write_to_fp(buffer)
            buffer.seek(0)
            audio_b64 = base64.b64encode(buffer.read()).decode("utf-8")
            return {
                "audio_b64": audio_b64,
                "mime_type": "audio/mp3",
                "voice": voice,
            }
        except Exception as e:
            logger.warning(f"TTS synthesis failed: {e}")
            return {
                "audio_b64": "",
                "mime_type": "audio/mp3",
                "error": str(e),
            }

    async def health(self) -> Dict[str, Any]:
        return {
            "provider": "audio",
            "features": ["speech_to_text", "text_to_speech"],
        }
