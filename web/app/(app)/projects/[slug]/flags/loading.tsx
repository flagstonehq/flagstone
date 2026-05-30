export default function FlagsLoading() {
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center justify-between border-b border-border px-6">
        <div className="h-5 w-16 animate-pulse rounded bg-border" />
        <div className="h-9 w-28 animate-pulse rounded bg-border" />
      </div>
      <div className="space-y-2 p-6">
        <div className="h-10 w-full animate-pulse rounded bg-border" />
        {Array.from({ length: 5 }).map((_, i) => (
          <div
            key={i}
            className="h-14 w-full animate-pulse rounded bg-border opacity-60"
          />
        ))}
      </div>
    </div>
  );
}
