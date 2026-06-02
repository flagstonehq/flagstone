import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuleEditor } from "@/components/rules/rule-editor";
import type { Environment, Rule } from "@/lib/types";

const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: vi.fn() }),
}));

const environments: Environment[] = [
  { id: "env-1", projectId: "proj-1", slug: "production", name: "Production" },
  { id: "env-2", projectId: "proj-1", slug: "staging", name: "Staging" },
];

const initialRule: Rule = {
  conditions: { attribute: "country", op: "eq", value: "AR" },
  value: null,
};

afterEach(() => {
  vi.clearAllMocks();
});

function editorProps(overrides: Partial<Parameters<typeof RuleEditor>[0]> = {}) {
  return {
    projectSlug: "my-app",
    flagKey: "test-flag",
    flagType: "boolean" as const,
    initialRules: [initialRule],
    initialEnabled: true,
    initialVersion: 3,
    environments,
    currentEnvSlug: "production",
    ...overrides,
  };
}

describe("RuleEditor", () => {
  it("renders with multiple initial rules", () => {
    render(
      <RuleEditor
        {...editorProps({
          initialRules: [initialRule, { ...initialRule, conditions: { attribute: "browser", op: "eq", value: "Chrome" }, value: true }],
        })}
      />,
    );
    expect(screen.getAllByText(/rule \d/i)).toHaveLength(2);
  });
  it("renders controls bar with enabled switch and env selector", () => {
    render(<RuleEditor {...editorProps()} />);
    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByText(/production/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });

  it("shows initial conditions", () => {
    render(<RuleEditor {...editorProps()} />);
    expect(screen.getByLabelText("Attribute")).toHaveValue("country");
    expect(screen.getByLabelText("Value")).toHaveValue("AR");
    expect(screen.getByText("Rule 1")).toBeInTheDocument();
  });

  it("disables save when no changes", () => {
    render(<RuleEditor {...editorProps()} />);
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });

  it("enables save after making a change", async () => {
    const user = userEvent.setup();
    render(<RuleEditor {...editorProps()} />);
    await user.type(screen.getByLabelText("Attribute"), "x");
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /save/i })).not.toBeDisabled();
    });
  });

  it("calls saveFlagEnvironment on save and shows success", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          flagId: "flag-1",
          environmentId: "env-1",
          enabled: true,
          rules: [initialRule],
          defaultValue: null,
          version: 4,
          createdAt: "2026-01-01T00:00:00Z",
          updatedAt: "2026-01-02T00:00:00Z",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<RuleEditor {...editorProps()} />);
    await user.type(screen.getByLabelText("Attribute"), "x");
    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => {
      expect(screen.getByText("Saved")).toBeInTheDocument();
    });
    expect(spy).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/projects/my-app/flags/test-flag/environments/production"),
      expect.objectContaining({ method: "PUT" }),
    );
  });

  it("shows error on version conflict", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({ error: { code: "VERSION_CONFLICT", message: "Version conflict" } }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<RuleEditor {...editorProps()} />);
    await user.type(screen.getByLabelText("Attribute"), "x");
    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => {
      expect(screen.getByText(/modified by someone else/i)).toBeInTheDocument();
    });
  });

  it("can add a new rule", async () => {
    const user = userEvent.setup();
    render(<RuleEditor {...editorProps()} />);
    await user.click(screen.getByRole("button", { name: /add rule/i }));
    expect(screen.getAllByText(/rule \d/i)).toHaveLength(2);
  });

  it("can delete a rule, keeping at least one", async () => {
    const user = userEvent.setup();
    render(
      <RuleEditor
        {...editorProps({
          initialRules: [
            initialRule,
            { conditions: { attribute: "browser", op: "eq", value: "Chrome" }, value: true },
          ],
        })}
      />,
    );
    await user.click(screen.getByRole("button", { name: /remove rule 1/i }));
    expect(screen.getByText("Rule 1")).toBeInTheDocument();
    expect(screen.queryByText("Rule 2")).not.toBeInTheDocument();
  });
});
