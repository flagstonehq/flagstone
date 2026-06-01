import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuditRow } from "@/components/audit/audit-row";
import type { AuditEntry } from "@/lib/types";

function entry(overrides: Partial<AuditEntry> = {}): AuditEntry {
  return {
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
    ...overrides,
  };
}

describe("AuditRow", () => {
  it("renders action verb and resource type", () => {
    render(<table><tbody><AuditRow entry={entry()} /></tbody></table>);
    expect(screen.getByText(/flag created/i)).toBeInTheDocument();
  });

  it("shows the actor ID prefix for a user", () => {
    render(
      <table>
        <tbody>
          <AuditRow
            entry={entry({
              actorId: "00000000-0000-4000-a000-0000000000aa",
              resourceId: "11111111-0000-4000-a000-111111111111",
            })}
          />
        </tbody>
      </table>,
    );
    expect(screen.getByText("00000000")).toBeInTheDocument();
  });

  it("shows 'System' for system actors", () => {
    render(<table><tbody><AuditRow entry={entry({ actorType: "system", actorId: null })} /></tbody></table>);
    expect(screen.getByText("System")).toBeInTheDocument();
  });

  it("shows 'API key' for api_key actors", () => {
    render(<table><tbody><AuditRow entry={entry({ actorType: "api_key" })} /></tbody></table>);
    expect(screen.getByText("API key")).toBeInTheDocument();
  });

  it("renders a relative timestamp", () => {
    render(<table><tbody><AuditRow entry={entry()} /></tbody></table>);
    expect(screen.getByText(/just now|ago/i)).toBeInTheDocument();
  });

  it("em-dash for missing resourceId", () => {
    render(<table><tbody><AuditRow entry={entry({ resourceId: null })} /></tbody></table>);
    expect(screen.getByText("\u2014")).toBeInTheDocument();
  });

  it("renders a known action icon", () => {
    render(
      <table>
        <tbody>
          <AuditRow entry={entry({ action: "project.created", resourceType: "project" })} />
        </tbody>
      </table>,
    );
    expect(screen.getByText(/project created/i)).toBeInTheDocument();
  });
});
