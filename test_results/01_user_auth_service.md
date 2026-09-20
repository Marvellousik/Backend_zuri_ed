# Verification Test Log: User & Auth Service

- **Execution Timestamp**: 2026-09-20T15:08:53.485963+00:00
- **Tests Executed**: 6
- **Passed**: 6 / 6 (100.0%)
- **Average Latency**: `8.85 ms`

---

## Individual Test Executions & Payloads

### [TEST-AUTH-01] Student Registration (POST /api/v1/auth/register)
- **Verdict**: ✅ PASS
- **Status Code**: `201`
- **Response Time**: `51.47 ms`
- **Endpoint**: `POST /api/v1/auth/register`

#### Equivalent cURL Request
```bash
curl -X POST "http://localhost:8080/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email": "student_ef44f2@veritas.edu.ng", "password": "SecurePassword2026!", "first_name": "Emeka", "last_name": "Adeleke", "institution_id": "inst_veritas", "role": "student"}'
```

#### Payload Package Sent
```json
{
  "email": "student_ef44f2@veritas.edu.ng",
  "password": "SecurePassword2026!",
  "first_name": "Emeka",
  "last_name": "Adeleke",
  "institution_id": "inst_veritas",
  "role": "student"
}
```

#### Response Package Received
```json
{
  "status": "success",
  "code": 201,
  "data": {
    "id": "usr_test_963d3130",
    "email": "student_ef44f2@veritas.edu.ng",
    "first_name": "Emeka",
    "last_name": "Adeleke",
    "role": "student",
    "institution_id": "inst_veritas",
    "is_verified": false,
    "created_at": "2026-09-20T15:08:40.919078+00:00"
  }
}
```

---

### [TEST-AUTH-02] Public Key Retrieval (GET /api/v1/auth/public-key)
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `0.01 ms`
- **Endpoint**: `GET /api/v1/auth/public-key`

#### Equivalent cURL Request
```bash
curl -X GET "http://localhost:8080/api/v1/auth/public-key" \
  -H "Accept: application/json"
```

#### Payload Package Sent
```json
{}
```

#### Response Package Received
```json
{
  "status": "success",
  "data": {
    "algorithm": "RS256",
    "key_type": "RSA-2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBC...\n-----END PUBLIC KEY-----"
  }
}
```

---

### [TEST-AUTH-03] Email Verification Token (POST /api/v1/auth/verify-email)
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `0.03 ms`
- **Endpoint**: `POST /api/v1/auth/verify-email`

#### Equivalent cURL Request
```bash
curl -X POST "http://localhost:8080/api/v1/auth/verify-email" \
  -H "Content-Type: application/json" \
  -d '{"token": "vtok_134841cff1254cf5a307a8c1a014f031"}'
```

#### Payload Package Sent
```json
{
  "token": "vtok_134841cff1254cf5a307a8c1a014f031"
}
```

#### Response Package Received
```json
{
  "status": "success",
  "message": "Email verified successfully",
  "data": {
    "user_id": "usr_test_963d3130",
    "email": "student_ef44f2@veritas.edu.ng",
    "is_verified": true
  }
}
```

---

### [TEST-AUTH-04] RS256 JWT Login & Token Generation (POST /api/v1/auth/login)
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `1.58 ms`
- **Endpoint**: `POST /api/v1/auth/login`

#### Equivalent cURL Request
```bash
curl -X POST "http://localhost:8080/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email": "student_ef44f2@veritas.edu.ng", "password": "SecurePassword2026!"}'
```

#### Payload Package Sent
```json
{
  "email": "student_ef44f2@veritas.edu.ng",
  "password": "SecurePassword2026!"
}
```

#### Response Package Received
```json
{
  "status": "success",
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c3JfdGVzdF85NjNkMzEzMCIsImVtYWlsIjoic3R1ZGVudF9lZjQ0ZjJAdmVyaXRhcy5lZHUubmciLCJpbnN0aXR1dGlvbl9pZCI6Imluc3RfdmVyaXRhcyIsInJvbGUiOiJzdHVkZW50IiwiaWF0IjoxNzg5OTE2OTIwLCJleHAiOjE3ODk5MTc4MjAsImlzcyI6Inp1cmktdXNlci1zZXJ2aWNlIn0.M_xZnQvB60vT1JgCBJXfRiXAsklKt0azadzAgmQQNjoDtD6V5SsShKxkwKfufLjnJ8pDSfostrInszFqbPEFn2nRQPT7-D5N7izE5pcok5potAf8gB3cRYl-A48gyPxeydALTLUWvz8mzrsFoekW8zMqJXDYj-U2Gd42aBccmZ5vFlcr-8g9qGwzYWUyDtRb6_Ly33Ozkjd4YsHZgUoiVuhjFgb4FZU545C23DJeZyxDeKDPxvuj8f_wc0GxPDl-hPsAlRMYrEI3mzYD2ej1uR2XIU9g0v_LzwgqQEocTWMfhHL9ea_qfLGSfjpHDB9ifxPCJHiL4gRdDlG7h9Arcw",
    "refresh_token": "rft_0ea84c692069426aac35e75876b487d6",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "usr_test_963d3130",
      "role": "student",
      "institution_id": "inst_veritas"
    }
  }
}
```

---

### [TEST-AUTH-05] Authenticated Profile Inspection (GET /api/v1/users/me)
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `0.0 ms`
- **Endpoint**: `GET /api/v1/users/me`

#### Equivalent cURL Request
```bash
curl -X GET "http://localhost:8080/api/v1/users/me" \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c3JfdGVzdF85NjNkMzEzMCIsImVtYWlsIjoic3R1ZGVudF9lZjQ0ZjJAdmVyaXRhcy5lZHUubmciLCJpbnN0aXR1dGlvbl9pZCI6Imluc3RfdmVyaXRhcyIsInJvbGUiOiJzdHVkZW50IiwiaWF0IjoxNzg5OTE2OTIwLCJleHAiOjE3ODk5MTc4MjAsImlzcyI6Inp1cmktdXNlci1zZXJ2aWNlIn0.M_xZnQvB60vT1JgCBJXfRiXAsklKt0azadzAgmQQNjoDtD6V5SsShKxkwKfufLjnJ8pDSfostrInszFqbPEFn2nRQPT7-D5N7izE5pcok5potAf8gB3cRYl-A48gyPxeydALTLUWvz8mzrsFoekW8zMqJXDYj-U2Gd42aBccmZ5vFlcr-8g9qGwzYWUyDtRb6_Ly33Ozkjd4YsHZgUoiVuhjFgb4FZU545C23DJeZyxDeKDPxvuj8f_wc0GxPDl-hPsAlRMYrEI3mzYD2ej1uR2XIU9g0v_LzwgqQEocTWMfhHL9ea_qfLGSfjpHDB9ifxPCJHiL4gRdDlG7h9Arcw"
```

#### Payload Package Sent
```json
{}
```

#### Response Package Received
```json
{
  "status": "success",
  "data": {
    "id": "usr_test_963d3130",
    "email": "student_ef44f2@veritas.edu.ng",
    "first_name": "Emeka",
    "last_name": "Adeleke",
    "role": "student",
    "active_institution_id": "inst_veritas"
  }
}
```

---

### [TEST-AUTH-06] Unauthorized Access Rejection (401 Verification)
- **Verdict**: ✅ PASS
- **Status Code**: `401`
- **Response Time**: `0.0 ms`
- **Endpoint**: `GET /api/v1/users/me`

#### Equivalent cURL Request
```bash
curl -X GET "http://localhost:8080/api/v1/users/me"
```

#### Payload Package Sent
```json
{}
```

#### Response Package Received
```json
{
  "error": "unauthorized",
  "message": "Missing or malformed Authorization header"
}
```

---
