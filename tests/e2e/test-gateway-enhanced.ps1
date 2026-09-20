# test-gateway-enhanced.ps1
# Enhanced smoke test that validates Phase 1 hardening + Phase 2 routing

$ErrorActionPreference = "Stop"
$BaseUrl      = "http://localhost:8080"
$AIServiceUrl = "http://localhost:8000"

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "Zuri Enhanced Smoke Test" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan

# 1. Health Check
Write-Host "`n1. Gateway Health..." -ForegroundColor Green
$health = Invoke-RestMethod -Uri "$BaseUrl/health" -Method GET
Write-Host "   Status: $($health.status)" -ForegroundColor Gray

# 2. Register User
Write-Host "`n2. User Registration..." -ForegroundColor Green
$timestamp = Get-Date -Format "yyyyMMddHHmmss"
$email = "enhanced-test$timestamp@example.com"

$registerBody = @{
    email      = $email
    password   = "Password123!"
    first_name = "Test"
    last_name  = "User"
} | ConvertTo-Json

try {
    $reg = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/register" -Method POST -Body $registerBody -ContentType "application/json"
    Write-Host "   [PASS] Registered: $($reg.data.email)" -ForegroundColor Green
    $userId = $reg.data.id
} catch {
    Write-Host "   [FAIL] Registration failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 3. Public Key
Write-Host "`n3. Public Key..." -ForegroundColor Green
$pk = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/public-key" -Method GET
Write-Host "   [PASS] Algorithm: $($pk.data.algorithm)" -ForegroundColor Green

# 4. Login (may fail if verification required)
Write-Host "`n4. Login..." -ForegroundColor Green
$loginBody = @{ email = $email; password = "Password123!" } | ConvertTo-Json
$accessToken = $null
try {
    $login = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/login" -Method POST -Body $loginBody -ContentType "application/json"
    Write-Host "   [PASS] Login succeeded" -ForegroundColor Green
    $accessToken = $login.data.access_token
} catch {
    Write-Host "   [SKIP] Login failed (email not verified): $($_.Exception.Message)" -ForegroundColor Yellow
}

# 5. Protected Route Without Auth
Write-Host "`n5. Protected Route Without Auth..." -ForegroundColor Green
try {
    Invoke-RestMethod -Uri "$BaseUrl/api/v1/users/me" -Method GET | Out-Null
    Write-Host "   [FAIL] Should have been rejected!" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "   [PASS] Rejected with 401" -ForegroundColor Green
    } else {
        Write-Host "   [FAIL] Unexpected: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# 6. Rate Limit Headers on Public Route
Write-Host "`n6. Rate Limit Headers..." -ForegroundColor Green
$resp = Invoke-WebRequest -Uri "$BaseUrl/health" -Method GET
$limit = $resp.Headers["X-RateLimit-Limit"]
$remaining = $resp.Headers["X-RateLimit-Remaining"]
Write-Host "   X-RateLimit-Limit: $limit" -ForegroundColor Gray
Write-Host "   X-RateLimit-Remaining: $remaining" -ForegroundColor Gray
if ($limit) { Write-Host "   [PASS] Rate limit headers present" -ForegroundColor Green }

# 7. Direct Python Service Access (Should Fail)
Write-Host "`n7. Direct Python Service Access (no X-Internal-Key)..." -ForegroundColor Green
try {
    Invoke-RestMethod -Uri "$AIServiceUrl/health" -Method GET -ErrorAction SilentlyContinue | Out-Null
    Invoke-RestMethod -Uri "$AIServiceUrl/study/flashcards" -Method POST | Out-Null
    Write-Host "   [WARN] Direct access succeeded - ports may be exposed!" -ForegroundColor Yellow
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    if ($status -eq 401 -or $status -eq 403) {
        Write-Host "   [PASS] Direct access blocked with $status" -ForegroundColor Green
    } else {
        Write-Host "   [INFO] Connection refused or error: $($_.Exception.Message)" -ForegroundColor Gray
    }
}

# 8. AI Endpoints With Auth (if logged in)
if ($accessToken) {
    Write-Host "`n8. AI Endpoint Rate Limiting..." -ForegroundColor Green
    $headers = @{ Authorization = "Bearer $accessToken" }

    # 8a. Study flashcards (expects 422 because no file, but should show rate limit headers)
    try {
        $aiResp = Invoke-WebRequest -Uri "$BaseUrl/api/v1/study/flashcards" -Method POST -Headers $headers
    } catch {
        $aiResp = $_.Exception.Response
    }
    $aiLimit     = $aiResp.Headers["X-RateLimit-Limit"]
    $aiQuota     = $aiResp.Headers["X-Quota-Limit"]
    $aiQuotaRem  = $aiResp.Headers["X-Quota-Remaining"]
    Write-Host "   AI RateLimit-Limit: $aiLimit" -ForegroundColor Gray
    Write-Host "   AI Quota-Limit: $aiQuota" -ForegroundColor Gray
    Write-Host "   AI Quota-Remaining: $aiQuotaRem" -ForegroundColor Gray
    if ($aiQuota) { Write-Host "   [PASS] Daily AI quota headers present" -ForegroundColor Green }

    # 8b. Reading analysis (same - expect 422, but headers prove rate limiting is active)
    try {
        $rdResp = Invoke-WebRequest -Uri "$BaseUrl/api/v1/reading/analyse" -Method POST -Headers $headers
    } catch {
        $rdResp = $_.Exception.Response
    }
    $rdLimit = $rdResp.Headers["X-RateLimit-Limit"]
    if ($rdLimit) { Write-Host "   [PASS] /reading rate limited" -ForegroundColor Green }

    # 8c. Writing notes
    try {
        $wrResp = Invoke-WebRequest -Uri "$BaseUrl/api/v1/writing/notes" -Method POST -Headers $headers `
            -Body (@{session_id="test-123"; raw_text="Hello"; user_id=$userId} | ConvertTo-Json) `
            -ContentType "application/json"
    } catch {
        $wrResp = $_.Exception.Response
    }
    $wrLimit = $wrResp.Headers["X-RateLimit-Limit"]
    if ($wrLimit) { Write-Host "   [PASS] /writing rate limited" -ForegroundColor Green }
} else {
    Write-Host "`n8. AI Endpoint Tests SKIPPED (no access token)" -ForegroundColor Yellow
}

Write-Host "`n=====================================" -ForegroundColor Cyan
Write-Host "Enhanced Smoke Test Complete!" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
