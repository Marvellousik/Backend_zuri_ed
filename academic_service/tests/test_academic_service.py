"""
Unit tests for AcademicService hierarchy and new academic domain models.
"""
from datetime import datetime, date, timezone
from unittest.mock import MagicMock

from academic_service.models.orm import (
    Institution,
    Faculty,
    Department,
    Program,
    AcademicSession,
    Semester,
)
from academic_service.models.schema import (
    ProgramResponse,
    InstitutionHierarchyResponse,
    AcademicSessionResponse,
    SemesterResponse,
)
from academic_service.services.academic_service import AcademicService


def test_get_institution_hierarchy_success():
    """Verify that get_institution_hierarchy builds complete faculty, department, and program hierarchy."""
    db_mock = MagicMock()

    # Setup mock Institution
    inst = Institution(
        id="veritas_uni",
        name="Veritas University",
        code="VUNA",
        domain="veritas.edu.ng",
        settings={"current_semester": "2025/2026_FIRST"},
    )

    # Setup mock Faculty
    fac = Faculty(
        id="fnas_veritas",
        institution_id="veritas_uni",
        name="Faculty of Natural and Applied Sciences",
        code="FNAS",
    )

    # Setup mock Department
    dept = Department(
        id="dept_cs_veritas",
        faculty_id="fnas_veritas",
        institution_id="veritas_uni",
        name="Department of Computer Science",
        code="CSC",
    )

    # Setup mock Program
    prog = Program(
        id="prog_cs_veritas",
        department_id="dept_cs_veritas",
        institution_id="veritas_uni",
        name="BSc Computer Science",
        code="CSC",
        degree_type="BSc",
        duration_years=4,
        created_at=datetime(2026, 1, 1, 0, 0, 0, tzinfo=timezone.utc),
    )

    # Mock queries
    # First query: Institution
    # Second query: Faculties
    # Third query: Departments
    # Fourth query: Programs by department
    # Fifth query: Programs by institution
    db_mock.query().filter().first.return_value = inst
    db_mock.query().filter().all.side_effect = [
        [fac],    # Faculty query
        [dept],   # Department query
        [prog],   # Department programs query
        [prog],   # Institution programs query
    ]

    # Test with (institution_id, db)
    hierarchy = AcademicService.get_institution_hierarchy(
        institution_id="veritas_uni",
        db=db_mock,
    )

    assert hierarchy is not None
    assert isinstance(hierarchy, InstitutionHierarchyResponse)
    assert hierarchy.id == "veritas_uni"
    assert hierarchy.name == "Veritas University"
    assert hierarchy.code == "VUNA"
    assert len(hierarchy.faculties) == 1
    assert hierarchy.faculties[0].code == "FNAS"
    assert len(hierarchy.faculties[0].departments) == 1
    assert hierarchy.faculties[0].departments[0].code == "CSC"
    assert len(hierarchy.faculties[0].departments[0].programs) == 1
    assert hierarchy.faculties[0].departments[0].programs[0].code == "CSC"
    assert hierarchy.faculties[0].departments[0].programs[0].degree_type == "BSc"
    assert len(hierarchy.programs) == 1
    assert hierarchy.programs[0].name == "BSc Computer Science"


def test_get_institution_hierarchy_argument_flexibility():
    """Verify that get_institution_hierarchy accepts (db, institution_id) as well."""
    db_mock = MagicMock()
    inst = Institution(id="inst_1", name="Test Uni", code="TU")
    db_mock.query().filter().first.return_value = inst
    db_mock.query().filter().all.return_value = []

    res = AcademicService.get_institution_hierarchy(db_mock, "inst_1")
    assert res is not None
    assert res.id == "inst_1"


def test_get_institution_hierarchy_not_found():
    """Verify that get_institution_hierarchy returns None when institution is not found."""
    db_mock = MagicMock()
    db_mock.query().filter().first.return_value = None

    res = AcademicService.get_institution_hierarchy("non_existent", db_mock)
    assert res is None


def test_orm_models_and_relationships():
    """Verify ORM instantiation and relationship attributes."""
    session = AcademicSession(
        id="session_2025_2026",
        institution_id="veritas_uni",
        name="2025/2026",
        start_date=date(2025, 9, 1),
        end_date=date(2026, 7, 31),
        is_current=True,
    )
    semester = Semester(
        id="sem_2025_2026_first",
        session_id="session_2025_2026",
        institution_id="veritas_uni",
        name="FIRST",
        start_date=date(2025, 9, 1),
        end_date=date(2026, 1, 31),
        is_current=True,
    )

    semester.academic_session = session
    assert semester.session == session

    prog = Program(
        id="p1",
        department_id="d1",
        institution_id="i1",
        name="Software Engineering",
        code="SEN",
        degree_type="BSc",
        duration_years=4,
    )
    dto = AcademicService._to_program_dto(prog)
    assert isinstance(dto, ProgramResponse)
    assert dto.code == "SEN"
    assert dto.duration_years == 4
