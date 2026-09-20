# Zuri Mobile — Screen Developer Guide

> **One-page cheat sheet for mobile developers building screens against the Zuri backend.**

**Staging Backend:** `https://staging.zuri.app`  
**WebSocket:** `wss://staging.zuri.app/api/v1/ws`  
**Auth:** RS256 JWT (access token = 15 min, refresh token = 30 days)

---

## 1. Quick Start

### Environment
```bash
# .env
cp .env.example .env
```

```env
API_BASE_URL=https://staging.zuri.app
WS_URL=wss://staging.zuri.app/api/v1/ws
```

### Required Headers on Every Request
```typescript
const headers = {
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${accessToken}`,
};
```

---

## 2. Auth Flow (All Screens)

### Register → `POST /api/v1/auth/register`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/register`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: 'user@example.com',
    password: 'SecurePass123!',
    first_name: 'John',
    last_name: 'Doe',
  }),
});

// Response: { data: { id, email, first_name, last_name, email_verified: false } }
```

### Login → `POST /api/v1/auth/login`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password }),
});

// SUCCESS (200): { data: { access_token, refresh_token, expires_at, user } }

// EMAIL NOT VERIFIED (403):
// {
//   "error": "Email verification required",
//   "code": "EMAIL_NOT_VERIFIED",
//   "message": "Please verify your email before logging in.",
//   "user_id": "uuid-here"
// }
```

**Screen behavior:**
- If `200` → save tokens, go to Home
- If `403` + `code === "EMAIL_NOT_VERIFIED"` → go to **Verify Email** screen, pass `user_id`

### Verify Email → `POST /api/v1/auth/verify-email`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/verify-email`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ user_id: userId, code: '123456' }),
});
```

### Resend Code → `POST /api/v1/auth/resend-verification`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/resend-verification`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ user_id: userId }),
});
```

### Forgot Password → `POST /api/v1/auth/forgot-password`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/forgot-password`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email }),
});
```

### Reset Password → `POST /api/v1/auth/reset-password`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/reset-password`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ token: resetToken, password: newPassword }),
});
```

### Refresh Token → `POST /api/v1/auth/refresh`
```typescript
const res = await fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ refresh_token: refreshToken }),
});

// Response: { data: { access_token, refresh_token, expires_at } }
```

**Auto-refresh pattern:**
- Check token expiry 5 minutes before expiration
- If 401 received on any request, try refresh once, then retry the original request
- If refresh fails, log user out

---

## 3. Screen-by-Screen API Reference

### 🏠 Home / Dashboard Screen

| Data | Method | Endpoint | Response Shape |
|------|--------|----------|----------------|
| Study streak | GET | `/api/v1/analytics/study-streak` | `{ data: { streak_days, longest_streak } }` |
| Study stats | GET | `/api/v1/analytics/study-stats` | `{ data: { total_sessions, total_minutes, quizzes_completed, average_score } }` |
| Topic mastery | GET | `/api/v1/analytics/topic-mastery` | `{ data: [ { topic, mastery_level, last_studied } ] }` |
| Recent courses | GET | `/api/v1/courses?limit=5&offset=0` | `{ data: [ { id, title, description, created_at } ] }` |
| AI usage | GET | `/api/v1/analytics/ai-usage?days=30` | `{ data: { total_requests, tokens_used, quota_limit } }` |

### 📚 Courses Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| List courses | GET | `/api/v1/courses?limit=20&offset=0` | — |
| Create course | POST | `/api/v1/courses` | `{ title, description }` |
| Get course | GET | `/api/v1/courses/:id` | — |
| Update course | PUT | `/api/v1/courses/:id` | `{ title?, description? }` |
| Delete course | DELETE | `/api/v1/courses/:id` | — |
| Get materials | GET | `/api/v1/courses/:id/materials` | — |

### 📄 Materials Screen

| Action | Method | Endpoint | Notes |
|--------|--------|----------|-------|
| List materials | GET | `/api/v1/materials?limit=20&offset=0` | — |
| Upload material | POST | `/api/v1/materials` | Returns presigned URL |
| Get material | GET | `/api/v1/materials/:id` | — |
| Delete material | DELETE | `/api/v1/materials/:id` | — |

**Upload flow (2-step):**
```typescript
// Step 1: Get presigned URL
const presign = await fetch(`${API_BASE_URL}/api/v1/materials`, {
  method: 'POST',
  headers: { ...authHeaders, 'Content-Type': 'application/json' },
  body: JSON.stringify({ title, content_type: 'application/pdf', file_size: 1024000 }),
});
const { presigned_url, material_id, public_url } = await presign.json();

// Step 2: Upload file to presigned_url (PUT request, no auth needed)
await fetch(presigned_url, {
  method: 'PUT',
  headers: { 'Content-Type': 'application/pdf' },
  body: fileBlob,
});
```

### 🎯 Quizzes Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| List quizzes | GET | `/api/v1/quizzes?limit=20&offset=0` | — |
| Get quiz | GET | `/api/v1/quizzes/:id` | — |
| Start quiz | POST | `/api/v1/quizzes/:id/start` | — |
| Submit answer | POST | `/api/v1/quiz-attempts/:attemptId/answers` | `{ question_id, selected_answer }` |
| Complete quiz | POST | `/api/v1/quiz-attempts/:attemptId/complete` | — |
| Quiz history | GET | `/api/v1/analytics/quiz-history?limit=20&offset=0` | — |
| Generate AI quiz | POST | `/api/v1/ai/generate/quiz` | `{ query, user_id, quiz_type, num_questions }` |

### 🗂️ Flashcards Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| List decks | GET | `/api/v1/flashcard-decks?limit=20&offset=0` | — |
| Get deck | GET | `/api/v1/flashcard-decks/:id` | — |
| Create deck | POST | `/api/v1/flashcard-decks` | `{ title, description }` |
| Generate AI flashcards | POST | `/api/v1/ai/generate/flashcards` | `{ query, user_id, num_cards }` |
| Get study history | GET | `/api/v1/study/history?limit=20&offset=0` | — |
| Get flashcard session | GET | `/api/v1/study/flashcards/:sessionId` | — |

### 💬 AI Chat Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| Send message | POST | `/api/v1/ai/chat` | `{ query, user_id, conversation_id?, material_id?, context_chunks? }` |
| Stream chat | POST | `/api/v1/ai/chat/stream` | Same body, returns SSE stream |
| Get conversation | GET | `/api/v1/ai/conversation/:id` | — |
| Delete conversation | DELETE | `/api/v1/ai/conversation/:id` | — |
| Generate summary | POST | `/api/v1/ai/generate/summary` | `{ query, user_id, material_id? }` |

**SSE Stream parsing:**
```typescript
const response = await fetch(`${API_BASE_URL}/api/v1/ai/chat/stream`, {
  method: 'POST',
  headers: { ...authHeaders, 'Content-Type': 'application/json' },
  body: JSON.stringify({ query, user_id }),
});

const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = '';

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  buffer += decoder.decode(value, { stream: true });
  
  // Parse SSE events: "event: token\ndata: {\"token\": \"hello\"}\n\n"
  const events = buffer.split('\n\n');
  buffer = events.pop() || '';
  
  for (const event of events) {
    if (event.startsWith('data: ')) {
      const data = JSON.parse(event.slice(6));
      if (data.token) onToken(data.token);
      if (data.complete) onComplete(data);
    }
  }
}
```

### 📖 Reading Assistant Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| Extract text | POST | `/api/v1/reading/extract` | `FormData` with file |
| Analyse document | POST | `/api/v1/reading/analyse` | `FormData` with file, user_id, summary_type, voice |
| Async analyse | POST | `/api/v1/reading/analyse/async` | Same as above |
| Check status | GET | `/api/v1/reading/analyse/status/:jobId` | — |
| Get session | GET | `/api/v1/reading/:sessionId?user_id=xxx` | — |
| Text-to-speech | POST | `/api/v1/ai/text-to-speech` | `{ text, language }` |

### 🎙️ Writing Assistant Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| Transcribe audio | POST | `/api/v1/ai/speech-to-text` | `FormData` with audio file |
| Generate notes | POST | `/api/v1/writing/notes` | `{ transcription, user_id }` |
| Transcribe (gateway) | POST | `/api/v1/writing/transcribe` | `FormData` with audio file |
| Get history | GET | `/api/v1/writing/history` | — |
| Get session | GET | `/api/v1/writing/notes/:id` | — |

### 🎯 Goals Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| List goals | GET | `/api/v1/analytics/goals` | — |
| Create goal | POST | `/api/v1/analytics/goals` | `{ title, description, target_date, category }` |
| Complete goal | POST | `/api/v1/analytics/goals/:id/complete` | — |
| Record study session | POST | `/api/v1/analytics/study-sessions` | `{ duration_minutes, topic, material_id? }` |

### 👤 Profile / Settings Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| Get profile | GET | `/api/v1/users/me` | — |
| Update profile | PUT | `/api/v1/users/me/profile` | `{ first_name?, last_name?, timezone? }` |
| Get settings | GET | `/api/v1/users/me/settings` | — |
| Update settings | PUT | `/api/v1/users/me/settings` | `{ theme?, notifications_enabled?, language? }` |
| List sessions | GET | `/api/v1/users/me/sessions` | — |
| Revoke session | DELETE | `/api/v1/users/me/sessions/:sessionId` | — |
| Logout all | POST | `/api/v1/auth/logout-all` | — |
| Logout | POST | `/api/v1/auth/logout` | — |

### 🔔 Notifications Screen

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| Register FCM token | POST | `/api/v1/notifications/devices/register` | `{ token, platform: 'ios' \| 'android' }` |
| Unregister token | DELETE | `/api/v1/notifications/devices/:token` | — |
| Get preferences | GET | `/api/v1/notifications/preferences` | — |
| Update preferences | PUT | `/api/v1/notifications/preferences` | `{ email_enabled?, push_enabled?, quiet_hours_start?, quiet_hours_end? }` |

---

## 4. WebSocket (Real-time)

**Connection:**
```typescript
const ws = new WebSocket(`wss://staging.zuri.app/api/v1/ws?token=${accessToken}`);

ws.onopen = () => {
  ws.send(JSON.stringify({ type: 'ping' }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  // msg.type: 'quiz_completed' | 'goal_progress' | 'material_uploaded' | 'sync' | 'connection'
};
```

**Presence update:**
```typescript
ws.send(JSON.stringify({
  type: 'presence',
  status: 'online', // 'online' | 'away' | 'offline'
  current_activity: 'studying_quiz',
}));
```

---

## 5. Error Handling Patterns

### Common Response Codes

| Status | Meaning | Screen Action |
|--------|---------|---------------|
| `200` | Success | Continue |
| `201` | Created | Show success, navigate |
| `400` | Bad Request | Show validation errors |
| `401` | Unauthorized | Try refresh token, else logout |
| `403` + `EMAIL_NOT_VERIFIED` | Email not verified | Navigate to Verify Email |
| `404` | Not Found | Show "not found" state |
| `429` | Rate Limited | Show "too many requests, wait" |
| `500` | Server Error | Show "something went wrong", retry |

### Error Response Shape
```typescript
interface ApiError {
  error: string;      // Short code
  message: string;    // Human readable
  code?: string;      // e.g. "EMAIL_NOT_VERIFIED"
  user_id?: string;   // Present for email verification
}
```

### Request Wrapper
```typescript
async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = await getAccessToken();
  
  const res = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.headers || {}),
    },
  });
  
  if (res.status === 401) {
    const refreshed = await refreshToken();
    if (refreshed) return apiRequest(endpoint, options);
    throw new AuthError('Session expired');
  }
  
  if (!res.ok) {
    const err = await res.json();
    throw new ApiError(err.code || err.message, res.status, err);
  }
  
  return res.json();
}
```

---

## 6. Testing Against Staging

### Health Check
```bash
curl https://staging.zuri.app/health
```

### Quick Auth Test
```bash
# Register
curl -X POST https://staging.zuri.app/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@mobile.dev","password":"Test123!","first_name":"Mobile","last_name":"Dev"}'

# Login (will fail with EMAIL_NOT_VERIFIED until you verify)
curl -X POST https://staging.zuri.app/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@mobile.dev","password":"Test123!"}'
```

### With Token
```bash
# Get courses
curl https://staging.zuri.app/api/v1/courses \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

---

## 7. Mobile-Specific Notes

### Android Emulator
```typescript
const API_BASE_URL = __DEV__ ? 'http://10.0.2.2:8080' : 'https://staging.zuri.app';
```

### iOS Simulator
```typescript
const API_BASE_URL = __DEV__ ? 'http://localhost:8080' : 'https://staging.zuri.app';
```

### Physical Device
Use your machine's LAN IP (`192.168.1.xxx:8080`) or ngrok.

### Token Storage
- **Access/Refresh tokens:** Use Keychain (iOS) / Keystore (Android) — never AsyncStorage
- **User profile:** AsyncStorage is fine

### File Uploads
- PDFs: Use document picker → get presigned URL → PUT to MinIO
- Audio: Record → base64 or multipart → POST to `/api/v1/ai/speech-to-text` or `/api/v1/writing/transcribe`

---

## 8. Screen Checklist

Before marking a screen "done", verify:

- [ ] Handles `401` by attempting token refresh
- [ ] Handles `403 EMAIL_NOT_VERIFIED` by routing to Verify Email
- [ ] Shows loading state while fetching
- [ ] Shows empty state when no data
- [ ] Shows error state with retry button
- [ ] Works on slow network (test with network throttling)
- [ ] Works offline where applicable (cached data)

---

*Last updated: 2026-05-20*  
*Backend version: staging @ https://staging.zuri.app (all services healthy)*
