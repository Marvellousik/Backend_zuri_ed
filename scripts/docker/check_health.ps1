<#
.SYNOPSIS
    Probes all Zuri backend endpoints and reports health status.
#>

$endpoints = @(
    @{ Name = "API Gateway";             Url = "http://localhost:8080/health" },
    @{ Name = "MinIO Storage Engine";    Url = "http://localhost:9000/minio/health/live" }
)

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "       ZURI BACKEND HEALTH & ENDPOINT STATUS PROBE         " -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan

foreach ($ep in $endpoints) {
    try {
        $res = Invoke-RestMethod -Uri $ep.Url -TimeoutSec 3 -ErrorAction Stop
        Write-Host "  [OK]   $($ep.Name.PadRight(25)) -> $($ep.Url)" -ForegroundColor Green
    } catch {
        Write-Host "  [FAIL] $($ep.Name.PadRight(25)) -> $($ep.Url) (Not responding)" -ForegroundColor Red
    }
}

Write-Host "`nDocker Container Status:" -ForegroundColor Yellow
docker compose ps
Write-Host "============================================================" -ForegroundColor Cyan
