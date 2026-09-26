# Zuri Backend — Docker Operations & Management Suite

This folder contains automated operational scripts for managing the 11-container Zuri microservices architecture.

## Available Scripts

| Script | Purpose |
| :--- | :--- |
| **`start_backend.ps1`** (Root & here) | Validates environment, checks Docker daemon, spins up all 11 containers, monitors healthchecks, and displays the endpoint dashboard. |
| **`stop_backend.ps1`** | Gracefully terminates and removes all containers and networks. |
| **`check_health.ps1`** | Probes service health endpoints and outputs live container status. |

## Quick Usage

### Start full backend
```powershell
.\start_backend.ps1
```

### Rebuild and start (when code changes)
```powershell
.\start_backend.ps1 -Build
```

### Stop all containers
```powershell
.\scripts\docker\stop_backend.ps1
```

### Probe endpoint health
```powershell
.\scripts\docker\check_health.ps1
```
