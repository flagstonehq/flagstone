export default function LoginLoading() {
  return (
    <main className="min-h-screen flex items-center justify-center bg-bg">
      <div className="w-full max-w-sm">
        <div className="bg-surface rounded-xl border border-border p-8 shadow-sm space-y-4">
          <div className="h-8 w-32 mx-auto rounded bg-border animate-pulse" />
          <div className="h-4 w-48 mx-auto rounded bg-border animate-pulse" />
          <div className="space-y-2 pt-4">
            <div className="h-4 w-16 rounded bg-border animate-pulse" />
            <div className="h-10 w-full rounded bg-border animate-pulse" />
          </div>
          <div className="space-y-2">
            <div className="h-4 w-20 rounded bg-border animate-pulse" />
            <div className="h-10 w-full rounded bg-border animate-pulse" />
          </div>
          <div className="h-10 w-full rounded bg-border animate-pulse" />
        </div>
      </div>
    </main>
  );
}
