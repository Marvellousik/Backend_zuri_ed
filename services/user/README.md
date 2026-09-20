# User Service

The User Service is the identity management service for Zuri. It handles user authentication, authorization, profile management, and session management.

## Features

- **User Registration** - Create new user accounts with email verification
- **Authentication** - JWT-based authentication with RS256 signing
- **Token Management** - Access tokens (15 min) and refresh tokens (30 days) with rotation
- **Profile Management** - Update user profiles (name, school, department, etc.)
- **Session Management** - View and revoke active sessions across devices
- **Password Management** - Change password and password reset flow
- **Email Verification** - Verify email addresses with 6-digit codes

## Architecture

```
services/user/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── handler/             # HTTP handlers
│   │   ├── auth_handler.go
│   │   ├── user_handler.go
│   │   └── session_handler.go
│   ├── service/             # Business logic
│   │   └── user_service.go
│   ├── repository/          # Data access layer
│   │   ├── user_repository.go
│   │   ├── refresh_token_repository.go
│   │   ├── session_repository.go
│   │   ├── password_reset_repository.go
│   │   ├── jwt_key_repository.go
│   │   └── mock_repository.go  # Test mocks
│   ├── model/               # Domain models
│   │   └── user.go
│   └── middleware/          # HTTP middleware
│       └── validator.go
├── Dockerfile
└── README.md
```

## API Endpoints

### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Register a new user |
| POST | `/api/v1/auth/login` | Authenticate and get tokens |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| POST | `/api/v1/auth/verify-email` | Verify email address |
| POST | `/api/v1/auth/resend-verification` | Resend verification email |
| POST | `/api/v1/auth/forgot-password` | Request password reset |
| POST | `/api/v1/auth/reset-password` | Reset password with token |
| GET | `/api/v1/auth/public-key` | Get JWT public key |
| GET | `/health` | Health check |

### Protected Endpoints (require X-User-ID header)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/logout` | Logout current session |
| POST | `/api/v1/auth/logout-all` | Logout all sessions |
| GET | `/api/v1/users/me` | Get current user profile |
| PUT | `/api/v1/users/me` | Update user profile |
| POST | `/api/v1/users/me/change-password` | Change password |
| GET | `/api/v1/users/me/sessions` | List active sessions |
| DELETE | `/api/v1/users/me/sessions/:id` | Revoke a session |

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |
| `PRIVATE_KEY_ENCRYPTION_KEY` | Master key for encrypting RSA private key (min 32 chars) |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8081 | HTTP server port |
| `LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `BCRYPT_COST` | 12 | Bcrypt hashing cost |
| `ACCESS_TOKEN_TTL` | 15m | Access token lifetime |
| `REFRESH_TOKEN_TTL` | 720h | Refresh token lifetime (30 days) |

## Running Locally

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Redis 7+

### Setup

1. Start dependencies:
```bash
docker-compose -f infra/docker-compose.yml up -d postgres redis
```

2. Run migrations:
```bash
make migrate-up
```

3. Run the service:
```bash
export DATABASE_URL="postgres://zuri:zuri_secret@localhost:5432/zuri?sslmode=disable"
export REDIS_URL="localhost:6379"
export PRIVATE_KEY_ENCRYPTION_KEY="your-secure-master-key-min-32-chars-long"
make run-user-service
```

### Using Docker

```bash
# Build and run with all dependencies
docker-compose -f infra/docker-compose.yml up -d
```

## Testing

```bash
# Run unit tests
make test

# Run with coverage
make test-coverage
```

## Security

- **Password Hashing**: bcrypt with cost 12
- **JWT Signing**: RS256 (RSA 2048-bit)
- **Private Key Storage**: AES-256-GCM encrypted in PostgreSQL
- **Refresh Tokens**: SHA-256 hashed before storage
- **Rate Limiting**: Built-in for sensitive operations

## Token Flow

1. **Login** → Returns access token (15 min) + refresh token (30 days)
2. **Access Protected Resources** → Include access token in Authorization header
3. **Token Refresh** → Use refresh token to get new access token
4. **Logout** → Revoke refresh token, blacklist access token

## Database Schema

The service uses the `auth` schema with the following tables:

- `auth.users` - User accounts
- `auth.refresh_tokens` - Refresh token storage
- `auth.user_sessions` - Active sessions
- `auth.jwt_keys` - RSA key storage
- `auth.password_resets` - Password reset tokens
- `auth.token_blacklist` - Revoked access tokens
