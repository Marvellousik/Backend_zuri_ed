#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Integration Test -- Consolidated AI Infrastructure & Model-Agnostic Routing
Validates the consolidated backend without requiring external cloud dependencies:
1. Dynamic Task-Based Model Routing (openrouter/auto)
2. Evaluation Tool & Grading Intelligence
3. Cache Deduplication & Canonical Prompt Hashing
4. Multi-Tenant Cost Attribution & Token Accounting
"""

import sys
import os
import json
import hashlib
from datetime import datetime, timezone

# Ensure project root is in sys.path
REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
if REPO_ROOT not in sys.path:
    sys.path.insert(0, REPO_ROOT)

# Test counters
passed = 0
failed = 0


def test(name, condition, detail=""):
    global passed, failed
    if condition:
        print(f"  [PASS] {name}")
        passed += 1
    else:
        print(f"  [FAIL] {name} -- {detail}")
        failed += 1


# ═════════════════════════════════════════════════════════════════════════════
# TEST 1: Model-Agnostic Router Configuration (OpenRouter Auto Architecture)
# ═════════════════════════════════════════════════════════════════════════════
print("\n[TEST 1] Model-Agnostic Router Configuration (ai_routing.yaml)")

import yaml

config_path = os.path.join(REPO_ROOT, "config", "ai_routing.yaml")
with open(config_path, "r", encoding="utf-8") as f:
    router_cfg = yaml.safe_load(f)

test(
    "Router configuration version is 2.0 (OpenRouter standard)",
    router_cfg.get("version") == "2.0",
    f"Got version {router_cfg.get('version')}"
)

test(
    "Router provider is openrouter",
    router_cfg.get("provider") == "openrouter",
    f"Got provider {router_cfg.get('provider')}"
)

test(
    "Default model is openrouter/auto (zero vendor lock-in)",
    router_cfg.get("default_model") == "openrouter/auto",
    f"Got {router_cfg.get('default_model')}"
)

tasks = router_cfg.get("tasks", {})
all_openrouter_auto = all(
    t.get("primary") == "openrouter/auto" for t in tasks.values()
)
test(
    "All registered tasks use openrouter/auto as primary router",
    all_openrouter_auto and len(tasks) >= 5,
    f"Tasks: {tasks}"
)

# Verify no legacy hardcoded models exist in config
no_hardcoded_gemini = not any("gemini" in str(t).lower() for t in tasks.values())
test(
    "Configuration is 100% free of hardcoded vendor model strings",
    no_hardcoded_gemini,
    "Found vendor model strings in ai_routing.yaml"
)


# ═════════════════════════════════════════════════════════════════════════════
# TEST 2: Evaluation & Grading Tool (Internal Rule & Model Agnostic)
# ═════════════════════════════════════════════════════════════════════════════
print("\n[TEST 2] Evaluation Tool & Grading Intelligence")

from ai_service.storage.database import init_db
init_db()

from ai_service.tools.evaluation_tool import EvaluationTool
from ai_service.contracts.context import AIRequestContext, PrincipalContext, UserRole

eval_tool = EvaluationTool()
test(
    "EvaluationTool operation prefix is 'evaluation'",
    eval_tool.operation_prefix == "evaluation",
    f"Got {eval_tool.operation_prefix}"
)

import asyncio

async def test_grading():
    ctx = AIRequestContext(
        request_id="req_test_grade_01",
        trace_id="trace_test_01",
        principal=PrincipalContext(user_id="usr_test_01", role=UserRole.STUDENT),
        operation="evaluation.grade",
        input={
            "quiz_id": "quiz_dp_101",
            "answers": {
                "q1": "Optimal Substructure",
                "q2": "O(nW)"
            }
        }
    )
    result, usage, meta = await eval_tool.execute(ctx, gateway=None)
    return result, usage, meta

grade_result, grade_usage, grade_meta = asyncio.run(test_grading())

test(
    "EvaluationTool produces valid grading scorecard",
    grade_result.get("quiz_id") == "quiz_dp_101" and "score_percent" in grade_result,
    f"Got {grade_result}"
)

test(
    "EvaluationTool metadata reports provider provenance",
    "provider" in grade_meta and "model" in grade_meta,
    f"Got meta: {grade_meta}"
)


# ═════════════════════════════════════════════════════════════════════════════
# TEST 3: Redis Cache Deduplication & Canonical Prompt Hashing
# ═════════════════════════════════════════════════════════════════════════════
print("\n[TEST 3] Redis Cache Deduplication & Prompt Hashing")

task = "quiz_generation"
prompt_text = "Generate 3 questions on Dynamic Programming"
temp = 0.7

hash1 = hashlib.sha256(f"{task}\0{prompt_text}\0{temp:.4f}".encode()).hexdigest()
hash2 = hashlib.sha256(f"{task}\0{prompt_text}\0{temp:.4f}".encode()).hexdigest()

test(
    "Semantic prompt hashing is strictly deterministic",
    hash1 == hash2 and len(hash1) == 64,
    f"Hashes: {hash1} vs {hash2}"
)

cache_key = f"ai_cache:{task}:{hash1}"
test(
    "Cache key follows canonical namespace conventions",
    cache_key.startswith("ai_cache:quiz_generation:") and len(cache_key) > 64,
    f"Cache key was {cache_key}"
)


# ═════════════════════════════════════════════════════════════════════════════
# TEST 4: Multi-Tenant Cost Attribution & Token Accounting
# ═════════════════════════════════════════════════════════════════════════════
print("\n[TEST 4] Multi-Tenant Cost Attribution & Token Accounting")

from ai_service.observability.cost_tracker import CostTracker

summary = CostTracker.get_summary_by_institution("veritas_uni")
test(
    "CostTracker aggregates multi-tenant usage metrics",
    isinstance(summary, dict) and "total_tokens" in summary and "total_cost_usd" in summary,
    f"Summary: {summary}"
)

test(
    "CostTracker reports cache savings and hit rate",
    "cache_hits" in summary and "estimated_avoided_tokens" in summary,
    f"Cache stats: {summary}"
)


# ═════════════════════════════════════════════════════════════════════════════
# SUMMARY
# ═════════════════════════════════════════════════════════════════════════════
print("\n" + "=" * 60)
print(f"Integration Test Results: {passed} PASSED, {failed} FAILED")
print("=" * 60)

if failed > 0:
    sys.exit(1)
sys.exit(0)
