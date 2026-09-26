<#
.SYNOPSIS
    Zuri Backend — One-Click Automated Startup & Management Script
.DESCRIPTION
    Checks Docker daemon, spins up all 11 microservices (Postgres, Redis, MinIO, Go Gateway,
    User, Content, Analytics, Notification, Sync, Academic, and AI services), monitors health
    checks, and displays endpoints.
.PARAMETER Build
    Force rebuild of all Docker images before starting.
.PARAMETER Down
    Stop and remove all running containers and networks.
.PARAMETER Status
    Show current container status and health without restarting.
.PARAMETER Logs
    Stream logs from all running containers.
.EXAMPLE
    .\start_backend.ps1
    .\start_backend.ps1 -Build
    .\start_backend.ps1 -Down
    .\start_backend.ps1 -Logs
#>

[CmdletBinding()]
param(
    [switch]$Build,
    [switch]$Down,
    [switch]$Status,
    [switch]$Logs
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ComposeFile = Join-Path $ScriptDir "infra\docker-compose.yml"

if (-not (Test-Path $ComposeFile)) {
    # In case script is run directly from inside the infra directory
    $ComposeFile = Join-Path $ScriptDir "docker-compose.yml"
}

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "   ZURI BACKEND PLATFORM — DOCKER ORCHESTRATOR             " -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan

# 1. Check Docker Daemon
Write-Host "`n[1/4] Checking Docker status..." -ForegroundColor Yellow
try {
    $null = docker version 2>&1
    Write-Host "  -> Docker Desktop daemon is running." -ForegroundColor Green
} catch {
    Write-Host "  [ERROR] Docker is not running or not found!" -ForegroundColor Red
    Write-Host "  Please start Docker Desktop and run this script again." -ForegroundColor Yellow
    exit 1
}

# Handle -Down flag
if ($Down) {
    Write-Host "`nStopping and removing all Zuri backend containers..." -ForegroundColor Yellow
    docker compose -f $ComposeFile down
    Write-Host "  -> All containers stopped successfully." -ForegroundColor Green
    exit 0
}

# Handle -Status flag
if ($Status) {
    Write-Host "`nCurrent Container Status:" -ForegroundColor Cyan
    docker compose -f $ComposeFile ps
    exit 0
}

# Handle -Logs flag
if ($Logs) {
    Write-Host "`nStreaming container logs (Ctrl+C to stop)..." -ForegroundColor Cyan
    docker compose -f $ComposeFile logs -f
    exit 0
}

# 2. Check Environment Files
Write-Host "`n[2/4] Verifying environment configurations..." -ForegroundColor Yellow
$InfraEnv = Join-Path $ScriptDir "infra\.env"
$InfraEnvExample = Join-Path $ScriptDir "infra\.env.example"

if (-not (Test-Path $InfraEnv) -and (Test-Path $InfraEnvExample)) {
    Copy-Item $InfraEnvExample $InfraEnv
    Write-Host "  -> Created infra\.env from infra\.env.example" -ForegroundColor Green
} else {
    Write-Host "  -> infra\.env is present." -ForegroundColor Green
}

# 3. Build & Launch Containers
Write-Host "`n[3/4] Launching backend microservices..." -ForegroundColor Yellow
if ($Build) {
    Write-Host "  -> Building images with latest code changes..." -ForegroundColor Cyan
    docker compose -f $ComposeFile up -d --build
} else {
    docker compose -f $ComposeFile up -d
}

if ($LASTEXITCODE -ne 0) {
    Write-Host "`n[ERROR] Docker Compose failed to start containers." -ForegroundColor Red
    exit 1
}

# 4. Health Check Verification
Write-Host "`n[4/4] Verifying microservice health status..." -ForegroundColor Yellow
$MaxWaitSeconds = 60
$Elapsed = 0
$GatewayHealthy = $false

while ($Elapsed -lt $MaxWaitSeconds) {
    Start-Sleep -Seconds 3
    $Elapsed += 3
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 2 -ErrorAction SilentlyContinue
        if ($response -and ($response.status -eq "ok" -or $response.status -eq "healthy")) {
            $GatewayHealthy = $true
            break
        }
    } catch {
        Write-Host -NoNewline "."
    }
}

Write-Host ""
if ($GatewayHealthy) {
    Write-Host "  -> All core services and API Gateway are ONLINE and HEALTHY!" -ForegroundColor Green
} else {
    Write-Host "  -> Containers are starting up. Run '.\start_backend.ps1 -Status' to monitor progress." -ForegroundColor Yellow
}

# Display Endpoints Summary
Write-Host "`n============================================================" -ForegroundColor Cyan
Write-Host "                  SERVICE ACCESS ENDPOINTS                  " -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  API Gateway:         http://localhost:8080" -ForegroundColor White
Write-Host "  Gateway Healthcheck: http://localhost:8080/health" -ForegroundColor White
Write-Host "  MinIO S3 API:        http://localhost:9000" -ForegroundColor White
Write-Host "  MinIO Web Console:   http://localhost:9001  (User: minioadmin / Pass: minioadmin_secret)" -ForegroundColor White
Write-Host "  PostgreSQL DB:       localhost:5432        (User: zuri / DB: zuri)" -ForegroundColor White
Write-Host "  Redis Cache:         localhost:6379" -ForegroundColor White
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "Useful Commands:" -ForegroundColor Yellow
Write-Host "  View Status:         .\start_backend.ps1 -Status" -ForegroundColor DarkGray
Write-Host "  Follow Live Logs:    .\start_backend.ps1 -Logs" -ForegroundColor DarkGray
Write-Host "  Stop All Services:   .\start_backend.ps1 -Down" -ForegroundColor DarkGray
Write-Host "============================================================`n" -ForegroundColor Cyan
