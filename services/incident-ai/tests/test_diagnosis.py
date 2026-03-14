from __future__ import annotations

import json
from datetime import datetime, timezone

import httpx
import pytest
import respx

from app.context_clients import ContextClients
from app.llm.provider import LLMProvider
from app.models import AlertEvent, IncidentSummary
from app.service import IncidentService


class FakeRepository:
    def __init__(self) -> None:
        self.incidents = []

    async def insert_incident(self, alert_id, project_id, diagnosis, actions):
        incident = {
            "id": "incident-1",
            "alert_id": alert_id,
            "project_id": project_id,
            "ai_diagnosis": diagnosis,
            "ai_actions": actions,
            "resolved_at": None,
            "created_at": datetime.now(timezone.utc),
        }
        self.incidents.append(incident)
        return type("Incident", (), incident)()

    async def list_incidents(self, project_id, limit, offset):
        return [], 0

    async def get_incident(self, incident_id):
        for incident in self.incidents:
            if incident["id"] == incident_id:
                return type("Incident", (), incident)()
        return None

    async def append_action(self, incident_id, entry):
        for incident in self.incidents:
            if incident["id"] == incident_id:
                incident.setdefault("ai_actions", []).append(entry)
        return await self.get_incident(incident_id)


class PagingRepository(FakeRepository):
    def __init__(self) -> None:
        super().__init__()
        self.last_list_args = None

    async def list_incidents(self, project_id, limit, offset):
        self.last_list_args = (project_id, limit, offset)
        incident = IncidentSummary(
            id="incident-page-1",
            alert_id="alert-page-1",
            project_id=project_id,
            ai_diagnosis={
                "root_cause": "paging test",
                "confidence": 0.9,
                "reasoning": "regression",
                "recommended_actions": [],
                "auto_execute": False,
            },
            ai_actions=[],
            resolved_at=None,
            created_at=datetime.now(timezone.utc),
        )
        return [incident], 42


class FakePublisher:
    def __init__(self) -> None:
        self.subjects = []

    async def publish(self, subject, payload):
        self.subjects.append((subject, payload))


class FakeMemoryStore:
    def __init__(self, search_results=None) -> None:
        self.search_results = list(search_results or [])
        self.search_calls = []
        self.upserts = []

    async def ensure_collection(self):
        return None

    async def search_similar(self, project_id, query_text, limit=3, exclude_incident_id=None):
        self.search_calls.append((project_id, query_text, limit, exclude_incident_id))
        return list(self.search_results)

    async def upsert_incident_memory(self, incident_id, project_id, alert_id, summary, actions_taken):
        self.upserts.append((incident_id, project_id, alert_id, summary, actions_taken))


class CapturingProvider(LLMProvider):
    def __init__(self, payloads):
        self.payloads = list(payloads)
        self.prompts = []

    async def diagnose(self, prompt: str) -> str:
        self.prompts.append(prompt)
        payload = self.payloads.pop(0)
        if isinstance(payload, Exception):
            raise payload
        return payload


@pytest.mark.asyncio
@respx.mock
async def test_diagnosis_parsed_correctly():
    respx.get("http://observability:8086/metrics/project-1").mock(return_value=httpx.Response(200, json=[{"error_rate_pct": 7.5}]))
    respx.get("http://deployment-engine:8085/deployments").mock(return_value=httpx.Response(200, json=[]))
    provider = CapturingProvider([
        json.dumps(
            {
                "root_cause": "A rollout introduced application errors.",
                "confidence": 0.91,
                "reasoning": "The error rate stayed high after deploy.",
                "recommended_actions": [{"action": "restart_pods", "params": {}}],
                "auto_execute": False,
            }
        )
    ])
    service = IncidentService(FakeRepository(), provider, ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log")
    incident = await service.process_alert(
        AlertEvent(
            type="alert.fired",
            org_id="org-1",
            project_id="project-1",
            created_at=datetime.now(timezone.utc),
            alert_id="alert-1",
            severity="critical",
            metric="error_rate_pct",
            value=7.5,
            threshold=5.0,
        )
    )
    assert incident.ai_diagnosis["root_cause"] == "A rollout introduced application errors."
    assert incident.ai_diagnosis["confidence"] > 0


@pytest.mark.asyncio
@respx.mock
async def test_deployment_context_in_prompt():
    """Verify that structured deployment history appears in the prompt sent to the LLM."""
    import json as _json
    from datetime import datetime, timezone, timedelta

    recent_deploy_ts = (datetime.now(timezone.utc) - timedelta(minutes=10)).isoformat()
    respx.get("http://observability:8086/metrics/project-1").mock(
        return_value=httpx.Response(200, json=[{"error_rate_pct": 2.0}])
    )
    respx.get("http://deployment-engine:8085/deployments").mock(
        return_value=httpx.Response(200, json=[{
            "id": "dep-99",
            "status": "running",
            "started_at": recent_deploy_ts,
            "image": "myapp:v2.3.1",
            "environment": "production",
        }])
    )
    provider = CapturingProvider([
        _json.dumps({
            "root_cause": "recent rollout",
            "confidence": 0.88,
            "reasoning": "error rate matches deploy",
            "recommended_actions": [{"action": "rollback_deployment", "params": {}}],
            "auto_execute": False,
        })
    ])
    service = IncidentService(
        FakeRepository(), provider,
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    await service.process_alert(
        AlertEvent(
            type="alert.fired",
            org_id="org-1",
            project_id="project-1",
            created_at=datetime.now(timezone.utc),
            alert_id="alert-deploy",
            severity="critical",
            metric="error_rate_pct",
            value=6.0,
            threshold=5.0,
        )
    )
    assert provider.prompts, "expected provider to receive a prompt"
    prompt = provider.prompts[0]
    assert "dep-99" in prompt, "deployment_id should appear in the prompt"
    assert "myapp:v2.3.1" in prompt, "deployment image should appear in the prompt"
    assert "Deploy is recent" in prompt, "recent-deploy warning should be injected"


@pytest.mark.asyncio
@respx.mock
async def test_latency_alert_routing():
    """Verify that a p99_latency_ms alert results in scale_pods recommendation."""
    from datetime import datetime, timezone

    respx.get("http://observability:8086/metrics/project-lat").mock(
        return_value=httpx.Response(200, json=[{"p99_latency_ms": 2800}])
    )
    respx.get("http://deployment-engine:8085/deployments").mock(
        return_value=httpx.Response(200, json=[])
    )
    # Swap to the real MockProvider so routing logic is exercised end-to-end
    from app.llm.provider import MockProvider
    service = IncidentService(
        FakeRepository(), MockProvider(),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    incident = await service.process_alert(
        AlertEvent(
            type="alert.fired",
            org_id="org-1",
            project_id="project-lat",
            created_at=datetime.now(timezone.utc),
            alert_id="alert-lat",
            severity="warning",
            metric="p99_latency_ms",
            value=2800,
            threshold=2000,
        )
    )
    actions = [a["action"] for a in incident.ai_diagnosis.get("recommended_actions", [])]
    assert "scale_pods" in actions, f"expected scale_pods in recommended_actions, got {actions}"


@pytest.mark.asyncio
@respx.mock
async def test_zero_pod_alert_routing():
    """Verify that a ready_pod_count=0 alert results in restart_pods recommendation."""
    from datetime import datetime, timezone

    respx.get("http://observability:8086/metrics/project-zp").mock(
        return_value=httpx.Response(200, json=[{"ready_pod_count": 0}])
    )
    respx.get("http://deployment-engine:8085/deployments").mock(
        return_value=httpx.Response(200, json=[])
    )
    from app.llm.provider import MockProvider
    service = IncidentService(
        FakeRepository(), MockProvider(),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    incident = await service.process_alert(
        AlertEvent(
            type="alert.fired",
            org_id="org-1",
            project_id="project-zp",
            created_at=datetime.now(timezone.utc),
            alert_id="alert-zp",
            severity="critical",
            metric="ready_pod_count",
            value=0,
            threshold=1,
        )
    )
    actions = [a["action"] for a in incident.ai_diagnosis.get("recommended_actions", [])]
    assert "restart_pods" in actions, f"expected restart_pods in recommended_actions, got {actions}"


@pytest.mark.asyncio
async def test_recommended_action_sequence_per_rule():
    """Verify that each alert rule maps to expected first action."""
    from app.llm.provider import MockProvider
    service = IncidentService(
        FakeRepository(), MockProvider(),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    assert service.recommended_action_sequence("ready-pods-zero")[0]["action"] == "restart_pods"
    assert service.recommended_action_sequence("p99-latency-high")[0]["action"] == "scale_pods"
    assert service.recommended_action_sequence("error-rate-high")[0]["action"] == "rollback_deployment"
    assert service.recommended_action_sequence("cpu-high")[0]["action"] == "scale_pods"
    assert service.recommended_action_sequence("pod-restarts-high")[0]["action"] == "restart_pods"
    # unknown rule falls back to restart_pods
    assert service.recommended_action_sequence("unknown-rule")[0]["action"] == "restart_pods"


@pytest.mark.asyncio
@respx.mock
async def test_low_confidence_no_autoexecute():
    respx.get("http://observability:8086/metrics/project-1").mock(return_value=httpx.Response(200, json=[]))
    respx.get("http://deployment-engine:8085/deployments").mock(return_value=httpx.Response(200, json=[]))
    provider = CapturingProvider([
        json.dumps(
            {
                "root_cause": "Unknown issue.",
                "confidence": 0.5,
                "reasoning": "Low confidence diagnosis.",
                "recommended_actions": [],
                "auto_execute": True,
            }
        )
    ])
    service = IncidentService(FakeRepository(), provider, ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log")
    incident = await service.process_alert(AlertEvent(type="alert.fired", org_id="org-1", project_id="project-1", created_at=datetime.now(timezone.utc), alert_id="alert-1", severity="critical", metric="error_rate_pct", value=8, threshold=5))
    assert incident.ai_diagnosis["auto_execute"] is False


@pytest.mark.asyncio
@respx.mock
async def test_auto_execute_runs_actions_and_publishes_autoheal():
    respx.get("http://observability:8086/metrics/project-1").mock(return_value=httpx.Response(200, json=[]))
    respx.get("http://deployment-engine:8085/deployments").mock(return_value=httpx.Response(200, json=[]))

    provider = CapturingProvider([
        json.dumps(
            {
                "root_cause": "Pods are not ready.",
                "confidence": 0.92,
                "reasoning": "Ready pod count dropped to zero.",
                "recommended_actions": [{"action": "restart_pods", "params": {}}],
                "auto_execute": True,
            }
        )
    ])
    publisher = FakePublisher()
    repository = FakeRepository()
    service = IncidentService(
        repository,
        provider,
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        publisher,
        "http://deployment-engine:8085",
        "/tmp/incident-ai-test-audit.log",
    )

    incident = await service.process_alert(
        AlertEvent(
            type="alert.fired",
            org_id="org-1",
            project_id="project-1",
            created_at=datetime.now(timezone.utc),
            alert_id="alert-auto-1",
            severity="critical",
            metric="ready_pod_count",
            value=0,
            threshold=1,
        )
    )

    assert incident.ai_diagnosis["auto_execute"] is True
    assert len(incident.ai_actions) >= 1
    assert incident.ai_actions[0]["action"] == "restart_pods"
    assert incident.ai_actions[0]["source"] == "auto"

    subjects = [subject for subject, _payload in publisher.subjects]
    assert "incident.created" in subjects
    assert "autoheal.triggered" in subjects


@pytest.mark.asyncio
@respx.mock
async def test_qdrant_similar_included():
    respx.get("http://observability:8086/metrics/project-1").mock(return_value=httpx.Response(200, json=[]))
    respx.get("http://deployment-engine:8085/deployments").mock(return_value=httpx.Response(200, json=[]))
    provider = CapturingProvider([
        json.dumps(
            {
                "root_cause": "Known issue.",
                "confidence": 0.9,
                "reasoning": "Matched prior incident.",
                "recommended_actions": [],
                "auto_execute": False,
            }
        )
    ])
    memory_store = FakeMemoryStore([
        {"incident_id": "old-1", "summary": "Previous restart fixed the issue.", "score": 0.91}
    ])
    service = IncidentService(FakeRepository(), provider, ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log", memory_store=memory_store)
    await service.process_alert(AlertEvent(type="alert.fired", org_id="org-1", project_id="project-1", created_at=datetime.now(timezone.utc), alert_id="alert-1", severity="critical", metric="error_rate_pct", value=8, threshold=5))
    assert "Previous restart fixed the issue." in provider.prompts[0]


@pytest.mark.asyncio
@respx.mock
async def test_qdrant_memory_upsert_called_after_processing():
    respx.get("http://observability:8086/metrics/project-1").mock(return_value=httpx.Response(200, json=[]))
    respx.get("http://deployment-engine:8085/deployments").mock(return_value=httpx.Response(200, json=[]))
    provider = CapturingProvider([
        json.dumps(
            {
                "root_cause": "Memory persistence check.",
                "confidence": 0.9,
                "reasoning": "Store incident for later retrieval.",
                "recommended_actions": [],
                "auto_execute": False,
            }
        )
    ])
    memory_store = FakeMemoryStore([])
    service = IncidentService(
        FakeRepository(),
        provider,
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(),
        "http://deployment-engine:8085",
        "/tmp/incident-ai-test-audit.log",
        memory_store=memory_store,
    )

    await service.process_alert(
        AlertEvent(
            type="alert.fired",
            org_id="org-1",
            project_id="project-1",
            created_at=datetime.now(timezone.utc),
            alert_id="alert-memory-1",
            severity="warning",
            metric="cpu_pct",
            value=95,
            threshold=80,
        )
    )

    assert len(memory_store.upserts) == 1
    assert memory_store.upserts[0][0] == "incident-1"


@pytest.mark.asyncio
async def test_get_similar_uses_memory_store_and_excludes_self():
    repository = FakeRepository()
    await repository.insert_incident(
        "alert-1",
        "project-1",
        {"root_cause": "Known issue", "confidence": 0.9, "reasoning": "x", "recommended_actions": [], "auto_execute": False},
        [{"action": "restart_pods", "status": "accepted"}],
    )
    memory_store = FakeMemoryStore([
        {"incident_id": "incident-older", "score": 0.93, "summary": "Older similar issue"}
    ])
    service = IncidentService(
        repository,
        CapturingProvider([]),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(),
        "http://deployment-engine:8085",
        "/tmp/incident-ai-test-audit.log",
        memory_store=memory_store,
    )

    results = await service.get_similar("incident-1")

    assert len(results) == 1
    assert results[0]["incident_id"] == "incident-older"
    assert memory_store.search_calls[0][3] == "incident-1"


@pytest.mark.asyncio
@respx.mock
async def test_retry_on_llm_timeout():
    respx.get("http://observability:8086/metrics/project-1").mock(return_value=httpx.Response(200, json=[]))
    respx.get("http://deployment-engine:8085/deployments").mock(return_value=httpx.Response(200, json=[]))
    provider = CapturingProvider([
        TimeoutError("first timeout"),
        TimeoutError("second timeout"),
        json.dumps(
            {
                "root_cause": "Recovered on retry.",
                "confidence": 0.88,
                "reasoning": "Third attempt succeeded.",
                "recommended_actions": [],
                "auto_execute": False,
            }
        ),
    ])
    service = IncidentService(FakeRepository(), provider, ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log")
    incident = await service.process_alert(AlertEvent(type="alert.fired", org_id="org-1", project_id="project-1", created_at=datetime.now(timezone.utc), alert_id="alert-1", severity="critical", metric="error_rate_pct", value=8, threshold=5))
    assert incident.ai_diagnosis["root_cause"] == "Recovered on retry."
    assert len(provider.prompts) == 3


@pytest.mark.asyncio
@respx.mock
async def test_action_rollback_called():
    respx.post("http://deployment-engine:8085/deployments/dep-1/rollback").mock(return_value=httpx.Response(200, json={"status": "rolled_back"}))
    repository = FakeRepository()
    await repository.insert_incident("alert-1", "project-1", {"root_cause": "x", "confidence": 0.9}, [])
    service = IncidentService(repository, CapturingProvider([]), ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log")
    response = await service.trigger_manual_action("incident-1", request=type("Req", (), {"action": "rollback_deployment", "params": {"deployment_id": "dep-1"}})())
    assert response.status == "ok"


@pytest.mark.asyncio
async def test_list_incidents_returns_paginated_response_contract():
    repository = PagingRepository()
    service = IncidentService(
        repository,
        CapturingProvider([]),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(),
        "http://deployment-engine:8085",
        "/tmp/incident-ai-test-audit.log",
    )

    result = await service.list_incidents("project-paging", 5, 10)

    assert repository.last_list_args == ("project-paging", 5, 10)
    assert result.limit == 5
    assert result.offset == 10
    assert result.total == 42
    assert len(result.items) == 1
    assert result.items[0].id == "incident-page-1"