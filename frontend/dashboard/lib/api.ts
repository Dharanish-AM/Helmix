import type { AuthUser } from "@/lib/auth-store";

const apiBaseURL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export function authURL(path: string) {
  return `${apiBaseURL}${path}`;
}

export async function fetchCurrentUser(token: string): Promise<AuthUser> {
  const response = await fetch(authURL("/api/v1/auth/me"), {
    headers: {
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });

  if (response.status === 401) {
    throw new Error("unauthorized");
  }
  if (!response.ok) {
    throw new Error(`auth_me_failed:${response.status}`);
  }

  const payload = (await response.json()) as { user: AuthUser };
  return payload.user;
}

export type ConnectedRepo = {
  repo_id: string;
  project_id: string;
  project_name: string;
  github_repo: string;
  default_branch: string;
  detected_stack?: {
    runtime?: string;
    framework?: string;
    [key: string]: unknown;
  };
  connected_at: string;
};

export async function fetchConnectedRepos(
  token: string,
  query = "",
  limit = 20
): Promise<ConnectedRepo[]> {
  const params = new URLSearchParams();
  if (query.trim() !== "") {
    params.set("q", query.trim());
  }
  params.set("limit", String(limit));

  const response = await fetch(authURL(`/api/v1/repos/projects?${params.toString()}`), {
    headers: {
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });

  if (response.status === 401) {
    throw new Error("unauthorized");
  }
  if (!response.ok) {
    throw new Error(`connected_repos_failed:${response.status}`);
  }

  const payload = (await response.json()) as { items: ConnectedRepo[] };
  return payload.items ?? [];
}

export async function connectRepository(
  token: string,
  githubRepo: string,
  defaultBranch = "main"
): Promise<ConnectedRepo> {
  const response = await fetch(authURL("/api/v1/repos/projects"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ github_repo: githubRepo, default_branch: defaultBranch }),
  });

  if (response.status === 401) {
    throw new Error("unauthorized");
  }
  if (!response.ok) {
    throw new Error(`connect_repo_failed:${response.status}`);
  }

  return response.json() as Promise<ConnectedRepo>;
}

export async function triggerRepositoryAnalysis(
  token: string,
  repoId: string,
  githubRepo: string
): Promise<void> {
  const response = await fetch(authURL("/api/v1/auth/repos/analyze"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ repo_id: repoId, github_repo: githubRepo }),
  });

  if (response.status === 401) {
    throw new Error("unauthorized");
  }
  if (!response.ok) {
    throw new Error(`analyze_repo_failed:${response.status}`);
  }
}

export type GitHubRepository = {
  id: number;
  name: string;
  full_name: string;
  private: boolean;
  default_branch: string;
  updated_at: string;
};

export async function fetchGitHubRepositories(
  token: string,
  query = "",
  limit = 50
): Promise<GitHubRepository[]> {
  const params = new URLSearchParams();
  if (query.trim() !== "") {
    params.set("q", query.trim());
  }
  params.set("limit", String(limit));

  const response = await fetch(authURL(`/api/v1/auth/github/repos?${params.toString()}`), {
    headers: {
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });

  if (response.status === 401) {
    throw new Error("unauthorized");
  }
  if (!response.ok) {
    throw new Error(`github_repos_failed:${response.status}`);
  }

  const payload = (await response.json()) as { items: GitHubRepository[] };
  return payload.items ?? [];
}

// ---------------------------------------------------------------------------
// Phase 3 – Observability
// ---------------------------------------------------------------------------

export type MetricSnapshot = {
  project_id: string;
  captured_at: string;
  cpu_pct: number;
  memory_pct: number;
  req_per_sec: number;
  error_rate_pct: number;
  p99_latency_ms: number;
  pod_count: number;
  ready_pod_count: number;
};

export type Alert = {
  id: string;
  project_id: string;
  rule: string;
  severity: "warning" | "critical";
  status: "open" | "resolved";
  title: string;
  description: string;
  metric: string;
  value: number;
  threshold: number;
  fired_at: string;
  resolved_at: string | null;
};

export async function fetchCurrentMetrics(
  token: string,
  projectId: string
): Promise<MetricSnapshot | null> {
  const response = await fetch(
    authURL(`/api/v1/observability/metrics/${projectId}/current`),
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" }
  );
  if (response.status === 404) return null;
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`metrics_failed:${response.status}`);
  return response.json() as Promise<MetricSnapshot>;
}

export async function fetchAlerts(
  token: string,
  projectId: string
): Promise<Alert[]> {
  const response = await fetch(
    authURL(`/api/v1/observability/alerts/${projectId}`),
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" }
  );
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`alerts_failed:${response.status}`);
  return response.json() as Promise<Alert[]>;
}

// ---------------------------------------------------------------------------
// Phase 3 – Incidents
// ---------------------------------------------------------------------------

export type RecommendedAction = {
  action: string;
  params: Record<string, unknown>;
};

export type Diagnosis = {
  root_cause: string;
  confidence: number;
  reasoning: string;
  recommended_actions: RecommendedAction[];
  auto_execute: boolean;
};

export type Incident = {
  id: string;
  alert_id: string;
  project_id: string;
  ai_diagnosis: Diagnosis;
  ai_actions: Record<string, unknown>[];
  resolved_at: string | null;
  created_at: string;
};

export type SimilarIncident = {
  incident_id: string;
  score: number;
  summary: string;
};

export type IncidentPage = {
  items: Incident[];
  total: number;
  limit: number;
  offset: number;
};

export type DeploymentSummary = {
  id: string;
  repo_id: string;
  commit_sha: string;
  branch: string;
  status: string;
  environment: string;
  image_tag?: string;
  created_at: string;
  deployed_at?: string | null;
};

export async function fetchIncidents(
  token: string,
  projectId: string,
  limit = 20,
  offset = 0
): Promise<IncidentPage> {
  const response = await fetch(
    authURL(
      `/api/v1/incidents/projects/${encodeURIComponent(projectId)}?limit=${limit}&offset=${offset}`
    ),
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" }
  );
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`incidents_failed:${response.status}`);
  return response.json() as Promise<IncidentPage>;
}

export async function fetchIncidentDetail(
  token: string,
  incidentId: string
): Promise<Incident> {
  const response = await fetch(
    authURL(`/api/v1/incidents/${encodeURIComponent(incidentId)}`),
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" }
  );
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`incident_detail_failed:${response.status}`);
  return response.json() as Promise<Incident>;
}

export async function fetchSimilarIncidents(
  token: string,
  incidentId: string
): Promise<SimilarIncident[]> {
  const response = await fetch(
    authURL(`/api/v1/incidents/${encodeURIComponent(incidentId)}/similar`),
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" }
  );
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`incident_similar_failed:${response.status}`);
  return response.json() as Promise<SimilarIncident[]>;
}

export async function fetchProjectDeployments(
  token: string,
  projectId: string,
  limit = 20
): Promise<DeploymentSummary[]> {
  const response = await fetch(
    authURL(
      `/api/v1/deployments/deployments?project_id=${encodeURIComponent(projectId)}&limit=${limit}`
    ),
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" }
  );
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`deployments_failed:${response.status}`);
  return response.json() as Promise<DeploymentSummary[]>;
}

export async function triggerIncidentAction(
  token: string,
  incidentId: string,
  action: string,
  params: Record<string, unknown> = {}
): Promise<{ incident_id: string; action: string; status: string }> {
  const response = await fetch(
    authURL(`/api/v1/incidents/${encodeURIComponent(incidentId)}/actions`),
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ action, params }),
    }
  );
  if (response.status === 401) throw new Error("unauthorized");
  if (!response.ok) throw new Error(`action_failed:${response.status}`);
  return response.json() as Promise<{
    incident_id: string;
    action: string;
    status: string;
  }>;
}