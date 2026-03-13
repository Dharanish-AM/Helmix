"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

import {
  fetchIncidentDetail,
  fetchIncidents,
  fetchProjectDeployments,
  fetchSimilarIncidents,
  triggerIncidentAction,
  type DeploymentSummary,
  type Incident,
  type SimilarIncident,
} from "@/lib/api";
import { useAuthStore } from "@/lib/auth-store";
import { parseAlertRuleQuery, suggestedActionForAlertRule } from "./presets";

type ActionState = {
  incidentId: string;
  action: string;
  loading: boolean;
  result: string | null;
  error: string | null;
};

type ActionSummary = {
  action: string;
  status: string;
  recordedAt: Date;
  error?: string;
};

const CONFIDENCE_CLASS = (confidence: number) => {
  if (confidence >= 0.85) return "text-emerald-700 font-semibold";
  if (confidence >= 0.65) return "text-amber-700";
  return "text-red-600";
};

const ACTIONS = [
  { label: "Restart Pods", value: "restart_pods" },
  { label: "Rollback Deploy", value: "rollback_deployment" },
  { label: "Scale Pods ×3", value: "scale_pods" },
];

export default function IncidentsPage() {
  const router = useRouter();
  const { token } = useAuthStore();
  const [projectId, setProjectId] = useState("");
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [deployments, setDeployments] = useState<DeploymentSummary[]>([]);
  const [deploymentIdByIncident, setDeploymentIdByIncident] = useState<
    Record<string, string>
  >({});
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionState, setActionState] = useState<ActionState | null>(null);
  const [actionSummaryByIncident, setActionSummaryByIncident] = useState<
    Record<string, ActionSummary>
  >({});
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshIntervalSeconds, setRefreshIntervalSeconds] = useState(15);
  const [lastRefreshedAt, setLastRefreshedAt] = useState<Date | null>(null);
  const [deploymentEnvironmentFilter, setDeploymentEnvironmentFilter] =
    useState("all");
  const [deploymentStatusFilter, setDeploymentStatusFilter] = useState("all");
  const [expandedIncidentId, setExpandedIncidentId] = useState<string | null>(
    null
  );
  const [detailByIncidentId, setDetailByIncidentId] = useState<
    Record<string, Incident>
  >({});
  const [similarByIncidentId, setSimilarByIncidentId] = useState<
    Record<string, SimilarIncident[]>
  >({});
  const [detailLoadingByIncidentId, setDetailLoadingByIncidentId] = useState<
    Record<string, boolean>
  >({});
  const [pageSize, setPageSize] = useState(10);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalIncidents, setTotalIncidents] = useState(0);
  const [selectedActionByIncident, setSelectedActionByIncident] = useState<
    Record<string, string>
  >({});
  const [triageContext, setTriageContext] = useState<{
    alertId: string;
    alertRule: string;
    alertMetric: string;
  } | null>(null);

  useEffect(() => {
    if (!token) {
      router.replace("/login");
    }
  }, [token, router]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const projectFromQuery = params.get("project_id")?.trim() ?? "";
    const alertId = params.get("alert_id")?.trim() ?? "";
    const alertRule = parseAlertRuleQuery(params);
    const alertMetric = params.get("alert_metric")?.trim() ?? "";

    if (projectFromQuery && projectFromQuery !== projectId) {
      setProjectId(projectFromQuery);
    }

    if (alertId || alertRule || alertMetric) {
      setTriageContext({
        alertId,
        alertRule,
        alertMetric,
      });
    } else {
      setTriageContext(null);
    }
  }, [projectId]);

  const handleFetch = useCallback(async (options?: { silent?: boolean }) => {
    if (!token || !projectId.trim()) return;
    const silent = options?.silent ?? false;
    const offset = (currentPage - 1) * pageSize;
    if (!silent) {
      setIsLoading(true);
    }
    setLoadError(null);
    try {
      const [incidentData, deploymentData] = await Promise.all([
        fetchIncidents(token, projectId.trim(), pageSize, offset),
        fetchProjectDeployments(token, projectId.trim(), 20),
      ]);
      setIncidents(incidentData.items);
      setTotalIncidents(incidentData.total);
      setDeployments(deploymentData);
      setLastRefreshedAt(new Date());
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "unknown error";
      if (message === "unauthorized") {
        router.replace("/login");
        return;
      }
      setLoadError(message);
    } finally {
      if (!silent) {
        setIsLoading(false);
      }
    }
  }, [currentPage, pageSize, projectId, router, token]);

  useEffect(() => {
    if (!token || !projectId.trim()) {
      return;
    }
    void handleFetch();
  }, [handleFetch, projectId, token]);

  useEffect(() => {
    if (!autoRefresh || !token || !projectId.trim()) {
      return;
    }
    const intervalId = setInterval(() => {
      void handleFetch({ silent: true });
    }, refreshIntervalSeconds * 1000);
    return () => clearInterval(intervalId);
  }, [autoRefresh, handleFetch, projectId, refreshIntervalSeconds, token]);

  const suggestedActionFromRule = useMemo(() => {
    const alertRule = triageContext?.alertRule?.trim();
    if (!alertRule) {
      return "";
    }
    return suggestedActionForAlertRule(alertRule);
  }, [triageContext?.alertRule]);

  useEffect(() => {
    if (!incidents.length || !suggestedActionFromRule) {
      return;
    }
    setSelectedActionByIncident((prev) => {
      const next = { ...prev };
      for (const incident of incidents) {
        if (!next[incident.id]) {
          next[incident.id] = suggestedActionFromRule;
        }
      }
      return next;
    });
  }, [incidents, suggestedActionFromRule]);

  const deploymentEnvironmentOptions = useMemo(() => {
    const values = new Set<string>();
    for (const deployment of deployments) {
      if (deployment.environment) {
        values.add(deployment.environment);
      }
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [deployments]);

  const deploymentStatusOptions = useMemo(() => {
    const values = new Set<string>();
    for (const deployment of deployments) {
      if (deployment.status) {
        values.add(deployment.status);
      }
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [deployments]);

  const filteredDeployments = useMemo(() => {
    return deployments.filter((deployment) => {
      const environmentMatches =
        deploymentEnvironmentFilter === "all" ||
        deployment.environment === deploymentEnvironmentFilter;
      const statusMatches =
        deploymentStatusFilter === "all" ||
        deployment.status === deploymentStatusFilter;
      return environmentMatches && statusMatches;
    });
  }, [deployments, deploymentEnvironmentFilter, deploymentStatusFilter]);

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(totalIncidents / pageSize)),
    [pageSize, totalIncidents]
  );

  useEffect(() => {
    if (currentPage > totalPages) {
      setCurrentPage(totalPages);
    }
  }, [currentPage, totalPages]);

  const actionBadgeCounts = useMemo(() => {
    let accepted = 0;
    let failed = 0;
    let running = 0;

    if (actionState?.loading) {
      running = 1;
    }
    for (const summary of Object.values(actionSummaryByIncident)) {
      if (summary.status === "failed") {
        failed += 1;
      } else {
        accepted += 1;
      }
    }
    return { accepted, failed, running };
  }, [actionState?.loading, actionSummaryByIncident]);

  async function handleAction(incident: Incident, action: string) {
    if (!token) return;
    const deploymentId = deploymentIdByIncident[incident.id]?.trim() ?? "";
    const params: Record<string, unknown> = {};
    if (action === "scale_pods") {
      params.replicas = 3;
    }
    if (
      deploymentId &&
      (action === "scale_pods" ||
        action === "rollback_deployment" ||
        action === "restart_pods")
    ) {
      params.deployment_id = deploymentId;
    }

    setActionState({
      incidentId: incident.id,
      action,
      loading: true,
      result: null,
      error: null,
    });
    try {
      const res = await triggerIncidentAction(token, incident.id, action, params);
      setActionState((prev) =>
        prev ? { ...prev, loading: false, result: res.status } : null
      );
      setActionSummaryByIncident((prev) => ({
        ...prev,
        [incident.id]: {
          action,
          status: res.status,
          recordedAt: new Date(),
        },
      }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "unknown error";
      setActionState((prev) =>
        prev ? { ...prev, loading: false, error: message } : null
      );
      setActionSummaryByIncident((prev) => ({
        ...prev,
        [incident.id]: {
          action,
          status: "failed",
          recordedAt: new Date(),
          error: message,
        },
      }));
    }
  }

  async function handleToggleDetail(incidentId: string) {
    if (expandedIncidentId === incidentId) {
      setExpandedIncidentId(null);
      return;
    }
    setExpandedIncidentId(incidentId);
    if (!token) return;
    if (detailByIncidentId[incidentId] && similarByIncidentId[incidentId]) {
      return;
    }

    setDetailLoadingByIncidentId((prev) => ({ ...prev, [incidentId]: true }));
    try {
      const [detail, similar] = await Promise.all([
        fetchIncidentDetail(token, incidentId),
        fetchSimilarIncidents(token, incidentId),
      ]);
      setDetailByIncidentId((prev) => ({ ...prev, [incidentId]: detail }));
      setSimilarByIncidentId((prev) => ({ ...prev, [incidentId]: similar }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "unknown error";
      setLoadError(message);
    } finally {
      setDetailLoadingByIncidentId((prev) => ({ ...prev, [incidentId]: false }));
    }
  }

  return (
    <main className="mx-auto min-h-screen w-full max-w-6xl px-6 py-10">
      <h1 className="mb-6 text-2xl font-bold text-gray-800">Incidents</h1>

      {triageContext ? (
        <div className="mb-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          <p className="font-semibold">Opened from Observability alert</p>
          <p className="mt-1 text-xs">
            {triageContext.alertRule ? `Rule: ${triageContext.alertRule}` : "Rule: n/a"}
            {" | "}
            {triageContext.alertMetric ? `Metric: ${triageContext.alertMetric}` : "Metric: n/a"}
            {" | "}
            {triageContext.alertId ? `Alert ID: ${triageContext.alertId}` : "Alert ID: n/a"}
          </p>
          {suggestedActionFromRule ? (
            <p className="mt-1 text-xs font-semibold">
              Suggested action preset: {suggestedActionFromRule}
            </p>
          ) : null}
        </div>
      ) : null}

      <div className="mb-3 flex flex-wrap gap-3">
        <input
          type="text"
          placeholder="Project ID"
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleFetch()}
          className="w-72 rounded-xl border border-gray-300 px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400"
        />
        <button
          onClick={() => void handleFetch()}
          disabled={isLoading || !projectId.trim()}
          className="rounded-xl bg-amber-500 px-5 py-2 text-sm font-semibold text-white hover:bg-amber-600 disabled:opacity-50"
        >
          {isLoading ? "Loading…" : "Load Incidents"}
        </button>
        <button
          onClick={() => void handleFetch({ silent: true })}
          disabled={!projectId.trim()}
          className="rounded-xl border border-amber-300 bg-amber-50 px-5 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100 disabled:opacity-40"
        >
          Refresh
        </button>
      </div>

      <div className="mb-6 flex flex-wrap items-center gap-3">
        <label className="flex items-center gap-2 text-sm text-gray-600">
          <input
            type="checkbox"
            checked={autoRefresh}
            onChange={(event) => setAutoRefresh(event.target.checked)}
            className="h-4 w-4 rounded border-gray-300 text-amber-500 focus:ring-amber-300"
          />
          Auto-refresh
        </label>
        <select
          value={refreshIntervalSeconds}
          onChange={(event) => setRefreshIntervalSeconds(Number(event.target.value))}
          className="rounded-lg border border-gray-300 px-2 py-1 text-sm"
          disabled={!autoRefresh}
        >
          <option value={10}>10s</option>
          <option value={15}>15s</option>
          <option value={30}>30s</option>
          <option value={60}>60s</option>
        </select>
        {lastRefreshedAt ? (
          <span className="text-xs text-gray-500">
            Last refreshed: {lastRefreshedAt.toLocaleTimeString()}
          </span>
        ) : null}
      </div>

      <div className="mb-6 flex flex-wrap items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-500">
          Deployment Picker Filters
        </p>
        <label className="flex items-center gap-2 text-sm text-gray-600">
          Environment
          <select
            value={deploymentEnvironmentFilter}
            onChange={(event) =>
              setDeploymentEnvironmentFilter(event.target.value)
            }
            className="rounded-lg border border-gray-300 px-2 py-1 text-sm"
          >
            <option value="all">All</option>
            {deploymentEnvironmentOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-2 text-sm text-gray-600">
          Status
          <select
            value={deploymentStatusFilter}
            onChange={(event) => setDeploymentStatusFilter(event.target.value)}
            className="rounded-lg border border-gray-300 px-2 py-1 text-sm"
          >
            <option value="all">All</option>
            {deploymentStatusOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <span className="text-xs text-gray-500">
          Showing {filteredDeployments.length} of {deployments.length} deployments
        </span>
      </div>

      {loadError && (
        <p className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {loadError}
        </p>
      )}

      {incidents.length === 0 && !isLoading && !loadError && projectId && (
        <p className="text-sm text-gray-500">No incidents found for this project.</p>
      )}

      {(incidents.length > 0 || totalIncidents > 0) && (
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
              accepted: {actionBadgeCounts.accepted}
            </span>
            <span className="rounded-full bg-red-50 px-3 py-1 text-xs font-semibold text-red-700">
              failed: {actionBadgeCounts.failed}
            </span>
            <span className="rounded-full bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700">
              running: {actionBadgeCounts.running}
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-sm text-gray-600">
            <label className="flex items-center gap-2">
              Per page
              <select
                value={pageSize}
                onChange={(event) => {
                  setPageSize(Number(event.target.value));
                  setCurrentPage(1);
                }}
                className="rounded-lg border border-gray-300 px-2 py-1 text-sm"
              >
                <option value={5}>5</option>
                <option value={10}>10</option>
                <option value={20}>20</option>
                <option value={50}>50</option>
              </select>
            </label>
            <button
              onClick={() => setCurrentPage((page) => Math.max(1, page - 1))}
              disabled={currentPage <= 1}
              className="rounded-lg border border-gray-300 px-2 py-1 text-xs font-semibold disabled:opacity-40"
            >
              Prev
            </button>
            <span className="text-xs text-gray-500">
              Page {currentPage} / {totalPages}
            </span>
            <button
              onClick={() => setCurrentPage((page) => Math.min(totalPages, page + 1))}
              disabled={currentPage >= totalPages}
              className="rounded-lg border border-gray-300 px-2 py-1 text-xs font-semibold disabled:opacity-40"
            >
              Next
            </button>
          </div>
        </div>
      )}

      <div className="space-y-4">
        {incidents.map((incident) => {
          const diagnosis = incident.ai_diagnosis;
          const isActive =
            actionState?.incidentId === incident.id && actionState.loading;

          return (
            <div
              key={incident.id}
              className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm"
            >
              <div className="mb-2 flex items-center justify-between">
                <span className="font-mono text-xs text-gray-400">
                  {incident.id}
                </span>
                <span className="text-xs text-gray-400">
                  {new Date(incident.created_at).toLocaleString()}
                </span>
              </div>

              <p className="mb-1 text-sm font-semibold text-gray-800">
                Root cause:{" "}
                <span className="font-normal">{diagnosis.root_cause}</span>
              </p>
              <p className="mb-1 text-sm text-gray-600">
                Reasoning:{" "}
                <span className="text-gray-500">{diagnosis.reasoning}</span>
              </p>
              <p className="mb-3 text-sm">
                Confidence:{" "}
                <span className={CONFIDENCE_CLASS(diagnosis.confidence)}>
                  {(diagnosis.confidence * 100).toFixed(0)}%
                </span>
              </p>

              <div className="mb-3 flex flex-wrap items-center gap-2">
                <button
                  onClick={() => void handleToggleDetail(incident.id)}
                  className="rounded-lg border border-slate-300 bg-slate-50 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                >
                  {expandedIncidentId === incident.id ? "Hide Details" : "View Details"}
                </button>
                {actionSummaryByIncident[incident.id] ? (
                  <span className="rounded-lg bg-emerald-50 px-3 py-1.5 text-xs font-medium text-emerald-700">
                    Last action: {actionSummaryByIncident[incident.id].action} ({actionSummaryByIncident[incident.id].status}) at {actionSummaryByIncident[incident.id].recordedAt.toLocaleTimeString()}
                  </span>
                ) : null}
              </div>

              {diagnosis.recommended_actions.length > 0 && (
                <div className="mb-3">
                  <p className="mb-1 text-xs font-semibold uppercase text-gray-400">
                    AI-recommended actions
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {diagnosis.recommended_actions.map((ra, idx) => (
                      <span
                        key={idx}
                        className="rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-700"
                      >
                        {ra.action}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {incident.resolved_at === null && (
                <div className="space-y-3">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <label
                      htmlFor={`deployment-id-${incident.id}`}
                      className="text-xs font-semibold uppercase tracking-wide text-gray-500"
                    >
                      Deployment (optional)
                    </label>
                    <select
                      id={`deployment-id-${incident.id}`}
                      value={deploymentIdByIncident[incident.id] ?? ""}
                      onChange={(event) =>
                        setDeploymentIdByIncident((prev) => ({
                          ...prev,
                          [incident.id]: event.target.value,
                        }))
                      }
                      className="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-amber-300 sm:max-w-xs"
                    >
                      <option value="">No deployment selected</option>
                      {filteredDeployments.map((deployment) => (
                        <option key={deployment.id} value={deployment.id}>
                          {deployment.id} | {deployment.environment} | {deployment.status}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <label
                      htmlFor={`selected-action-${incident.id}`}
                      className="text-xs font-semibold uppercase tracking-wide text-gray-500"
                    >
                      Selected Action
                    </label>
                    <select
                      id={`selected-action-${incident.id}`}
                      value={selectedActionByIncident[incident.id] ?? "restart_pods"}
                      onChange={(event) =>
                        setSelectedActionByIncident((prev) => ({
                          ...prev,
                          [incident.id]: event.target.value,
                        }))
                      }
                      className="rounded-lg border border-gray-300 px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-amber-300"
                    >
                      {ACTIONS.map(({ label, value }) => (
                        <option key={`${incident.id}-${value}`} value={value}>
                          {label}
                        </option>
                      ))}
                    </select>
                    <button
                      onClick={() =>
                        handleAction(
                          incident,
                          selectedActionByIncident[incident.id] ?? "restart_pods"
                        )
                      }
                      disabled={isActive}
                      className="rounded-xl border border-emerald-300 bg-emerald-50 px-4 py-1.5 text-xs font-semibold text-emerald-900 hover:bg-emerald-100 disabled:opacity-40"
                    >
                      Run Selected
                    </button>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {ACTIONS.map(({ label, value }) => (
                      <button
                        key={value}
                        onClick={() => handleAction(incident, value)}
                        disabled={isActive}
                        className={`rounded-xl border px-4 py-1.5 text-xs font-semibold disabled:opacity-40 ${
                          selectedActionByIncident[incident.id] === value
                            ? "border-emerald-300 bg-emerald-50 text-emerald-900 hover:bg-emerald-100"
                            : "border-amber-300 bg-amber-50 text-amber-900 hover:bg-amber-100"
                        }`}
                      >
                        {isActive && actionState?.action === value
                          ? "Running…"
                          : selectedActionByIncident[incident.id] === value
                            ? `${label} (preset)`
                            : label}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {actionState?.incidentId === incident.id &&
                !actionState.loading && (
                  <p
                    className={`mt-2 text-xs font-medium ${
                      actionState.error
                        ? "text-red-600"
                        : "text-emerald-700"
                    }`}
                  >
                    {actionState.error
                      ? `Action failed: ${actionState.error}`
                      : `Action accepted (${actionState.result})`}
                  </p>
                )}

              {expandedIncidentId === incident.id ? (
                <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 p-4">
                  {detailLoadingByIncidentId[incident.id] ? (
                    <p className="text-sm text-slate-600">Loading incident detail…</p>
                  ) : (
                    <>
                      <div className="mb-3">
                        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Action History
                        </p>
                        {detailByIncidentId[incident.id]?.ai_actions?.length ? (
                          <div className="mt-2 space-y-2">
                            {detailByIncidentId[incident.id].ai_actions.map(
                              (entry, index) => (
                                <div
                                  key={`${incident.id}-action-${index}`}
                                  className="rounded-lg bg-white px-3 py-2 text-xs text-slate-700"
                                >
                                  {String(entry.action ?? "unknown_action")} | {String(entry.status ?? "unknown_status")}
                                </div>
                              )
                            )}
                          </div>
                        ) : (
                          <p className="mt-1 text-xs text-slate-500">No actions recorded yet.</p>
                        )}
                      </div>

                      <div>
                        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Similar Incidents
                        </p>
                        {similarByIncidentId[incident.id]?.length ? (
                          <div className="mt-2 space-y-2">
                            {similarByIncidentId[incident.id].map((similar) => (
                              <div
                                key={`${incident.id}-similar-${similar.incident_id}`}
                                className="rounded-lg bg-white px-3 py-2 text-xs text-slate-700"
                              >
                                {similar.incident_id} | score {similar.score.toFixed(2)} | {similar.summary}
                              </div>
                            ))}
                          </div>
                        ) : (
                          <p className="mt-1 text-xs text-slate-500">No similar incidents available.</p>
                        )}
                      </div>
                    </>
                  )}
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </main>
  );
}
