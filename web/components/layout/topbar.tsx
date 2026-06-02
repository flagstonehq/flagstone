import type { ReactNode } from "react";
import Link from "next/link";
import { ChevronLeft } from "lucide-react";

interface TopbarProps {
  title: string;
  action?: ReactNode;
  backHref?: string;
  backLabel?: string;
}

export function Topbar({ title, action, backHref, backLabel }: TopbarProps) {
  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-border bg-surface px-4 sm:px-6">
      <div className="flex items-center gap-2 min-w-0">
        {backHref && (
          <Link
            href={backHref}
            className="flex items-center gap-1 text-text-secondary hover:text-text transition-colors shrink-0"
            aria-label={backLabel ?? "Go back"}
          >
            <ChevronLeft className="h-4 w-4" />
            <span className="text-sm hidden sm:inline">{backLabel ?? "Back"}</span>
          </Link>
        )}
        {backHref && <span className="text-border text-sm shrink-0">/</span>}
        <h1 className="text-base font-semibold text-text truncate">{title}</h1>
      </div>
      {action && <div className="shrink-0 ml-2">{action}</div>}
    </header>
  );
}
