# Zuri API Documentation

Base URL: `http://localhost:8080` (all requests go through the API Gateway)

All protected endpoints require: `Authorization: Bearer <access_token>`

---

## Gateway

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | No | Health check with upstream service status |

**Response:**
```json
{
  "service": "gateway",
  "status": "healthy",
  "upstream": {
    "user": "healthy",
    "content": "healthy",
    "analytics": "healthy",
    "notification": "healthy",
    "sync": "healthy"
  }
}
```

---

## User Service (Port 8081)

### Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | No | Register new user |
| POST | `/api/v1/auth/login` | No | Login, returns JWT |
| POST | `/api/v1/auth/refresh` | No | Refresh access token |
| POST | `/api/v1/auth/verify-email` | No | Verify email with 6-digit code |
| POST | `/api/v1/auth/resend-verification` | Yes | Resend verification email |
| POST | `/api/v1/auth/forgot-password` | No | Request password reset |
| POST | `/api/v1/auth/reset-password` | No | Reset password with token |
| GET | `/api/v1/auth/public-key` | No | Get JWT RS256 public key |
| POST | `/api/v1/auth/logout` | Yes | Logout current session |
| POST | `/api/v1/auth/logout-all` | Yes | Logout all devices |

#### POST /api/v1/auth/register
```json
// Request
{
  "email": "user@example.com",       // required, valid email
  "password": "SecurePass123!",      // required, min 8 chars
  "first_name": "John",
  "last_name": "Doe",
  "school": "MIT",
  "department": "Computer Science",
  "academic_level": "undergraduate"  // undergraduate|postgraduate|doctoral|staff
}

// Response 201
{
  "message": "User registered successfully. Please check your email for verification code.",
  "data": {
    "id": "uuid",
    "email": "user@example.com",
    "first_name": "John",
    "last_name": "Doe",
    "full_name": "John Doe",
    "timezone": "UTC",
    "email_verified": false,
    "created_at": "2026-03-26T11:00:00Z"
  }
}
```

#### POST /api/v1/auth/login
```json
// Request
{
  "email": "user@example.com",    // required
  "password": "SecurePass123!"    // required
}

// Response 200
{
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "base64-token",
    "token_type": "Bearer",
    "expires_at": "2026-03-26T11:15:00Z",
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "first_name": "John",
      "last_name": "Doe",
      "full_name": "John Doe",
      "timezone": "UTC",
      "email_verified": false,
      "created_at": "2026-03-26T11:00:00Z"
    }
  }
}
```

#### POST /api/v1/auth/refresh
```json
// Request
{ "refresh_token": "base64-token" }

// Response 200 — same as login response
```

#### POST /api/v1/auth/verify-email
```json
// Request (query param: ?user_id=uuid)
{ "code": "123456" }

// Response 200
{ "message": "Email verified successfully" }
```

#### POST /api/v1/auth/forgot-password
```json
// Request
{ "email": "user@example.com" }

// Response 200 (always, to prevent enumeration)
{ "message": "If the email exists, a password reset link has been sent" }
```

#### POST /api/v1/auth/reset-password
```json
// Request
{ "token": "reset-token", "new_password": "NewPass123!" }

// Response 200
{ "message": "Password reset successfully" }
```

#### GET /api/v1/auth/public-key
```json
// Response 200
{
  "data": {
    "public_key": "-----BEGIN PUBLIC KEY-----\n...",
    "algorithm": "RS256"
  }
}
```

### User Profile

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/users/me` | Yes | Get current user profile |
| PUT | `/api/v1/users/me` | Yes | Update profile |
| POST | `/api/v1/users/me/change-password` | Yes | Change password |

#### PUT /api/v1/users/me
```json
// Request (all fields optional)
{
  "first_name": "John",
  "last_name": "Doe",
  "school": "MIT",
  "department": "CS",
  "academic_level": "postgraduate",
  "timezone": "America/New_York"
}
```

#### POST /api/v1/users/me/change-password
```json
// Request
{
  "current_password": "OldPass123!",
  "new_password": "NewPass456!"       // min 8 chars
}
```

### Sessions

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/users/me/sessions` | Yes | List active sessions |
| DELETE | `/api/v1/users/me/sessions/:id` | Yes | Revoke a session |

---

## Content Service (Port 8082)

### Courses

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/courses` | Yes | List user's courses |
| POST | `/api/v1/courses` | Yes | Create course |
| GET | `/api/v1/courses/:id` | Yes | Get course by ID |
| PUT | `/api/v1/courses/:id` | Yes | Update course |
| DELETE | `/api/v1/courses/:id` | Yes | Delete course |
| GET | `/api/v1/courses/:id/materials` | Yes | Get course materials |

#### POST /api/v1/courses
```json
// Request
{
  "name": "Machine Learning 101",    // required
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
    "description": "Intro to ML",
    "color": "#3B82F6",
    "semester": "Fall",
    "year": 2026,
    "created_at": "2026-03-26T11:00:00Z",
    "updated_at": "2026-03-26T11:00:00Z"
  }
}
```

### Materials

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/materials` | Yes | List materials (?limit=20&offset=0) |
| POST | `/api/v1/materials` | Yes | Create material |
| GET | `/api/v1/materials/:id` | Yes | Get material by ID |
| PUT | `/api/v1/materials/:id` | Yes | Update material |
| DELETE | `/api/v1/materials/:id` | Yes | Delete material |
| POST | `/api/v1/materials/:id/presign` | Yes | Get presigned upload URL |

#### POST /api/v1/materials
```json
// Request
{
  "title": "My Notes",              // required
  "description": "Study notes",
  "content_type": "pdf",
  "file_size": 10000,
  "course_id": "uuid"               // optional
}

// Response 201
{
  "data": {
    "id": "uuid",
    "user_id": "uuid",
    "title": "My Notes",
    "file_size": 10000,
    "processing_status": "pending",
    "created_at": "2026-03-26T11:00:00Z",
    "updated_at": "2026-03-26T11:00:00Z"
  }
}
```

### Quizzes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/quizzes` | Yes | List user's quizzes |
| POST | `/api/v1/quizzes` | Yes | Create quiz |
| GET | `/api/v1/quizzes/:id` | Yes | Get quiz with questions |
| PUT | `/api/v1/quizzes/:id` | Yes | Update quiz |
| DELETE | `/api/v1/quizzes/:id` | Yes | Delete quiz |

#### POST /api/v1/quizzes
```json
// Request
{
  "title": "ML Quiz 1",             // required
  "description": "Test your knowledge",
  "course_id": "uuid",
  "time_limit_minutes": 15,
  "difficulty": "medium",
  "questions": [
    {
      "question_text": "What is supervised learning?",
      "question_type": "multiple_choice",
      "options": [
        {"text": "Learning with labels", "is_correct": true},
        {"text": "Learning without labels", "is_correct": false}
      ],
      "correct_answer": "Learning with labels",
      "explanation": "Supervised learning uses labeled data",
      "points": 10
    }
  ]
}
```

### Flashcard Decks

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/flashcard-decks` | Yes | List decks |
| POST | `/api/v1/flashcard-decks` | Yes | Create deck |
| GET | `/api/v1/flashcard-decks/:id` | Yes | Get deck with cards |
| PUT | `/api/v1/flashcard-decks/:id` | Yes | Update deck |
| DELETE | `/api/v1/flashcard-decks/:id` | Yes | Delete deck |
| POST | `/api/v1/flashcard-decks/:id/cards` | Yes | Add card to deck |

---

## Analytics Service (Port 8083)

### Quiz Attempts

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/quizzes/:id/start` | Yes | Start quiz attempt |
| GET | `/api/v1/quiz-attempts` | Yes | List attempts (?limit=20&offset=0) |
| GET | `/api/v1/quiz-attempts/:id` | Yes | Get attempt details |
| POST | `/api/v1/quiz-attempts/:id/answers` | Yes | Submit answer |
| POST | `/api/v1/quiz-attempts/:id/complete` | Yes | Complete attempt (auto-grades) |

#### POST /api/v1/quiz-attempts/:id/answers
```json
// Request
{
  "question_id": "uuid",
  "answer": "Learning with labels",
  "time_taken_seconds": 30
}
```

### Study Analytics

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/analytics/study-streak` | Yes | Current streak (consecutive days) |
| GET | `/api/v1/analytics/study-stats` | Yes | Overall study statistics |
| POST | `/api/v1/analytics/study-sessions` | Yes | Record study session |
| GET | `/api/v1/analytics/topic-mastery` | Yes | Topic mastery scores |
| GET | `/api/v1/analytics/topics-for-review` | Yes | Topics due for spaced repetition |

#### GET /api/v1/analytics/study-stats
```json
// Response 200
{
  "data": {
    "current_streak": 5,
    "total_study_days": 30,
    "total_study_minutes": 1200,
    "total_quizzes_completed": 15,
    "total_materials_reviewed": 8,
    "last_study_date": "2026-03-26"
  }
}
```

#### POST /api/v1/analytics/study-sessions
```json
// Request
{
  "session_date": "2026-03-26",
  "duration_minutes": 45,
  "quizzes_completed": 2,
  "materials_reviewed": 1
}
```

### Learning Goals

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/analytics/goals` | Yes | List goals (?include_completed=true) |
| POST | `/api/v1/analytics/goals` | Yes | Create goal |
| POST | `/api/v1/analytics/goals/:id/complete` | Yes | Mark goal complete |

#### POST /api/v1/analytics/goals
```json
// Request
{
  "title": "Complete ML course",
  "description": "Finish all quizzes",
  "target_date": "2026-04-30",
  "goal_type": "course_completion",
  "course_id": "uuid"
}
```

### AI Usage

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/analytics/ai-usage` | Yes | Token usage stats (?days=30) |
| POST | `/api/v1/analytics/ai-interactions` | Yes | Track AI interaction |

#### POST /api/v1/analytics/ai-interactions
```json
// Request
{
  "interaction_type": "chat",
  "prompt_tokens": 150,
  "completion_tokens": 200,
  "total_tokens": 350,
  "model": "gemini-pro"
}
```

---

## AI Orchestrator (Port 5005)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/ai/chat` | Yes | Chat with Gemini AI |
| POST | `/api/v1/ai/generate/quiz` | Yes | Generate quiz from text |
| POST | `/api/v1/ai/generate/summary` | Yes | Generate summary |
| POST | `/api/v1/ai/generate/flashcards` | Yes | Generate flashcards |
| GET | `/api/v1/ai/conversation/:id` | Yes | Get conversation history |
| DELETE | `/api/v1/ai/conversation/:id` | Yes | Clear conversation |

#### POST /api/v1/ai/chat
```json
// Request
{
  "query": "What is machine learning?",
  "user_id": "uuid",
  "context_chunks": ["chunk1 text", "chunk2 text"],
  "material_id": "uuid",              // optional
  "conversation_id": "uuid"            // optional, for follow-ups
}

// Response 200
{
  "response": "Machine learning is...",
  "conversation_id": "uuid",
  "tokens_used": 135,
  "model": "gemini-pro",
  "sources": ["chunk_0", "chunk_1"]
}
```

#### POST /api/v1/ai/generate/quiz
```json
// Request — same as chat
// Response
{ "quiz": "Generated quiz text...", "type": "quiz" }
```

#### POST /api/v1/ai/generate/summary
```json
// Request — same as chat
// Response
{ "summary": "Generated summary...", "type": "summary" }
```

#### POST /api/v1/ai/generate/flashcards
```json
// Request — same as chat
// Response
{ "flashcards": "Generated flashcards...", "type": "flashcards" }
```

---

## Retrieval Service (Port 5003)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/ai/retrieve` | Yes | Vector search (RAG context) |

#### POST /api/v1/ai/retrieve
```json
// Request
{
  "query": "neural networks",
  "user_id": "uuid",
  "material_id": "uuid",    // optional filter
  "top_k": 5                // max 10
}

// Response 200
{
  "query": "neural networks",
  "query_embedding_preview": [-0.069, -0.038, 0.056, 0.001, -0.030],
  "results": [
    {
      "chunk_id": "uuid",
      "material_id": "uuid",
      "chunk_text": "Neural networks are...",
      "similarity_score": 0.95,
      "chunk_index": 0
    }
  ],
  "cached": false,
  "note": "Search returned 3 chunks."
}
```

---

## Audio Service (Port 5004)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/ai/speech-to-text` | Yes | Transcribe audio file |
| GET | `/api/v1/ai/languages` | Yes | List supported languages |

#### POST /api/v1/ai/speech-to-text
```
Content-Type: multipart/form-data

Fields:
  audio: <file>          (MP3, WAV, M4A, OGG, FLAC, AAC, MP4, WEBM)
  language: "en-US"      (optional, default: en-US)
```
```json
// Response 200
{
  "text": "Transcribed text here",
  "confidence": 0.95,
  "language": "en-US",
  "original_format": ".mp3"
}
```

#### GET /api/v1/ai/languages
```json
// Response 200
{
  "supported_languages": {
    "en-US": "English (US)",
    "en-GB": "English (UK)",
    "es-ES": "Spanish",
    "fr-FR": "French",
    "de-DE": "German",
    "it-IT": "Italian",
    "pt-BR": "Portuguese (Brazil)",
    "ja-JP": "Japanese",
    "zh-CN": "Chinese (Simplified)",
    "ko-KR": "Korean",
    "ar-SA": "Arabic",
    "hi-IN": "Hindi",
    "ru-RU": "Russian",
    "auto": "Auto-detect"
  }
}
```

---

## Ingestion Service (Port 5002)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/process` | Internal | Process PDF document |
| GET | `/task/:id` | Internal | Get task status |

#### POST /process
```json
// Request
{
  "material_id": "uuid",
  "user_id": "uuid",
  "file_url": "s3://bucket/file.pdf"
}

// Response 200
{
  "task_id": "uuid",
  "status": "completed",
  "message": "Document processed successfully! Created 12 chunks.",
  "chunks_created": 12,
  "storage_method": "json_fallback"
}
```

---

## Evaluation Service (Port 5006)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/grade-quiz` | Internal | Auto-grade quiz submission |
| POST | `/log-interaction` | Internal | Log AI usage metrics |
| POST | `/feedback` | Internal | Submit user feedback |
| GET | `/analytics/:user_id` | Internal | User analytics |
| GET | `/analytics/system/summary` | Internal | System-wide analytics |

#### POST /grade-quiz
```json
// Request
{
  "quiz_id": "uuid",
  "user_id": "uuid",
  "answers": {"q1-uuid": "answer1", "q2-uuid": "answer2"},
  "time_taken_seconds": 300
}

// Response 200
{
  "attempt_id": "uuid",
  "quiz_id": "uuid",
  "user_id": "uuid",
  "score": 80.0,
  "correct_answers": {},
  "feedback": {}
}
```

#### POST /feedback
```json
// Request
{
  "user_id": "uuid",
  "rating": 5,                    // 1-5
  "comment": "Very helpful!",
  "feature_type": "chat_response",
  "interaction_id": "uuid"
}
```

---

## Notification Service (Port 8084)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/notifications/preferences` | Yes | Get notification preferences |
| PUT | `/api/v1/notifications/preferences` | Yes | Update preferences |
| POST | `/api/v1/notifications/devices/register` | Yes | Register push device |
| DELETE | `/api/v1/notifications/devices/:token` | Yes | Unregister device |
| GET | `/api/v1/notifications/reminders` | Yes | List active reminders |
| POST | `/api/v1/notifications/reminders` | Yes | Create reminder |
| DELETE | `/api/v1/notifications/reminders/:id` | Yes | Cancel reminder |
| GET | `/api/v1/notifications/history` | Yes | Notification history (?limit=50) |

#### PUT /api/v1/notifications/preferences
```json
// Request (all fields optional)
{
  "push_enabled": true,
  "email_enabled": true,
  "email_frequency": "immediate",          // immediate|daily_digest|weekly_digest
  "quiet_hours_start": 22,                 // 10 PM
  "quiet_hours_end": 8,                    // 8 AM
  "timezone": "UTC",
  "notify_on_quiz_completion": true,
  "notify_on_streak_achievement": true,
  "notify_on_goal_completion": true,
  "notify_on_material_processed": true,
  "notify_on_study_reminder": true
}
```

#### POST /api/v1/notifications/reminders
```json
// Request
{
  "type": "study_reminder",
  "title": "Time to study!",
  "body": "Review your ML notes",
  "scheduled_for": "2026-03-27T09:00:00Z",
  "recurrence": "daily",                   // daily|weekly|none
  "recurrence_end_date": "2026-04-30",
  "entity_type": "course",
  "entity_id": "uuid"
}
```

---

## Sync Service (Port 8085)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/presence` | Yes | Get current user presence |
| PUT | `/api/v1/presence` | Yes | Update presence status |
| GET | `/api/v1/presence/online` | Yes | List online users |
| GET | `/api/v1/sync/state` | Yes | Device sync state (?device_id=xxx) |
| POST | `/api/v1/sync/ack` | Yes | Acknowledge sync |
| GET | `/api/v1/sync/events` | Yes | Get events (?since=RFC3339&limit=50) |
| POST | `/api/v1/sync/events` | Yes | Create sync event |
| GET | `/api/v1/ws` | Yes | WebSocket connection |

#### PUT /api/v1/presence
```json
// Request
{
  "status": "online",                // online|away|offline|busy
  "status_message": "Studying ML",
  "activity_type": "viewing_course",
  "activity_data": {"course_id": "uuid"}
}
```

#### POST /api/v1/sync/ack
```json
// Request
{
  "device_id": "device-123",
  "last_event_id": "uuid",
  "sync_cursor": "cursor-value"
}
```

#### POST /api/v1/sync/events
```json
// Request
{
  "event_type": "material.created",
  "event_name": "New material uploaded",
  "course_id": "uuid",
  "payload": {"material_id": "uuid", "title": "Notes"}
}
```

---

## Error Responses

All services return errors in this format:

```json
// 400 Bad Request
{ "message": "invalid request body" }

// 401 Unauthorized
{ "message": "invalid token" }

// 403 Forbidden
{ "message": "access denied" }

// 404 Not Found
{ "message": "course not found" }

// 409 Conflict
{ "message": "email already registered" }

// 429 Too Many Requests
{ "message": "rate limit exceeded" }

// 500 Internal Server Error
{ "message": "internal server error" }

// 503 Service Unavailable (circuit breaker open)
{ "message": "service temporarily unavailable" }
```

---

## Rate Limits

| Endpoint Type | Limit |
|---------------|-------|
| Normal endpoints | 100 requests/minute |
| AI endpoints (`/api/v1/ai/*`) | 20 requests/minute |

Rate limit headers returned on every response:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 2026-03-26T12:00:00Z
```
