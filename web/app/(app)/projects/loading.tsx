export default function ProjectsLoading() {
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center justify-between border-b border-border px-6">
        <div className="h-5 w-24 animate-pulse rounded bg-border" />
        <div className="h-9 w-32 animate-pulse rounded bg-border" />
      </div>

      <div className="grid grid-cols-1 gap-4 p-6 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="animate-pulse space-y-3 rounded-xl border border-border bg-surface p-5"
          >
            <div className="h-5 w-2/3 rounded bg-border" />
            <div className="h-4 w-1/3 rounded bg-border" />
            <div className="h-4 w-1/2 rounded bg-border" />
          </div>
        ))}
      </div>
    </div>
  );
}
