# Zuri Tests

This directory contains automated integration tests, end-to-end smoke test scripts, and test fixtures for the Zuri polyglot microservices system.

---

## Directory Structure

```
tests/
├── integration/          # Python & cross-service integration tests
│   └── integration_test_phase1_phase2.py
├── e2e/                  # End-to-end API Gateway smoke test scripts
│   ├── test-gateway.ps1
│   └── test-gateway-enhanced.ps1
├── fixtures/             # Test payloads and mock files
│   └── test_rag.txt
└── scripts/              # Ad-hoc database & service verification scripts
    └── verify_refresh.go
```

---

## Running Tests

### Go Unit Tests
Unit tests are located in individual service internal directories (e.g. `services/user/internal/service/` and `services/gateway/internal/proxy/`):
```bash
go test ./services/...
```

### Python Integration Tests
Integration tests verify cost tracking, document truncation, model routing, and prompt formatting across Python services:
```bash
python3 tests/integration/integration_test_phase1_phase2.py
```

### Gateway End-to-End Smoke Tests
Smoke test the API Gateway and upstream services:
```powershell
pwsh tests/e2e/test-gateway.ps1
# or enhanced suite:
pwsh tests/e2e/test-gateway-enhanced.ps1
```
