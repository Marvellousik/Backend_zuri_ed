"""
Request tracing and latency timing for LexiAssist AI Infrastructure.
"""
import time
from typing import Dict, Optional
from ai_service.contracts.usage import TimingMetrics


class LatencyTracker:
    """Tracks multi-stage latencies during request lifecycle."""

    def __init__(self):
        self._start_time = time.time()
        self.metrics = TimingMetrics()
        self._stage_starts: Dict[str, float] = {}

    def start_stage(self, stage_name: str):
        self._stage_starts[stage_name] = time.time()

    def end_stage(self, stage_name: str):
        if stage_name in self._stage_starts:
            duration_ms = int((time.time() - self._stage_starts.pop(stage_name)) * 1000)
            if stage_name == "queue":
                self.metrics.queue_ms += duration_ms
            elif stage_name == "retrieval":
                self.metrics.retrieval_ms += duration_ms
            elif stage_name == "tool":
                self.metrics.tool_ms += duration_ms
            elif stage_name == "generation":
                self.metrics.generation_ms += duration_ms
            elif stage_name == "validation":
                self.metrics.validation_ms += duration_ms

    def finish(self) -> TimingMetrics:
        self.metrics.total_duration_ms = int((time.time() - self._start_time) * 1000)
        return self.metrics
