"""
Academic Chat Tool for LexiAssist AI Infrastructure.
Provides conversational AI with RAG context support, citation, and conversation history.
"""
from typing import Dict, Any, Tuple, AsyncGenerator, List
from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.adapters.base import ModelCapability, ModelRequirements, StreamChunk


class ChatTool(BaseTool):
    """Handles chat.generate and chat.stream operations."""

    @property
    def operation_prefix(self) -> str:
        return "chat"

    def _build_prompt(self, query: str, context_chunks: List[str], history: List[Dict[str, str]]) -> str:
        context_str = ""
        if context_chunks:
            docs = "\n\n".join([f"[Document {i+1}]\n{c}" for i, c in enumerate(context_chunks[:5])])
            context_str = f"DOCUMENT CONTEXT:\n{docs}\n\nUse the document context above to answer. Cite sources as [Document X]."
        else:
            context_str = "Answer the question using your academic knowledge clearly and concisely."

        history_str = ""
        if history:
            history_str = "PREVIOUS CONVERSATION:\n" + "\n".join(
                [f"{msg.get('role', 'user')}: {msg.get('content', '')}" for msg in history[-4:]]
            ) + "\n\n"

        return f"""You are LexiAssist, an AI academic tutor.

{context_str}

{history_str}USER QUESTION: {query}

Provide a well-structured response using markdown, bold key terms, and bullet points where helpful."""

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        query = ctx.input.get("query", "")
        context_chunks = ctx.input.get("context_chunks", [])
        history = ctx.input.get("history", [])

        prompt = self._build_prompt(query, context_chunks, history)
        
        req = ModelRequirements(
            capability=ModelCapability.FAST_CHAT,
            cost_sensitive=True,
            preferred_model=ctx.policy.requested_capability,
        )
        model_name = gateway.model_router.resolve_model(req, estimated_input_chars=len(prompt))

        response = await gateway.gemini_adapter.generate(
            prompt=prompt,
            model_name=model_name,
            temperature=0.4,
        )

        result = {
            "response": response.content,
            "session_id": ctx.session_id,
            "sources": [f"chunk_{i}" for i in range(len(context_chunks))],
        }

        usage = TokenUsage(
            input_tokens=response.input_tokens,
            output_tokens=response.output_tokens,
            total_tokens=response.total_tokens,
            estimated_cost_usd=response.cost_usd,
        )

        model_info = {"provider": "google", "model": model_name}
        return result, usage, model_info

    async def execute_stream(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> AsyncGenerator[StreamChunk, None]:
        query = ctx.input.get("query", "")
        context_chunks = ctx.input.get("context_chunks", [])
        history = ctx.input.get("history", [])

        prompt = self._build_prompt(query, context_chunks, history)
        
        req = ModelRequirements(capability=ModelCapability.FAST_CHAT, low_latency=True)
        model_name = gateway.model_router.resolve_model(req, estimated_input_chars=len(prompt))

        async for chunk in gateway.gemini_adapter.generate_stream(prompt=prompt, model_name=model_name):
            yield chunk
