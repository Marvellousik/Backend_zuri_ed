"""
Unit tests for Model Adapters and Capability Routing.
"""
import pytest
from ai_service.adapters.base import ModelCapability, ModelRequirements
from ai_service.gateway.model_router import ModelRouter
from ai_service.adapters.gemini_adapter import GeminiAdapter


def test_model_router_capability_resolution():
    router = ModelRouter()

    # Fast chat with small context -> flash-lite
    req_chat = ModelRequirements(capability=ModelCapability.FAST_CHAT)
    assert router.resolve_model(req_chat, estimated_input_chars=500) == "gemini-2.5-flash-lite"

    # Deep reasoning -> pro
    req_deep = ModelRequirements(capability=ModelCapability.DEEP_REASONING)
    assert router.resolve_model(req_deep, estimated_input_chars=1000) == "gemini-2.5-pro"

    # Structured generation -> flash
    req_struct = ModelRequirements(capability=ModelCapability.STRUCTURED_GENERATION)
    assert router.resolve_model(req_struct, estimated_input_chars=2000) == "gemini-2.5-flash"


def test_gemini_cost_calculation():
    # 1000 input tokens on flash = 0.0003, 1000 output = 0.0025 -> total = 0.0028
    cost = GeminiAdapter.calculate_cost("gemini-2.5-flash", 1000, 1000)
    assert round(cost, 5) == 0.0028
