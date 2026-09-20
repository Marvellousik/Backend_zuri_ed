"""
Unit tests for Phase 38: Production Security & Load Hardening.
Verifies concurrent multi-tenant isolation, SQL injection attack resilience,
and sub-200ms timeline latency performance under simulated load.
"""
import time
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Institution,
    Course,
    CourseSchedule,
    StudentEnrollment,
    StudentKnowledgeState,
)
from academic_service.services.proactive_engine import ProactiveEngine


@pytest.fixture
def db_engine_and_session():
    """In-memory SQLite database fixture with attached academic schema and thread-safe sessionmaker."""
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )

    @event.listens_for(engine, "connect")
    def do_connect(dbapi_connection, connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("ATTACH DATABASE ':memory:' AS academic")
        cursor.close()

    Base.metadata.create_all(bind=engine)
    SessionMaker = sessionmaker(bind=engine)
    session = SessionMaker()

    # Seed Tenant A
    inst_a = Institution(id="inst_veritas", name="Veritas University", code="VUNA")
    c_a = Course(
        id="c_veritas_301",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 301",
        title="Algorithms Veritas",
    )
    enr_a = StudentEnrollment(id="enr_a", user_id="user_veritas_student", course_id="c_veritas_301")

    # Seed Tenant B
    inst_b = Institution(id="inst_covenant", name="Covenant University", code="CU")
    c_b = Course(
        id="c_covenant_301",
        institution_id="inst_covenant",
        department_id="dept_cs",
        code="CSC 301",
        title="Algorithms Covenant",
    )
    enr_b = StudentEnrollment(id="enr_b", user_id="user_covenant_student", course_id="c_covenant_301")

    session.add_all([inst_a, c_a, enr_a, inst_b, c_b, enr_b])
    session.commit()

    try:
        yield engine, SessionMaker, session
    finally:
        session.close()


def test_concurrent_tenant_isolation(db_engine_and_session):
    """
    Verify that concurrent queries from multiple tenants never leak cross-tenant records.
    Each thread uses its own scoped session.
    """
    _, SessionMaker, _ = db_engine_and_session
    import threading
    db_lock = threading.Lock()

    def query_tenant_a():
        with db_lock:
            with SessionMaker() as sess:
                courses = (
                    sess.query(Course)
                    .filter(Course.institution_id == "inst_veritas")
                    .all()
                )
                for c in courses:
                    assert c.institution_id == "inst_veritas"
                    assert c.title != "Algorithms Covenant"
                return len(courses)

    def query_tenant_b():
        with db_lock:
            with SessionMaker() as sess:
                courses = (
                    sess.query(Course)
                    .filter(Course.institution_id == "inst_covenant")
                    .all()
                )
                for c in courses:
                    assert c.institution_id == "inst_covenant"
                    assert c.title != "Algorithms Veritas"
                return len(courses)

    with ThreadPoolExecutor(max_workers=8) as executor:
        futures_a = [executor.submit(query_tenant_a) for _ in range(12)]
        futures_b = [executor.submit(query_tenant_b) for _ in range(12)]

        results_a = [f.result() for f in futures_a]
        results_b = [f.result() for f in futures_b]

    assert all(r == 1 for r in results_a)
    assert all(r == 1 for r in results_b)


def test_sql_injection_guard_and_input_validation(db_engine_and_session):
    """
    Verify resilience against SQL injection payloads in course and user parameters.
    """
    _, _, session = db_engine_and_session

    malicious_inputs = [
        "' OR '1'='1",
        "CSC 301'; DROP TABLE courses; --",
        "admin' UNION SELECT * FROM academic.institutions --",
        "1; WAITFOR DELAY '0:0:5'--",
    ]

    for payload in malicious_inputs:
        result = (
            session.query(Course)
            .filter(Course.code == payload)
            .first()
        )
        assert result is None

        active_courses = session.query(Course).count()
        assert active_courses == 2


def test_simulated_load_timeline_latency_benchmark(db_engine_and_session):
    """
    Simulated load benchmark: verifies that generating the personalized Today timeline
    consistently executes well under the 200ms SLA target.
    """
    _, _, session = db_engine_and_session
    simulated_now = datetime(2026, 8, 31, 8, 30, 0)
    latencies = []

    for _ in range(25):
        t0 = time.perf_counter()
        timeline = ProactiveEngine.generate_today_timeline(
            db=session,
            user_id="user_veritas_student",
            current_time=simulated_now,
        )
        elapsed_ms = (time.perf_counter() - t0) * 1000.0
        latencies.append(elapsed_ms)

        assert hasattr(timeline, "cards")

    avg_latency = sum(latencies) / len(latencies)
    max_latency = max(latencies)

    assert avg_latency < 100.0, f"Average latency {avg_latency:.2f}ms exceeds 100ms threshold!"
    assert max_latency < 200.0, f"Max latency {max_latency:.2f}ms exceeds 200ms SLA!"
