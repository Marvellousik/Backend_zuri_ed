# PowerShell Test Script for API Gateway

$ErrorActionPreference = "Stop"
$BaseUrl = "http://localhost:8080"

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "API Gateway Test Script" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan

# 1. Health Check
Write-Host "`n1. Testing Gateway Health..." -ForegroundColor Green
$health = Invoke-RestMethod -Uri "$BaseUrl/health" -Method GET
Write-Host "   Status: $($health.status)" -ForegroundColor Gray
Write-Host "   Upstream Services:" -ForegroundColor Gray
$health.upstream.PSObject.Properties | ForEach-Object {
    Write-Host "     - $($_.Name): $($_.Value)" -ForegroundColor Gray
}

# 2. Register User Through Gateway
Write-Host "`n2. Testing User Registration Through Gateway..." -ForegroundColor Green
$timestamp = Get-Date -Format "yyyyMMddHHmmss"
$email = "gateway-test$timestamp@example.com"

$registerBody = @{
    email = $email
    password = "Password123!"
    first_name = "Gateway"
    last_name = "Test"
} | ConvertTo-Json

try {
    $registerResponse = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/register" -Method POST -Body $registerBody -ContentType "application/json"
    Write-Host "   ✓ User registered: $($registerResponse.data.email)" -ForegroundColor Green
    Write-Host "   User ID: $($registerResponse.data.id)" -ForegroundColor Yellow
    $userId = $registerResponse.data.id
} catch {
    Write-Host "   ✗ Registration failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 3. Test Public Key Endpoint
Write-Host "`n3. Testing Public Key Endpoint..." -ForegroundColor Green
$publicKey = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/public-key" -Method GET
Write-Host "   ✓ Public key retrieved (algorithm: $($publicKey.data.algorithm))" -ForegroundColor Green

# 4. Test Login (Expect 401 without verification)
Write-Host "`n4. Testing Login (may fail if email not verified)..." -ForegroundColor Green
$loginBody = @{
    email = $email
    password = "Password123!"
} | ConvertTo-Json

try {
    $loginResponse = Invoke-RestMethod -Uri "$BaseUrl/api/v1/auth/login" -Method POST -Body $loginBody -ContentType "application/json"
    Write-Host "   ✓ Login successful" -ForegroundColor Green
    $accessToken = $loginResponse.data.access_token
    Write-Host "   Access Token received" -ForegroundColor Yellow
} catch {
    Write-Host "   ⚠ Login failed (expected if email not verified): $($_.Exception.Message)" -ForegroundColor Yellow
}

# 5. Test Protected Endpoint Without Auth (Should fail)
Write-Host "`n5. Testing Protected Endpoint Without Auth (should fail)..." -ForegroundColor Green
try {
    $profile = Invoke-RestMethod -Uri "$BaseUrl/api/v1/users/me" -Method GET
    Write-Host "   ✗ Should have failed!" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "   ✓ Correctly rejected with 401" -ForegroundColor Green
    } else {
        Write-Host "   ✗ Unexpected error: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# 6. Test Rate Limit Headers
Write-Host "`n6. Checking Rate Limit Headers..." -ForegroundColor Green
$response = Invoke-WebRequest -Uri "$BaseUrl/health" -Method GET
$limit = $response.Headers["X-RateLimit-Limit"]
$remaining = $response.Headers["X-RateLimit-Remaining"]
Write-Host "   Rate Limit: $limit" -ForegroundColor Gray
Write-Host "   Remaining: $remaining" -ForegroundColor Gray

Write-Host "`n=====================================" -ForegroundColor Cyan
Write-Host "Gateway Tests Complete!" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "`nTest user created: $email" -ForegroundColor Yellow
Write-Host "User ID: $userId" -ForegroundColor Yellow
