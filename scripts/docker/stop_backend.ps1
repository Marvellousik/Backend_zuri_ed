<#
.SYNOPSIS
    Stops and removes all Zuri backend Docker containers.
#>

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = (Get-Item $ScriptDir).Parent.Parent.FullName
$ComposeFile = Join-Path $ProjectRoot "infra\docker-compose.yml"

Write-Host "Stopping all Zuri backend microservices..." -ForegroundColor Yellow
docker compose -f $ComposeFile down
Write-Host "All containers and networks stopped cleanly." -ForegroundColor Green
