"""
Unit tests for Phase 35: Institutional Knowledge Graph Service.
Verifies ConceptDAG cycle prevention, curricular gap detection across courses,
and topological sorting of prerequisite learning paths.
"""
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import Base, Course
from academic_service.services.knowledge_graph_service import (
    KnowledgeGraphService,
    ConceptDAG,
    REL_PREREQUISITE_OF,
    REL_CO_OCCURS_WITH,
)


@pytest.fixture
def db_session():
    """In-memory SQLite database session fixture with attached academic schema."""
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
    Session = sessionmaker(bind=engine)
    session = Session()
    try:
        yield session
    finally:
        session.close()


def test_concept_dag_prerequisites_and_cycle_prevention():
    """
    Verify ConceptDAG relationship tracking and cycle rejection.
    """
    dag = ConceptDAG()
    dag.add_relationship("Variables", "Control Flow", REL_PREREQUISITE_OF)
    dag.add_relationship("Control Flow", "Recursion", REL_PREREQUISITE_OF)
    dag.add_relationship("Recursion", "Dynamic Programming", REL_PREREQUISITE_OF)

    # Transitive prerequisites for Dynamic Programming should include all three
    prereqs = dag.get_prerequisites_transitive("Dynamic Programming")
    assert "Variables" in prereqs
    assert "Control Flow" in prereqs
    assert "Recursion" in prereqs

    # Attempting to add Dynamic Programming -> Variables would create a cycle; must be rejected
    added = dag.add_relationship("Dynamic Programming", "Variables", REL_PREREQUISITE_OF)
    assert added is False


def test_detect_curricular_gaps(db_session):
    """
    Verify detection of curricular gaps between preceding Course A and subsequent Course B.
    """
    course_a = Course(
        id="course_csc101",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 101",
        title="Introduction to Computing",
        syllabus=[
            {"topic": "Variables & Data Types", "subtopics": ["Integers", "Floats"]},
            {"topic": "Control Flow & Loops", "subtopics": ["If-Else", "While Loops"]},
        ],
    )

    course_b = Course(
        id="course_csc301",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 301",
        title="Data Structures and Algorithms",
        syllabus=[
            {
                "topic": "Divide and Conquer",
                "subtopics": ["MergeSort", "QuickSort"],
                "prerequisites": ["Functions & Recursion", "Control Flow & Loops"],
            },
            {
                "topic": "Dynamic Programming",
                "subtopics": ["Optimal Substructure", "Memoization"],
                "prerequisites": ["Functions & Recursion"],
            },
        ],
    )

    db_session.add_all([course_a, course_b])
    db_session.commit()

    gap_report = KnowledgeGraphService.detect_curricular_gaps(
        db=db_session,
        course_a_id="CSC 101",
        course_b_id="CSC 301",
    )

    assert gap_report["is_gap_detected"] is True
    assert "Functions & Recursion" in gap_report["missing_prerequisites"]
    assert "Control Flow & Loops" in gap_report["covered_prerequisites"]
    assert gap_report["curricular_alignment_score"] < 100
    assert len(gap_report["recommendations"]) > 0
    assert "Functions & Recursion" in gap_report["recommendations"][0]


def test_get_learning_path_topological_sort(db_session):
    """
    Verify synthesis of topologically sorted prerequisite learning path for a target concept.
    """
    course = Course(
        id="course_csc301",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 301",
        title="Algorithms",
        syllabus=[
            {"topic": "Functions & Recursion", "prerequisites": ["Control Flow & Loops"]},
            {"topic": "Dynamic Programming", "prerequisites": ["Functions & Recursion"]},
            {"topic": "Optimal Substructure", "prerequisites": ["Dynamic Programming"]},
        ],
    )
    db_session.add(course)
    db_session.commit()

    path_data = KnowledgeGraphService.get_learning_path(
        db=db_session,
        target_concept="Optimal Substructure",
    )

    assert path_data["target_concept"] == "Optimal Substructure"
    assert path_data["total_steps"] >= 3
    assert path_data["estimated_total_hours"] > 0

    ordered_concepts = [s["concept"] for s in path_data["learning_path"]]

    # Foundational prerequisites must appear before target concept
    assert "Optimal Substructure" in ordered_concepts
    target_idx = ordered_concepts.index("Optimal Substructure")

    # If Dynamic Programming is present, it must appear before Optimal Substructure
    if "Dynamic Programming" in ordered_concepts:
        dp_idx = ordered_concepts.index("Dynamic Programming")
        assert dp_idx < target_idx

    # If Functions & Recursion is present, it must appear before Dynamic Programming
    if "Functions & Recursion" in ordered_concepts and "Dynamic Programming" in ordered_concepts:
        rec_idx = ordered_concepts.index("Functions & Recursion")
        dp_idx = ordered_concepts.index("Dynamic Programming")
        assert rec_idx < dp_idx
