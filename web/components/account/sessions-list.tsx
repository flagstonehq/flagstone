"use client";

import { useState } from "react";
import type { Session } from "@/lib/types";
import { revokeSession, revokeAllSessions, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Loader2, LogOut, Monitor, Smartphone } from "lucide-react";

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function SessionRow({ session, onRevoke, revoking }: { session: Session; onRevoke: () => void; revoking: boolean }) {
  const isMobile = !session.userAgent || /mobile|android|iphone|ipad/i.test(session.userAgent ?? "");
  return (
    <div className="flex items-center justify-between rounded-lg border border-border px-4 py-3">
      <div className="flex items-start gap-3">
        {isMobile ? <Smartphone className="mt-0.5 h-4 w-4 shrink-0 text-text-secondary" /> : <Monitor className="mt-0.5 h-4 w-4 shrink-0 text-text-secondary" />}
        <div className="space-y-0.5">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">
              {session.ipAddress ?? "Unknown IP"}
            </span>
            {session.isCurrent && (
              <span className="rounded-full bg-primary-bg px-2 py-0.5 text-[10px] font-medium text-primary">Current</span>
            )}
          </div>
          {session.userAgent && (
            <p className="text-xs text-text-secondary truncate max-w-xs">{session.userAgent}</p>
          )}
          <p className="text-xs text-text-secondary">
            Created {formatDate(session.createdAt)} &middot; Expires {formatDate(session.expiresAt)}
          </p>
        </div>
      </div>
      <Button
        variant="ghost"
        size="sm"
        onClick={onRevoke}
        disabled={session.isCurrent || revoking}
        title={session.isCurrent ? "Cannot revoke current session" : "Sign out this session"}
      >
        {revoking ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <LogOut className="h-3.5 w-3.5" />}
      </Button>
    </div>
  );
}

export function SessionsList({ initial }: { initial: Session[] }) {
  const [sessions, setSessions] = useState<Session[]>(initial);
  const [revokingIds, setRevokingIds] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);

  async function handleRevoke(id: string) {
    setRevokingIds((prev) => new Set(prev).add(id));
    setError(null);
    try {
      await revokeSession(id);
      setSessions((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to revoke session.");
    } finally {
      setRevokingIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  }

  async function handleRevokeAll() {
    setError(null);
    try {
      await revokeAllSessions();
      setSessions((prev) => prev.filter((s) => s.isCurrent));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to revoke sessions.");
    }
  }

  const otherSessions = sessions.filter((s) => !s.isCurrent);

  return (
    <section>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold">Active sessions</h2>
        {otherSessions.length > 0 && (
          <Button variant="outline" size="sm" onClick={handleRevokeAll}>
            <LogOut className="mr-1.5 h-3.5 w-3.5" />
            Sign out other sessions
          </Button>
        )}
      </div>
      {error && <p className="mb-3 text-sm text-danger">{error}</p>}
      {sessions.length === 0 ? (
        <p className="text-sm text-text-secondary">No active sessions.</p>
      ) : (
        <div className="space-y-2">
          {sessions.map((session) => (
            <SessionRow
              key={session.id}
              session={session}
              onRevoke={() => handleRevoke(session.id)}
              revoking={revokingIds.has(session.id)}
            />
          ))}
        </div>
      )}
    </section>
  );
}
