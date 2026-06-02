import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { FlagRow } from "@/components/flags/flag-row";
import type { Flag, Environment } from "@/lib/types";
vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn() }),
}));
const flag: Flag = {
  id: "f1",
  projectId: "p1",
  key: "new-checkout",
  name: "New Checkout Flow",
  type: "boolean",
  description: null,
  defaultValue: false,
  archivedAt: null,
  createdAt: "2026-06-15T12:00:00Z",
  updatedAt: "2026-06-15T12:00:00Z",
};
const envs: Environment[] = [{ id: "e1", projectId: "p1", slug: "production", name: "Production" }];
describe("FlagRow", () => {
  it("renders flag key, name and type badge", () => {
    render(
      <table>
        <tbody>
          <FlagRow flag={flag} environments={envs} projectSlug="my-app" enabledMap={{}} />
        </tbody>
      </table>,
    );
    expect(screen.getByText("new-checkout")).toBeInTheDocument();
    expect(screen.getByText("New Checkout Flow")).toBeInTheDocument();
    expect(screen.getByText("boolean")).toBeInTheDocument();
  });
  it("renders one toggle per environment", () => {
    render(
      <table>
        <tbody>
          <FlagRow flag={flag} environments={envs} projectSlug="my-app" enabledMap={{}} />
        </tbody>
      </table>,
    );
    expect(
      screen.getByRole("switch", { name: /toggle new-checkout in production/i }),
    ).toBeInTheDocument();
  });
});
