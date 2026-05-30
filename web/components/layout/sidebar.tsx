"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  FolderOpen,
  Flag,
  Users,
  Key,
  ScrollText,
  Settings,
  LogOut,
  ChevronLeft,
} from "lucide-react";
import { cn } from "@/lib/utils";

function useProjectSlug(): string | null {
  const pathname = usePathname();
  // Match /projects/:slug/...
  const match = pathname.match(/^\/projects\/([^/]+)/);
  return match?.[1] ?? null;
}

const GLOBAL_NAV = [
  { href: "/projects", label: "Projects", icon: FolderOpen },
];

function projectNav(slug: string) {
  return [
    { href: `/projects/${slug}/flags`,    label: "Flags",    icon: Flag },
    { href: `/projects/${slug}/segments`, label: "Segments", icon: Users },
    { href: `/projects/${slug}/api-keys`, label: "API Keys", icon: Key },
    { href: `/projects/${slug}/audit`,    label: "Audit",    icon: ScrollText },
    { href: `/projects/${slug}/settings`, label: "Settings", icon: Settings },
  ];
}

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const projectSlug = useProjectSlug();

  const nav = projectSlug ? projectNav(projectSlug) : GLOBAL_NAV;

  async function handleLogout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.push("/login");
    router.refresh();
  }

  return (
    <aside className="flex h-screen w-56 shrink-0 flex-col border-r border-border bg-surface">
      <div className="flex h-14 items-center border-b border-border px-4">
        <span className="text-lg font-bold text-primary">⚑ Flagstone</span>
      </div>

      {projectSlug && (
        <div className="px-2 pt-2">
          <Link
            href="/projects"
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-hover hover:text-text"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            All projects
          </Link>
        </div>
      )}

      <nav className="flex-1 space-y-0.5 p-2">
        {nav.map(({ href, label, icon: Icon }) => (
          <Link
            key={href}
            href={href}
            className={cn(
              "flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
              pathname.startsWith(href)
                ? "bg-primary-bg text-primary"
                : "text-text-secondary hover:bg-hover hover:text-text",
            )}
          >
            <Icon className="h-4 w-4 shrink-0" />
            {label}
          </Link>
        ))}
      </nav>

      <div className="border-t border-border p-2">
        <button
          onClick={handleLogout}
          className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-hover hover:text-text"
        >
          <LogOut className="h-4 w-4 shrink-0" />
          Sign out
        </button>
      </div>
    </aside>
  );
}
