# Database Setup for Zuri - The Simple Version

> This guide assumes you know nothing about databases. We will use **Supabase** because it is free and handles everything for us.

---

## The Problem (In Plain English)

Zuri needs a database to store:
- User accounts and passwords
- Courses, quizzes, flashcards
- AI conversation history
- File metadata

**BUT** Zuri also needs a special database feature called **`pgvector`**. This feature lets the AI search through your uploaded PDFs intelligently ("find me the part about neural networks").

**Render's built-in database does NOT have `pgvector`.** So we cannot use it.

**The fix:** Use **Supabase** instead. It is free, has `pgvector`, and works perfectly with Render.

---

## Step 1: Create a Supabase Account (Free)

1. Go to [supabase.com](https://supabase.com)
2. Click **Start your project**
3. Sign up with GitHub (easiest)
4. Click **New project**
   - Organization: whatever you want
   - Project name: `zuri-prod`
   - Database password: **write this down somewhere safe**
   - Region: pick the closest to your users (e.g., `US East` if most users are in the US)
5. Click **Create new project**
6. Wait ~2 minutes for it to spin up

---

## Step 2: Get Your Database Connection String

1. In your Supabase project dashboard, click the **Settings** icon (gear, bottom left)
2. Click **Database**
3. Under **Connection string**, click **URI**
4. It looks like this:

```
postgresql://postgres:YOUR_SUPABASE_PASSWORD@aws-1-eu-north-1.pooler.supabase.com:6543/postgres
```

5. **Copy this string.** You will paste it into Render.
6. **Important:** Replace `[YOUR-PASSWORD]` with the actual password you wrote down.

The final string should look like:
```
postgresql://postgres:MySecretPassword123@db.abcdefghijklm.supabase.co:5432/postgres
```

---

## Step 3: Enable pgvector in Supabase

1. In Supabase, click **SQL Editor** (left sidebar)
2. Click **New query**
3. Paste this exact SQL and click **Run**:

```sql
-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Verify it worked
SELECT * FROM pg_extension WHERE extname = 'vector';
```

4. You should see one row in the results. Done.

---

## Step 4: Update render.yaml for Supabase

Open `render.yaml` in your project. **Delete** this entire section:

```yaml
  # DELETE THIS WHOLE BLOCK
  - type: postgres
    name: zuri-postgres
    plan: starter
    ipAllowList: []
    disk:
      name: postgres-data
      mountPath: /var/lib/postgresql/data
      sizeGB: 10
```

Now, **everywhere** in `render.yaml` where you see:

```yaml
      - key: DATABASE_URL
        fromDatabase:
          name: zuri-postgres
          property: connectionString
```

Replace it with:

```yaml
      - key: DATABASE_URL
        value: postgresql://postgres:MySecretPassword123@db.abcdefghijklm.supabase.co:5432/postgres
```

**Use YOUR actual Supabase connection string, not the example above.**

---

## Step 5: Run the Database Migrations (Create Tables)

Migrations are just SQL files that create all the tables Zuri needs.

### Option A: Run from your computer (easiest)

1. Install a database tool called `psql`. If you have Docker:

```bash
docker run -it --rm postgres:15-alpine psql "postgresql://postgres:YOUR_SUPABASE_PASSWORD@aws-1-eu-north-1.pooler.supabase.com:6543/postgres"
```

2. You are now inside the database. Run each migration:

```sql
\i infra/migrations/001_create_auth_schema.sql
\i infra/migrations/002_create_content_schema.sql
\i infra/migrations/003_create_analytics_schema.sql
\i infra/migrations/004_create_notification_schema.sql
\i infra/migrations/005_create_sync_schema.sql
\i infra/migrations/006_create_ai_schema.sql
\i infra/migrations/007_add_role_column.sql
```

3. Type `\q` to exit.

### Option B: Run from Supabase Dashboard (no tools needed)

1. In Supabase, click **SQL Editor**
2. Click **New query**
3. Open each migration file from your computer:
   - `infra/migrations/001_create_auth_schema.sql`
   - `infra/migrations/002_create_content_schema.sql`
   - etc.
4. Copy the entire contents of the file
5. Paste into Supabase SQL Editor
6. Click **Run**
7. Repeat for all 7 migration files

---

## Step 6: Verify Tables Exist

In Supabase SQL Editor, run:

```sql
SELECT schema_name, table_name
FROM information_schema.tables
WHERE table_schema IN ('auth', 'content', 'analytics', 'notification', 'sync', 'ai')
ORDER BY schema_name, table_name;
```

You should see about **20 tables** listed. If you do, the database is ready.

---

## Step 7: Deploy to Render

Now that your database exists and has tables, deploy your services:

1. Push your updated `render.yaml` to GitHub
2. Go to [dashboard.render.com](https://dashboard.render.com)
3. Click **New** -> **Blueprint**
4. Connect your repo
5. Render will deploy everything

**Important:** The first time services start, they will connect to your Supabase database and work immediately because the tables already exist.

---

## What We Just Did (Simple Diagram)

```
BEFORE ( confusing )
-------------------
Your code  -->  Render tries to create PostgreSQL
                BUT Render's PostgreSQL is missing pgvector
                --> AI search breaks

AFTER ( what we just set up )
------------------------------
Your code  -->  Render runs your services
                |
                v
            Supabase PostgreSQL (has pgvector)
                |
                +-- auth schema (users, passwords)
                +-- content schema (courses, quizzes)
                +-- analytics schema (study streaks)
                +-- notification schema (emails)
                +-- sync schema (real-time)
                +-- ai schema (AI sessions)
```

---

## Common Questions

### "Do I need to run migrations every time I deploy?"

**No.** You only run migrations **once** when setting up the database. After that, the tables exist forever.

If you add NEW migrations later (new features), you only run those new ones.

### "What if I mess up the database?"

Supabase has a **Table Editor** where you can see and edit everything. Go to Supabase Dashboard -> Table Editor. You can view data, delete rows, etc.

### "Is my database password safe?"

Yes. In Render, environment variables are encrypted. In Supabase, only you can see the password.

**Never** commit your database password to GitHub. The `render.yaml` uses environment variables, so the password is never in your code.

### "What does `postgresql://` mean?"

It is just the "address" of your database. Think of it like a phone number:
- `postgres` = username
- `MySecretPassword123` = password
- `db.abcdefghijklm.supabase.co` = server address
- `5432` = port
- `postgres` = database name

### "Can I use Render's native Redis?"

**Yes.** Redis is just a cache. It does not need `pgvector`. The `render.yaml` already uses Render's free Redis. No changes needed.

---

## Troubleshooting

### "Services say 'database connection refused'"

- Check that your `DATABASE_URL` in Render is exactly correct (no typos)
- Check that Supabase project is Active (not paused)
- In Supabase Settings -> Database, make sure **IPv4** is enabled for direct connections

### "Migrations fail with 'permission denied'"

- In Supabase SQL Editor, run: `ALTER USER postgres WITH SUPERUSER;`
- Then run migrations again

### "I see 'relation does not exist' errors"

- This means migrations did not run
- Go back to Step 5 and run all 7 migration files

---

## Updated render.yaml (Supabase Version)

I have also created `render-supabase.yaml` in this folder. It is the same as `render.yaml` but:
- **No Render native PostgreSQL** (uses Supabase instead)
- **All services already point to Supabase** via `DATABASE_URL` env var
- You just need to replace the placeholder connection string with yours

Open `render-supabase.yaml`, find:
```yaml
value: postgresql://postgres:YOUR_PASSWORD@db.YOUR_PROJECT.supabase.co:5432/postgres
```

Replace it with your real Supabase connection string. Save. Push to GitHub. Deploy.
