from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field


class AlertEvent(BaseModel):
    id: str | None = None
    type: str
    org_id: str = ""
    project_id: str
    created_at: datetime
    alert_id: str
    severity: str
    metric: str
    value: float
    threshold: float


class RecommendedAction(BaseModel):
    action: str
    params: dict[str, Any] = Field(default_factory=dict)


class Diagnosis(BaseModel):
    root_cause: str
    confidence: float
    reasoning: str
    recommended_actions: list[RecommendedAction] = Field(default_factory=list)
    auto_execute: bool = False


class IncidentSummary(BaseModel):
    id: str
    alert_id: str
    project_id: str
    ai_diagnosis: dict[str, Any]
    ai_actions: list[dict[str, Any]] = Field(default_factory=list)
    resolved_at: datetime | None = None
    created_at: datetime


class IncidentListResponse(BaseModel):
    items: list[IncidentSummary] = Field(default_factory=list)
    total: int = 0
    limit: int = 20
    offset: int = 0


class ManualActionRequest(BaseModel):
    action: str
    params: dict[str, Any] = Field(default_factory=dict)


class ManualActionResponse(BaseModel):
    incident_id: str
    action: str
    status: str
    result: dict[str, Any] = Field(default_factory=dict)


class DeploymentContext(BaseModel):
    """Structured summary of a recent deployment for prompt injection."""

    deployment_id: str = ""
    status: str = ""  # running | succeeded | failed | rolled_back
    started_at: str = ""
    image: str = ""
    environment: str = ""
    minutes_since_deploy: float | None = None


class ClassificationRequest(BaseModel):
    files: list[dict[str, str]] = Field(default_factory=list)


class ClassificationResponse(BaseModel):
    runtime: str = ""
    framework: str = ""
    database: list[str] = Field(default_factory=list)
    containerized: bool = False
    port: int = 0
    build_command: str = ""
    test_command: str = ""
    confidence: float = 0.0