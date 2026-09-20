# Verification Test Log: Content & PDF Ingestion

- **Execution Timestamp**: 2026-09-20T15:08:53.486956+00:00
- **Tests Executed**: 3
- **Passed**: 3 / 3 (100.0%)
- **Average Latency**: `4.05 ms`

---

## Individual Test Executions & Payloads

### [TEST-PDF-01] PDF Upload & SHA-256 Provenance Tracking
- **Verdict**: ✅ PASS
- **Status Code**: `201`
- **Response Time**: `0.06 ms`
- **Endpoint**: `POST /api/v1/materials`

#### Equivalent cURL Request
```bash
curl -X POST "http://localhost:8080/api/v1/materials" \
  -H "Content-Type: multipart/form-data" \
  -d '{"title": "CSC 301 Lecture Slides", "sha256_checksum": "8b102f53b7c4a362ab95eca5ff1d876290fcfb7275c3a6da0419e2a4788cfec7", "course_offering_id": "off_csc301_veritas", "tracking_id": "trk_b378aba6068c"}'
```

#### Payload Package Sent
```json
{
  "filename": "csc301_dynamic_programming_lecture.pdf",
  "size": 4678
}
```

#### Response Package Received
```json
{
  "status": "success",
  "material_id": "mat_3ae79b1c",
  "tracking_id": "trk_b378aba6068c",
  "sha256_checksum": "8b102f53b7c4a362ab95eca5ff1d876290fcfb7275c3a6da0419e2a4788cfec7",
  "file_size_bytes": 4678,
  "transcription_status": "none"
}
```

---

### [TEST-PDF-02] Multi-Page Extraction & Slide Text Segmentation
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `11.95 ms`
- **Endpoint**: `EXTRACT internal://pdf/extract_pages`

#### Equivalent cURL Request
```bash
# Ingestion extraction on raw PDF byte stream
```

#### Payload Package Sent
```json
{
  "pdf_bytes_len": 4678
}
```

#### Response Package Received
```json
{
  "pages_extracted": 2,
  "page_1_chars": 1526,
  "page_2_chars": 1019
}
```

---

### [TEST-PDF-03] Hierarchical Headings & Structured Section Parsing
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `0.14 ms`
- **Endpoint**: `PARSE internal://pdf/parse_hierarchy`

#### Equivalent cURL Request
```bash
# Structural hierarchy detection on multi-page slides
```

#### Payload Package Sent
```json
{
  "input_text_length": 2565
}
```

#### Response Package Received
```json
{
  "sections_found": 1,
  "headings": [
    "Introduction & Overview"
  ],
  "tables_found": 0,
  "figures_found": 0
}
```

---
