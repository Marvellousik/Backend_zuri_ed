# Zuri - Render.com Deployment Guide

> One-click blueprint deployment for the full Zuri backend.

---

## Prerequisites

1. **GitHub account** with your Zuri repo pushed
2. **Render.com account** (free tier available)
3. **API keys ready:**
   - Google Gemini API key
   - Groq API key (optional)
   - Weaviate URL + API key (optional)
   - Cohere API key (optional)
   - SMTP credentials (optional - for email)
   - Firebase service account JSON (optional - for push notifications)

---

## Critical: PostgreSQL + pgvector

**Render's native PostgreSQL does NOT include the `pgvector` extension.** You have two options:

### Option A: Use Supabase for PostgreSQL (Recommended)

1. Create a free project at [supabase.com](https://supabase.com)
2. Enable `pgvector` extension in the SQL editor:
   ```sql
   CREATE EXTENSION IF NOT EXISTS vector;
   ```
3. Copy the connection string from Settings -> Database
4. In Render dashboard, manually override `DATABASE_URL` for all services to use your Supabase URL
5. Delete the `zuri-postgres` service from `render.yaml` before deploying

### Option B: Use Docker-based PostgreSQL with pgvector on Render

Replace the native postgres service in `render.yaml` with:

```yaml
  - type: pserv
    name: zuri-postgres
    runtime: docker
    image: ankane/pgvector:latest
    plan: starter
    envVars:
      - key: POSTGRES_USER
        value: zuri
      - key: POSTGRES_PASSWORD
        generateValue: true
      - key: POSTGRES_DB
        value: zuri
    disk:
      name: pgdata
      mountPath: /var/lib/postgresql/data
      sizeGB: 10
```

**Note:** Docker-based PostgreSQL on Render does NOT get automatic backups. Use Option A for production data.

---

## Step-by-Step Deployment

### Step 1: Prepare Your Repo

1. Ensure `render.yaml` is at the **root** of your repo
2. Ensure all Dockerfiles referenced in `render.yaml` exist:
   - `services/*/Dockerfile`
   - `zuri-Python Services/services/*/Dockerfile`
   - `zuri-ai-main/Dockerfile`
3. Commit and push to GitHub

### Step 2: Connect to Render

1. Go to [dashboard.render.com](https://dashboard.render.com)
2. Click **New** -> **Blueprint**
3. Connect your GitHub repository
4. Render will read `render.yaml` and preview all services

### Step 3: Configure Secrets

Before deploying, set these secret environment variables in the Render dashboard:

| Secret | Where to Set |
|--------|-------------|
| `GEMINI_API_KEY` | Environment Group: `zuri-secrets` |
| `GOOGLE_API_KEY` | Environment Group: `zuri-secrets` |
| `GROQ_API_KEY` | Environment Group: `zuri-secrets` |
| `WEAVIATE_URL` | Environment Group: `zuri-secrets` |
| `WEAVIATE_API_KEY` | Environment Group: `zuri-secrets` |
| `COHERE_API_KEY` | Environment Group: `zuri-secrets` |
| `SMTP_HOST` | Environment Group: `zuri-secrets` |
| `SMTP_USERNAME` | Environment Group: `zuri-secrets` |
| `SMTP_PASSWORD` | Environment Group: `zuri-secrets` |

**How to set:**
1. In Render dashboard, go to **Environment** section
2. Create an **Environment Group** called `zuri-secrets`
3. Add each secret key-value pair
4. Link the environment group to all your services

### Step 4: Attach Environment Group

Add this to each service in `render.yaml` that needs API keys:

```yaml
    envVars:
      - fromGroup: zuri-secrets
```

Or do it manually in the Render dashboard after first deploy.

### Step 5: Run Database Migrations

After PostgreSQL is running, execute migrations. You can do this via Render Shell:

1. Go to your PostgreSQL service in Render dashboard
2. Open **Shell** tab
3. Run:

```bash
psql $DATABASE_URL -f /path/to/migrations/001_create_auth_schema.sql
psql $DATABASE_URL -f /path/to/migrations/002_create_content_schema.sql
psql $DATABASE_URL -f /path/to/migrations/003_create_analytics_schema.sql
psql $DATABASE_URL -f /path/to/migrations/004_create_notification_schema.sql
psql $DATABASE_URL -f /path/to/migrations/005_create_sync_schema.sql
psql $DATABASE_URL -f /path/to/migrations/006_create_ai_schema.sql
psql $DATABASE_URL -f /path/to/migrations/007_add_role_column.sql
```

**Alternative:** Add an init container or startup script to your User Service that runs migrations automatically.

### Step 6: Upload Firebase Service Account (for Push Notifications)

1. Go to `zuri-notification-service` in Render dashboard
2. Go to **Disks** tab
3. The disk is mounted at `/etc/secrets`
4. Use Render Shell to upload your Firebase service account JSON:

```bash
cat > /etc/secrets/firebase-service-account.json << 'EOF'
{
  "type": "service_account",
  "project_id": "your-project",
  ...
}
EOF
```

### Step 7: Verify Deployment

Once all services show **Live**, test:

```bash
# Get your Gateway URL from Render dashboard
curl https://zuri-gateway-xxxxx.onrender.com/health

# Expected: {"service":"gateway","status":"healthy","upstream":{...}}
```

### Step 8: Update Frontend Environment

In your Next.js frontend (deployed on Vercel), update:

```env
NEXT_PUBLIC_API_GATEWAY_URL=https://zuri-gateway-xxxxx.onrender.com
```

---

## Service URLs After Deploy

| Service | Render Type | Access |
|---------|------------|--------|
| Gateway | Web (public) | `https://zuri-gateway-xxx.onrender.com` |
| User Service | Private Service | Internal only |
| Content Service | Private Service | Internal only |
| Analytics Service | Private Service | Internal only |
| Notification Service | Private Service | Internal only |
| Sync Service | Private Service | Internal only |
| AI Orchestrator | Private Service | Internal only |
| AI Monolith | Private Service | Internal only |
| Ingestion | Private Service | Internal only |
| Retrieval | Private Service | Internal only |
| Audio | Private Service | Internal only |
| Evaluation | Private Service | Internal only |
| PostgreSQL | Managed | Internal only |
| Redis | Managed | Internal only |
| MinIO | Private Service | Internal only |

---

## Render Plans & Pricing

| Service | Plan | Monthly Cost | Notes |
|---------|------|-------------|-------|
| Gateway | Starter | $7 | Public HTTPS entry point |
| 5 Go Services | Starter x5 | $35 | Private services |
| 6 Python Services | Starter x6 | $42 | Private services |
| PostgreSQL | Starter | $15 | Or use Supabase free tier |
| Redis | Free | $0 | 1 GB limit |
| MinIO | Starter | $7 | File storage |
| **Total** | | **~$106/mo** | With Supabase: **~$91/mo** |

**To reduce costs:**
- Combine lightweight services into fewer containers
- Use Supabase free tier for PostgreSQL
- Use Upstash free tier for Redis instead of Render Redis
- Start with fewer replicas, scale up as needed

---

## Auto-Deploy on Git Push

Render automatically redeploys services when you push to your connected branch.

To control which services rebuild:
- Render uses Docker layer caching
- Only services with changed code or base image rebuild

---

## Logs & Monitoring

| Tool | Location |
|------|----------|
| Service Logs | Render Dashboard -> Service -> Logs |
| Metrics | Render Dashboard -> Service -> Metrics |
| Custom Alerts | Use Log Streams to Datadog / Papertrail |

---

## Common Issues

### "Build failed: Dockerfile not found"

Ensure `dockerfilePath` in `render.yaml` is correct relative to repo root:
```yaml
dockerfilePath: ./services/gateway/Dockerfile  # NOT services/gateway/Dockerfile
```

### "Private Service cannot be reached"

Private Services on Render are only accessible from other Render services in the same account. They do NOT have public URLs. The Gateway proxies requests to them internally.

### "Database connection refused"

- Verify `DATABASE_URL` env var is set correctly
- Check that PostgreSQL service is `Live` before other services start
- Render auto-restarts dependent services if Postgres fails

### "pgvector extension not available"

You are using Render native PostgreSQL which lacks pgvector. Switch to Supabase or the Docker-based pgvector image.

### "AI service times out"

Render free/starter plans have 30-second request timeouts for web services. For long AI generations:
- Use streaming responses
- Or upgrade Python AI services to Standard plan (extends timeout)
- Or use background jobs (Redis queue) for long tasks

---

## Production Checklist

Before going live:

- [ ] All secrets set in Environment Group (no hardcoded keys)
- [ ] `INTERNAL_API_KEY` is auto-generated (not `dev-internal-key`)
- [ ] `BYPASS_EMAIL_VERIFICATION=false`
- [ ] `ALLOWED_ORIGINS` set to your actual frontend domain
- [ ] Database migrations executed
- [ ] Firebase service account JSON uploaded to notification service disk
- [ ] Gateway health check returns `healthy`
- [ ] Frontend `NEXT_PUBLIC_API_GATEWAY_URL` points to Render Gateway URL
- [ ] Custom domain configured (optional) in Render Dashboard -> Gateway -> Settings -> Custom Domain
- [ ] SSL certificate auto-provisioned by Render
- [ ] Rate limits tested: `curl -I https://your-gateway.com/api/v1/ai/chat`

---

## Next Steps

1. Deploy using the blueprint
2. Run database migrations
3. Test registration/login via Gateway URL
4. Connect your Vercel frontend to the Render backend
5. Set up monitoring and alerting
