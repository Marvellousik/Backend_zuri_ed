---
name: zuri-ai-services
description: >-
  Use this skill when developing, testing, or modifying the Python AI services in Zuri, including
  the AI microservices (zuri-Python Services/) and the AI monolith (zuri-ai-main/), managing
  Gemini/Cohere integrations, prompt engineering, or vector retrieval with pgvector.
---

# Zuri Python AI Services Development & Workflows

This skill guides the implementation, testing, prompt design, and debugging of all Python-based AI microservices and the AI Monolith in Zuri.

---

## 1. AI Architecture Overview

Zuri uses a dual AI setup:
1. **AI Microservices (`zuri-Python Services/services/`)**: High-throughput specialized FastAPI services.
2. **AI Monolith (`zuri-ai-main/`)**: Full-featured learning assistant suite combining reading, writing, and study tools.

### A. Python AI Microservices (`zuri-Python Services/services/`)

| Service | Port | Entry Point | Core Responsibilities | Key Dependencies |
| :--- | :--- | :--- | :--- | :--- |
| **Orchestrator** | `:5005` | `orchestrator/main.py` | Google Gemini API proxy, dynamic model routing (`gemini-2.5-flash-lite`, `gemini-2.5-flash`, `gemini-2.5-pro`), quiz & summary generation, AI usage cost tracking webhook | `google-generativeai`, `fastapi`, `httpx` |
| **Ingestion** | `:5002` | `ingestion/main.py` | PDF parsing, recursive text chunking, document parsing | `pdfplumber`, `pypdfium2`, `pydantic` |
| **Retrieval** | `:5003` | `retrieval/main.py` | Vector search using pgvector cosine distance on `ai.zuri_chunks`, RAG context assembly, Cohere embeddings | `cohere`, `pgvector`, `sqlalchemy` |
| **Audio** | `:5004` | `audio/main.py` | Speech-to-text transcription, text-to-speech generation | `SpeechRecognition`, `pydub`, `gTTS` |
| **Evaluation** | `:5006` | `evaluation/main.py` | Quiz grading, automated feedback generation, AI token/cost calculation | `fastapi`, `sqlalchemy` |

### B. AI Monolith (`zuri-ai-main/`)

| Subsystem | Key Files | Functionality |
| :--- | :--- | :--- |
| **Core Engine** | `zuricore.py`, `api.py` | `ZuriEngine` class, Weaviate & PostgreSQL fallback RAG retrieval, interactive chat |
| **Reading Assistant** | `reading_assistant/reading_engine.py`, `tts_engine.py`, `job_manager.py` | Reading analysis, paragraph-by-paragraph TTS, asynchronous audio generation job queue |
| **Study Buddy** | `study_buddy/flashcards.py`, `quizzes.py`, `routes.py` | Flashcard extraction, multiple-choice quiz generation with explanations |
| **Writing Assistant** | `writing_assistant/routes.py` | Lecture audio transcription, structured study note formatting |
| **Async Worker** | `worker.py`, `job_queue.py` | Background task processing for heavy AI audio & ingestion jobs |

---

## 2. Dynamic Model Router & Gemini Pricing

The orchestrator and evaluation services compute model selection and costs dynamically:

| Model | Input Cost (per 1k tokens) | Output Cost (per 1k tokens) | Target Use Case |
| :--- | :--- | :--- | :--- |
| `gemini-2.5-flash-lite` | `$0.00010` | `$0.00040` | Chat, fast summaries, basic Q&A |
| `gemini-2.5-flash` | `$0.00030` | `$0.00250` | Standard quizzes, complex explanations |
| `gemini-2.5-pro` | `$0.00125` | `$0.01000` | In-depth essay analysis, complex code review |

> [!TIP]
> When handling rate limits (HTTP 429), the orchestrator automatically backs off with exponential jitter, leverages the Redis cache (`shared/ai_cache.py`), and falls back to `gemini-2.5-flash-lite`.

---

## 3. Vector Embeddings & pgvector Schema

Embeddings are stored in the PostgreSQL database under the `ai` schema:
- **Table**: `ai.zuri_chunks` (defined in `infra/migrations/008_create_document_chunks.sql`)
- **Dimensions**: 1024-dimensional vectors (Cohere `embed-multilingual-v3.0`)
- **Indexing**: Cosine distance vector indexing (`vector_cosine_ops`)
- **Queries**: Always filter by `course` or `doc_id` before computing vector distance for optimal performance.

---

## 4. Verification & Testing Workflows

### A. Python AST Syntax Validation
Always run the syntax validator before committing changes to Python services:
```bash
python3 scripts/check_syntax.py
# or via Makefile
make check-syntax
```
This validates all 15 core service entrypoints across `zuri-Python Services/` and `zuri-ai-main/`.

### B. Python Integration Tests
To test cost calculation, prompt formatting, document truncation, and model routing:
```bash
python3 tests/integration/integration_test_phase1_phase2.py
```

### C. Audio & Temporary Files Management
- Audio temporary files must be cleaned up in-memory or written to `temp_audio/`.
- Never commit media files (`.ogg`, `.wav`, `.mp3`) to Git. Verify with `git status` that `.gitkeep` is preserved while generated files are ignored.
