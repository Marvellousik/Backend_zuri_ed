"""
Institutional Knowledge Graph Service for Lexi (Phase 35).
Implements an academic concept Directed Acyclic Graph (DAG) with PREREQUISITE_OF and CO_OCCURS_WITH relationships.
- Detects curricular gaps between preceding (Course A) and subsequent (Course B) courses.
- Synthesizes personalized learning paths via topological sorting of prerequisite dependencies.
"""
import logging
from collections import defaultdict, deque
from typing import Dict, Any, List, Set, Optional, Tuple
from sqlalchemy.orm import Session

from academic_service.models.orm import Course

logger = logging.getLogger(__name__)

# Relationship Edge Types
REL_PREREQUISITE_OF = "PREREQUISITE_OF"  # A -> B: Concept A must be learned before Concept B
REL_CO_OCCURS_WITH = "CO_OCCURS_WITH"    # A <-> B: Concepts are studied concurrently within a module


class ConceptDAG:
    """In-memory Directed Acyclic Graph engine for academic concept dependencies."""

    def __init__(self):
        self.nodes: Dict[str, Dict[str, Any]] = {}
        # Adjacency list: node -> list of (target_node, relation_type, weight)
        self.adj: Dict[str, List[Tuple[str, str, float]]] = defaultdict(list)
        # Reverse adjacency list for prerequisites: node -> list of required prerequisite nodes
        self.prereqs: Dict[str, Set[str]] = defaultdict(set)

    def add_concept(self, name: str, course_id: Optional[str] = None, description: Optional[str] = None):
        """Register a concept node."""
        norm = name.strip()
        if norm not in self.nodes:
            self.nodes[norm] = {
                "name": norm,
                "courses": [course_id] if course_id else [],
                "description": description or f"Academic concept: {norm}",
            }
        elif course_id and course_id not in self.nodes[norm]["courses"]:
            self.nodes[norm]["courses"].append(course_id)

    def add_relationship(self, source: str, target: str, rel_type: str = REL_PREREQUISITE_OF, weight: float = 1.0):
        """Add a directed relationship between concepts. Enforces DAG property for PREREQUISITE_OF."""
        s = source.strip()
        t = target.strip()
        self.add_concept(s)
        self.add_concept(t)

        if rel_type == REL_PREREQUISITE_OF:
            # Check if adding this edge would create a cycle
            if self._has_path(t, s):
                logger.warning(f"Cycle detected: Cannot add {s} -> {t} (path {t} -> {s} already exists). Skipping.")
                return False
            self.prereqs[t].add(s)

        self.adj[s].append((t, rel_type, weight))
        return True

    def _has_path(self, start: str, end: str) -> bool:
        """BFS to check if a directed prerequisite path exists between start and end."""
        visited = set()
        queue = deque([start])
        while queue:
            curr = queue.popleft()
            if curr == end:
                return True
            visited.add(curr)
            for neighbor, rel, _ in self.adj.get(curr, []):
                if rel == REL_PREREQUISITE_OF and neighbor not in visited:
                    queue.append(neighbor)
        return False

    def get_prerequisites_transitive(self, concept: str) -> Set[str]:
        """Finds all transitive prerequisites needed for concept."""
        norm = concept.strip()
        all_prereqs = set()
        queue = deque(list(self.prereqs.get(norm, set())))
        while queue:
            curr = queue.popleft()
            if curr not in all_prereqs:
                all_prereqs.add(curr)
                for p in self.prereqs.get(curr, set()):
                    if p not in all_prereqs:
                        queue.append(p)
        return all_prereqs

    def topological_sort(self, target_concept: str) -> List[str]:
        """
        Returns topological ordering of all prerequisites leading to target_concept.
        Foundations appear first, target appears last.
        """
        target = target_concept.strip()
        relevant_nodes = self.get_prerequisites_transitive(target)
        relevant_nodes.add(target)

        # In-degree count among relevant nodes
        in_degree = {n: 0 for n in relevant_nodes}
        for node in relevant_nodes:
            for p in self.prereqs.get(node, set()):
                if p in relevant_nodes:
                    in_degree[node] += 1

        # Queue nodes with in_degree 0
        queue = deque([n for n, deg in in_degree.items() if deg == 0])
        ordered = []

        while queue:
            curr = queue.popleft()
            ordered.append(curr)
            for neighbor, rel, _ in self.adj.get(curr, []):
                if rel == REL_PREREQUISITE_OF and neighbor in relevant_nodes:
                    in_degree[neighbor] -= 1
                    if in_degree[neighbor] == 0:
                        queue.append(neighbor)

        # In case target is not yet in ordered (or isolated)
        if target not in ordered:
            ordered.append(target)

        return ordered


class KnowledgeGraphService:
    """
    Institutional curriculum knowledge graph and curricular intelligence engine.
    """

    @classmethod
    def build_graph_from_courses(cls, courses: List[Course]) -> ConceptDAG:
        """
        Constructs a ConceptDAG from course syllabi, extracting topics, subtopics,
        and explicit prerequisite declarations.
        """
        dag = ConceptDAG()

        # Seed default foundational CS concept graph dependencies
        default_edges = [
            ("Variables & Data Types", "Control Flow & Loops"),
            ("Control Flow & Loops", "Functions & Recursion"),
            ("Functions & Recursion", "Divide and Conquer"),
            ("Functions & Recursion", "Dynamic Programming"),
            ("Arrays & Pointers", "Linked Lists"),
            ("Linked Lists", "Stacks and Queues"),
            ("Stacks and Queues", "Trees & BST"),
            ("Trees & BST", "Graph Representations"),
            ("Graph Representations", "BFS and DFS"),
            ("BFS and DFS", "Shortest Path Algorithms"),
            ("Divide and Conquer", "MergeSort"),
            ("Divide and Conquer", "QuickSort"),
            ("Dynamic Programming", "Optimal Substructure"),
            ("Dynamic Programming", "Memoization"),
            ("Dynamic Programming", "Bellman Equations"),
            ("Optimal Substructure", "Bellman-Ford Algorithm"),
            ("Shortest Path Algorithms", "Bellman-Ford Algorithm"),
        ]

        for s, t in default_edges:
            dag.add_relationship(s, t, REL_PREREQUISITE_OF)

        for course in courses:
            if not course.syllabus:
                continue

            for mod in course.syllabus:
                topic = mod.get("topic", "").strip()
                if not topic:
                    continue
                dag.add_concept(topic, course.id)

                subtopics = mod.get("subtopics", [])
                for st in subtopics:
                    st_clean = st.strip()
                    dag.add_concept(st_clean, course.id)
                    dag.add_relationship(topic, st_clean, REL_CO_OCCURS_WITH)

                prereqs = mod.get("prerequisites") or mod.get("prereqs") or []
                if isinstance(prereqs, list):
                    for pr in prereqs:
                        pr_clean = pr.strip()
                        dag.add_concept(pr_clean)
                        dag.add_relationship(pr_clean, topic, REL_PREREQUISITE_OF)

        return dag

    @classmethod
    def detect_curricular_gaps(
        cls,
        db: Session,
        course_a_id: str,
        course_b_id: str,
    ) -> Dict[str, Any]:
        """
        Compares Course A (preceding course) against Course B (subsequent course).
        Flags concepts required as prerequisites for Course B that are missing or
        insufficiently addressed in Course A's curriculum.
        """
        course_a = (
            db.query(Course)
            .filter((Course.id == course_a_id) | (Course.code.ilike(course_a_id.strip())))
            .first()
        )
        course_b = (
            db.query(Course)
            .filter((Course.id == course_b_id) | (Course.code.ilike(course_b_id.strip())))
            .first()
        )

        if not course_a or not course_b:
            raise ValueError(f"Both courses must exist to detect curricular gaps ({course_a_id}, {course_b_id}).")

        # Build concept graph
        dag = cls.build_graph_from_courses([course_a, course_b])

        # Extract concepts covered in Course A
        concepts_a: Set[str] = set()
        if course_a.syllabus:
            for mod in course_a.syllabus:
                if mod.get("topic"):
                    concepts_a.add(mod["topic"].strip())
                for st in mod.get("subtopics", []):
                    concepts_a.add(st.strip())

        # Extract prerequisite requirements for Course B
        required_for_b: Set[str] = set()
        if course_b.syllabus:
            for mod in course_b.syllabus:
                prereqs = mod.get("prerequisites") or mod.get("prereqs") or []
                for p in prereqs:
                    required_for_b.add(p.strip())
                # Also include graph prerequisites for topics in B
                topic = mod.get("topic", "").strip()
                if topic:
                    required_for_b.update(dag.get_prerequisites_transitive(topic))

        # Identify missing prerequisites
        missing = sorted(list(required_for_b - concepts_a))
        covered = sorted(list(required_for_b.intersection(concepts_a)))

        total_req = len(required_for_b)
        alignment_pct = int((len(covered) / max(1, total_req)) * 100)

        recommendations = []
        if missing:
            for m in missing[:3]:
                recommendations.append(
                    f"Incorporate module or bridge tutorial on '{m}' into {course_a.code} to prepare students for {course_b.code}."
                )
        else:
            recommendations.append(f"{course_a.code} provides 100% prerequisite coverage for {course_b.code}.")

        return {
            "course_a": {"id": course_a.id, "code": course_a.code, "title": course_a.title},
            "course_b": {"id": course_b.id, "code": course_b.code, "title": course_b.title},
            "covered_prerequisites": covered,
            "missing_prerequisites": missing,
            "curricular_alignment_score": alignment_pct,
            "recommendations": recommendations,
            "is_gap_detected": len(missing) > 0,
        }

    @classmethod
    def get_learning_path(
        cls,
        db: Session,
        target_concept: str,
    ) -> Dict[str, Any]:
        """
        Computes the complete topologically ordered learning path required to master target_concept.
        Roots (foundational concepts) appear first, progressing towards target_concept.
        """
        all_courses = db.query(Course).all()
        dag = cls.build_graph_from_courses(all_courses)

        ordered_concepts = dag.topological_sort(target_concept)

        steps = []
        for idx, concept in enumerate(ordered_concepts, start=1):
            prereqs_direct = list(dag.prereqs.get(concept, []))
            stage = "Foundational" if idx <= max(1, len(ordered_concepts) // 3) else ("Intermediate" if idx < len(ordered_concepts) else "Target Concept")
            steps.append({
                "step_number": idx,
                "concept": concept,
                "stage": stage,
                "prerequisites_needed": prereqs_direct,
                "estimated_study_hours": 2 if stage == "Foundational" else 4,
            })

        total_hours = sum(s["estimated_study_hours"] for s in steps)

        return {
            "target_concept": target_concept,
            "total_steps": len(steps),
            "estimated_total_hours": total_hours,
            "learning_path": steps,
            "prerequisite_depth": len(steps) - 1,
        }
