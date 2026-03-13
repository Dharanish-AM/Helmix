export default function DashboardPage() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-5xl flex-col gap-6 px-6 py-16">
      <div className="rounded-2xl border border-amber-200 bg-white/80 p-8 shadow-sm backdrop-blur">
        <p className="text-sm font-semibold uppercase tracking-wide text-amber-700">Helmix</p>
        <h1 className="mt-2 text-3xl font-bold text-gray-900">Dashboard bootstrap complete</h1>
        <p className="mt-3 text-gray-600">
          Connect your repository to start stack detection and autonomous infrastructure generation.
        </p>
      </div>
    </main>
  );
}
