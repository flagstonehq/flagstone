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

const GLOBAL_ENV = [{ href: "/projects", label: "Projects", icon: FolderOpen }];

function projectNav(slug: string) {
  return [
    { href: `/projects/${slug}/flags`, label: "Flags", icon: Flag },
    { href: `/projects/${slug}/segments`, label: "Segments", icon: Users },
    { href: `/projects/${slug}/api-keys`, label: "API Keys", icon: Key },
    { href: `/projects/${slug}/audit`, label: "Audit", icon: ScrollText },
    { href: `/projects/${slug}/settings`, label: "Settings", icon: Settings },
  ];
}

interface SidebarProps {
  projectSlug?: string;
}

export function Sidebar({ projectSlug }: SidebarProps) {
  const pathname = usePathname();
  const router = useRouter();

  const nav = projectSlug ? projectNav(projectSlug) : GLOBAL_ENV;

  async function handleLogout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.push("/login");
    router.refresh();
  }

  return (
    <aside className="border-border bg-surface flex w-56 shrink-0 flex-col border-r">
      {/* Logo */}
      <div className="border-border flex h-14 items-center border-b px-4">
        <span className="text-primary text-lg font-bold">⚑ Flagstone</span>
      </div>
      {/* Back to projects (only inside a project) */}
      {projectSlug && (
        <div className="px-2 pt-2">
          <Link
            href="/projects"
            className="text-text-secondary hover:text-text hover:bg-hover flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-colors"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            All projects
          </Link>
        </div>
      )}
      {/* Nav */}
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
      {/* Logout */}
      <div className="border-border border-t p-2">
        <button
          onClick={handleLogout}
          className="text-text-secondary hover:bg-hover hover:text-text flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors"
        >
          <LogOut className="h-4 w-4 shrink-0" />
          Sign out
        </button>
      </div>
    </aside>
  );
}
