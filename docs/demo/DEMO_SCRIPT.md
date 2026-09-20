# Zuri Demo Script — Full Walkthrough

**Time: ~15 minutes | Audience: Boss/Stakeholder**

---

## 10 Minutes Before Demo

Open CMD:
```batch
cd C:\Users\PC\Downloads\zuri\infra
docker-compose up -d
```

Wait 60 seconds. Verify:
```batch
curl http://localhost:8080/health
```

All 5 upstream services must show `"healthy"`. If yes, you're ready.

---

## Part 1: Architecture Overview (2 min — just talk)

> "Zuri is a fully containerized microservices platform for AI-powered learning. Let me show you what's running."

```batch
docker-compose ps
```

> "14 containers — 6 Go backend services, 5 Python AI services, plus PostgreSQL, Redis, and MinIO object storage. Everything starts with one command: `docker-compose up -d`."

> "The architecture is: Client → API Gateway → Microservices → Database. The Gateway handles JWT authentication, rate limiting at 100 requests/minute for normal endpoints and 20/minute for AI, plus a circuit breaker that protects against cascading failures if the AI service goes down."

```batch
curl http://localhost:8080/health
```

> "The Gateway probes every upstream service. All healthy."

---

## Part 2: User Registration & Auth (3 min)

> "Let me walk through the full user lifecycle."

**Register:**
```batch
curl -X POST http://localhost:8080/api/v1/auth/register -H "Content-Type: application/json" -d "{\"email\":\"boss-demo@example.com\",\"password\":\"DemoPass123!\",\"first_name\":\"John\",\"last_name\":\"Smith\",\"school\":\"MIT\",\"department\":\"Computer Science\",\"academic_level\":\"postgraduate\"}"
```

> "User created. Password is bcrypt-hashed with cost 12. A 6-digit verification code is generated and stored — in production this gets emailed via our Notification Service."

**Get the verification code** (show the database has it):
```batch
docker exec zuri-postgres psql -U zuri -d zuri -c "SELECT email, verification_code FROM auth.users WHERE email='boss-demo@example.com';"
```

> "Here's the verification code in the database. In production, the user receives this by email. Let me verify it."

**Verify email** (replace CODE with the actual code from above, and USER_ID from the register response):
```batch
curl -X POST "http://localhost:8080/api/v1/auth/verify-email?user_id=USER_ID_HERE" -H "Content-Type: application/json" -d "{\"code\":\"CODE_HERE\"}"
```

**Login:**
```batch
curl -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"email\":\"boss-demo@example.com\",\"password\":\"DemoPass123!\"}"
```

> "We get an RS256-signed JWT access token — 15 minute TTL — plus a refresh token valid for 30 days. The RSA private key is AES-256-GCM encrypted at rest in the database. Only the User Service can decrypt it."

**Save the token:**
```batch
set TOKEN=paste-the-access-token-here
```

**View profile:**
```batch
curl http://localhost:8080/api/v1/users/me -H "Authorization: Bearer %TOKEN%"
```

> "The Gateway validates the JWT, extracts the user ID, and injects it as an X-User-ID header. Every downstream service uses that for row-level access control — you can only see your own data."

---

## Part 3: Content Management (2 min)

> "Now let's create some academic content."

**Create a course:**
```batch
curl -X POST http://localhost:8080/api/v1/courses -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"name\":\"Deep Learning\",\"description\":\"Neural networks and beyond\",\"semester\":\"Fall\",\"year\":2026}"
```

**List courses:**
```batch
curl http://localhost:8080/api/v1/courses -H "Authorization: Bearer %TOKEN%"
```

**Upload a material:**
```batch
curl -X POST http://localhost:8080/api/v1/materials -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"title\":\"Lecture 1 - Intro to Neural Networks\",\"description\":\"Week 1 notes\",\"content_type\":\"pdf\",\"file_size\":50000}"
```

> "Material created with processing status 'pending'. When a real PDF is uploaded to MinIO, the Ingestion Service picks it up, extracts text, chunks it, generates 384-dimensional embeddings using sentence-transformers, and stores them for vector search."

**Create a flashcard deck:**
```batch
curl -X POST http://localhost:8080/api/v1/flashcard-decks -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"title\":\"ML Key Concepts\",\"description\":\"Core terms\",\"cards\":[{\"front_text\":\"What is a neural network?\",\"back_text\":\"A computing system inspired by biological neural networks that can learn from data\",\"difficulty\":\"easy\"},{\"front_text\":\"What is backpropagation?\",\"back_text\":\"An algorithm for training neural networks by computing gradients of the loss function\",\"difficulty\":\"medium\"}]}"
```

> "Flashcard deck created with 2 cards inline. Students can use these for spaced repetition study."

---

## Part 4: AI Features — The Core (3 min)

> "This is the heart of the platform — AI-powered learning."

**AI Chat with Gemini:**
```batch
curl -X POST http://localhost:8080/api/v1/ai/chat -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"Explain the difference between supervised and unsupervised learning with examples\",\"user_id\":\"demo\",\"context_chunks\":[]}"
```

> "This goes Gateway → AI Orchestrator → Google Gemini. The response includes token count for usage tracking. The circuit breaker monitors this — 3 failures and it opens for 60 seconds to prevent cascading failures."

**AI Chat WITH context (RAG):**
```batch
curl -X POST http://localhost:8080/api/v1/ai/chat -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"Based on these notes, what are the key takeaways?\",\"user_id\":\"demo\",\"context_chunks\":[\"Neural networks consist of layers of interconnected nodes. Each connection has a weight that is adjusted during training.\",\"Backpropagation computes the gradient of the loss function with respect to each weight by the chain rule.\",\"Common activation functions include ReLU, sigmoid, and tanh. ReLU is most popular for hidden layers.\"]}"
```

> "This is RAG in action — Retrieval Augmented Generation. The AI answers based on the actual document chunks we provide, not just general knowledge. In production, these chunks come from the Retrieval Service automatically."

**Vector search (RAG retrieval):**
```batch
curl -X POST http://localhost:8080/api/v1/ai/retrieve -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"how do neural networks learn\",\"user_id\":\"demo\",\"top_k\":3}"
```

> "The query is converted to a 384-dimensional vector using the same sentence-transformers model used during ingestion, then we do cosine similarity search. In production this uses pgvector for sub-millisecond search across millions of chunks."

**Generate a quiz from content:**
```batch
curl -X POST http://localhost:8080/api/v1/ai/generate/quiz -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"Generate a quiz about neural networks, backpropagation, and activation functions\",\"user_id\":\"demo\",\"context_chunks\":[]}"
```

> "AI-generated quizzes from study material. Students can also generate summaries and flashcards the same way."

**Generate a summary:**
```batch
curl -X POST http://localhost:8080/api/v1/ai/generate/summary -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"query\":\"Summarize the key concepts of deep learning including neural networks, training, and optimization\",\"user_id\":\"demo\",\"context_chunks\":[]}"
```

**Speech-to-text languages:**
```batch
curl http://localhost:8080/api/v1/ai/languages -H "Authorization: Bearer %TOKEN%"
```

> "14 languages supported for speech-to-text. Students can record voice notes and get them transcribed."

---

## Part 5: Quiz & Analytics (2 min)

> "Let me show the quiz-taking and analytics pipeline."

**Create a quiz with questions:**
```batch
curl -X POST http://localhost:8080/api/v1/quizzes -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"title\":\"Neural Networks Quiz\",\"description\":\"Test your knowledge\",\"time_limit_minutes\":10,\"difficulty\":\"medium\",\"questions\":[{\"question_text\":\"What is the most common activation function for hidden layers?\",\"question_type\":\"multiple_choice\",\"options\":[{\"text\":\"Sigmoid\",\"is_correct\":false},{\"text\":\"ReLU\",\"is_correct\":true},{\"text\":\"Tanh\",\"is_correct\":false}],\"correct_answer\":\"ReLU\",\"explanation\":\"ReLU is preferred because it avoids the vanishing gradient problem\",\"points\":10}]}"
```

> "Quiz created with questions, options, correct answers, and explanations — all stored in the content schema."

**Start a quiz attempt** (use the quiz ID from above):
```batch
curl -X POST http://localhost:8080/api/v1/quizzes/QUIZ_ID_HERE/start -H "Authorization: Bearer %TOKEN%"
```

> "The Analytics Service tracks the attempt — start time, which user, which quiz."

**Check study stats:**
```batch
curl http://localhost:8080/api/v1/analytics/study-stats -H "Authorization: Bearer %TOKEN%"
```

**Check study streak:**
```batch
curl http://localhost:8080/api/v1/analytics/study-streak -H "Authorization: Bearer %TOKEN%"
```

> "We track study streaks — consecutive days of activity — topic mastery using a spaced repetition algorithm, and AI token usage for cost monitoring."

**Create a learning goal:**
```batch
curl -X POST http://localhost:8080/api/v1/analytics/goals -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"title\":\"Master Neural Networks\",\"description\":\"Complete all quizzes with 80%+ score\",\"target_date\":\"2026-06-30\",\"goal_type\":\"course_completion\"}"
```

> "Students can set learning goals and track progress."

---

## Part 6: Notifications & Real-Time Sync (1 min)

> "We also have notification and real-time sync services."

**Get notification preferences:**
```batch
curl http://localhost:8080/api/v1/notifications/preferences -H "Authorization: Bearer %TOKEN%"
```

> "Users can configure push notifications, email digests, quiet hours, and choose which events trigger notifications — quiz completions, streak achievements, goal completions, study reminders."

**Create a study reminder:**
```batch
curl -X POST http://localhost:8080/api/v1/notifications/reminders -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"type\":\"study_reminder\",\"title\":\"Time to study!\",\"body\":\"Review your Deep Learning notes\",\"scheduled_for\":\"2026-03-27T09:00:00Z\",\"recurrence\":\"daily\"}"
```

> "Daily recurring study reminder. The Notification Service respects quiet hours and timezone settings."

**Check presence / sync:**
```batch
curl http://localhost:8080/api/v1/presence -H "Authorization: Bearer %TOKEN%"
```

> "The Sync Service tracks user presence across devices and provides WebSocket connections for real-time updates — when you upload a document on your laptop, it appears on your phone instantly."

---

## Part 7: Security Demo (1 min)

> "Let me show the security features."

**Rate limiting** (run this fast):
```batch
for /L %i in (1,1,5) do @curl -s http://localhost:8080/health >nul
curl -I http://localhost:8080/health 2>nul | findstr "RateLimit"
```

> "Redis sliding window rate limiting. Every response includes rate limit headers."

**Invalid token rejection:**
```batch
curl http://localhost:8080/api/v1/users/me -H "Authorization: Bearer invalid-token"
```

> "Instant 401. The Gateway validates every JWT using the RS256 public key fetched from the User Service at startup."

**Logout (token blacklisting):**
```batch
curl -X POST http://localhost:8080/api/v1/auth/logout -H "Authorization: Bearer %TOKEN%"
```

> "On logout, the refresh token is revoked in the database and the access token JTI is blacklisted in Redis for its remaining TTL."

---

## Potential Boss Questions & Answers

**Q: "How does the AI know about the student's documents?"**
> "RAG pipeline — when a PDF is uploaded, the Ingestion Service extracts text, chunks it into ~500 character segments, generates vector embeddings, and stores them. When the student asks a question, the Retrieval Service finds the most relevant chunks via cosine similarity, and those chunks are passed as context to Gemini."

**Q: "What happens if the AI service goes down?"**
> "Circuit breaker pattern. After 3 consecutive failures, the breaker opens for 60 seconds — all AI requests get a fast 503 instead of hanging. After 60 seconds it tries one request; if it succeeds, the circuit closes."

**Q: "How do you handle multiple devices?"**
> "The Sync Service maintains WebSocket connections per device. Changes are broadcast via Redis Pub/Sub. Each device tracks its sync cursor so it only receives events it hasn't seen."

**Q: "Is this production-ready?"**
> "The core architecture is production-ready — JWT RS256 with encrypted keys, bcrypt passwords, rate limiting, circuit breakers, connection pooling, graceful shutdown. For production deployment we'd add Kubernetes manifests, SSL/TLS, monitoring with Prometheus/Grafana, and log aggregation."

**Q: "How many users can this handle?"**
> "Each Go service handles thousands of concurrent connections. PostgreSQL connection pool is set to 25 per service. Redis handles rate limiting and caching. The AI endpoints are the bottleneck at 20 RPM per user, which is a Gemini API constraint."

**Q: "What's the tech stack?"**
> "Go with Echo/Gin frameworks for the backend, Python with FastAPI for AI services, PostgreSQL with 5 schemas, Redis for caching/sessions/rate limiting, MinIO for S3-compatible file storage, and Docker Compose for orchestration. All 14 containers."

**Q: "How do you verify email without an SMTP server?"**
> "The verification code is stored in both the database and Redis. In development, we can query it directly. In production, the Notification Service sends it via SMTP. The code expires after 15 minutes."
