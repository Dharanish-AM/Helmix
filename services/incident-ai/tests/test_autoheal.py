from __future__ import annotations

import pytest
import respx
import httpx

from app.context_clients import ContextClients
from app.llm.provider import MockProvider
from app.service import IncidentService

from .test_diagnosis import FakePublisher, FakeRepository


@pytest.mark.asyncio
@respx.mock
async def test_scale_pods_action():
    respx.patch("http://deployment-engine:8085/deployments/dep-1/scale").mock(return_value=httpx.Response(200, json={"status": "scaled"}))
    service = IncidentService(FakeRepository(), MockProvider(), ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log")
    result = await service.execute_actions_until_success([{"action": "scale_pods", "params": {"deployment_id": "dep-1", "replicas": 5}}])
    assert result[0]["status"] == "ok"


@pytest.mark.asyncio
@respx.mock
async def test_action_failure_tries_next():
    respx.patch("http://deployment-engine:8085/deployments/dep-1/scale").mock(return_value=httpx.Response(500, json={"error": "boom"}))
    respx.post("http://deployment-engine:8085/deployments/dep-1/restart").mock(return_value=httpx.Response(200, json={"status": "restarted"}))
    service = IncidentService(FakeRepository(), MockProvider(), ContextClients("http://observability:8086", "http://deployment-engine:8085"), FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log")
    result = await service.execute_actions_until_success([
        {"action": "scale_pods", "params": {"deployment_id": "dep-1", "replicas": 5}},
        {"action": "restart_pods", "params": {"deployment_id": "dep-1"}},
    ])
    assert result[-1]["status"] == "ok"


@pytest.mark.asyncio
async def test_restart_pods_without_deployment_id_accepted():
    """restart_pods with no deployment_id should return status=accepted without HTTP calls."""
    service = IncidentService(
        FakeRepository(), MockProvider(),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    result = await service.execute_actions_until_success([
        {"action": "restart_pods", "params": {}},
    ])
    assert result[0]["status"] == "accepted"


@pytest.mark.asyncio
async def test_rollback_without_deployment_id_accepted():
    """rollback_deployment with no deployment_id should return status=accepted (skip) without raising."""
    service = IncidentService(
        FakeRepository(), MockProvider(),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    result = await service.execute_actions_until_success([
        {"action": "rollback_deployment", "params": {}},
    ])
    assert result[0]["status"] == "accepted"


@pytest.mark.asyncio
async def test_scale_pods_without_deployment_id_accepted():
    """scale_pods with no deployment_id should return status=accepted without raising."""
    service = IncidentService(
        FakeRepository(), MockProvider(),
        ContextClients("http://observability:8086", "http://deployment-engine:8085"),
        FakePublisher(), "http://deployment-engine:8085", "/tmp/incident-ai-test-audit.log",
    )
    result = await service.execute_actions_until_success([
        {"action": "scale_pods", "params": {"replicas": 3}},
    ])
    assert result[0]["status"] == "accepted"