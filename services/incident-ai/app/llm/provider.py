from __future__ import annotations

import json
from abc import ABC, abstractmethod


class LLMProvider(ABC):
    @abstractmethod
    async def diagnose(self, prompt: str) -> str:
        raise NotImplementedError


class MockProvider(LLMProvider):
    async def diagnose(self, prompt: str) -> str:
        lower_prompt = prompt.lower()
        recommended_actions: list[dict[str, object]] = []

        # Latency degradation — scale horizontally first
        if "p99_latency_ms" in lower_prompt or "p99-latency-high" in lower_prompt or "latency" in lower_prompt:
            recommended_actions.append({"action": "scale_pods", "params": {}})
            root_cause = "Sustained p99 latency breach suggests resource exhaustion or slow upstream."
            reasoning = "Scaling out additional pods should distribute load and reduce tail latency."

        # Zero ready pods — restart is immediate remediation
        elif "ready_pod_count" in lower_prompt or "ready-pods-zero" in lower_prompt or "zero ready" in lower_prompt:
            recommended_actions.append({"action": "restart_pods", "params": {}})
            recommended_actions.append({"action": "scale_pods", "params": {"replicas": 2}})
            root_cause = "All pods are unavailable — likely a crash loop or failed deployment."
            reasoning = "Restarting pods clears crash loops; scaling ensures a replacement is scheduled."

        # Error-rate spike after a recent deploy — rollback first
        elif ("error_rate_pct" in lower_prompt or "error-rate-high" in lower_prompt) and "deploy is recent" in lower_prompt:
            recommended_actions.append({"action": "rollback_deployment", "params": {}})
            recommended_actions.append({"action": "restart_pods", "params": {}})
            root_cause = "Error rate spiked immediately after a recent deployment."
            reasoning = "Rolling back the bad deploy and restarting pods should restore service."

        # Generic error rate
        elif "error_rate_pct" in lower_prompt or "error-rate-high" in lower_prompt:
            recommended_actions.append({"action": "restart_pods", "params": {}})
            root_cause = "Sustained application errors likely indicate unhealthy pods."
            reasoning = "Restarting pods clears transient error states."

        # CPU saturation
        elif "cpu_pct" in lower_prompt or "cpu-high" in lower_prompt:
            recommended_actions.append({"action": "scale_pods", "params": {"replicas": 4}})
            root_cause = "CPU utilisation exceeded threshold across multiple intervals."
            reasoning = "Horizontal scaling distributes load across more instances."

        # Pod crash / restart loop
        elif "pod_restarts" in lower_prompt or "pod-restarts-high" in lower_prompt:
            recommended_actions.append({"action": "restart_pods", "params": {}})
            root_cause = "Repeated pod restarts indicate a crash loop or OOM condition."
            reasoning = "Restarting clears the crash state; rollback resolves bad code."

        else:
            recommended_actions.append({"action": "restart_pods", "params": {}})
            root_cause = "Sustained application errors likely indicate a bad rollout or unhealthy pods."
            reasoning = "Metrics show a sustained breach; restarting pods is the default first step."

        payload = {
            "root_cause": root_cause,
            "confidence": 0.91,
            "reasoning": reasoning,
            "recommended_actions": recommended_actions,
            "auto_execute": False,
        }
        return json.dumps(payload)


def get_provider(provider_name: str) -> LLMProvider:
    return MockProvider()