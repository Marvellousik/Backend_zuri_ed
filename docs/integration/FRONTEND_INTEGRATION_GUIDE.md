# Zuri — Frontend Integration Guide

**Base URL:** `http://localhost:8080` (API Gateway)
**Auth:** Bearer JWT in `Authorization` header for all protected endpoints
**Content-Type:** `application/json` unless noted otherwise (file uploads use `multipart/form-data`)

---

## Table of Contents

1. [Authentication Flow](#1-authentication-flow)
2. [User Profile](#2-user-profile)
3. [Content — Courses & Materials](#3-content--courses--materials)
4. [AI — Writing Assistant](#4-ai--writing-assistant)
5. [AI — Reading Assistant](#5-ai--reading-assistant)
6. [AI — Study Buddy](#6-ai--study-buddy)
7. [Analytics](#7-analytics)
8. [Notifications](#8-notifications)
9. [Sync & Presence](#9-sync--presence)
10. [Error Handling](#10-error-handling)
11. [Quick Start Code Examples](#11-quick-start-code-examples)

---

## 1. Authentication Flow

All protected endpoints require: `Authorization: Bearer <access_token>`

### Register

```
POST /api/v1/auth/register
```

```json
{
  "email": "student@university.edu",
  "password": "SecurePass123!",
  "first_name": "Jane",
  "last_name": "Doe"
}
```

**Response (201):**
```json
{
  "data": {
    "id": "uuid",
    "email": "student@university.edu",
    "first_name": "Jane",
    "last_name": "Doe",
    "full_name": "Jane Doe",
    "email_verified": false,
    "created_at": "2026-03-31T12:00:00Z"
  },
  "message": "User registered successfully. Please check your email for verification code."
}
```

### Login

```
POST /api/v1/auth/login
```

```json
{
  "email": "student@university.edu",
  "password": "SecurePass123!"
}
```

**Response (200):**
```json
{
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "base64-string",
    "token_type": "Bearer",
    "expires_at": "2026-03-31T12:15:00Z",
    "user": {
      "id": "uuid",
      "email": "student@university.edu",
      "first_name": "Jane",
      "last_name": "Doe",
      "full_name": "Jane Doe",
      "email_verified": false,
      "created_at": "2026-03-31T12:00:00Z"
    }
  }
}
```

**Important:**
- `access_token` expires in **15 minutes** — store it in memory (not localStorage)
- `refresh_token` expires in **30 days** — store in httpOnly cookie or secure storage
- `user.id` is needed for AI endpoints — store it after login

### Refresh Token

```
POST /api/v1/auth/refresh
```

```json
{
  "refresh_token": "base64-string"
}
```

Returns a new token pair. The old refresh token is revoked (rotation).

### Logout

```
POST /api/v1/auth/logout          (requires Bearer token)
```

### Other Auth Endpoints

| Method | Path | Body | Notes |
|--------|------|------|-------|
| POST | `/api/v1/auth/verify-email` | `{"user_id": "uuid", "code": "123456"}` | 6-digit code |
| POST | `/api/v1/auth/resend-verification` | `{"user_id": "uuid"}` | Rate limited: 1/min |
| POST | `/api/v1/auth/forgot-password` | `{"email": "..."}` | Always returns 200 |
| POST | `/api/v1/auth/reset-password` | `{"token": "...", "new_password": "..."}` | |

---

## 2. User Profile

All require Bearer token.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/users/me` | Get current user profile |
| PUT | `/api/v1/users/me` | Update profile |
| POST | `/api/v1/users/me/change-password` | Change password |
| GET | `/api/v1/users/me/sessions` | List active sessions/devices |
| DELETE | `/api/v1/users/me/sessions/:id` | Revoke a session |

### Update Profile

```json
{
  "first_name": "Jane",
  "last_name": "Doe",
  "school": "MIT",
  "department": "Computer Science",
  "academic_level": "undergraduate",
  "timezone": "America/New_York"
}
```

---

## 3. Content — Courses & Materials

All require Bearer token.

### Courses

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/courses` | List user's courses |
| POST | `/api/v1/courses` | Create a course |
| GET | `/api/v1/courses/:id` | Get course details |
| PUT | `/api/v1/courses/:id` | Update course |
| DELETE | `/api/v1/courses/:id` | Delete course |

**Create Course:**
```json
{
  "name": "Machine Learning 101",
  "description": "Intro to ML",
  "semester": "Fall",
  "year": 2026
}
```

### Materials

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/materials` | List materials |
| POST | `/api/v1/materials` | Create material metadata |
| GET | `/api/v1/materials/:id` | Get material |

### Quizzes & Flashcards (Content Service)

| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/quizzes` | List/create quizzes |
| GET | `/api/v1/quizzes/:id` | Get quiz with questions |
| GET/POST | `/api/v1/flashcard-decks` | List/create flashcard decks |
| GET | `/api/v1/flashcard-decks/:id` | Get deck with cards |

---

## 4. AI — Writing Assistant

Live lecture transcription → structured notes. All require Bearer token.

### Transcribe Audio Chunk

```
POST /api/v1/writing/transcribe
Content-Type: multipart/form-data
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| audio | File | ✅ | — | 5–15 second audio chunk (webm/wav/mp3/m4a, max 25MB) |
| session_id | string | ❌ | auto-generated | Omit on first chunk; pass returned value on subsequent chunks |
| language | string | ❌ | "en" | BCP-47 code (en, fr, es, etc.) |

**Response:** `text/event-stream` (SSE)

```
event: session
 session_id: <uuid>

The mitochondria  is the powerhouse  of the cell.
data: [DONE]
```

**Frontend pattern:**
1. Start recording → every 5–15s, stop chunk, send to this endpoint, start next chunk
2. Accumulate all `raw_text` from SSE responses on the client
3. When lecture ends → send full accumulated text to `/writing/notes`

### Generate Structured Notes

```
POST /api/v1/writing/notes
Content-Type: application/json
```

```json
{
  "session_id": "uuid-from-transcribe",
  "raw_text": "Full accumulated transcript from all chunks...",
  "subject": "Biology",
  "save": true,
  "user_id": "user-uuid-from-login"
}
```

**Response (200):**
```json
{
  "session_id": "uuid",
  "user_id": "uuid",
  "structured_notes": "## Mitochondria\n- **Mitochondria** is the **powerhouse of the cell**\n- Produces **ATP** through **oxidative phosphorylation**..."
}
```

`structured_notes` is **markdown** — render it with a markdown renderer.

### Retrieve Past Notes

```
GET /api/v1/writing/notes/:session_id
```

**Response:** Same as NotesSessionDetail with `subject`, `created_at`, `structured_notes`.

### Notes History

```
GET /api/v1/writing/history
```

**Response:**
```json
[
  {
    "session_id": "uuid",
    "subject": "Biology",
    "created_at": "2026-03-31T12:00:00Z"
  }
]
```

---

## 5. AI — Reading Assistant

Upload a document → get summary + vocabulary + TTS audio. All require Bearer token.

### Analyze Document

```
POST /api/v1/reading/analyse
Content-Type: multipart/form-data
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| file | File | ✅ | — | .pdf, .txt, or .docx |
| user_id | string | ✅ | — | User UUID from login |
| summary_type | string | ❌ | "concise" | "brief", "concise", or "detailed" |
| voice | string | ❌ | "Zephyr" | TTS voice: Zephyr, Puck, Athena, Aria, Nova |
| speaker_label | string | ❌ | "Reader" | Speaker label for TTS |
| temperature | float | ❌ | 1.0 | TTS expressiveness (0.0–1.0) |

**Response (200):**
```json
{
  "session_id": "uuid",
  "user_id": "uuid",
  "summary_type": "concise",
  "summary": "This document discusses...",
  "vocab_terms": [
    {
      "term": "Oxidative Phosphorylation",
      "definition": "The metabolic pathway in which cells use enzymes to oxidize nutrients...",
      "context_snippet": "...the inner membrane contains the electron transport chain..."
    }
  ],
  "tts_audio_b64": "UklGRi4AAABXQVZFZm10IBAAAA...",
  "audio_mime_type": "audio/wav",
  "voice": "Zephyr"
}
```

**Playing the audio:**
```javascript
const audioBytes = Uint8Array.from(atob(response.tts_audio_b64), c => c.charCodeAt(0));
const blob = new Blob([audioBytes], { type: response.audio_mime_type });
const url = URL.createObjectURL(blob);
const audio = new Audio(url);
audio.play();
```

> ⚠️ `tts_audio_b64` can be very large for long documents. Consider lazy-loading.

### Retrieve Past Reading Session

```
GET /api/v1/reading/:session_id
```

**Response:** Same structure as analyze response with `filename`, `created_at`.

---

## 6. AI — Study Buddy

Generate flashcards and quizzes from uploaded documents. All require Bearer token.

### Generate Flashcards

```
POST /api/v1/study/flashcards
Content-Type: multipart/form-data
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| file | File | ✅ | — | .pdf, .txt, or .docx |
| user_id | string | ✅ | — | User UUID |
| num_cards | int | ❌ | 10 | Number of flashcards (1–50) |

**Response (200):**
```json
{
  "session_id": "uuid",
  "user_id": "uuid",
  "filename": "biology-notes.pdf",
  "num_requested": 10,
  "num_generated": 10,
  "flashcards": [
    {
      "front": "What is the powerhouse of the cell?",
      "back": "The mitochondria — it produces ATP through oxidative phosphorylation.",
      "topic": "Cell Biology"
    }
  ]
}
```

### Retrieve Flashcard Session

```
GET /api/v1/study/flashcards/:session_id
```

### Generate Quiz

```
POST /api/v1/study/quiz
Content-Type: multipart/form-data
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| file | File | ✅ | — | .pdf, .txt, or .docx |
| user_id | string | ✅ | — | User UUID |
| quiz_type | string | ✅ | — | "multiple_choice" or "theory" |
| num_questions | int | ❌ | 5 | Number of questions (1–30) |

**Multiple Choice Response:**
```json
{
  "session_id": "uuid",
  "user_id": "uuid",
  "filename": "notes.pdf",
  "quiz_type": "multiple_choice",
  "num_requested": 5,
  "num_generated": 5,
  "questions": [
    {
      "question": "What organelle is responsible for ATP production?",
      "options": {
        "A": "Nucleus",
        "B": "Mitochondria",
        "C": "Ribosome",
        "D": "Golgi apparatus"
      },
      "correct_answer": "B",
      "explanation": "The mitochondria produces ATP through oxidative phosphorylation.",
      "topic": "Cell Biology"
    }
  ]
}
```

**Theory Response:**
```json
{
  "questions": [
    {
      "question": "Explain the process of oxidative phosphorylation.",
      "model_answer": "Oxidative phosphorylation is...",
      "marking_guide": ["Mention electron transport chain", "Describe proton gradient", "Explain ATP synthase"],
      "marks": 5,
      "topic": "Cell Biology"
    }
  ]
}
```

### Retrieve Quiz Session

```
GET /api/v1/study/quiz/:session_id
```

### Study History

```
GET /api/v1/study/history
```

**Response:**
```json
[
  {
    "session_id": "uuid",
    "session_type": "flashcard",
    "filename": "notes.pdf",
    "created_at": "2026-03-31T12:00:00Z",
    "quiz_type": null,
    "num_cards": 10,
    "num_questions": null
  },
  {
    "session_id": "uuid",
    "session_type": "quiz",
    "filename": "chapter5.pdf",
    "created_at": "2026-03-31T11:00:00Z",
    "quiz_type": "multiple_choice",
    "num_cards": null,
    "num_questions": 5
  }
]
```

---

## 7. Analytics

All require Bearer token.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/analytics/study-streak` | Current study streak (consecutive days) |
| GET | `/api/v1/analytics/study-stats` | Total study time, quizzes completed, etc. |
| GET | `/api/v1/analytics/topic-mastery` | Mastery scores per topic |
| GET | `/api/v1/analytics/quiz-history` | Past quiz attempts with scores |
| GET | `/api/v1/analytics/ai-usage` | AI token consumption stats |
| GET/POST | `/api/v1/analytics/goals` | Learning goals |
| POST | `/api/v1/analytics/goals/:id/complete` | Mark goal complete |
| POST | `/api/v1/quizzes/:id/start` | Start a quiz attempt |
| POST | `/api/v1/quiz-attempts/:id/answers` | Submit answers |
| POST | `/api/v1/quiz-attempts/:id/complete` | Complete attempt (triggers grading) |

---

## 8. Notifications

All require Bearer token.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/notifications/preferences` | Get notification preferences (auto-creates defaults) |
| PUT | `/api/v1/notifications/preferences` | Update preferences |
| POST | `/api/v1/notifications/devices/register` | Register push device token |
| DELETE | `/api/v1/notifications/devices/:token` | Unregister device |
| GET | `/api/v1/notifications/reminders` | List active reminders |
| POST | `/api/v1/notifications/reminders` | Create a reminder |
| DELETE | `/api/v1/notifications/reminders/:id` | Cancel a reminder |
| GET | `/api/v1/notifications/history` | Notification history |

### Preferences Response

```json
{
  "id": "uuid",
  "user_id": "uuid",
  "push_enabled": true,
  "email_enabled": true,
  "email_frequency": "immediate",
  "quiet_hours_start": 22,
  "quiet_hours_end": 8,
  "timezone": "UTC",
  "notify_on_quiz_completion": true,
  "notify_on_streak_achievement": true,
  "notify_on_goal_completion": true,
  "notify_on_material_processed": true,
  "notify_on_study_reminder": true
}
```

### Update Preferences (partial update)

```json
{
  "push_enabled": false,
  "quiet_hours_start": 23,
  "quiet_hours_end": 7,
  "timezone": "America/New_York"
}
```

---

## 9. Sync & Presence

All require Bearer token.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sync/state?device_id=xxx` | Get device sync state |
| POST | `/api/v1/sync/ack` | Acknowledge sync |
| GET | `/api/v1/sync/events?since=ISO8601` | Get events since timestamp |
| POST | `/api/v1/sync/events` | Create sync event |
| GET | `/api/v1/presence` | Get current user's presence |
| PUT | `/api/v1/presence` | Update presence |
| GET | `/api/v1/presence/online` | List online users |

### WebSocket

```
GET /api/v1/ws
Headers: Authorization: Bearer <token>
```

Upgrade to WebSocket for real-time updates. Messages are JSON:

```json
{
  "type": "event",
  "timestamp": "2026-03-31T12:00:00Z",
  "payload": {
    "event_type": "material.created",
    "data": { ... }
  }
}
```

**Event types:** `material.created`, `material.updated`, `quiz.completed`, `course.updated`, `progress.updated`, `streak.updated`, `goal.updated`

---

## 10. Error Handling

All errors follow this format:

```json
{
  "error": "description of what went wrong"
}
```

Or for the User/Content/Analytics services:

```json
{
  "message": "error description"
}
```

### HTTP Status Codes

| Code | Meaning | Action |
|------|---------|--------|
| 200 | Success | — |
| 201 | Created | — |
| 400 | Bad request / validation error | Check request body |
| 401 | Unauthorized / token expired | Refresh token or re-login |
| 404 | Not found | Resource doesn't exist or wrong user |
| 415 | Unsupported file type | Only .pdf, .txt, .docx accepted |
| 422 | Unprocessable (empty file, no speech, etc.) | Show user-friendly message |
| 429 | Rate limited | Wait and retry. Check `X-RateLimit-Reset` header |
| 500 | Server error | Retry or show error |
| 503 | Service unavailable (circuit breaker open) | Wait 60s and retry |

### Rate Limit Headers

Every response includes:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 2026-03-31T12:01:00Z
```

AI endpoints (`/api/v1/ai/*`, `/api/v1/writing/*`, `/api/v1/reading/*`, `/api/v1/study/*`) have a stricter limit of **20 requests/minute**.

---

## 11. Quick Start Code Examples

### React/Next.js — Auth Hook

```typescript
// lib/api.ts
const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export async function apiRequest(path: string, options: RequestInit = {}) {
  const token = sessionStorage.getItem('access_token');
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      ...options.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });

  if (res.status === 401) {
    // Try refresh
    const refreshed = await refreshToken();
    if (refreshed) return apiRequest(path, options);
    // Redirect to login
    window.location.href = '/login';
  }

  return res;
}

export async function login(email: string, password: string) {
  const res = await fetch(`${API_BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  if (res.ok) {
    sessionStorage.setItem('access_token', data.data.access_token);
    sessionStorage.setItem('refresh_token', data.data.refresh_token);
    sessionStorage.setItem('user_id', data.data.user.id);
  }
  return data;
}
```

### Upload File for Flashcards

```typescript
export async function generateFlashcards(file: File, numCards = 10) {
  const userId = sessionStorage.getItem('user_id');
  const form = new FormData();
  form.append('file', file);
  form.append('user_id', userId!);
  form.append('num_cards', String(numCards));

  const res = await apiRequest('/api/v1/study/flashcards', {
    method: 'POST',
    body: form,
    // Do NOT set Content-Type — browser sets it with boundary
  });
  return res.json();
}
```

### SSE Transcription Stream

```typescript
export async function transcribeChunk(
  audioBlob: Blob,
  sessionId?: string,
  language = 'en'
) {
  const userId = sessionStorage.getItem('user_id');
  const token = sessionStorage.getItem('access_token');
  const form = new FormData();
  form.append('audio', audioBlob, 'chunk.webm');
  if (sessionId) form.append('session_id', sessionId);
  form.append('language', language);

  const res = await fetch('http://localhost:8080/api/v1/writing/transcribe', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });

  // Read SSE stream
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let text = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    text += decoder.decode(value, { stream: true });
    // Update UI with accumulated text
  }

  return text;
}
```

### Play TTS Audio from Reading Assistant

```typescript
export function playAudio(base64Audio: string, mimeType = 'audio/wav') {
  const bytes = Uint8Array.from(atob(base64Audio), c => c.charCodeAt(0));
  const blob = new Blob([bytes], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const audio = new Audio(url);
  audio.play();
  audio.onended = () => URL.revokeObjectURL(url);
}
```

---

## Key Notes for Frontend Dev

1. **Store `user_id` after login** — the AI endpoints need it in request bodies
2. **File uploads** — do NOT set `Content-Type` header manually; let the browser set it with the multipart boundary
3. **SSE streaming** — `/writing/transcribe` returns a stream, not JSON. Use `ReadableStream` or `EventSource`
4. **Audio responses** — `tts_audio_b64` is base64-encoded WAV. Decode and play client-side
5. **Session IDs** — every AI action returns a `session_id`. Store these to enable history/retrieval
6. **Token refresh** — access tokens expire in 15 minutes. Implement automatic refresh
7. **Rate limits** — AI endpoints are limited to 20 req/min. Show a "please wait" UI when `429` is returned
8. **Markdown rendering** — `structured_notes` and `summary` fields contain markdown. Use a renderer like `react-markdown`
