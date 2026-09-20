"""
Script to generate realistic synthetic academic materials for testing:
1. Multi-page academic PDF with course header, syllabus topics, headings, complexity table, and timetable slot.
2. Academic lecture text notes for TTS audio synthesis and summarization.
3. Syllabus document for timetable extraction and curriculum gap analysis.
"""
import os
from reportlab.lib.pagesizes import letter
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib import colors

MATERIAL_DIR = os.path.join(os.path.dirname(__file__))
os.makedirs(MATERIAL_DIR, exist_ok=True)

def generate_pdf_lecture():
    pdf_path = os.path.join(MATERIAL_DIR, "csc301_dynamic_programming_lecture.pdf")
    doc = SimpleDocTemplate(
        pdf_path,
        pagesize=letter,
        rightMargin=54, leftMargin=54, topMargin=54, bottomMargin=54
    )
    
    styles = getSampleStyleSheet()
    title_style = ParagraphStyle('TitleStyle', parent=styles['Heading1'], fontSize=18, leading=22, textColor=colors.HexColor("#1A365D"))
    h2_style = ParagraphStyle('H2Style', parent=styles['Heading2'], fontSize=14, leading=18, textColor=colors.HexColor("#2B6CB0"))
    body_style = ParagraphStyle('BodyStyle', parent=styles['Normal'], fontSize=10, leading=14, textColor=colors.HexColor("#2D3748"))
    bullet_style = ParagraphStyle('BulletStyle', parent=body_style, leftIndent=20, firstLineIndent=-10)

    story = []

    # Page 1: Overview and Syllabus Binding
    story.append(Paragraph("VERITAS UNIVERSITY ABUJA", title_style))
    story.append(Paragraph("Department of Computer Science — Academic Year 2025/2026", body_style))
    story.append(Paragraph("Course Code: CSC 301 | Course Title: Advanced Algorithms and Data Structures", h2_style))
    story.append(Spacer(1, 10))
    story.append(Paragraph("<b>Lecture Module:</b> Dynamic Programming, Memoization, and Optimal Substructure", body_style))
    story.append(Paragraph("<b>Timetable Slot:</b> Every Monday, 10:00 AM – 12:00 PM | Proposed Venue: Lecture Theatre 2 (LT2)", body_style))
    story.append(Paragraph("<b>Instructor:</b> Prof. K. O. Adeleke | Credit Units: 3.0", body_style))
    story.append(Spacer(1, 15))

    story.append(Paragraph("1. Fundamental Principles of Dynamic Programming", h2_style))
    story.append(Paragraph(
        "Dynamic Programming (DP) is an algorithmic paradigm that solves complex problems by breaking them "
        "down into simpler subproblems. It is applicable when subproblems overlap and exhibits optimal substructure. "
        "Unlike Divide and Conquer which solves subproblems independently, DP guarantees that each subproblem "
        "is solved exactly once and cached.",
        body_style
    ))
    story.append(Spacer(1, 10))

    story.append(Paragraph("• <b>Overlapping Subproblems:</b> The problem can be broken down into subproblems which are reused several times.", bullet_style))
    story.append(Paragraph("• <b>Optimal Substructure:</b> The optimal solution of the problem can be constructed from optimal solutions of subproblems.", bullet_style))
    story.append(Paragraph("• <b>Memoization (Top-Down):</b> Recursive formulation storing computed solutions in a hash table or array.", bullet_style))
    story.append(Paragraph("• <b>Tabulation (Bottom-Up):</b> Iterative construction starting from base cases and building upward.", bullet_style))
    story.append(Spacer(1, 15))

    # Complexity Table
    data = [
        ["Algorithm Technique", "Time Complexity", "Space Complexity", "Recursion Overhead"],
        ["Naive Recursive Fibonacci", "O(2^n)", "O(n) stack", "High (exponential calls)"],
        ["Top-Down Memoized DP", "O(n)", "O(n) cache + stack", "Medium (linear depth)"],
        ["Bottom-Up Tabulated DP", "O(n)", "O(n) table", "None (iterative loop)"],
        ["Space-Optimized DP", "O(n)", "O(1) memory", "None (two scalar pointers)"]
    ]
    t = Table(data, colWidths=[150, 90, 110, 150])
    t.setStyle(TableStyle([
        ('BACKGROUND', (0, 0), (-1, 0), colors.HexColor("#2B6CB0")),
        ('TEXTCOLOR', (0, 0), (-1, 0), colors.whitesmoke),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ('FONTSIZE', (0, 0), (-1, 0), 9),
        ('BOTTOMPADDING', (0, 0), (-1, 0), 6),
        ('BACKGROUND', (0, 1), (-1, -1), colors.HexColor("#EDF2F7")),
        ('GRID', (0, 0), (-1, -1), 0.5, colors.HexColor("#CBD5E0")),
        ('FONTSIZE', (0, 1), (-1, -1), 8),
    ]))
    story.append(t)

    # Page 2: 0/1 Knapsack Problem & Bellman Equation
    story.append(PageBreak())
    story.append(Paragraph("2. Case Study: The 0/1 Knapsack Problem", h2_style))
    story.append(Paragraph(
        "Given weights w[1..n] and values v[1..n] with maximum capacity W. For each item i, decide whether to include "
        "(1) or exclude (0) it to maximize total value without exceeding W.",
        body_style
    ))
    story.append(Spacer(1, 10))
    story.append(Paragraph("<b>Bellman Recurrence Relation:</b>", body_style))
    story.append(Paragraph(
        "DP[i, w] = DP[i-1, w] if w[i] > w<br/>"
        "DP[i, w] = max(DP[i-1, w], DP[i-1, w - w[i]] + v[i]) if w[i] <= w",
        body_style
    ))
    story.append(Spacer(1, 15))

    story.append(Paragraph("3. Practical Applications in Industry", h2_style))
    story.append(Paragraph(
        "Dynamic programming algorithms power essential computational systems worldwide:",
        body_style
    ))
    story.append(Paragraph("• <b>Bioinformatics:</b> Needleman-Wunsch and Smith-Waterman algorithms for DNA sequence alignment.", bullet_style))
    story.append(Paragraph("• <b>Networking:</b> Bellman-Ford shortest path routing protocols in autonomous internet backbones.", bullet_style))
    story.append(Paragraph("• <b>Natural Language Processing:</b> Viterbi algorithm in Hidden Markov Models for speech recognition.", bullet_style))
    story.append(Paragraph("• <b>Compiler Design:</b> Register allocation and DAG instruction scheduling.", bullet_style))
    story.append(Spacer(1, 20))

    story.append(Paragraph("<b>Important Student Notice:</b>", body_style))
    story.append(Paragraph(
        "Assignment 1 is due on Friday by 23:59. Midterm diagnostic assessment covering Divide & Conquer and "
        "Dynamic Programming will be conducted during the next scheduled class.",
        body_style
    ))

    doc.build(story)
    print(f"Generated PDF: {pdf_path} ({os.path.getsize(pdf_path)} bytes)")
    return pdf_path

def generate_lecture_text():
    text_path = os.path.join(MATERIAL_DIR, "dynamic_programming_notes.txt")
    content = """CSC 301: Advanced Algorithms — Dynamic Programming Lecture Notes
University: Veritas University, Abuja
Module: Dynamic Programming, Memoization, and Optimal Substructure

1. Overview
Dynamic Programming is an algorithmic technique for solving optimization problems by breaking them down into simpler overlapping subproblems. Instead of solving identical subproblems repeatedly, Dynamic Programming solves each subproblem once and stores the result in memory for constant time lookup.

2. Core Criteria for Dynamic Programming
A problem can be efficiently solved with Dynamic Programming if and only if it exhibits two fundamental properties:
Property 1: Overlapping Subproblems. The recursive space contains identical subproblems evaluated repeatedly. For example, in computing the nth Fibonacci number, calculating Fib(n-1) and Fib(n-2) both independently compute Fib(n-3).
Property 2: Optimal Substructure. The optimal solution to the global problem incorporates optimal solutions to its constituent subproblems.

3. Top-Down Memoization vs Bottom-Up Tabulation
Top-Down Memoization preserves the intuitive recursive structure of the problem while introducing a memoization cache such as a hash map or array. When a subproblem is called, the algorithm first checks if the result exists in the cache.
Bottom-Up Tabulation eliminates recursion completely. The algorithm initializes base cases in a table and iteratively computes subsequent states in topological order until reaching the target state.

4. 0/1 Knapsack Problem Formulation
In the 0/1 Knapsack problem, we are given n items with weights w_1 to w_n and values v_1 to v_n, and a maximum knapsack capacity W. For each item i, we can either choose to include it or exclude it.
The recurrence relation is:
DP(i, w) equals DP(i-1, w) if weight w_i exceeds w.
Otherwise, DP(i, w) equals the maximum of DP(i-1, w) and DP(i-1, w - w_i) plus v_i.
The time complexity is O(n times W), which is pseudo-polynomial in nature.

5. Key Vocabulary and Definitions
- Memoization: Caching the output of deterministic function calls to avoid recomputation.
- Optimal Substructure: Property where an optimal solution is composed of optimal solutions to subproblems.
- Overlapping Subproblems: Repeated evaluation of identical subproblems in recursion trees.
- State Transition: The mathematical formula expressing a state in terms of previously computed states.
"""
    with open(text_path, "w", encoding="utf-8") as f:
        f.write(content.strip())
    print(f"Generated Text: {text_path} ({len(content)} characters)")
    return text_path

def generate_syllabus_text():
    syllabus_path = os.path.join(MATERIAL_DIR, "csc301_course_syllabus.txt")
    content = """COURSE SYLLABUS — VERITAS UNIVERSITY
Course: CSC 301 (Advanced Algorithms and Data Structures)
Credit Units: 3.0 | Semester: 2025/2026 First Semester
Department: Computer Science | Faculty: Natural and Applied Sciences

LECTURE TIMETABLE AND SCHEDULE:
- Lecture Day: Monday
- Lecture Time: 10:00 AM - 12:00 PM
- Official Venue: Lecture Theatre 2 (LT2)
- Lab Practical: Wednesday 02:00 PM - 04:00 PM in Software Lab 1

SYLLABUS MODULES AND PREREQUISITE GRAPH:
Module 1: Asymptotic Analysis and Recurrence Relations (Master Theorem, Substitution Method)
Module 2: Divide and Conquer (MergeSort, QuickSort, Closest Pair of Points)
Module 3: Dynamic Programming (Memoization, Tabulation, 0/1 Knapsack, LCS)
Module 4: Greedy Algorithms (Huffman Coding, Fractional Knapsack, Kruskal's MST)
Module 5: Graph Theory (Breadth-First Search, Depth-First Search, Dijkstra, Bellman-Ford)

COURSE PREREQUISITES:
- Prerequisite 1: CSC 201 (Data Structures and Algorithms I)
- Prerequisite 2: MTH 201 (Discrete Mathematics)
"""
    with open(syllabus_path, "w", encoding="utf-8") as f:
        f.write(content.strip())
    print(f"Generated Syllabus: {syllabus_path} ({len(content)} characters)")
    return syllabus_path

if __name__ == "__main__":
    generate_pdf_lecture()
    generate_lecture_text()
    generate_syllabus_text()
    print("All test materials successfully created in tests/test_materials/")
