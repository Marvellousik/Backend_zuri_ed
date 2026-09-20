"""
Hybrid Vector & Lexical Retrieval Tool for LexiAssist AI Infrastructure (Phase 21).
Combines dense pgvector cosine similarity (1024-dim Cohere embeddings) and sparse full-text search
(PostgreSQL tsvector / lexical token matching) using Reciprocal Rank Fusion (RRF k=60).
Enforces strict multi-tenant institutional and course boundaries.
"""
import re
import json
import logging
from typing import Dict, Any, Tuple, List, Optional, Sequence
from sqlalchemy import func
from sqlalchemy.orm import Session

from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.storage.database import get_db_session
from ai_service.storage.models import LexiChunk

logger = logging.getLogger(__name__)

RRF_K_DEFAULT = 60


def compute_cosine_similarity(v1: Sequence[float], v2: Sequence[float]) -> float:
    """Computes cosine similarity between two numeric vectors in Python."""
    if not v1 or not v2 or len(v1) != len(v2):
        return 0.0
    dot = sum(a * b for a, b in zip(v1, v2))
    norm1 = sum(a * a for a in v1) ** 0.5
    norm2 = sum(b * b for b in v2) ** 0.5
    if norm1 == 0.0 or norm2 == 0.0:
        return 0.0
    return float(dot / (norm1 * norm2))


def compute_token_match_score(text: str, query: str) -> float:
    """
    Fallback lexical scoring when PostgreSQL tsvector is unavailable.
    Performs tokenized term-frequency matching with phrase proximity weighting.
    """
    if not text or not query:
        return 0.0
    tokens = [t.lower() for t in re.findall(r"\w+", query) if len(t) > 1]
    if not tokens:
        return 0.0
    text_lower = text.lower()
    score = 0.0

    # Phrase match bonus
    if query.lower() in text_lower:
        score += 5.0

    for tok in tokens:
        count = text_lower.count(tok)
        if count > 0:
            score += 1.0 + 0.1 * min(count, 10)

    return score


def reciprocal_rank_fusion(
    dense_results: List[Tuple[LexiChunk, float]],
    sparse_results: List[Tuple[LexiChunk, float]],
    k: int = RRF_K_DEFAULT,
    top_k: int = 5,
) -> List[Dict[str, Any]]:
    """
    Combines dense and sparse ranked lists using Reciprocal Rank Fusion:
    Score(d) = sum(1.0 / (k + rank_i(d))) across dense rank and sparse rank.
    Ranks are 1-based (1, 2, 3...).
    """
    scores: Dict[str, float] = {}
    chunk_map: Dict[str, LexiChunk] = {}
    dense_ranks: Dict[str, int] = {}
    sparse_ranks: Dict[str, int] = {}
    sim_scores: Dict[str, float] = {}

    for rank_idx, (chunk, sim) in enumerate(dense_results):
        rank = rank_idx + 1
        cid = chunk.id
        chunk_map[cid] = chunk
        dense_ranks[cid] = rank
        sim_scores[cid] = sim
        scores[cid] = scores.get(cid, 0.0) + (1.0 / (k + rank))

    for rank_idx, (chunk, _lex_score) in enumerate(sparse_results):
        rank = rank_idx + 1
        cid = chunk.id
        chunk_map[cid] = chunk
        sparse_ranks[cid] = rank
        scores[cid] = scores.get(cid, 0.0) + (1.0 / (k + rank))

    # Sort documents by RRF score descending. Ties broken by dense rank then sparse rank.
    sorted_ids = sorted(
        scores.keys(),
        key=lambda cid: (
            scores[cid],
            -dense_ranks.get(cid, 999999),
            -sparse_ranks.get(cid, 999999),
        ),
        reverse=True,
    )

    top_ids = sorted_ids[:top_k]
    merged: List[Dict[str, Any]] = []

    for cid in top_ids:
        chunk = chunk_map[cid]
        heading = getattr(chunk, "heading", None)
        section = getattr(chunk, "section", None)
        heading_or_section = heading or section or ""

        merged.append({
            "chunk_id": chunk.id,
            "doc_id": chunk.doc_id,
            "course": chunk.course,
            "chunk_index": chunk.chunk_index,
            "heading": heading,
            "section": section,
            "heading_or_section": heading_or_section,
            "text": chunk.chunk_text,
            "rrf_score": round(scores[cid], 6),
            "dense_rank": dense_ranks.get(cid),
            "sparse_rank": sparse_ranks.get(cid),
            "similarity_score": round(sim_scores[cid], 4) if cid in sim_scores else None,
        })

    return merged


def search_dense(
    session: Session,
    query_vector: Optional[List[float]],
    institution_id: Optional[str] = None,
    course_code: Optional[str] = None,
    material_id: Optional[str] = None,
    candidate_k: int = 20,
) -> List[Tuple[LexiChunk, float]]:
    """
    Executes dense vector search on LexiChunk.embedding with multi-tenant isolation.
    Uses pgvector native cosine distance if available, with Python fallback.
    """
    if not query_vector:
        return []

    # 1. Attempt PostgreSQL pgvector native query
    try:
        distance = LexiChunk.embedding.cosine_distance(query_vector)
        q = session.query(LexiChunk, distance.label("distance"))
        if institution_id:
            q = q.filter(LexiChunk.institution_id == institution_id)
        if course_code:
            q = q.filter(LexiChunk.course == course_code)
        if material_id:
            q = q.filter(LexiChunk.doc_id == material_id)

        rows = q.order_by("distance").limit(candidate_k).all()
        return [(row.LexiChunk, round(1.0 - float(row.distance), 4)) for row in rows]
    except Exception as e:
        logger.debug(f"Native pgvector search skipped or unavailable ({e}); falling back to Python cosine similarity")

    # 2. In-memory / Python cosine similarity fallback (for test / mock / SQLite environments)
    try:
        q = session.query(LexiChunk)
        if institution_id:
            q = q.filter(LexiChunk.institution_id == institution_id)
        if course_code:
            q = q.filter(LexiChunk.course == course_code)
        if material_id:
            q = q.filter(LexiChunk.doc_id == material_id)

        candidates = q.all()
        scored: List[Tuple[LexiChunk, float]] = []
        for c in candidates:
            emb = c.embedding
            if isinstance(emb, str):
                try:
                    emb = json.loads(emb)
                except Exception:
                    continue
            if emb and len(emb) == len(query_vector):
                sim = compute_cosine_similarity(query_vector, emb)
                scored.append((c, sim))

        scored.sort(key=lambda x: x[1], reverse=True)
        return scored[:candidate_k]
    except Exception as e:
        logger.warning(f"Dense fallback search failed: {e}")
        return []


def search_sparse(
    session: Session,
    query: str,
    institution_id: Optional[str] = None,
    course_code: Optional[str] = None,
    material_id: Optional[str] = None,
    candidate_k: int = 20,
) -> List[Tuple[LexiChunk, float]]:
    """
    Executes sparse lexical search with multi-tenant isolation.
    Uses PostgreSQL to_tsvector @@ plainto_tsquery when available, with token match fallback.
    """
    if not query or not query.strip():
        return []

    # 1. Attempt PostgreSQL full-text search
    try:
        ts_vec = func.to_tsvector("english", LexiChunk.chunk_text)
        ts_q = func.plainto_tsquery("english", query)
        match_expr = ts_vec.op("@@")(ts_q)
        rank_expr = func.ts_rank(ts_vec, ts_q).label("lexical_rank")

        q = session.query(LexiChunk, rank_expr).filter(match_expr)
        if institution_id:
            q = q.filter(LexiChunk.institution_id == institution_id)
        if course_code:
            q = q.filter(LexiChunk.course == course_code)
        if material_id:
            q = q.filter(LexiChunk.doc_id == material_id)

        rows = q.order_by(rank_expr.desc()).limit(candidate_k).all()
        if rows:
            return [(row.LexiChunk, float(row.lexical_rank)) for row in rows]
    except Exception as e:
        logger.debug(f"PostgreSQL tsvector query unavailable ({e}); falling back to lexical token matching")

    # 2. Fallback SQL / Python token matching
    try:
        q = session.query(LexiChunk)
        if institution_id:
            q = q.filter(LexiChunk.institution_id == institution_id)
        if course_code:
            q = q.filter(LexiChunk.course == course_code)
        if material_id:
            q = q.filter(LexiChunk.doc_id == material_id)

        candidates = q.all()
        scored: List[Tuple[LexiChunk, float]] = []
        for c in candidates:
            score = compute_token_match_score(c.chunk_text, query)
            if score > 0:
                scored.append((c, score))

        scored.sort(key=lambda x: x[1], reverse=True)
        return scored[:candidate_k]
    except Exception as e:
        logger.warning(f"Sparse fallback search failed: {e}")
        return []


class RetrievalTool(BaseTool):
    """
    Handles retrieval.search operations using Phase 21 Hybrid Vector Retrieval (pgvector + RRF).
    Fuses dense 1024-dim Cohere embeddings and sparse lexical full-text search with constant k=60.
    """

    @property
    def operation_prefix(self) -> str:
        return "retrieval"

    @classmethod
    def hybrid_search(
        cls,
        session: Session,
        query: str,
        query_vector: Optional[List[float]] = None,
        institution_id: Optional[str] = None,
        course_code: Optional[str] = None,
        material_id: Optional[str] = None,
        top_k: int = 5,
        k: int = RRF_K_DEFAULT,
        candidate_k: int = 20,
    ) -> List[Dict[str, Any]]:
        """
        Executes hybrid dense + sparse retrieval and applies RRF (k=60) directly.
        """
        dense_results = search_dense(
            session=session,
            query_vector=query_vector,
            institution_id=institution_id,
            course_code=course_code,
            material_id=material_id,
            candidate_k=candidate_k,
        )
        sparse_results = search_sparse(
            session=session,
            query=query,
            institution_id=institution_id,
            course_code=course_code,
            material_id=material_id,
            candidate_k=candidate_k,
        )
        return reciprocal_rank_fusion(
            dense_results=dense_results,
            sparse_results=sparse_results,
            k=k,
            top_k=top_k,
        )

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        query = ctx.input.get("query", "")
        top_k = ctx.parameters.get("top_k", 5)
        course_code = ctx.course.course_id or ctx.input.get("course_code") or ctx.input.get("course_id")
        institution_id = ctx.tenant.institution_id or ctx.input.get("institution_id")
        material_id = ctx.input.get("material_id") or ctx.input.get("doc_id")
        rrf_k = ctx.parameters.get("k", RRF_K_DEFAULT)
        candidate_k = ctx.parameters.get("candidate_k", max(top_k * 4, 20))

        if not query:
            raise ValueError("Query cannot be empty for retrieval")

        # 1. Resolve or compute query embedding
        query_vector = ctx.input.get("query_vector") or ctx.input.get("query_embedding")
        if not query_vector and hasattr(gateway, "cohere_adapter") and gateway.cohere_adapter:
            try:
                query_vector = await gateway.cohere_adapter.embed_query(query)
            except Exception as e:
                logger.warning(f"Query embedding generation failed: {e}; relying on sparse retrieval")
                query_vector = None

        # 2. Execute Hybrid Search within database session
        with get_db_session() as session:
            dense_results = search_dense(
                session=session,
                query_vector=query_vector,
                institution_id=institution_id,
                course_code=course_code,
                material_id=material_id,
                candidate_k=candidate_k,
            )
            sparse_results = search_sparse(
                session=session,
                query=query,
                institution_id=institution_id,
                course_code=course_code,
                material_id=material_id,
                candidate_k=candidate_k,
            )

            # 3. Fuse with Reciprocal Rank Fusion (k=60)
            merged_results = reciprocal_rank_fusion(
                dense_results=dense_results,
                sparse_results=sparse_results,
                k=rrf_k,
                top_k=top_k,
            )

        result = {
            "query": query,
            "results_count": len(merged_results),
            "chunks": merged_results,
            "fusion_method": "rrf",
            "k": rrf_k,
            "dense_candidates_count": len(dense_results),
            "sparse_candidates_count": len(sparse_results),
        }

        usage = TokenUsage(total_tokens=len(query) // 4)
        return result, usage, {
            "provider": "hybrid_pgvector_rrf",
            "model": "embed-multilingual-v3.0 + tsvector",
        }
