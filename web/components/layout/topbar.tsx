import type { ReactNode } from "react";

interface TopbarProps {
  title: string;
  action?: ReactNode;
}

export function Topbar({ title, action }: TopbarProps) {
  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-border bg-surface px-6">
      <h1 className="text-base font-semibold text-text">{title}</h1>
      {action && <div>{action}</div>}
    </header>
  );
}
