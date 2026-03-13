export default function LoginPage() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-3xl items-center justify-center px-6">
      <div className="w-full rounded-2xl border border-amber-200 bg-white p-10 text-center shadow-sm">
        <h1 className="text-3xl font-bold text-gray-900">Connect with GitHub</h1>
        <p className="mt-3 text-gray-600">Authentication flow wiring starts in Phase 1.</p>
        <a
          className="mt-8 inline-block rounded-lg bg-gray-900 px-5 py-3 text-sm font-semibold text-white"
          href="/api/v1/auth/github"
        >
          Continue with GitHub
        </a>
      </div>
    </main>
  );
}
