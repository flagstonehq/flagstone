import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuditTable } from "@/components/audit/audit-table";
import { AuditFilters } from "@/components/audit/audit-filters";
import { getAuditLog } from "@/lib/api";
import type { AuditEntry } from "@/lib/types";

const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: vi.fn() }),
  usePathname: () => "/projects/my-app/audit",
  useSearchParams: () => new URLSearchParams(),
}));

const baseEntry: AuditEntry = {
  id: "00000000-0000-4000-a000-000000000001",
  actorId: "00000000-0000-4000-a000-0000000000a1",
  actorType: "user",
  action: "flag.created",
  resourceType: "flag",
  resourceId: "00000000-0000-4000-a000-0000000000b2",
  changes: null,
  ipAddress: null,
  userAgent: null,
  createdAt: new Date().toISOString(),
};

afterEach(() => {
  vi.clearAllMocks();
});

describe("AuditTable", () => {
  it("renders entries from mock", () => {
    render(
      <AuditTable
        entries={[
          baseEntry,
          {
            ...baseEntry,
            id: "00000000-0000-4000-a000-000000000002",
            action: "flag.archived",
            resourceType: "flag",
          },
        ]}
      />,
    );
    expect(screen.getByText(/flag created/i)).toBeInTheDocument();
    expect(screen.getByText(/flag archived/i)).toBeInTheDocument();
  });

  it("shows empty state when no entries", () => {
    render(<AuditTable entries={[]} />);
    expect(screen.getByText(/no audit entries/i)).toBeInTheDocument();
  });
});

describe("AuditFilters", () => {
  it("renders with pre-populated values", () => {
    render(<AuditFilters resourceType="flag" action="flag.created" />);
    expect(screen.getByDisplayValue("flag")).toBeInTheDocument();
  });

  it("does not show reset button when no filters active", () => {
    render(<AuditFilters />);
    expect(screen.queryByRole("button", { name: /reset/i })).not.toBeInTheDocument();
  });

  it("shows reset button when filters are active", () => {
    render(<AuditFilters action="flag.created" />);
    expect(screen.getByRole("button", { name: /reset/i })).toBeInTheDocument();
  });

  it("navigates on date change", async () => {
    const user = userEvent.setup();
    render(<AuditFilters />);
    const sinceInput = screen.getByLabelText(/from/i);
    await user.clear(sinceInput);
    await user.type(sinceInput, "2026-06-01");
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(expect.stringContaining("since=2026-06-01"));
    });
  });
});

describe("getAuditLog", () => {
  it("calls the audit endpoint with query params", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ entries: [], total: 0, limit: 20, offset: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await getAuditLog({ action: "flag.created", limit: 5, offset: 10 });
    expect(spy).toHaveBeenCalledWith(expect.stringContaining("/api/v1/audit?"), expect.any(Object));
    const url = spy.mock.calls[0][0] as string;
    expect(url).toContain("action=flag.created");
    expect(url).toContain("limit=5");
    expect(url).toContain("offset=10");
  });

  it("omits empty params", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ entries: [], total: 0, limit: 20, offset: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await getAuditLog({});
    expect(spy.mock.calls[0][0] as string).not.toContain("?");
  });

  it("returns pagination fields", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          entries: [baseEntry],
          total: 1,
          limit: 20,
          offset: 0,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    const result = await getAuditLog({});
    expect(result.total).toBe(1);
    expect(result.limit).toBe(20);
    expect(result.offset).toBe(0);
    expect(result.entries).toHaveLength(1);
  });
});
