"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import {
  fetchConnectedRepos,
  fetchCurrentMetrics,
  fetchAlerts,
  fetchTelemetrySource,
  saveTelemetrySource,
  scrapeTelemetrySource,
  type ConnectedRepo,
  type MetricSnapshot,
  type Alert,
  type TelemetrySource,
} from "@/lib/api";
import { useAuthStore } from "@/lib/auth-store";

const SEVERITY_BADGE: Record<string, string> = {
  critical: "bg-red-100 text-red-800 border-red-200",
  warning: "bg-amber-100 text-amber-800 border-amber-200",
};

const STATUS_BADGE: Record<string, string> = {
  open: "bg-rose-50 text-rose-700",
  resolved: "bg-emerald-50 text-emerald-700",
};

type MetricBarProps = {
  label: string;
  value: number;
  unit: string;
  threshold?: number;
  max?: number;
};

function MetricBar({ label, value, unit, threshold, max = 100 }: MetricBarProps) {
  const pct = Math.min((value / max) * 100, 100);
  const isBreaching = threshold !== undefined && value > threshold;
  return (
    <div>
      <div className="mb-1 flex justify-between text-xs text-gray-500">
        <span>{label}</span>
        <span className={isBreaching ? "font-bold text-red-600" : ""}>
          {value.toFixed(1)} {unit}
          {threshold !== undefined && (
            <span className="ml-1 text-gray-400">/ {threshold} thr</span>
          )}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100">
        <div
          className={`h-2 rounded-full transition-all ${
            isBreaching ? "bg-red-500" : "bg-emerald-400"
          }`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function formatAlertNumber(value: number | null): string {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "n/a";
  }
  return value.toFixed(2);
}

function formatAlertTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "n/a";
  }
  return parsed.toLocaleTimeString();
}

export default function ObservabilityPage() {
  const router = useRouter();
  const { token } = useAuthStore();
  const [projectId, setProjectId] = useState("");
  const [projects, setProjects] = useState<ConnectedRepo[]>([]);
  const [metrics, setMetrics] = useState<MetricSnapshot | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [telemetrySource, setTelemetrySource] = useState<TelemetrySource | null>(null);
  const [telemetrySourceType, setTelemetrySourceType] = useState("helmix-json");
  const [telemetryMetricsURL, setTelemetryMetricsURL] = useState("");
  const [telemetryEnabled, setTelemetryEnabled] = useState(true);
  const [telemetrySaveState, setTelemetrySaveState] = useState<{
    loading: boolean;
    message: string | null;
    error: string | null;
  }>({ loading: false, message: null, error: null });
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshIntervalSeconds, setRefreshIntervalSeconds] = useState(15);
  const [lastRefreshedAt, setLastRefreshedAt] = useState<Date | null>(null);

  useEffect(() => {
    if (!token) {
      router.replace("/login");
    }
  }, [token, router]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const projectFromQuery = params.get("project_id")?.trim() ?? "";
    if (projectFromQuery && projectFromQuery !== projectId) {
      setProjectId(projectFromQuery);
    }
  }, [projectId]);

  useEffect(() => {
    if (!token) {
      return;
    }

    let isCancelled = false;
    fetchConnectedRepos(token)
      .then((items) => {
        if (!isCancelled) {
          setProjects(items);
        }
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "unknown error";
        if (message === "unauthorized") {
          router.replace("/login");
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [router, token]);

  useEffect(() => {
    if (!token || !projectId.trim()) {
      setTelemetrySource(null);
      setTelemetryMetricsURL("");
      setTelemetrySourceType("helmix-json");
      setTelemetryEnabled(true);
      return;
    }

    let isCancelled = false;
    fetchTelemetrySource(token, projectId.trim())
      .then((source) => {
        if (isCancelled) {
          return;
        }
        setTelemetrySource(source);
        setTelemetryMetricsURL(source?.metrics_url ?? "");
        setTelemetrySourceType(source?.source_type ?? "helmix-json");
        setTelemetryEnabled(source?.enabled ?? true);
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "unknown error";
        if (message === "unauthorized") {
          router.replace("/login");
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [projectId, router, token]);

  const handleFetch = useCallback(async (options?: { silent?: boolean }) => {
    if (!token || !projectId.trim()) return;
    const silent = options?.silent ?? false;
    if (!silent) {
      setIsLoading(true);
    }
    setLoadError(null);
    try {
      const [m, a] = await Promise.all([
        fetchCurrentMetrics(token, projectId.trim()),
        fetchAlerts(token, projectId.trim()),
      ]);
      setMetrics(m);
      setAlerts(a);
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
  }, [projectId, router, token]);

  useEffect(() => {
    if (!autoRefresh || !token || !projectId.trim()) {
      return;
    }
    const intervalId = setInterval(() => {
      void handleFetch({ silent: true });
    }, refreshIntervalSeconds * 1000);
    return () => clearInterval(intervalId);
  }, [autoRefresh, handleFetch, projectId, refreshIntervalSeconds, token]);

  const alertItems = Array.isArray(alerts) ? alerts : [];
  const openAlerts = alertItems.filter((a) => a.status === "open");
  const resolvedAlerts = alertItems.filter((a) => a.status === "resolved");

  function openIncidentTriage(alert: Alert) {
    if (!projectId.trim()) {
      return;
    }
    const params = new URLSearchParams({
      project_id: projectId.trim(),
      alert_id: alert.id,
      alert_rule: alert.rule ?? "",
      alert_metric: alert.metric ?? "",
    });
    router.push(`/dashboard/incidents?${params.toString()}`);
  }

  async function handleSaveTelemetrySource() {
    if (!token || !projectId.trim() || !telemetryMetricsURL.trim()) {
      return;
    }
    setTelemetrySaveState({ loading: true, message: null, error: null });
    try {
      const saved = await saveTelemetrySource(token, projectId.trim(), {
        source_type: telemetrySourceType,
        metrics_url: telemetryMetricsURL.trim(),
        scrape_interval_seconds: telemetrySource?.scrape_interval_seconds ?? 30,
        enabled: telemetryEnabled,
      });
      setTelemetrySource(saved);
      setTelemetrySaveState({ loading: false, message: "Telemetry source saved.", error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "unknown error";
      setTelemetrySaveState({ loading: false, message: null, error: message });
    }
  }

  async function handleScrapeTelemetrySource() {
    if (!token || !projectId.trim()) {
      return;
    }
    setTelemetrySaveState({ loading: true, message: null, error: null });
    try {
      await scrapeTelemetrySource(token, projectId.trim());
      await handleFetch({ silent: true });
      const refreshedSource = await fetchTelemetrySource(token, projectId.trim());
      setTelemetrySource(refreshedSource);
      setTelemetrySaveState({ loading: false, message: "Telemetry scrape completed.", error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "unknown error";
      setTelemetrySaveState({ loading: false, message: null, error: message });
    }
  }

  return (
    <main className="mx-auto min-h-screen w-full max-w-6xl px-6 py-10">
      <h1 className="mb-6 text-2xl font-bold text-gray-800">Observability</h1>

      <div className="mb-3 flex flex-wrap gap-3">
        <select
          value={projectId}
          onChange={(event) => setProjectId(event.target.value)}
          className="w-80 rounded-xl border border-gray-300 px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400"
        >
          <option value="">Select connected project</option>
          {projects.map((project) => (
            <option key={project.project_id} value={project.project_id}>
              {project.project_name} ({project.github_repo})
            </option>
          ))}
        </select>
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
          {isLoading ? "Loading…" : "Load Metrics"}
        </button>
        <button
          onClick={() => void handleFetch({ silent: true })}
          disabled={!projectId.trim()}
          className="rounded-xl border border-amber-300 bg-amber-50 px-5 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100 disabled:opacity-40"
        >
          Refresh
        </button>
      </div>

      {projectId && projects.some((project) => project.project_id === projectId) ? (
        <p className="mb-4 text-xs text-gray-500">
          Viewing {projects.find((project) => project.project_id === projectId)?.project_name}.
        </p>
      ) : null}

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

      <section className="mb-6 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-base font-semibold text-gray-800">Real Telemetry Source</h2>
            <p className="text-sm text-gray-500">
              Configure a real metrics endpoint for this project. Supported source types are a Helmix JSON snapshot endpoint or Prometheus/OpenMetrics text exposition with Helmix metric names.
            </p>
          </div>
          {telemetrySource?.last_scraped_at ? (
            <p className="text-xs text-gray-500">
              Last scrape: {new Date(telemetrySource.last_scraped_at).toLocaleString()}
            </p>
          ) : null}
        </div>
        <div className="mt-4 grid gap-3 md:grid-cols-[180px_minmax(0,1fr)_120px]">
          <select
            value={telemetrySourceType}
            onChange={(event) => setTelemetrySourceType(event.target.value)}
            className="rounded-xl border border-gray-300 px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400"
          >
            <option value="helmix-json">Helmix JSON</option>
            <option value="prometheus">Prometheus</option>
          </select>
          <input
            type="url"
            placeholder="https://your-app.example.com/metrics"
            value={telemetryMetricsURL}
            onChange={(event) => setTelemetryMetricsURL(event.target.value)}
            className="rounded-xl border border-gray-300 px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400"
          />
          <label className="flex items-center gap-2 rounded-xl border border-gray-200 px-4 py-2 text-sm text-gray-600">
            <input
              type="checkbox"
              checked={telemetryEnabled}
              onChange={(event) => setTelemetryEnabled(event.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-amber-500 focus:ring-amber-300"
            />
            Enabled
          </label>
        </div>
        <div className="mt-3 flex flex-wrap gap-3">
          <button
            onClick={() => void handleSaveTelemetrySource()}
            disabled={telemetrySaveState.loading || !projectId.trim() || !telemetryMetricsURL.trim()}
            className="rounded-xl bg-slate-900 px-4 py-2 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
          >
            Save Source
          </button>
          <button
            onClick={() => void handleScrapeTelemetrySource()}
            disabled={telemetrySaveState.loading || !projectId.trim()}
            className="rounded-xl border border-emerald-300 bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-800 hover:bg-emerald-100 disabled:opacity-50"
          >
            Scrape Now
          </button>
        </div>
        {telemetrySaveState.message ? (
          <p className="mt-3 text-sm text-emerald-700">{telemetrySaveState.message}</p>
        ) : null}
        {telemetrySaveState.error ? (
          <p className="mt-3 text-sm text-red-700">{telemetrySaveState.error}</p>
        ) : null}
        {telemetrySource?.last_error ? (
          <p className="mt-3 text-sm text-amber-700">Last scrape error: {telemetrySource.last_error}</p>
        ) : null}
      </section>

      {loadError && (
        <p className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {loadError}
        </p>
      )}

      {/* Open alert banner */}
      {openAlerts.length > 0 && (
        <div className="mb-6 rounded-2xl border border-red-200 bg-red-50 px-5 py-4">
          <p className="mb-1 text-sm font-bold text-red-800">
            {openAlerts.length} open alert{openAlerts.length !== 1 ? "s" : ""}
          </p>
          <ul className="list-inside list-disc space-y-0.5 text-sm text-red-700">
            {openAlerts.map((a) => (
              <li key={a.id}>
                <span
                  className={`mr-1 rounded px-1.5 py-0.5 text-xs font-semibold ${SEVERITY_BADGE[a.severity] ?? ""}`}
                >
                  {a.severity}
                </span>
                {a.title}
                <button
                  onClick={() => openIncidentTriage(a)}
                  className="ml-2 rounded border border-red-300 bg-white px-2 py-0.5 text-xs font-semibold text-red-700 hover:bg-red-100"
                >
                  Open in Incidents
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Current metrics card */}
      {metrics && (
        <div className="mb-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-base font-semibold text-gray-700">
              Current Snapshot
            </h2>
            <span className="text-xs text-gray-400">
              {new Date(metrics.captured_at).toLocaleString()}
            </span>
          </div>
          <div className="space-y-4">
            <MetricBar
              label="CPU"
              value={metrics.cpu_pct}
              unit="%"
              threshold={85}
            />
            <MetricBar
              label="Memory"
              value={metrics.memory_pct}
              unit="%"
              threshold={90}
            />
            <MetricBar
              label="Error Rate"
              value={metrics.error_rate_pct}
              unit="%"
              threshold={5}
              max={30}
            />
            <MetricBar
              label="P99 Latency"
              value={metrics.p99_latency_ms}
              unit="ms"
              threshold={2000}
              max={5000}
            />
          </div>

          <div className="mt-5 grid grid-cols-3 gap-4 border-t border-gray-100 pt-4">
            <div className="text-center">
              <p className="text-2xl font-bold text-gray-800">
                {metrics.req_per_sec.toFixed(1)}
              </p>
              <p className="text-xs text-gray-400">req/s</p>
            </div>
            <div className="text-center">
              <p className="text-2xl font-bold text-gray-800">
                {metrics.ready_pod_count}
                <span className="text-base text-gray-400">
                  /{metrics.pod_count}
                </span>
              </p>
              <p className="text-xs text-gray-400">ready pods</p>
            </div>
          </div>
        </div>
      )}

      {/* Alerts table */}
      {alertItems.length > 0 && (
        <div className="rounded-2xl border border-gray-200 bg-white shadow-sm">
          <div className="border-b border-gray-100 px-5 py-3">
            <h2 className="text-sm font-semibold text-gray-700">
              Alerts ({alertItems.length})
            </h2>
          </div>
          <table className="w-full text-sm">
            <thead className="border-b border-gray-100 bg-gray-50">
              <tr>
                {["Rule", "Metric", "Value", "Threshold", "Severity", "Status", "Fired", "Actions"].map(
                  (col) => (
                    <th
                      key={col}
                      className="px-4 py-2 text-left text-xs font-semibold uppercase text-gray-400"
                    >
                      {col}
                    </th>
                  )
                )}
              </tr>
            </thead>
            <tbody>
              {[...openAlerts, ...resolvedAlerts].map((alert) => (
                <tr
                  key={alert.id}
                  className="border-b border-gray-50 last:border-0 hover:bg-gray-50"
                >
                  <td className="px-4 py-3 font-mono text-xs">{alert.rule}</td>
                  <td className="px-4 py-3 text-gray-600">{alert.metric}</td>
                  <td className="px-4 py-3 font-semibold">
                    {formatAlertNumber(alert.value)}
                  </td>
                  <td className="px-4 py-3 text-gray-400">
                    {formatAlertNumber(alert.threshold)}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded border px-2 py-0.5 text-xs font-semibold ${
                        SEVERITY_BADGE[alert.severity] ?? ""
                      }`}
                    >
                      {alert.severity}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded px-2 py-0.5 text-xs font-semibold ${
                        STATUS_BADGE[alert.status] ?? ""
                      }`}
                    >
                      {alert.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-400">
                    {formatAlertTime(alert.fired_at)}
                  </td>
                  <td className="px-4 py-3">
                    {alert.status === "open" ? (
                      <button
                        onClick={() => openIncidentTriage(alert)}
                        className="rounded border border-amber-300 bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-900 hover:bg-amber-100"
                      >
                        Investigate
                      </button>
                    ) : (
                      <span className="text-xs text-gray-400">-</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!metrics && !isLoading && !loadError && projectId && (
        <p className="text-sm text-gray-500">
          No metrics snapshot found for this project.
        </p>
      )}
    </main>
  );
}
