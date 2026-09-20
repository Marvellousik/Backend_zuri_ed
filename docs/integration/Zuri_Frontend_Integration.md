# Zuri — Frontend Integration

**Backend:** Fully deployed, all 15 containers healthy  
**Base URL:** `https://staging.zuri.app`  
**Health check:** `GET https://staging.zuri.app/health`

All protected endpoints require: `Authorization: Bearer <access_token>`  
Default content type: `application/json` — file uploads use `multipart/form-data`  
Rate limits: 100 req/min standard, 20 req/min on all AI endpoints  
Access token expires: 15 minutes | Refresh token expires: 30 days

---

## Environment Variables

Update your `.env.local` with these values:

```env
NEXT_PUBLIC_API_GATEWAY_URL=https://staging.zuri.app
NEXT_PUBLIC_API_PROXY_URL=https://staging.zuri.app
NEXT_PUBLIC_USE_MOCK_API=false
```

> `NEXT_PUBLIC_USE_MOCK_API=false` is required. Without it the app runs against mock data instead of the real backend.

---

## Before You Start

- Store `access_token` in memory (React state/context) — not localStorage
- Store `refresh_token` in an httpOnly cookie or secure storage
- Store `user.id` after login — AI endpoints require it in the request body
- Never set `Content-Type` manually on file upload requests — let the browser set it with the multipart boundary
- `/writing/transcribe` returns a stream, not JSON — use `ReadableStream`, not `res.json()`
- `structured_notes` and `summary` fields return markdown — use `react-markdown`
- File uploads accept `.pdf`, `.txt`, `.docx` only
- On `429`, check `X-RateLimit-Reset` header and show a wait state

---

## Authentication

| Method | Path | Auth |
|---|---|---|
| POST | `/api/v1/auth/register` | No |
| POST | `/api/v1/auth/login` | No |
| POST | `/api/v1/auth/refresh` | No |
| POST | `/api/v1/auth/verify-email` | No |
| POST | `/api/v1/auth/resend-verification` | Yes |
| POST | `/api/v1/auth/forgot-password` | No |
| POST | `/api/v1/auth/reset-password` | No |
| POST | `/api/v1/auth/logout` | Yes |
| POST | `/api/v1/auth/logout-all` | Yes |

#### Register
```json
// Request
{
  "email": "student@university.edu",
  "password": "SecurePass123!",
  "first_name": "Jane",
  "last_name": "Doe",
  "school": "University of Lagos",
  "department": "Computer Science",
  "academic_level": "undergraduate"
}
// academic_level: undergraduate | postgraduate | doctoral | staff

// Response 201
{
  "message": "User registered successfully. Please check your email for verification code.",
  "data": {
    "id": "uuid",
    "email": "student@university.edu",
    "first_name": "Jane",
    "last_name": "Doe",
    "full_name": "Jane Doe",
    "email_verified": false,
    "created_at": "2026-05-15T12:00:00Z"
  }
}
```

#### Login
```json
// Request
{
  "email": "student@university.edu",
  "password": "SecurePass123!"
}

// Response 200
{
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "base64-string",
    "token_type": "Bearer",
    "expires_at": "2026-05-15T12:15:00Z",
    "user": {
      "id": "uuid",
      "email": "student@university.edu",
      "first_name": "Jane",
      "last_name": "Doe",
      "full_name": "Jane Doe",
      "email_verified": false,
      "created_at": "2026-05-15T12:00:00Z"
    }
  }
}
```

#### Refresh Token
```json
// Request
{ "refresh_token": "base64-string" }
// Response 200 — same structure as login. Old refresh token is immediately revoked.
```

#### Verify Email
```json
// POST /api/v1/auth/verify-email?user_id=uuid
{ "code": "123456" }
// Response 200: { "message": "Email verified successfully" }
```

#### Forgot / Reset Password
```json
// POST /api/v1/auth/forgot-password
{ "email": "student@university.edu" }
// Always returns 200 regardless of whether email exists

// POST /api/v1/auth/reset-password
{ "token": "reset-token-from-email", "new_password": "NewPass123!" }
```

---

## User Profile

| Method | Path |
|---|---|
| GET | `/api/v1/users/me` |
| PUT | `/api/v1/users/me` |
| POST | `/api/v1/users/me/change-password` |
| GET | `/api/v1/users/me/sessions` |
| DELETE | `/api/v1/users/me/sessions/:id` |

```json
// PUT /api/v1/users/me — all fields optional
{
  "first_name": "Jane",
  "last_name": "Doe",
  "school": "University of Lagos",
  "department": "Computer Science",
  "academic_level": "postgraduate",
  "timezone": "Africa/Lagos"
}

// POST /api/v1/users/me/change-password
{
  "current_password": "OldPass123!",
  "new_password": "NewPass456!"
}
```

---

## Courses

| Method | Path |
|---|---|
| GET | `/api/v1/courses` |
| POST | `/api/v1/courses` |
| GET | `/api/v1/courses/:id` |
| PUT | `/api/v1/courses/:id` |
| DELETE | `/api/v1/courses/:id` |
| GET | `/api/v1/courses/:id/materials` |

```json
// POST /api/v1/courses
{
  "name": "Machine Learning 101",
  "description": "Intro to ML",
  "color": "#3B82F6",
  "semester": "Fall",
  "year": 2026
}

// Response 201
{
  "data": {
    "id": "uuid",
    "user_id": "uuid",
    "name": "Machine Learning 101",
    "color": "#3B82F6",
    "semester": "Fall",
    "year": 2026,
    "created_at": "2026-05-15T12:00:00Z"
  }
}
```

---

## Materials

| Method | Path |
|---|---|
| GET | `/api/v1/materials` |
| POST | `/api/v1/materials` |
| GET | `/api/v1/materials/:id` |
| PUT | `/api/v1/materials/:id` |
| DELETE | `/api/v1/materials/:id` |
| POST | `/api/v1/materials/:id/presign` |

```json
// POST /api/v1/materials
{
  "title": "Week 3 Lecture Notes",
  "content_type": "pdf",
  "file_size": 204800,
  "course_id": "uuid"
}

// Response 201
{
  "data": {
    "id": "uuid",
    "title": "Week 3 Lecture Notes",
    "processing_status": "pending",
    "created_at": "2026-05-15T12:00:00Z"
  }
}
```

---

## Quizzes

| Method | Path |
|---|---|
| GET | `/api/v1/quizzes` |
| POST | `/api/v1/quizzes` |
| GET | `/api/v1/quizzes/:id` |
| PUT | `/api/v1/quizzes/:id` |
| DELETE | `/api/v1/quizzes/:id` |

```json
// POST /api/v1/quizzes
{
  "title": "ML Quiz 1",
  "course_id": "uuid",
  "time_limit_minutes": 15,
  "difficulty": "medium",
  "questions": [
    {
      "question_text": "What is supervised learning?",
      "question_type": "multiple_choice",
      "options": [
        { "text": "Learning with labels", "is_correct": true },
        { "text": "Learning without labels", "is_correct": false }
      ],
      "correct_answer": "Learning with labels",
      "explanation": "Supervised learning uses labeled training data",
      "points": 10
    }
  ]
}
```

---

## Flashcard Decks

| Method | Path |
|---|---|
| GET | `/api/v1/flashcard-decks` |
| POST | `/api/v1/flashcard-decks` |
| GET | `/api/v1/flashcard-decks/:id` |
| PUT | `/api/v1/flashcard-decks/:id` |
| DELETE | `/api/v1/flashcard-decks/:id` |
| POST | `/api/v1/flashcard-decks/:id/cards` |

---

## AI — Chat & Text Generation

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/ai/chat` | Chat with AI |
| POST | `/api/v1/ai/generate/quiz` | Generate quiz from pasted text |
| POST | `/api/v1/ai/generate/summary` | Generate summary from pasted text |
| POST | `/api/v1/ai/generate/flashcards` | Generate flashcards from pasted text |
| GET | `/api/v1/ai/conversation/:id` | Get conversation history |
| DELETE | `/api/v1/ai/conversation/:id` | Clear conversation |

```json
// POST /api/v1/ai/chat
{
  "query": "What is gradient descent?",
  "user_id": "uuid",
  "context_chunks": ["relevant text chunk..."],
  "material_id": "uuid",       // optional
  "conversation_id": "uuid"    // optional — pass on follow-ups to maintain context
}

// Response 200
{
  "response": "Gradient descent is...",
  "conversation_id": "uuid",
  "tokens_used": 135,
  "model": "gemini-pro"
}

// /ai/generate/* endpoints use the same request shape
// Responses:
// summary:    { "summary": "...", "type": "summary" }
// quiz:       { "quiz": "...", "type": "quiz" }
// flashcards: { "flashcards": "...", "type": "flashcards" }
```

---

## AI — Writing Assistant

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/writing/transcribe` | Transcribe audio chunk — returns SSE stream |
| POST | `/api/v1/writing/notes` | Convert transcript to structured notes |
| GET | `/api/v1/writing/notes/:session_id` | Retrieve past notes session |
| GET | `/api/v1/writing/history` | List all notes sessions |

```
// POST /api/v1/writing/transcribe
// Content-Type: multipart/form-data
// Fields:
//   audio       — 5–15s chunk, webm/wav/mp3/m4a, max 25MB (required)
//   session_id  — omit on first chunk, pass returned value on subsequent chunks
//   language    — BCP-47 code e.g. "en", defaults to "en"

// Response: text/event-stream (SSE)
event: session
data: <session_uuid>

data: The mitochondria is the powerhouse of the cell.

data: [DONE]
```

```json
// POST /api/v1/writing/notes
{
  "session_id": "uuid-from-transcribe",
  "raw_text": "Full accumulated transcript...",
  "subject": "Biology",
  "save": true,
  "user_id": "uuid"
}

// Response 200
{
  "session_id": "uuid",
  "structured_notes": "## Mitochondria\n- **Mitochondria** is the powerhouse..."
}
// structured_notes is markdown — render with react-markdown
```

---

## AI — Reading Assistant

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/reading/analyse` | Upload document, get summary + vocab + TTS audio |
| GET | `/api/v1/reading/:session_id` | Retrieve past reading session |

```
// POST /api/v1/reading/analyse
// Content-Type: multipart/form-data
// Fields:
//   file          — .pdf, .txt, or .docx (required)
//   user_id       — UUID from login (required)
//   summary_type  — "brief" | "concise" (default) | "detailed"
//   voice         — "Zephyr" (default) | "Puck" | "Athena" | "Aria" | "Nova"
//   temperature   — TTS expressiveness 0.0–1.0, defaults to 1.0
```

```json
// Response 200
{
  "session_id": "uuid",
  "summary": "This document discusses...",
  "vocab_terms": [
    {
      "term": "Oxidative Phosphorylation",
      "definition": "The metabolic pathway...",
      "context_snippet": "...the inner membrane..."
    }
  ],
  "tts_audio_b64": "UklGRi4AAAB...",
  "audio_mime_type": "audio/wav",
  "voice": "Zephyr"
}
```

Playing the audio:
```javascript
const bytes = Uint8Array.from(atob(response.tts_audio_b64), c => c.charCodeAt(0));
const blob = new Blob([bytes], { type: response.audio_mime_type });
const url = URL.createObjectURL(blob);
const audio = new Audio(url);
audio.play();
audio.onended = () => URL.revokeObjectURL(url);
```

> `tts_audio_b64` can be large for long documents — lazy load, do not auto-play.

---

## AI — Study Buddy

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/study/flashcards` | Generate flashcards from file |
| GET | `/api/v1/study/flashcards/:session_id` | Retrieve flashcard session |
| POST | `/api/v1/study/quiz` | Generate quiz from file |
| GET | `/api/v1/study/quiz/:session_id` | Retrieve quiz session |
| GET | `/api/v1/study/history` | All flashcard and quiz sessions |

```
// POST /api/v1/study/flashcards
// Content-Type: multipart/form-data
// Fields:
//   file       — .pdf, .txt, or .docx (required)
//   user_id    — UUID from login (required)
//   num_cards  — 1–50, defaults to 10
```

```json
// Response 200
{
  "session_id": "uuid",
  "filename": "biology-notes.pdf",
  "num_generated": 10,
  "flashcards": [
    {
      "front": "What is the powerhouse of the cell?",
      "back": "The mitochondria — produces ATP through oxidative phosphorylation.",
      "topic": "Cell Biology"
    }
  ]
}
```

```
// POST /api/v1/study/quiz
// Content-Type: multipart/form-data
// Fields:
//   file           — .pdf, .txt, or .docx (required)
//   user_id        — UUID from login (required)
//   quiz_type      — "multiple_choice" or "theory" (required)
//   num_questions  — 1–30, defaults to 5
```

```json
// Multiple choice response
{
  "session_id": "uuid",
  "quiz_type": "multiple_choice",
  "questions": [
    {
      "question": "What organelle produces ATP?",
      "options": { "A": "Nucleus", "B": "Mitochondria", "C": "Ribosome", "D": "Golgi apparatus" },
      "correct_answer": "B",
      "explanation": "The mitochondria produces ATP through oxidative phosphorylation.",
      "topic": "Cell Biology"
    }
  ]
}

// Theory response
{
  "questions": [
    {
      "question": "Explain the process of oxidative phosphorylation.",
      "model_answer": "Oxidative phosphorylation is...",
      "marking_guide": ["Mention electron transport chain", "Describe proton gradient"],
      "marks": 5,
      "topic": "Cell Biology"
    }
  ]
}
```

---

## RAG — Vector Search

```json
// POST /api/v1/ai/retrieve
{
  "query": "neural networks backpropagation",
  "user_id": "uuid",
  "material_id": "uuid",  // optional
  "top_k": 5              // max 10
}

// Response 200
{
  "results": [
    {
      "chunk_id": "uuid",
      "material_id": "uuid",
      "chunk_text": "Neural networks learn via backpropagation...",
      "similarity_score": 0.95
    }
  ]
}
// Pass chunk_text values as context_chunks in /api/v1/ai/chat
```

---

## Speech to Text

```
// POST /api/v1/ai/speech-to-text
// Content-Type: multipart/form-data
// Fields:
//   audio     — MP3, WAV, M4A, OGG, FLAC, AAC, MP4, WEBM (required)
//   language  — defaults to "en-US"
```

```json
// Response 200
{
  "text": "Transcribed text here",
  "confidence": 0.95,
  "language": "en-US"
}
```

---

## Analytics

| Method | Path |
|---|---|
| GET | `/api/v1/analytics/study-streak` |
| GET | `/api/v1/analytics/study-stats` |
| GET | `/api/v1/analytics/topic-mastery` |
| GET | `/api/v1/analytics/ai-usage?days=30` |
| GET | `/api/v1/analytics/goals` |
| POST | `/api/v1/analytics/goals` |
| POST | `/api/v1/analytics/goals/:id/complete` |
| POST | `/api/v1/quizzes/:id/start` |
| POST | `/api/v1/quiz-attempts/:id/answers` |
| POST | `/api/v1/quiz-attempts/:id/complete` |

```json
// GET /api/v1/analytics/study-stats — Response 200
{
  "data": {
    "current_streak": 5,
    "total_study_days": 30,
    "total_study_minutes": 1200,
    "total_quizzes_completed": 15,
    "total_materials_reviewed": 8
  }
}
```

---

## Notifications

| Method | Path |
|---|---|
| GET | `/api/v1/notifications/preferences` |
| PUT | `/api/v1/notifications/preferences` |
| POST | `/api/v1/notifications/devices/register` |
| DELETE | `/api/v1/notifications/devices/:token` |
| GET | `/api/v1/notifications/reminders` |
| POST | `/api/v1/notifications/reminders` |
| DELETE | `/api/v1/notifications/reminders/:id` |
| GET | `/api/v1/notifications/history` |

---

## Sync & WebSocket

| Method | Path |
|---|---|
| GET | `/api/v1/sync/state?device_id=xxx` |
| POST | `/api/v1/sync/ack` |
| GET | `/api/v1/sync/events?since=<ISO8601>` |
| PUT | `/api/v1/presence` |
| GET | `/api/v1/ws` — upgrade to WebSocket |

WebSocket messages:
```json
{
  "type": "event",
  "payload": {
    "event_type": "material.created",
    "data": {}
  }
}
```

Event types: `material.created`, `material.updated`, `quiz.completed`, `course.updated`, `progress.updated`, `streak.updated`, `goal.updated`

---

## Error Format

```json
{ "message": "error description" }   // User, Content, Analytics, Sync
{ "error": "error description" }     // AI Monolith services
```

| Code | Meaning |
|---|---|
| 400 | Bad request |
| 401 | Token expired — refresh and retry |
| 404 | Resource not found |
| 415 | Unsupported file type |
| 422 | Unprocessable — empty file, no speech detected |
| 429 | Rate limited — check `X-RateLimit-Reset` header |
| 503 | Circuit breaker open — wait 60s and retry |
