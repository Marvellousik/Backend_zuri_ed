# Verification Test Log: Unified AI Gateway & OpenRouter

- **Execution Timestamp**: 2026-09-20T15:08:53.489435+00:00
- **Tests Executed**: 2
- **Passed**: 2 / 2 (100.0%)
- **Average Latency**: `242.77 ms`

---

## Individual Test Executions & Payloads

### [TEST-GW-01] OpenRouter Adapter & Dynamic Model Router Suite (Go Engine)
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `485.52 ms`
- **Endpoint**: `GO_TEST /api/v1/ai/route/complete`

#### Equivalent cURL Request
```bash
go test -v ./services/gateway/internal/ai/...
```

#### Payload Package Sent
```json
{
  "test_targets": [
    "TestOpenRouterAdapter",
    "TestModelRouter",
    "TestCache",
    "TestCostTracker"
  ]
}
```

#### Response Package Received
```json
{
  "exit_code": 0,
  "summary": "18 AI Gateway tests passed in Go",
  "sample": "=== RUN   TestComputePromptHash\n--- PASS: TestComputePromptHash (0.00s)\n=== RUN   TestCache_GetAndSet\n--- PASS: TestCache_GetAndSet (0.00s)\n=== RUN   TestUsageRecord_Validation\n--- PASS: TestUsageReco"
}
```

---

### [TEST-GW-02] Redis Semantic Prompt Hash Canonicalization & Cache Keying
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `0.02 ms`
- **Endpoint**: `HASH internal://ai_cache/compute_hash`

#### Equivalent cURL Request
```bash
# SHA-256 canonical prompt hashing: task + normalized messages + temperature
```

#### Payload Package Sent
```json
{
  "task": "quiz_generation",
  "temperature": 0.7
}
```

#### Response Package Received
```json
{
  "deterministic_match": true,
  "prompt_hash": "635397b1fad588e11646d4de6ba88af39a61137112df3ed48a87d58208052ef3"
}
```

---
