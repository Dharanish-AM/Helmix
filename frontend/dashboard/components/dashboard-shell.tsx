"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { fetchCurrentUser } from "@/lib/api";
import { type AuthUser, useAuthStore } from "@/lib/auth-store";

type RepoCard = {
  name: string;
  stack: string;
  status: string;
  lastDeploy: string;
};

type DashboardShellProps = {
  tokenFromQuery?: string | null;
};

const emptyRepos: RepoCard[] = [];

export function DashboardShell({ tokenFromQuery = null }: DashboardShellProps) {
  const router = useRouter();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
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
              {emptyRepos.length} linked
            </span>
          </div>

          {emptyRepos.length === 0 ? (
            <div className="mt-8 rounded-[1.75rem] border border-dashed border-amber-300 bg-gradient-to-br from-amber-50 to-orange-50 p-10 text-center">
              <div className="mx-auto flex h-24 w-24 items-center justify-center rounded-full bg-white text-4xl shadow-sm">[]</div>
              <h3 className="mt-6 text-xl font-bold text-slate-900">No repositories connected yet</h3>
              <p className="mt-3 text-sm text-slate-600">
                Use the connect flow to authorize a GitHub repository, then Helmix will analyze the stack and prepare the next automation stages.
              </p>
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
              The authenticated dashboard flow is wired. Repository search and submission will connect to the next API slice once the repo list endpoints are added.
            </p>
            <div className="mt-6 space-y-4">
              <input
                className="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600 outline-none"
                disabled
                placeholder="Search your GitHub repositories"
                type="text"
              />
              <button className="w-full rounded-2xl bg-slate-950 px-5 py-3 text-sm font-semibold text-white opacity-60" disabled type="button">
                Confirm Repository
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </main>
  );
}