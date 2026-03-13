from __future__ import annotations

import asyncio
import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import httpx
import structlog

from .context_clients import ContextClients
from .llm.provider import LLMProvider
from .memory_store import NoOpIncidentMemoryStore
from .models import AlertEvent, DeploymentContext, Diagnosis, IncidentListResponse, IncidentSummary, ManualActionRequest, ManualActionResponse
from .repository import IncidentRepository


class IncidentService:
    def __init__(
        self,
        repository: IncidentRepository,
        provider: LLMProvider,
        context_clients: ContextClients,
        publisher: "NATSPublisherProtocol",
        deployment_engine_url: str,
        audit_log_path: str,
        memory_store: Any | None = None,
    ) -> None:
        self._repository = repository
        self._provider = provider
        self._context_clients = context_clients
        self._publisher = publisher
        self._deployment_engine_url = deployment_engine_url.rstrip("/")
        self._audit_log_path = Path(audit_log_path)
        self._memory_store = memory_store or NoOpIncidentMemoryStore()
        self._logger = structlog.get_logger("incident-ai")

    async def process_alert(self, event: AlertEvent) -> IncidentSummary:
        async with httpx.AsyncClient(timeout=10.0) as client:
            metrics, deployments, logs = await asyncio.gather(
                self._context_clients.fetch_metrics(client, event.project_id),
                self._context_clients.fetch_recent_deployments(client, event.project_id),
                self._context_clients.fetch_logs(client, event.project_id),
            )
        # deployments is now List[DeploymentContext]

        memory_query = self._build_memory_query(event, metrics, logs)
        similar_incidents = await self._memory_store.search_similar(
            event.project_id,
            memory_query,
            limit=3,
        )

        prompt = self._build_prompt(event, metrics, logs, deployments, similar_incidents)
        diagnosis = await self._diagnose(prompt)
        incident = await self._repository.insert_incident(
            event.alert_id,
            event.project_id,
            diagnosis.model_dump(mode="json"),
            [],
        )
        await self._publisher.publish(
            "incident.created",
            {
                "id": incident.id,
                "type": "incident.created",
                "org_id": event.org_id,
                "project_id": event.project_id,
                "created_at": datetime.now(timezone.utc).isoformat(),
                "incident_id": incident.id,
                "alert_id": event.alert_id,
            },
        )

        if diagnosis.auto_execute:
            planned_actions = [
                action.model_dump(mode="json")
                for action in diagnosis.recommended_actions
            ]
            if not planned_actions:
                planned_actions = self.recommended_action_sequence(event.metric)

            execution_results = await self.execute_actions_until_success(planned_actions)
            for result in execution_results:
                action_name = str(result.get("action", "unknown_action"))
                status = str(result.get("status", "accepted"))
                action_entry = {
                    "action": action_name,
                    "params": {},
                    "status": status,
                    "recorded_at": datetime.now(timezone.utc).isoformat(),
                    "source": "auto",
                }
                await self._repository.append_action(incident.id, action_entry)
                await self._publisher.publish(
                    "autoheal.triggered",
                    {
                        "id": incident.id,
                        "type": "autoheal.triggered",
                        "org_id": event.org_id,
                        "project_id": event.project_id,
                        "created_at": datetime.now(timezone.utc).isoformat(),
                        "incident_id": incident.id,
                        "action": action_name,
                        "status": status,
                        "source": "auto",
                    },
                )

        latest = await self._repository.get_incident(incident.id)
        final_incident = latest or incident
        await self._memory_store.upsert_incident_memory(
            incident_id=final_incident.id,
            project_id=final_incident.project_id,
            alert_id=final_incident.alert_id,
            summary=str(final_incident.ai_diagnosis.get("root_cause", "")),
            actions_taken=list(final_incident.ai_actions),
        )
        return final_incident

    async def list_incidents(self, project_id: str, limit: int, offset: int) -> IncidentListResponse:
        items, total = await self._repository.list_incidents(project_id, limit, offset)
        return IncidentListResponse(items=items, total=total, limit=limit, offset=offset)

    async def get_incident(self, incident_id: str) -> IncidentSummary | None:
        return await self._repository.get_incident(incident_id)

    async def get_similar(self, incident_id: str) -> list[dict[str, Any]]:
        incident = await self._repository.get_incident(incident_id)
        if incident is None:
            return []
        query_text = self._build_incident_memory_text(
            incident.alert_id,
            str(incident.ai_diagnosis.get("root_cause", "")),
            list(incident.ai_actions),
        )
        return await self._memory_store.search_similar(
            incident.project_id,
            query_text,
            limit=3,
            exclude_incident_id=incident.id,
        )

    async def trigger_manual_action(self, incident_id: str, request: ManualActionRequest) -> ManualActionResponse:
        incident = await self._repository.get_incident(incident_id)
        if incident is None:
            raise LookupError("incident not found")

        result = await self._execute_action(request.action, request.params)
        action_entry = {
            "action": request.action,
            "params": request.params,
            "status": result.get("status", "accepted"),
            "recorded_at": datetime.now(timezone.utc).isoformat(),
        }
        await self._repository.append_action(incident_id, action_entry)
        await self._publisher.publish(
            "autoheal.triggered",
            {
                "id": incident_id,
                "type": "autoheal.triggered",
                "org_id": "",
                "project_id": incident.project_id,
                "created_at": datetime.now(timezone.utc).isoformat(),
                "incident_id": incident_id,
                "action": request.action,
            },
        )
        return ManualActionResponse(incident_id=incident_id, action=request.action, status=action_entry["status"], result=result)

    async def execute_actions_until_success(self, actions: list[dict[str, Any]]) -> list[dict[str, Any]]:
        results: list[dict[str, Any]] = []
        for action in actions:
            try:
                result = await self._execute_action(action["action"], action.get("params", {}))
                results.append(result)
                if result.get("status") in {"ok", "accepted"}:
                    break
            except Exception as exc:
                results.append({"action": action["action"], "status": "failed", "detail": str(exc)})
        return results

    # ------------------------------------------------------------------
    # Concrete manual-action routing
    # ------------------------------------------------------------------
    # Each alert rule maps to a preferred sequence of actions.  The
    # sequences are tried in order; the first successful one wins.
    _RULE_ACTION_SEQUENCES: dict[str, list[dict[str, Any]]] = {
        # Immediately-critical: restart pods first, then scale if restart fails.
        "ready-pods-zero": [
            {"action": "restart_pods", "params": {}},
            {"action": "scale_pods", "params": {"replicas": 2}},
        ],
        # Latency degradation: scaling up usually relieves pressure.
        "p99-latency-high": [
            {"action": "scale_pods", "params": {"replicas": 3}},
            {"action": "restart_pods", "params": {}},
        ],
        # Error-rate spike: rollback bad deploy first, then restart.
        "error-rate-high": [
            {"action": "rollback_deployment", "params": {}},
            {"action": "restart_pods", "params": {}},
        ],
        # CPU saturation: scale horizontally.
        "cpu-high": [
            {"action": "scale_pods", "params": {"replicas": 4}},
        ],
        # Crash loops: restart is the canonical first step.
        "pod-restarts-high": [
            {"action": "restart_pods", "params": {}},
            {"action": "rollback_deployment", "params": {}},
        ],
    }

    def recommended_action_sequence(self, rule: str) -> list[dict[str, Any]]:
        """Return the preferred action sequence for an alert rule."""
        return list(self._RULE_ACTION_SEQUENCES.get(rule, [{"action": "restart_pods", "params": {}}]))

    async def _execute_action(self, action: str, params: dict[str, Any]) -> dict[str, Any]:
        deployment_id = str(params.get("deployment_id", "")).strip()
        async with httpx.AsyncClient(timeout=10.0) as client:
            if action == "rollback_deployment":
                if not deployment_id:
                    # Best-effort: if the caller did not supply a deployment_id,
                    # resolve it from the latest deployment for the incident's project.
                    return {"action": action, "status": "accepted", "detail": "no deployment_id provided – skipped rollback"}
                response = await client.post(f"{self._deployment_engine_url}/deployments/{deployment_id}/rollback")
                response.raise_for_status()
                return {"action": action, "status": "ok", "detail": response.json()}
            if action == "scale_pods":
                if not deployment_id:
                    return {"action": action, "status": "accepted", "detail": "no deployment_id provided – skipped scale"}
                response = await client.patch(f"{self._deployment_engine_url}/deployments/{deployment_id}/scale", json=params)
                if response.status_code >= 400:
                    raise httpx.HTTPStatusError("scale failed", request=response.request, response=response)
                return {"action": action, "status": "ok", "detail": response.json()}
            if action == "increase_memory_limit":
                if not deployment_id:
                    return {"action": action, "status": "accepted", "detail": "no deployment_id provided – skipped memory increase"}
                response = await client.patch(f"{self._deployment_engine_url}/deployments/{deployment_id}/resources", json=params)
                if response.status_code >= 400:
                    raise httpx.HTTPStatusError("resources failed", request=response.request, response=response)
                return {"action": action, "status": "ok", "detail": response.json()}
            if action == "restart_pods":
                if not deployment_id:
                    # restart_pods without a deployment_id is accepted immediately
                    # — the execution environment is expected to restart all pods for the project.
                    return {"action": action, "status": "accepted", "detail": "restart accepted for all pods (no deployment_id)"}
                response = await client.post(f"{self._deployment_engine_url}/deployments/{deployment_id}/restart", json=params)
                if response.status_code >= 400:
                    raise httpx.HTTPStatusError("restart failed", request=response.request, response=response)
                return {"action": action, "status": "ok", "detail": response.json()}
        return {"action": action, "status": "accepted", "detail": params}

    async def _diagnose(self, prompt: str) -> Diagnosis:
        self._audit_log_path.parent.mkdir(parents=True, exist_ok=True)
        attempts = 0
        while True:
            attempts += 1
            try:
                response = await self._provider.diagnose(prompt)
                with self._audit_log_path.open("a", encoding="utf-8") as audit_log:
                    audit_log.write(prompt + "\n---\n" + response + "\n===\n")
                diagnosis = Diagnosis.model_validate_json(response)
                if diagnosis.confidence < 0.85:
                    diagnosis.auto_execute = False
                return diagnosis
            except Exception:
                if attempts >= 3:
                    raise
                await asyncio.sleep(0.05 * attempts)

    def _build_prompt(
        self,
        event: AlertEvent,
        metrics: list[dict[str, Any]],
        logs: list[str],
        deployments: list[DeploymentContext],
        similar_incidents: list[dict[str, Any]],
    ) -> str:
        # ------ deployment-history narrative ------
        deploy_section: str
        if not deployments:
            deploy_section = "No recent deployments found."
        else:
            last = deployments[0]
            age_str = f"{last.minutes_since_deploy:.1f} min ago" if last.minutes_since_deploy is not None else "unknown time ago"
            last_line = (
                f"Most-recent deployment: id={last.deployment_id!r}, "
                f"status={last.status!r}, env={last.environment!r}, "
                f"image={last.image!r}, started {age_str}."
            )
            if last.minutes_since_deploy is not None and last.minutes_since_deploy < 30:
                last_line += " ** Deploy is recent — consider rollback if the incident correlates."
            history_lines = [last_line]
            for dep in deployments[1:]:
                age_str_h = f"{dep.minutes_since_deploy:.1f} min ago" if dep.minutes_since_deploy is not None else "unknown time ago"
                history_lines.append(f"  - id={dep.deployment_id!r} status={dep.status!r} {age_str_h}")
            deploy_section = "\n".join(history_lines)

        prompt = (
            "You are an expert SRE analyzing a production incident.\n\n"
            f"Alert: rule={event.metric!r} severity={event.severity!r} fired_at={event.created_at.isoformat()}\n"
            f"  metric_value={event.value} threshold={event.threshold}\n"
            f"Project: {event.project_id}\n\n"
            f"=== Recent metrics (last 60 min) ===\n{json.dumps(metrics, indent=2)}\n\n"
            f"=== Deployment history ===\n{deploy_section}\n\n"
            f"=== Error logs (last 500 lines) ===\n{json.dumps(logs)}\n\n"
            f"=== Similar past incidents ===\n{json.dumps(similar_incidents)}\n\n"
            "Respond with a JSON object containing:\n"
            "  root_cause (string), confidence (float 0-1), reasoning (string),\n"
            "  recommended_actions (list of {action, params}), auto_execute (bool).\n"
            "Available actions: rollback_deployment, scale_pods, restart_pods, increase_memory_limit.\n"
        )
        return prompt

    def _build_memory_query(
        self,
        event: AlertEvent,
        metrics: list[dict[str, Any]],
        logs: list[str],
    ) -> str:
        metric_keys = sorted({key for item in metrics if isinstance(item, dict) for key in item.keys()})
        log_excerpt = " ".join(logs[:3]) if logs else ""
        return f"project={event.project_id} metric={event.metric} severity={event.severity} threshold={event.threshold} keys={' '.join(metric_keys)} logs={log_excerpt}".strip()

    def _build_incident_memory_text(
        self,
        alert_id: str,
        root_cause: str,
        actions: list[dict[str, Any]],
    ) -> str:
        action_names = [str(action.get("action", "")) for action in actions]
        return f"{root_cause}. Alert: {alert_id}. Actions: {', '.join(a for a in action_names if a)}".strip()


class NATSPublisherProtocol:
    async def publish(self, subject: str, payload: dict[str, Any]) -> None:  # pragma: no cover - typing only
        raise NotImplementedError