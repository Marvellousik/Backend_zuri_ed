"""
Unit tests for Phase 25: Lecture Intelligence Pipeline.
Verifies transcript timestamp segmentation, pedagogical Markdown notes generation,
companion study deck creation (flashcards + quiz questions), and database integration.
"""
from datetime import date, time
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import Base, Course, Lecture
from academic_service.services.lecture_ingestion_service import LectureIngestionService


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


def test_transcript_timestamp_segmentation():
    """
    Verify parsing of transcript text into timestamped segments with chunk IDs.
    """
    transcript = """[00:00:10] Lecturer: Welcome class, today we start Lecture 14 on Dynamic Programming.
[00:01:45] Lecturer: Optimal substructure means an optimal solution contains optimal subproblem solutions.
[00:03:20] Student: Does that apply to all recursion problems?
[00:03:35] Lecturer: Only those where subproblems overlap and have independent choices.
"""
    segments = LectureIngestionService.parse_transcript_segments(transcript, "lec_14")

    assert len(segments) == 4
    assert segments[0]["timestamp"] == "00:00:10"
    assert segments[0]["speaker"] == "Lecturer"
    assert segments[0]["chunk_id"] == "lec_14_chunk_1"
    assert "Dynamic Programming" in segments[0]["text"]

    assert segments[1]["timestamp"] == "00:01:45"
    assert segments[1]["chunk_id"] == "lec_14_chunk_2"


def test_transcript_plain_text_segmentation_fallback():
    """
    Verify fallback segmentation when transcript text lacks explicit timestamps.
    """
    transcript = """Welcome class to our discussion of Bellman equations.

Today we will explore state values and policy iteration in dynamic programming.

Remember that base cases must be initialized to avoid infinite loops.
"""
    segments = LectureIngestionService.parse_transcript_segments(transcript, "lec_plain")

    assert len(segments) == 3
    assert segments[0]["timestamp"] == "00:00"
    assert segments[1]["timestamp"] == "02:00"
    assert segments[2]["timestamp"] == "04:00"


def test_pedagogical_notes_formatting():
    """
    Verify generation of structured pedagogical Markdown notes:
    - Learning Objectives
    - Core Lecture Topics Breakdown (with [Chunk <id>] citations)
    - Key Takeaways & Exam Pointers
    - Terminology & Definition List
    """
    course = Course(
        id="c_csc301",
        code="CSC 301",
        title="Data Structures and Algorithms",
    )
    lecture = Lecture(
        id="lec_14",
        course_id="c_csc301",
        lecture_number=14,
        title="Dynamic Programming & Optimal Substructure",
    )
    segments = [
        {
            "chunk_id": "lec_14_chunk_1",
            "timestamp": "00:01:15",
            "speaker": "Lecturer",
            "text": "An optimal solution to the shortest path contains optimal subpaths.",
        },
        {
            "chunk_id": "lec_14_chunk_2",
            "timestamp": "00:03:40",
            "speaker": "Lecturer",
            "text": "Memoization saves exponential recursive calls by looking up previously computed table values.",
        },
    ]
    topics = ["Optimal Substructure", "Memoization"]

    notes = LectureIngestionService.generate_pedagogical_notes(
        lecture=lecture,
        course=course,
        segments=segments,
        topics=topics,
    )

    assert "# Lecture 14: Dynamic Programming & Optimal Substructure" in notes
    assert "## Learning Objectives" in notes
    assert "## Core Lecture Topics & Conceptual Breakdown" in notes
    assert "[Chunk lec_14_chunk_1]" in notes
    assert "## Key Takeaways & Exam Pointers" in notes
    assert "## Terminology & Definition List" in notes
    assert "**Optimal Substructure**" in notes


def test_companion_study_deck_generation():
    """
    Verify automatic generation of companion study deck (flashcards + revision quiz questions).
    """
    lecture = Lecture(
        id="lec_14",
        course_id="c_csc301",
        lecture_number=14,
        title="Dynamic Programming",
    )
    segments = [
        {
            "chunk_id": "lec_14_chunk_1",
            "timestamp": "00:02:10",
            "text": "Optimal substructure is key for dynamic programming.",
        },
    ]
    topics = ["Optimal Substructure", "State Transitions"]

    deck = LectureIngestionService.generate_companion_study_deck(
        lecture=lecture,
        segments=segments,
        topics=topics,
    )

    assert deck["deck_id"] == "deck_lec_lec_14"
    assert len(deck["flashcards"]) > 0
    assert len(deck["quiz_questions"]) > 0

    fc = deck["flashcards"][0]
    assert fc["chunk_id"] == "lec_14_chunk_1"
    assert fc["timestamp"] == "00:02:10"
    assert "Optimal Substructure" in fc["front"]

    qq = deck["quiz_questions"][0]
    assert len(qq["options"]) == 4
    assert qq["correct_answer"] == "A"
    assert "[Chunk " in qq["explanation"]


@pytest.mark.anyio
async def test_end_to_end_process_lecture_transcript(db_session):
    """
    Verify complete process_lecture_recording_or_transcript pipeline with database session.
    """
    course = Course(
        id="course_csc301",
        institution_id="inst_veritas",
        department_id="dept_cs",
        code="CSC 301",
        title="Data Structures and Algorithms",
        syllabus=[
            {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Memoization"]},
        ],
    )
    db_session.add(course)

    lecture = Lecture(
        id="lec_transcript_test",
        course_id="course_csc301",
        lecture_number=14,
        title="Dynamic Programming and Memoization",
    )
    db_session.add(lecture)
    db_session.commit()

    transcript = """[00:00:15] Lecturer: Today we study Dynamic Programming and Optimal Substructure.
[00:02:30] Lecturer: Memoization is top-down caching of subproblem solutions.
[00:05:00] Lecturer: Next time we will compare it to bottom-up tabulation.
"""

    result = await LectureIngestionService.process_lecture_recording_or_transcript(
        db=db_session,
        lecture_id="lec_transcript_test",
        transcript_text=transcript,
        audio_url="https://recordings.lexi.internal/csc301/lec14.mp3",
    )

    assert result["lecture_id"] == "lec_transcript_test"
    assert result["status"] == "processed"
    assert result["segments_count"] == 3
    assert "Dynamic Programming" in result["topics_bound"]
    assert "Optimal Substructure" in result["topics_bound"]

    # Verify lecture record in DB was updated
    updated_lec = db_session.query(Lecture).filter(Lecture.id == "lec_transcript_test").first()
    assert updated_lec.is_processed is True
    assert updated_lec.slides_url == "https://recordings.lexi.internal/csc301/lec14.mp3"
    assert updated_lec.transcript_doc_id == "transcript_lec_transcript_test"
    assert "## Learning Objectives" in updated_lec.summary_text
