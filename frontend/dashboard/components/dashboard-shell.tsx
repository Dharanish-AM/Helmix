"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import {
  connectRepository,
  fetchConnectedRepos,
  fetchCurrentUser,
  fetchGitHubRepositories,
  triggerRepositoryAnalysis,
  type ConnectedRepo,
  type GitHubRepository,
} from "@/lib/api";
import { type AuthUser, useAuthStore } from "@/lib/auth-store";

type RepoCard = {
  id: string;
  projectId: string;
  projectName: string;
  name: string;
  stack: string;
  status: string;
  lastDeploy: string;
  analysisComplete: boolean;
};

type DashboardShellProps = {
  tokenFromQuery?: string | null;
};

export function DashboardShell({ tokenFromQuery = null }: DashboardShellProps) {
  const router = useRouter();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [repos, setRepos] = useState<RepoCard[]>([]);
  const [repoQuery, setRepoQuery] = useState("");
  const [reposLoading, setReposLoading] = useState(false);
  const [githubRepos, setGitHubRepos] = useState<GitHubRepository[]>([]);
  const [githubReposLoading, setGitHubReposLoading] = useState(false);
  const [repoActionError, setRepoActionError] = useState<string | null>(null);
  const [isConnectingRepo, setIsConnectingRepo] = useState(false);
  const analyzedRepoIDsRef = useRef<Set<string>>(new Set());
  const { token, user, setSession, clearSession } = useAuthStore();

  const activeToken = tokenFromQuery ?? token;

  useEffect(() => {
    if (!activeToken) {
      setIsLoading(false);
      setCurrentUser(null);
      return;
    }

    let isCancelled = false;
    setIsLoading(true);
    setLoadError(null);

    fetchCurrentUser(activeToken)
      .then((loadedUser) => {
        if (isCancelled) {
          return;
        }
        setCurrentUser(loadedUser);
        if (tokenFromQuery) {
          setSession(tokenFromQuery, loadedUser);
          router.replace("/dashboard");
          return;
        }
        setSession(activeToken, loadedUser);
      })
      .catch((error: Error) => {
        if (isCancelled) {
          return;
        }
        setLoadError(error.message);
      })
      .finally(() => {
        if (!isCancelled) {
          setIsLoading(false);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [activeToken, router, setSession, tokenFromQuery]);

  useEffect(() => {
    if (loadError !== "unauthorized") {
      return;
    }
    clearSession();
    router.replace("/login");
  }, [clearSession, loadError, router]);

  useEffect(() => {
    if (!activeToken || loadError) {
      return;
    }

    let isCancelled = false;
    setReposLoading(true);
    setRepoActionError(null);

    fetchConnectedRepos(activeToken)
      .then((items) => {
        if (isCancelled) {
          return;
        }
        setRepos(items.map(toRepoCard));
      })
      .catch((error: Error) => {
        if (isCancelled) {
          return;
        }
        setRepoActionError(error.message);
      })
      .finally(() => {
        if (!isCancelled) {
          setReposLoading(false);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [activeToken, loadError]);

  useEffect(() => {
    if (!isModalOpen || !activeToken) {
      return;
    }

    let isCancelled = false;
    const timeout = setTimeout(() => {
      setGitHubReposLoading(true);
      fetchGitHubRepositories(activeToken, repoQuery)
        .then((items) => {
          if (!isCancelled) {
            setGitHubRepos(items);
          }
        })
        .catch((error: Error) => {
          if (!isCancelled) {
            setRepoActionError(error.message);
          }
        })
        .finally(() => {
          if (!isCancelled) {
            setGitHubReposLoading(false);
          }
        });
    }, 250);

    return () => {
      isCancelled = true;
      window.clearTimeout(timeout);
    };
  }, [activeToken, isModalOpen, repoQuery]);

  useEffect(() => {
    if (!activeToken || repos.length === 0) {
      return;
    }

    const pendingRepos = repos.filter((repo) => {
      if (analyzedRepoIDsRef.current.has(repo.id)) {
        return false;
      }
      return !repo.analysisComplete;
    });
    if (pendingRepos.length === 0) {
      return;
    }

    pendingRepos.forEach((repo) => analyzedRepoIDsRef.current.add(repo.id));

    let isCancelled = false;
    Promise.allSettled(
      pendingRepos.map((repo) => triggerRepositoryAnalysis(activeToken, repo.id, repo.name))
    )
      .then(async () => {
        if (isCancelled) {
          return;
        }
        const refreshed = await fetchConnectedRepos(activeToken);
        if (!isCancelled) {
          setRepos(refreshed.map(toRepoCard));
        }
      })
      .catch(() => {
        // Keep the current UI state if analysis fails for any repo.
      });

    return () => {
      isCancelled = true;
    };
  }, [activeToken, repos]);

  if (!activeToken || isLoading) {
    return (
      <main className="mx-auto flex min-h-screen w-full max-w-6xl items-center justify-center px-6 py-16">
        <div className="rounded-3xl border border-amber-200 bg-white/90 px-8 py-6 text-sm font-semibold text-amber-900 shadow-sm">
          Loading your Helmix workspace...
        </div>
      </main>
    );
  }

  if (loadError || (!user && !currentUser)) {
    return null;
  }

  const sessionUser = currentUser ?? user;
  if (!sessionUser) {
    return null;
  }

  const canConnectRepo = repoQuery.trim().includes("/");
  const connectedRepoNames = new Set(repos.map((repo) => repo.name.toLowerCase()));

  function openProjectPage(path: string, projectId: string) {
    const params = new URLSearchParams({ project_id: projectId });
    router.push(`${path}?${params.toString()}`);
  }

  async function handleConnectRepository(fullName?: string, defaultBranch?: string) {
    const targetRepo = (fullName ?? repoQuery).trim();
    if (!activeToken || !targetRepo.includes("/") || isConnectingRepo) {
      return;
    }

    setIsConnectingRepo(true);
    setRepoActionError(null);
    try {
      const connected = await connectRepository(activeToken, targetRepo, defaultBranch || "main");
      await triggerRepositoryAnalysis(activeToken, connected.repo_id, connected.github_repo);
      const refreshed = await fetchConnectedRepos(activeToken);
      setRepos(refreshed.map(toRepoCard));
      const refreshedGitHubRepos = await fetchGitHubRepositories(activeToken, repoQuery);
      setGitHubRepos(refreshedGitHubRepos);
      setRepoQuery("");
      setIsModalOpen(false);
    } catch (error) {
      const message = error instanceof Error ? error.message : "connect_repo_failed";
      setRepoActionError(message);
    } finally {
      setIsConnectingRepo(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-6xl flex-col gap-6 px-6 py-12">
      <header className="flex flex-col gap-4 rounded-[2rem] border border-amber-200 bg-white/85 p-6 shadow-sm backdrop-blur md:flex-row md:items-center md:justify-between">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.35em] text-amber-700">Helmix</p>
          <h1 className="mt-2 text-3xl font-bold text-slate-900">{sessionUser.org_name}</h1>
          <p className="mt-2 text-sm text-slate-600">
            Signed in as {sessionUser.username}. Connect a repository to begin stack detection and infrastructure generation.
          </p>
        </div>
        <div className="flex items-center gap-4">
          <button
            className="rounded-full border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
            onClick={() => router.push("/dashboard/observability")}
            type="button"
          >
            Observability
          </button>
          <button
            className="rounded-full border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
            onClick={() => router.push("/dashboard/incidents")}
            type="button"
          >
            Incidents
          </button>
          <button
            className="rounded-full border border-slate-300 bg-slate-950 px-5 py-3 text-sm font-semibold text-white transition hover:bg-slate-800"
            onClick={() => setIsModalOpen(true)}
            type="button"
          >
            Connect Repository
          </button>
          <div className="flex items-center gap-3 rounded-full border border-slate-200 bg-slate-50 px-3 py-2">
            <div className="h-10 w-10 overflow-hidden rounded-full bg-amber-200">
              {sessionUser.avatar_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img alt={sessionUser.username} className="h-full w-full object-cover" src={sessionUser.avatar_url} />
              ) : (
                <div className="flex h-full w-full items-center justify-center text-sm font-bold text-slate-700">
                  {sessionUser.username.slice(0, 2).toUpperCase()}
                </div>
              )}
            </div>
            <div>
              <p className="text-sm font-semibold text-slate-900">{sessionUser.username}</p>
              <p className="text-xs uppercase tracking-wide text-slate-500">{sessionUser.role}</p>
            </div>
          </div>
        </div>
      </header>

      <section className="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
        <div className="rounded-[2rem] border border-amber-200 bg-white/80 p-6 shadow-sm backdrop-blur">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold uppercase tracking-wide text-amber-700">Repositories</p>
              <h2 className="mt-2 text-2xl font-bold text-slate-900">Connected Repos</h2>
            </div>
            <span className="rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-amber-800">
              {repos.length} linked
            </span>
          </div>

          {reposLoading ? (
            <div className="mt-8 rounded-[1.75rem] border border-dashed border-amber-300 bg-gradient-to-br from-amber-50 to-orange-50 p-10 text-center text-sm text-slate-600">
              Loading connected repositories...
            </div>
          ) : null}

          {!reposLoading && repos.length === 0 ? (
            <div className="mt-8 rounded-[1.75rem] border border-dashed border-amber-300 bg-gradient-to-br from-amber-50 to-orange-50 p-10 text-center">
              <div className="mx-auto flex h-24 w-24 items-center justify-center rounded-full bg-white text-4xl shadow-sm">[]</div>
              <h3 className="mt-6 text-xl font-bold text-slate-900">No repositories connected yet</h3>
              <p className="mt-3 text-sm text-slate-600">
                Use the connect flow to authorize a GitHub repository, then Helmix will analyze the stack and prepare the next automation stages.
              </p>
            </div>
          ) : null}

          {!reposLoading && repos.length > 0 ? (
            <div className="mt-6 space-y-3">
              {repos.map((repo) => (
                <div key={repo.id} className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="font-semibold text-slate-900">{repo.projectName}</p>
                      <p className="text-xs text-slate-500">{repo.name}</p>
                    </div>
                    <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-700">{repo.status}</span>
                  </div>
                  <p className="mt-1 text-sm text-slate-600">Stack: {repo.stack}</p>
                  <p className="mt-1 font-mono text-[11px] text-slate-500">Project ID: {repo.projectId}</p>
                  <p className="mt-1 text-xs text-slate-500">Connected {repo.lastDeploy}</p>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <button
                      className="rounded-full border border-amber-300 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-900 hover:bg-amber-100"
                      onClick={() => openProjectPage("/dashboard/observability", repo.projectId)}
                      type="button"
                    >
                      Observability
                    </button>
                    <button
                      className="rounded-full border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                      onClick={() => openProjectPage("/dashboard/incidents", repo.projectId)}
                      type="button"
                    >
                      Incidents + Deployments
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>

        <aside className="rounded-[2rem] border border-slate-200 bg-slate-950 p-6 text-slate-50 shadow-sm">
          <p className="text-sm font-semibold uppercase tracking-wide text-amber-300">Workspace Status</p>
          <h2 className="mt-2 text-2xl font-bold">Phase 1 Control Plane</h2>
          <div className="mt-6 space-y-4 text-sm text-slate-300">
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
              <p className="font-semibold text-white">Authentication</p>
              <p className="mt-1">OAuth callback, JWT issuance, refresh rotation, and gateway auth are active.</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
              <p className="font-semibold text-white">Repo Analysis</p>
              <p className="mt-1">Detection rules are implemented for Node, Python, Java, Go, Ruby, Docker, and DB hints.</p>
            </div>
            <div className="rounded-2xl border border-amber-300/20 bg-amber-300/10 p-4">
              <p className="font-semibold text-amber-200">Phase 3 Tools</p>
              <div className="mt-2 flex gap-2">
                <button
                  className="rounded-full bg-white/10 px-3 py-1 text-xs font-semibold text-amber-100 hover:bg-white/20"
                  onClick={() => router.push("/dashboard/observability")}
                  type="button"
                >
                  Open Observability
                </button>
                <button
                  className="rounded-full bg-white/10 px-3 py-1 text-xs font-semibold text-amber-100 hover:bg-white/20"
                  onClick={() => router.push("/dashboard/incidents")}
                  type="button"
                >
                  Open Incidents
                </button>
              </div>
            </div>
          </div>
        </aside>
      </section>

      {isModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 px-6">
          <div className="w-full max-w-xl rounded-[2rem] border border-amber-200 bg-white p-6 shadow-2xl">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-sm font-semibold uppercase tracking-wide text-amber-700">GitHub Repo Picker</p>
                <h3 className="mt-2 text-2xl font-bold text-slate-900">Connect a repository</h3>
              </div>
              <button className="text-sm font-semibold text-slate-500" onClick={() => setIsModalOpen(false)} type="button">
                Close
              </button>
            </div>
            <p className="mt-4 text-sm text-slate-600">
              Browse your GitHub repositories and click connect, or type <span className="font-semibold">owner/name</span> manually.
            </p>
            <div className="mt-6 space-y-4">
              <input
                className="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600 outline-none"
                onChange={(event) => setRepoQuery(event.target.value)}
                placeholder="Search your GitHub repositories"
                type="text"
                value={repoQuery}
              />
              {repoActionError ? <p className="text-xs text-rose-600">{repoActionError}</p> : null}
              <button
                className="w-full rounded-2xl bg-slate-950 px-5 py-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!canConnectRepo || isConnectingRepo}
                onClick={() => handleConnectRepository()}
                type="button"
              >
                {isConnectingRepo ? "Connecting..." : "Confirm Repository"}
              </button>

              {githubReposLoading ? (
                <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600">
                  Loading your GitHub repositories...
                </div>
              ) : null}

              {!githubReposLoading && githubRepos.length > 0 ? (
                <div className="max-h-48 space-y-2 overflow-auto rounded-2xl border border-slate-200 bg-slate-50 p-3">
                  {githubRepos.map((repo) => (
                    <div key={repo.id} className="flex items-center gap-2 rounded-xl border border-slate-200 bg-white p-2">
                      <button
                        className="flex-1 rounded-lg border border-transparent px-2 py-1 text-left text-sm text-slate-700 hover:border-amber-200"
                        onClick={() => setRepoQuery(repo.full_name)}
                        type="button"
                      >
                        <p className="font-semibold text-slate-900">{repo.full_name}</p>
                        <p className="text-xs text-slate-500">
                          {repo.private ? "Private" : "Public"} • Updated {formatRelativeTimestamp(repo.updated_at)}
                        </p>
                      </button>
                      <button
                        className="rounded-lg bg-slate-950 px-3 py-2 text-xs font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
                        disabled={isConnectingRepo || connectedRepoNames.has(repo.full_name.toLowerCase())}
                        onClick={() => handleConnectRepository(repo.full_name, repo.default_branch)}
                        type="button"
                      >
                        {connectedRepoNames.has(repo.full_name.toLowerCase()) ? "Connected" : "Connect"}
                      </button>
                    </div>
                  ))}
                </div>
              ) : null}

              {!githubReposLoading && githubRepos.length === 0 ? (
                <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600">
                  No repositories found for this account/filter.
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </main>
  );
}

function toRepoCard(repo: ConnectedRepo): RepoCard {
  const runtime = typeof repo.detected_stack?.runtime === "string" ? repo.detected_stack.runtime.trim() : "";
  const framework = typeof repo.detected_stack?.framework === "string" ? repo.detected_stack.framework.trim() : "";
  const analysisComplete = Boolean(repo.detected_stack && Object.keys(repo.detected_stack).length > 0);
  const stack = [runtime, framework].filter(Boolean).join(" / ");

  return {
    id: repo.repo_id,
    projectId: repo.project_id,
    projectName: repo.project_name || repo.github_repo,
    name: repo.github_repo,
    stack: stack || (analysisComplete ? "analyzed (stack unknown)" : "pending analysis"),
    status: analysisComplete ? "analyzed" : "linked",
    lastDeploy: formatRelativeTimestamp(repo.connected_at),
    analysisComplete,
  };
}

function formatRelativeTimestamp(value: string): string {
  const timestamp = new Date(value);
  if (Number.isNaN(timestamp.getTime())) {
    return "recently";
  }

  const diffMs = Date.now() - timestamp.getTime();
  const diffMinutes = Math.max(1, Math.floor(diffMs / (1000 * 60)));
  if (diffMinutes < 60) {
    return `${diffMinutes}m ago`;
  }

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours}h ago`;
  }

  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ago`;
}