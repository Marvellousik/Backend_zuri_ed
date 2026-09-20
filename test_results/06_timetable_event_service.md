# Verification Test Log: Timetable & Event Manager

- **Execution Timestamp**: 2026-09-20T15:08:53.488456+00:00
- **Tests Executed**: 3
- **Passed**: 3 / 3 (100.0%)
- **Average Latency**: `4.54 ms`

---

## Individual Test Executions & Payloads

### [TEST-TIME-01] Course & Timetable Slot Extraction from Syllabus Document
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `0.4 ms`
- **Endpoint**: `GET /api/v1/academic/courses/course_csc301_veritas`

#### Equivalent cURL Request
```bash
curl -X GET "http://localhost:8080/api/v1/academic/courses/course_csc301_veritas"
```

#### Payload Package Sent
```json
{
  "source_file": "csc301_course_syllabus.txt"
}
```

#### Response Package Received
```json
{
  "course_code": "CSC 301",
  "course_title": "Data Structures & Algorithms",
  "lecture_day": "Monday",
  "start_time": "10:00:00",
  "end_time": "12:00:00",
  "venue": "Lecture Theatre 2 (LT2)"
}
```

---

### [TEST-TIME-02] AI Academic Context Gap Detection (Missing Class Time/Venue)
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `8.07 ms`
- **Endpoint**: `GET /api/v1/academic/context-gaps`

#### Equivalent cURL Request
```bash
curl -X GET "http://localhost:8080/api/v1/academic/context-gaps" \
  -H "X-User-ID: usr_student_gap_test"
```

#### Payload Package Sent
```json
{
  "user_id": "usr_student_gap_test",
  "enrolled_courses": [
    "CSC 399"
  ]
}
```

#### Response Package Received
```json
{
  "gaps_detected": 1,
  "gap_id": "942c6a30-3fd0-4f72-a9fe-007af6822ac1",
  "prompt_question": "I have your CSC 399 (Research Methodology & Independent Study) class, but I don't know when it holds. What day and time is the lecture?"
}
```

---

### [TEST-TIME-03] Student Gap Verification Resolution & Timetable Slot Logging
- **Verdict**: ✅ PASS
- **Status Code**: `200`
- **Response Time**: `5.16 ms`
- **Endpoint**: `POST /api/v1/academic/context-gaps/942c6a30-3fd0-4f72-a9fe-007af6822ac1/resolve`

#### Equivalent cURL Request
```bash
curl -X POST "http://localhost:8080/api/v1/academic/context-gaps/942c6a30-3fd0-4f72-a9fe-007af6822ac1/resolve" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: usr_student_gap_test" \
  -d '{"response_value": "Monday 10:00 - 12:00 in Lecture Theatre 2 (LT2)"}'
```

#### Payload Package Sent
```json
{
  "response_value": "Monday 10:00 - 12:00 in Lecture Theatre 2 (LT2)"
}
```

#### Response Package Received
```json
{
  "gap_id": "942c6a30-3fd0-4f72-a9fe-007af6822ac1",
  "status": "resolved",
  "message": "Thank you! Your academic context has been updated."
}
```

---
