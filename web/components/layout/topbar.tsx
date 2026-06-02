import type { ReactNode } from "react";
import { ThemeToggle } from "@/components/layout/theme-toggle";

interface TopbarProps {
  title: string;
  action?: ReactNode;
}

export function Topbar({ title, action }: TopbarProps) {
  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-border bg-surface px-4 sm:px-6">
      <h1 className="text-base font-semibold text-text">{title}</h1>
      <div className="flex items-center gap-1">
        <ThemeToggle />
        {action && <div>{action}</div>}
      </div>
    </header>
  );
}
