"""
Unit tests for Phase 20: Context Engine & Assembler (ContextAssembler).
Verifies token budgeting, context block formatting, syllabus/mastery injection,
and verifiable [Chunk <chunk_id>] citation provenance.
"""
from datetime import time, date
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Course,
    Lecture,
    CourseSchedule,
    StudentKnowledgeState,
    LearningGap,
)
from academic_service.services.context_assembler import (
    ContextAssembler,
    AssembledContext,
    estimate_tokens,
    DEFAULT_MAX_CONTEXT_TOKENS,
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


def test_context_block_formatting_headers():
    """Verify that all four required structured context blocks are generated with exact headers."""
    assembler = ContextAssembler(max_context_tokens=4000)

    course = Course(
        id="course_csc301",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 301",
        title="Data Structures and Algorithms",
        level=300,
        credit_units=3,
        description="Core computer science course covering advanced data structures and algorithms.",
        syllabus=[
            {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Memoization"]},
        ],
    )

    retrieved_chunks = [
        {
            "chunk_id": "chunk_001",
            "doc_id": "doc_lecture14",
            "course": "CSC 301",
            "heading": "Optimal Substructure",
            "text": "A problem exhibits optimal substructure if an optimal solution to the problem contains within it optimal solutions to subproblems.",
        }
    ]

    result: AssembledContext = assembler.assemble(
        course=course,
        retrieved_chunks=retrieved_chunks,
    )

    assert "--- COURSE CONTEXT ---" in result.prompt_text
    assert "--- RELEVANT SYLLABUS ---" in result.prompt_text
    assert "--- STUDENT MASTERY CONTEXT ---" in result.prompt_text
    assert "--- RETRIEVED COURSE MATERIALS (AUTHORITATIVE) ---" in result.prompt_text

    # Verify course context block content
    assert "CSC 301 - Data Structures and Algorithms" in result.blocks["course_context"]
    assert "Level: 300 | Credit Units: 3" in result.blocks["course_context"]


def test_verifiable_chunk_citations_and_provenance():
    """
    Verify strict citation markers [Chunk <chunk_id>] for every retrieved passage.
    """
    assembler = ContextAssembler()

    chunks = [
        {
            "chunk_id": "chunk_alpha_101",
            "doc_id": "doc_syllabus_notes",
            "course": "CSC 301",
            "heading": "Bellman Formulations",
            "text": "The Bellman equation decomposes the value function into immediate payoff plus discounted continuation value.",
        },
        {
            "chunk_id": "chunk_beta_202",
            "doc_id": "doc_slide_deck_14",
            "course": "CSC 301",
            "section": "Memoization Table",
            "text": "Memoization stores intermediate subproblem results in a hash table or array indexed by state variables.",
        },
    ]

    result = assembler.assemble(retrieved_chunks=chunks)

    # Check presence in prompt
    assert "[Chunk chunk_alpha_101]" in result.prompt_text
    assert "[Chunk chunk_beta_202]" in result.prompt_text

    # Check citations list and IDs
    assert "[Chunk chunk_alpha_101]" in result.citations
    assert "[Chunk chunk_beta_202]" in result.citations
    assert result.citation_ids == ["chunk_alpha_101", "chunk_beta_202"]

    # Verify provenance metadata in chunk header
    assert "Doc: doc_syllabus_notes" in result.blocks["retrieved_chunks"]
    assert "Section: Bellman Formulations" in result.blocks["retrieved_chunks"]


def test_token_budget_allocation_and_enforcement():
    """
    Verify strict token budget allocation:
    max_context_tokens: default 4000; allocates syllabus 15%, mastery 15%, timetable/lecture 10%, retrieved chunks 60%.
    """
    max_budget = 1000
    # Budgets:
    # syllabus: 15% -> 150 tokens
    # mastery: 15% -> 150 tokens
    # timetable_lecture: 10% -> 100 tokens
    # retrieved_chunks: 60% -> 600 tokens
    assembler = ContextAssembler(max_context_tokens=max_budget)

    # Generate many large chunks to test boundary enforcement
    large_text = "This is authoritative academic lecture chunk text discussing optimal substructure in depth. " * 30
    many_chunks = [
        {
            "chunk_id": f"chunk_bulk_{i}",
            "doc_id": f"doc_{i}",
            "course": "CSC 301",
            "heading": f"Topic {i}",
            "text": large_text,
        }
        for i in range(15)
    ]

    # Generate many syllabus modules
    many_modules = [
        {
            "topic": f"Comprehensive Module {i}",
            "subtopics": [f"Deep Subtopic {i}.A", f"Deep Subtopic {i}.B", f"Deep Subtopic {i}.C"],
            "prerequisites": [f"Prereq {i}.1", f"Prereq {i}.2"],
        }
        for i in range(20)
    ]

    result = assembler.assemble(
        syllabus_modules=many_modules,
        retrieved_chunks=many_chunks,
        max_context_tokens=max_budget,
    )

    budgets = result.metadata["budgets"]
    assert budgets["syllabus"] == 150
    assert budgets["mastery"] == 150
    assert budgets["timetable_lecture"] == 100
    assert budgets["retrieved_chunks"] == 600

    # Ensure individual block tokens do not exceed their allocated budgets
    assert result.block_tokens["syllabus"] <= budgets["syllabus"] + 10
    assert result.block_tokens["mastery"] <= budgets["mastery"] + 10
    assert result.block_tokens["course_context"] <= budgets["timetable_lecture"] + 10
    assert result.block_tokens["retrieved_chunks"] <= budgets["retrieved_chunks"] + 10

    # Total prompt tokens must strictly respect max_context_tokens
    assert result.total_tokens <= max_budget + 20

    # Chunks omitted must be tracked
    assert result.chunks_included > 0
    assert result.chunks_omitted > 0
    assert result.chunks_included + result.chunks_omitted == 15


def test_syllabus_relevance_prioritization():
    """
    Verify that syllabus modules matching the query/topic are prioritized first.
    """
    modules = [
        {"topic": "Asymptotic Complexity", "subtopics": ["Big-O", "Theta", "Omega"]},
        {"topic": "Sorting Algorithms", "subtopics": ["MergeSort", "QuickSort"]},
        {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Memoization"]},
        {"topic": "Graph Algorithms", "subtopics": ["Dijkstra", "Bellman-Ford"]},
    ]

    assembler = ContextAssembler(max_context_tokens=4000)

    # When query focuses on Dynamic Programming
    result = assembler.assemble(
        syllabus_modules=modules,
        query="optimal substructure memoization dynamic programming",
        topic="Dynamic Programming",
    )

    syllabus_block = result.blocks["syllabus"]
    dp_pos = syllabus_block.find("Dynamic Programming")
    sorting_pos = syllabus_block.find("Sorting Algorithms")

    assert dp_pos != -1
    # Dynamic Programming module must appear before Sorting Algorithms
    assert dp_pos < sorting_pos


def test_mastery_and_learning_gap_injection():
    """
    Verify injection of student mastery signals, weak topics, and active learning gaps.
    Diagnosed high-severity gaps and struggling topics must appear with highest priority.
    """
    knowledge_states = [
        StudentKnowledgeState(
            id="ks_1",
            user_id="std_123",
            course_id="course_csc301",
            topic="Asymptotic Analysis",
            subtopic="Big-O Notation",
            mastery_score=95,
            status="mastered",
            total_attempts=12,
            correct_attempts=12,
        ),
        StudentKnowledgeState(
            id="ks_2",
            user_id="std_123",
            course_id="course_csc301",
            topic="Dynamic Programming",
            subtopic="Optimal Substructure",
            mastery_score=30,
            status="struggling",
            total_attempts=10,
            correct_attempts=3,
            last_error_summary="Confuses subproblem independence with overlapping structure.",
        ),
    ]

    active_gaps = [
        LearningGap(
            id="gap_1",
            user_id="std_123",
            course_id="course_csc301",
            topic="Dynamic Programming",
            subtopic="Base Cases in Recurrence",
            gap_description="Fails to define base cases in recurrence formulations.",
            severity="high",
            status="active",
        )
    ]

    assembler = ContextAssembler()
    result = assembler.assemble(
        knowledge_states=knowledge_states,
        active_gaps=active_gaps,
        topic="Dynamic Programming",
    )

    mastery_block = result.blocks["mastery"]

    # Verify active gap is present and highlighted
    assert "[Gap - HIGH]" in mastery_block
    assert "Base Cases in Recurrence" in mastery_block
    assert "Fails to define base cases" in mastery_block

    # Verify struggling topic appears before mastered topic
    struggling_pos = mastery_block.find("Optimal Substructure")
    mastered_pos = mastery_block.find("Big-O Notation")

    assert struggling_pos != -1
    assert mastered_pos != -1
    assert struggling_pos < mastered_pos
    assert "30% [struggling]" in mastery_block
    assert "Misconception: Confuses subproblem independence" in mastery_block


def test_database_resolution_integration(db_session):
    """
    Verify ContextAssembler resolves course, timetable, lectures, and mastery states directly from DB Session.
    """
    # 1. Seed Course
    course = Course(
        id="course_csc301",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 301",
        title="Data Structures and Algorithms",
        level=300,
        credit_units=3,
        description="Comprehensive course on advanced algorithms.",
        syllabus=[
            {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Memoization"]},
            {"topic": "Greedy Algorithms", "subtopics": ["Activity Selection", "Huffman Coding"]},
        ],
    )
    db_session.add(course)

    # 2. Seed Lectures
    lec1 = Lecture(
        id="lec_14",
        course_id="course_csc301",
        lecture_number=14,
        title="Optimal Substructure & Memoization",
        date=date(2026, 9, 22),
        topics_covered=["Optimal Substructure", "Memoization"],
        summary_text="Detailed examination of optimal substructure with recurrence formulation examples.",
    )
    db_session.add(lec1)

    # 3. Seed CourseSchedule
    sched1 = CourseSchedule(
        id="sched_1",
        course_id="course_csc301",
        institution_id="inst_veritas",
        day_of_week=1,  # Monday
        start_time=time(9, 0),
        end_time=time(11, 0),
        venue="Science Lab 2",
    )
    db_session.add(sched1)

    # 4. Seed StudentKnowledgeState & LearningGap
    ks = StudentKnowledgeState(
        id="ks_dp",
        user_id="user_veritas_student",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=40,
        status="struggling",
        total_attempts=8,
        correct_attempts=3,
        last_error_summary="Misses state transitions.",
    )
    gap = LearningGap(
        id="gap_dp",
        user_id="user_veritas_student",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        gap_description="Repeatedly struggles with overlapping subproblems vs greedy choice.",
        severity="high",
        status="active",
    )
    db_session.add_all([ks, gap])
    db_session.commit()

    # 5. Assemble context resolving directly from database session
    result = ContextAssembler.assemble_context(
        db=db_session,
        user_id="user_veritas_student",
        course_id="course_csc301",
        query="optimal substructure",
        topic="Dynamic Programming",
        retrieved_chunks=[
            {
                "chunk_id": "chunk_db_test_1",
                "doc_id": "doc_lec14_notes",
                "course": "CSC 301",
                "heading": "Optimal Substructure Definition",
                "text": "An optimal solution to the problem contains within it optimal solutions to subproblems.",
            }
        ],
    )

    # Verify resolution of DB-backed entities
    assert "CSC 301 - Data Structures and Algorithms" in result.blocks["course_context"]
    assert "Lecture 14: Optimal Substructure & Memoization" in result.blocks["course_context"]
    assert "Science Lab 2" in result.blocks["course_context"]
    assert "Dynamic Programming" in result.blocks["syllabus"]
    assert "[Gap - HIGH]" in result.blocks["mastery"]
    assert "Misses state transitions." in result.blocks["mastery"]
    assert "[Chunk chunk_db_test_1]" in result.blocks["retrieved_chunks"]
    assert result.citation_ids == ["chunk_db_test_1"]
