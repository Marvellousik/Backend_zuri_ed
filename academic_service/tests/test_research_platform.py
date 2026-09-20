"""
Unit tests for Phase 34: Academic Research Platform.
Verifies page-level paper chunking, literature matrix comparative synthesis
(methodology, dataset, findings, limitations), and exact citation provenance
[Paper: <id>, Page: <page>, Chunk: <chunk_id>].
"""
import pytest
from academic_service.services.research_platform_service import (
    ResearchPlatformService,
    PaperChunk,
)


def test_chunk_paper_pages_citation_provenance():
    """
    Verify that academic papers are chunked per page with strict citation markers:
    [Paper: <id>, Page: <page>, Chunk: <chunk_id>]
    """
    pages_text = {
        1: "Abstract: We propose an adaptive neural retrieval architecture.\n\nIntroduction: Information retrieval has evolved.",
        2: "Methodology: We utilize 1024-dimensional dense Cohere vectors combined with Reciprocal Rank Fusion k=60.",
        3: "Empirical Results: Our model achieves 94.2% top-5 recall on academic test collections.",
    }

    chunks = ResearchPlatformService.chunk_paper_pages("paper_rag_2025", pages_text)

    assert len(chunks) >= 3
    c1 = chunks[0]
    assert c1.paper_id == "paper_rag_2025"
    assert c1.page_number == 1
    assert c1.citation_marker == f"[Paper: paper_rag_2025, Page: 1, Chunk: {c1.chunk_id}]"

    c2 = chunks[1]
    assert c2.page_number == 2
    assert "Page: 2" in c2.citation_marker


def test_synthesize_literature_matrix_comparative_dimensions():
    """
    Verify synthesis of comparative literature matrix with methodology, dataset, findings,
    limitations, consensus points, and open debates with exact citations.
    """
    papers = [
        {
            "paper_id": "paper_karp_1972",
            "title": "Reducibility Among Combinatorial Problems",
            "authors": ["Richard M. Karp"],
            "year": 1972,
            "pages_text": {
                1: "Abstract: We present 21 NP-complete combinatorial problems.",
                2: "Methodology: Polynomial-time many-one reductions from SAT.",
                3: "Findings: Proved NP-completeness for Knapsack, Clique, and Hamiltonian Circuit.",
                4: "Limitations: Focuses on worst-case asymptotic bounds without average-case analysis.",
            },
        },
        {
            "paper_id": "paper_cook_1971",
            "title": "The Complexity of Theorem-Proving Procedures",
            "authors": ["Stephen Cook"],
            "year": 1971,
            "pages_text": {
                1: "Abstract: Introduction of non-deterministic Turing machine reductions.",
                2: "Methodology: Formal Turing reduction of arbitrary NP languages to boolean satisfiability.",
                3: "Findings: Proved SAT is NP-complete.",
                4: "Limitations: Restricted to decision problems.",
            },
        },
    ]

    research_q = "How do polynomial-time reductions establish NP-completeness across decision problems?"
    matrix_result = ResearchPlatformService.synthesize_literature_matrix(papers, research_q)

    assert matrix_result["research_question"] == research_q
    assert matrix_result["total_papers"] == 2
    assert len(matrix_result["matrix"]) == 2

    # Check row 1 dimensions
    row1 = matrix_result["matrix"][0]
    assert row1["paper_id"] == "paper_karp_1972"
    assert "Richard M. Karp" in row1["authors"]
    assert "methodology" in row1
    assert "dataset" in row1
    assert "findings" in row1
    assert "limitations" in row1

    # Check citations in row 1
    assert len(row1["citations"]) > 0
    for cite in row1["citations"]:
        assert cite.startswith("[Paper: paper_karp_1972, Page: ")
        assert ", Chunk: " in cite
        assert cite.endswith("]")

    # Check comparative synthesis and consensus
    assert len(matrix_result["consensus_points"]) > 0
    assert len(matrix_result["open_debates"]) > 0
    assert len(matrix_result["all_citations"]) >= 4


def test_empty_papers_raises_value_error():
    """Verify that passing an empty paper list raises a ValueError."""
    with pytest.raises(ValueError, match="At least one academic paper is required"):
        ResearchPlatformService.synthesize_literature_matrix([], "What is DP?")
