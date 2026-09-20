"""
Unit tests for Phase 24: Academic Document Intelligence.
Verifies hierarchical heading parsing, page tracking, table/figure extraction,
verifiable [Chunk <chunk_id>] citation claims, and syllabus module auto-binding.
"""
import pytest
from academic_service.models.orm import Course
from academic_service.services.document_intelligence_service import (
    DocumentIntelligenceService,
    DocumentIntelligenceResult,
)


def test_document_hierarchy_and_page_parsing():
    """
    Verify parsing of hierarchical headings (H1, H2, H3) and page number tracking.
    """
    doc_text = """[Page 1]
# Dynamic Programming
Dynamic programming is a method for solving complex problems by breaking them down into simpler subproblems.

[Page 2]
## Optimal Substructure
A problem exhibits optimal substructure if an optimal solution contains within it optimal solutions to subproblems.

### Bellman Formulations
The Bellman equation formalizes recursive state relationships.

--- Page 3 ---
# Greedy Algorithms
Greedy algorithms make the locally optimal choice at each stage.
"""
    result: DocumentIntelligenceResult = DocumentIntelligenceService.process_document(
        raw_text=doc_text,
        doc_id="doc_algorithms_ch4",
    )

    assert result.doc_id == "doc_algorithms_ch4"
    assert result.total_pages == 3
    assert len(result.sections) >= 3

    # Check hierarchy nesting: Level 1 should have children of level 2
    dp_h1 = next(h for h in result.hierarchy if "Dynamic Programming" in h["title"])
    assert dp_h1["level"] == 1
    assert dp_h1["page_number"] == 1
    assert len(dp_h1["children"]) >= 1

    opt_sub_h2 = dp_h1["children"][0]
    assert "Optimal Substructure" in opt_sub_h2["title"]
    assert opt_sub_h2["level"] == 2
    assert opt_sub_h2["page_number"] == 2

    # Level 3 under Level 2
    bellman_h3 = opt_sub_h2["children"][0]
    assert "Bellman Formulations" in bellman_h3["title"]
    assert bellman_h3["level"] == 3


def test_table_and_figure_extraction():
    """
    Verify detection and extraction of markdown tables and figures.
    """
    doc_text = """[Page 1]
# Algorithm Complexity Comparison

Figure 1.1: Recurrence tree state transitions
![State Tree](https://assets.lexi.internal/figures/dp_tree.png)

Here is the empirical comparison of running times:

| Algorithm | Best Case | Worst Case | Space |
| :--- | :--- | :--- | :--- |
| MergeSort | O(n log n) | O(n log n) | O(n) |
| QuickSort | O(n log n) | O(n^2) | O(log n) |

Table 2: Memory consumption comparison.
"""
    result = DocumentIntelligenceService.process_document(
        raw_text=doc_text,
        doc_id="doc_complexity",
    )

    assert len(result.figures) >= 1
    fig = result.figures[0]
    assert "Figure" in fig["label"]
    assert fig["page_number"] == 1

    assert len(result.tables) >= 1
    tbl = result.tables[0]
    assert "| MergeSort |" in tbl["content"]
    assert tbl["page_number"] == 1


def test_structured_summary_with_verifiable_chunk_citations():
    """
    Verify structured summary generation where claims cite exact chunk IDs [Chunk <chunk_id>].
    """
    doc_text = """# Memoization in Dynamic Programming
Memoization is an optimization technique used primarily to speed up computer programs by storing the results of expensive function calls.

## State Space Exploration
The state space of the problem is represented as a directed acyclic graph where each node is a subproblem.
"""
    result = DocumentIntelligenceService.process_document(
        raw_text=doc_text,
        doc_id="doc_memoization",
    )

    summary = result.summary
    assert summary.overview
    assert len(summary.claims) > 0

    for claim in summary.claims:
        assert len(claim.chunk_ids) > 0
        assert len(claim.citation_markers) > 0
        for marker in claim.citation_markers:
            assert marker.startswith("[Chunk ")
            assert marker.endswith("]")
            assert marker in claim.claim_text

    assert len(summary.citations) == len(summary.claims)
    assert all(c.startswith("[Chunk ") for c in summary.citations)


def test_auto_binding_to_course_syllabus():
    """
    Verify auto-binding of extracted topics and concepts to course syllabus modules.
    """
    syllabus = [
        {
            "topic": "Divide and Conquer",
            "subtopics": ["MergeSort", "QuickSort", "Master Theorem"],
        },
        {
            "topic": "Dynamic Programming",
            "subtopics": ["Optimal Substructure", "Overlapping Subproblems", "Memoization vs Tabulation"],
        },
        {
            "topic": "Graph Algorithms",
            "subtopics": ["BFS", "DFS", "Dijkstra"],
        },
    ]

    doc_text = """# Dynamic Programming Foundations
In this lecture we explore Optimal Substructure and Overlapping Subproblems.
We demonstrate how Memoization vs Tabulation impacts cache efficiency.
"""
    result = DocumentIntelligenceService.process_document(
        raw_text=doc_text,
        doc_id="doc_dp_lecture",
        syllabus=syllabus,
    )

    # Dynamic Programming module and subtopics should be bound
    assert "Dynamic Programming" in result.bound_syllabus_topics
    assert "Optimal Substructure" in result.bound_syllabus_topics
    assert "Overlapping Subproblems" in result.bound_syllabus_topics

    # Unrelated modules should not be bound
    assert "Graph Algorithms" not in result.bound_syllabus_topics


def test_empty_document_raises_value_error():
    """Verify that attempting to parse empty document text raises a ValueError."""
    with pytest.raises(ValueError, match="cannot be empty"):
        DocumentIntelligenceService.process_document("", "doc_empty")
