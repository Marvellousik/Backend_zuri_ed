"""
Unit tests for Phase 21: Hybrid Vector Retrieval (pgvector + RRF).
Verifies Reciprocal Rank Fusion (RRF k=60), dense and sparse ranking,
metadata provenance, and strict multi-tenant isolation.
"""
import uuid
import asyncio
from unittest.mock import MagicMock, AsyncMock, patch
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from ai_service.storage.models import Base, LexiChunk
from ai_service.contracts.context import (
    AIRequestContext,
    PrincipalContext,
    TenantContext,
    CourseContext,
    UserRole,
)
from ai_service.tools.retrieval_tool import (
    RetrievalTool,
    reciprocal_rank_fusion,
    search_dense,
    search_sparse,
    compute_cosine_similarity,
    compute_token_match_score,
    RRF_K_DEFAULT,
)


from sqlalchemy.pool import StaticPool
from sqlalchemy import event, text

@pytest.fixture
def db_session():
    """In-memory SQLite database session fixture with attached ai schema."""
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )

    @event.listens_for(engine, "connect")
    def do_connect(dbapi_connection, connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("ATTACH DATABASE ':memory:' AS ai")
        cursor.close()

    Base.metadata.create_all(bind=engine)
    Session = sessionmaker(bind=engine)
    session = Session()
    try:
        yield session
    finally:
        session.close()


def test_rrf_scoring_formula():
    """
    Verify RRF score calculation with constant k=60:
    Score(d) = sum(1.0 / (k + rank_i(d)))
    """
    chunk1 = LexiChunk(
        id="chunk_1",
        doc_id="doc_1",
        institution_id="inst_veritas",
        course="CSC 301",
        chunk_index=0,
        chunk_text="Dynamic programming memoization and optimal substructure.",
        heading="Introduction to DP",
        section="Optimal Substructure",
    )
    chunk2 = LexiChunk(
        id="chunk_2",
        doc_id="doc_1",
        institution_id="inst_veritas",
        course="CSC 301",
        chunk_index=1,
        chunk_text="Bellman equation and value iteration in dynamic programming.",
        heading="DP Equations",
        section="Bellman Formulations",
    )
    chunk3 = LexiChunk(
        id="chunk_3",
        doc_id="doc_2",
        institution_id="inst_veritas",
        course="CSC 301",
        chunk_index=0,
        chunk_text="Divide and conquer merge sort algorithm recursion.",
        heading="Sorting",
        section="Divide and Conquer",
    )

    # Dense ranking: chunk1 (rank 1), chunk2 (rank 2)
    dense_results = [(chunk1, 0.95), (chunk2, 0.85)]
    # Sparse ranking: chunk2 (rank 1), chunk1 (rank 2), chunk3 (rank 3)
    sparse_results = [(chunk2, 10.0), (chunk1, 8.0), (chunk3, 4.0)]

    merged = reciprocal_rank_fusion(
        dense_results=dense_results,
        sparse_results=sparse_results,
        k=60,
        top_k=5,
    )

    assert len(merged) == 3

    # Theoretical RRF scores for k=60:
    # chunk1: 1/(60+1) [dense 1] + 1/(60+2) [sparse 2] = 1/61 + 1/62 = 0.01639344 + 0.01612903 = 0.032522
    # chunk2: 1/(60+2) [dense 2] + 1/(60+1) [sparse 1] = 1/62 + 1/61 = 0.032522
    # chunk3: 0 [dense] + 1/(60+3) [sparse 3] = 1/63 = 0.015873
    expected_c1 = round(1.0 / 61 + 1.0 / 62, 6)
    expected_c3 = round(1.0 / 63, 6)

    c1_res = next(c for c in merged if c["chunk_id"] == "chunk_1")
    c2_res = next(c for c in merged if c["chunk_id"] == "chunk_2")
    c3_res = next(c for c in merged if c["chunk_id"] == "chunk_3")

    assert c1_res["rrf_score"] == expected_c1
    assert c2_res["rrf_score"] == expected_c1
    assert c3_res["rrf_score"] == expected_c3

    # Check rank metadata
    assert c1_res["dense_rank"] == 1
    assert c1_res["sparse_rank"] == 2
    assert c2_res["dense_rank"] == 2
    assert c2_res["sparse_rank"] == 1
    assert c3_res["dense_rank"] is None
    assert c3_res["sparse_rank"] == 3

    # Check section / heading metadata
    assert c1_res["heading"] == "Introduction to DP"
    assert c1_res["section"] == "Optimal Substructure"
    assert c1_res["heading_or_section"] == "Introduction to DP"


def test_cosine_similarity_calculation():
    """Verify vector cosine similarity calculation logic."""
    v1 = [1.0, 0.0, 0.0]
    v2 = [1.0, 0.0, 0.0]
    assert compute_cosine_similarity(v1, v2) == 1.0

    v_ortho = [0.0, 1.0, 0.0]
    assert compute_cosine_similarity(v1, v_ortho) == 0.0

    v_diag = [1.0, 1.0, 0.0]
    # Cosine between [1,0,0] and [1,1,0] is 1 / sqrt(2) ~ 0.7071
    sim = compute_cosine_similarity(v1, v_diag)
    assert round(sim, 4) == 0.7071


def test_token_match_score():
    """Verify fallback lexical token matching score."""
    text = "Dynamic programming solves problems by caching overlapping subproblems."
    query = "dynamic programming"
    score = compute_token_match_score(text, query)
    assert score > 0.0

    no_match_query = "quantum physics entanglement"
    score_zero = compute_token_match_score(text, no_match_query)
    assert score_zero == 0.0


def test_hybrid_search_and_multi_tenant_isolation(db_session):
    """
    Verify strict multi-tenant isolation across institution_id and course_id.
    Chunks from unauthorized tenants or other courses must NEVER be returned.
    """
    # Create test vectors of dimension 4
    vec_target = [1.0, 0.0, 0.0, 0.0]
    vec_close = [0.9, 0.1, 0.0, 0.0]
    vec_far = [0.0, 0.0, 1.0, 0.0]

    # Target tenant & course chunk
    target_chunk = LexiChunk(
        id="target_chunk_1",
        doc_id="doc_csc301_notes",
        institution_id="inst_veritas",
        course="CSC 301",
        chunk_index=0,
        chunk_text="Veritas CSC 301 lecture on dynamic programming optimal substructure.",
        heading="Chapter 4",
        section="Optimal Substructure",
        embedding=vec_target,
    )

    # Different tenant, same course
    other_tenant_chunk = LexiChunk(
        id="leak_other_tenant",
        doc_id="doc_other_notes",
        institution_id="inst_covenant",
        course="CSC 301",
        chunk_index=0,
        chunk_text="Covenant CSC 301 lecture on dynamic programming optimal substructure.",
        heading="Chapter 4",
        section="Optimal Substructure",
        embedding=vec_target,
    )

    # Same tenant, different course
    other_course_chunk = LexiChunk(
        id="leak_other_course",
        doc_id="doc_csc201_notes",
        institution_id="inst_veritas",
        course="CSC 201",
        chunk_index=0,
        chunk_text="Veritas CSC 201 lecture on dynamic programming optimal substructure.",
        heading="Chapter 1",
        section="Introduction",
        embedding=vec_target,
    )

    # Second target chunk with lower similarity
    target_chunk_2 = LexiChunk(
        id="target_chunk_2",
        doc_id="doc_csc301_notes",
        institution_id="inst_veritas",
        course="CSC 301",
        chunk_index=1,
        chunk_text="Veritas CSC 301 memoization table vs tabulation in dynamic programming.",
        heading="Chapter 4",
        section="Memoization",
        embedding=vec_close,
    )

    db_session.add_all([target_chunk, other_tenant_chunk, other_course_chunk, target_chunk_2])
    db_session.commit()

    # Search with strict Veritas + CSC 301 boundaries
    results = RetrievalTool.hybrid_search(
        session=db_session,
        query="dynamic programming optimal substructure",
        query_vector=vec_target,
        institution_id="inst_veritas",
        course_code="CSC 301",
        top_k=5,
        k=60,
    )

    result_ids = [r["chunk_id"] for r in results]

    # Verify target chunks are retrieved
    assert "target_chunk_1" in result_ids
    assert "target_chunk_2" in result_ids

    # Verify strict multi-tenant and course isolation: NO leaks
    assert "leak_other_tenant" not in result_ids, "Cross-tenant leak detected!"
    assert "leak_other_course" not in result_ids, "Cross-course leak detected!"

    # Top chunk should be target_chunk_1 (both vector cosine 1.0 and exact lexical match)
    top_result = results[0]
    assert top_result["chunk_id"] == "target_chunk_1"
    assert top_result["dense_rank"] == 1
    assert top_result["sparse_rank"] == 1
    assert top_result["heading"] == "Chapter 4"
    assert top_result["section"] == "Optimal Substructure"


def test_retrieval_tool_execute_with_gateway():
    """
    Verify RetrievalTool.execute async pipeline interface with mock gateway adapter.
    """
    tool = RetrievalTool()
    assert tool.operation_prefix == "retrieval"

    ctx = AIRequestContext(
        request_id="req_test_retrieval_01",
        trace_id="trace_test_01",
        session_id="sess_test_01",
        principal=PrincipalContext(user_id="user_student_1", role=UserRole.STUDENT),
        tenant=TenantContext(institution_id="inst_veritas"),
        course=CourseContext(course_id="CSC 301"),
        operation="retrieval.search",
        input={
            "query": "dynamic programming optimal substructure",
            "course_code": "CSC 301",
            "query_vector": [1.0, 0.0, 0.0, 0.0],
        },
        parameters={"top_k": 3, "k": 60},
    )

    mock_gateway = MagicMock()
    mock_gateway.cohere_adapter = AsyncMock()

    # Mock get_db_session to return mock DB
    mock_chunk = LexiChunk(
        id="chunk_mock_1",
        doc_id="doc_mock_1",
        institution_id="inst_veritas",
        course="CSC 301",
        chunk_index=0,
        chunk_text="Dynamic programming solves problems by caching overlapping subproblems.",
        heading="DP Basics",
        section="Subproblems",
    )

    with patch("ai_service.tools.retrieval_tool.get_db_session") as mock_get_session:
        mock_session = MagicMock()
        mock_get_session.return_value.__enter__.return_value = mock_session

        with patch("ai_service.tools.retrieval_tool.search_dense") as mock_dense:
            with patch("ai_service.tools.retrieval_tool.search_sparse") as mock_sparse:
                mock_dense.return_value = [(mock_chunk, 0.98)]
                mock_sparse.return_value = [(mock_chunk, 12.0)]

                res, usage, meta = asyncio.run(tool.execute(ctx, mock_gateway))

                assert res["query"] == "dynamic programming optimal substructure"
                assert res["fusion_method"] == "rrf"
                assert res["k"] == 60
                assert len(res["chunks"]) == 1
                assert res["chunks"][0]["chunk_id"] == "chunk_mock_1"
                assert res["chunks"][0]["dense_rank"] == 1
                assert res["chunks"][0]["sparse_rank"] == 1
                assert usage.total_tokens > 0
                assert meta["provider"] == "hybrid_pgvector_rrf"


def test_empty_query_raises_validation_error():
    """Verify that an empty search query raises a ValueError."""
    tool = RetrievalTool()
    ctx = AIRequestContext(
        request_id="req_err",
        trace_id="tr_err",
        principal=PrincipalContext(user_id="user_1", role=UserRole.STUDENT),
        tenant=TenantContext(institution_id="inst_veritas"),
        course=CourseContext(course_id="CSC 301"),
        operation="retrieval.search",
        input={"query": ""},
    )
    with pytest.raises(ValueError, match="Query cannot be empty"):
        asyncio.run(tool.execute(ctx, MagicMock()))
