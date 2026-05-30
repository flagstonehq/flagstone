export default function FlagsLoading() {
  return (
    <div className="flex h-full flex-col">
      <div className="border-border flex h-14 items-center justify-between border-b px-6">
        <div className="bg-border h-5 w-16 animate-pulse rounded" />
        <div className="bg-border h-9 w-28 animate-pulse rounded" />
      </div>
      <div className="space-y-2 p-6">
        <div className="bg-border h-10 w-full animate-pulse rounded" />
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="bg-border h-14 w-full animate-pulse rounded opacity-60" />
        ))}
      </div>
    </div>
  );
}
