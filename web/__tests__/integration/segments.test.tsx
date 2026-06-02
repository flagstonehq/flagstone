import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SegmentsTable } from "@/components/segments/segments-table";
import { CreateSegmentDialog } from "@/components/segments/create-segment-dialog";
import { SegmentRuleEditor } from "@/components/segments/segment-rule-editor";
import type { Segment, RuleConditionNode, LeafCondition } from "@/lib/types";

const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
  usePathname: () => "/projects/my-app/segments",
}));

const segments: Segment[] = [
  {
    id: "s1",
    projectId: "p1",
    key: "beta-users",
    name: "Beta Users",
    description: "Users enrolled in the beta program",
    rules: { attribute: "plan", op: "eq", value: "beta" } as RuleConditionNode,
    archivedAt: null,
    createdAt: new Date(Date.now() - 604800000).toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: "s2",
    projectId: "p1",
    key: "premium-customers",
    name: "Premium Customers",
    description: null,
    rules: { attribute: "plan", op: "eq", value: "premium" } as RuleConditionNode,
    archivedAt: null,
    createdAt: new Date(Date.now() - 259200000).toISOString(),
    updatedAt: new Date().toISOString(),
  },
];

afterEach(() => {
  vi.clearAllMocks();
});

describe("SegmentsTable", () => {
  it("renders segments with key, name, and description", () => {
    render(<SegmentsTable segments={segments} projectSlug="my-app" />);
    expect(screen.getByText("beta-users")).toBeInTheDocument();
    expect(screen.getByText("Beta Users")).toBeInTheDocument();
    expect(screen.getByText("Users enrolled in the beta program")).toBeInTheDocument();
    expect(screen.getByText("premium-customers")).toBeInTheDocument();
    expect(screen.getByText("Premium Customers")).toBeInTheDocument();
  });

  it("shows empty state when no segments", () => {
    render(<SegmentsTable segments={[]} projectSlug="my-app" />);
    expect(screen.getByText("No segments yet")).toBeInTheDocument();
  });

  it("renders archive buttons for each segment", () => {
    render(<SegmentsTable segments={segments} projectSlug="my-app" />);
    const buttons = screen.getAllByRole("button", { name: /archive/i });
    expect(buttons).toHaveLength(2);
  });
});

describe("CreateSegmentDialog", () => {
  it("opens dialog on trigger click", async () => {
    const user = userEvent.setup();
    render(<CreateSegmentDialog projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /new segment/i }));
    expect(screen.getByRole("heading", { name: /create segment/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
  });

  it("calls API on valid submit", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          segment: { id: "s-new", key: "test-segment", name: "Test Segment" },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<CreateSegmentDialog projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /new segment/i }));
    await user.type(screen.getByLabelText(/key/i), "test-segment");
    await user.type(screen.getByLabelText(/name/i), "Test Segment");
    await user.click(screen.getByRole("button", { name: /create segment/i }));
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });

  it("shows error on duplicate key", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: { code: "KEY_CONFLICT", message: "Segment key already exists" },
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<CreateSegmentDialog projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /new segment/i }));
    await user.type(screen.getByLabelText(/key/i), "duplicate-segment");
    await user.type(screen.getByLabelText(/name/i), "Duplicate");
    await user.click(screen.getByRole("button", { name: /create segment/i }));
    await waitFor(() => {
      expect(screen.getByText(/already exists/i)).toBeInTheDocument();
    });
  });
});

describe("SegmentRuleEditor", () => {
  it("renders conditions from initial rules", () => {
    const rules: RuleConditionNode = {
      attribute: "plan",
      op: "eq",
      value: "beta",
    } as LeafCondition;
    render(
      <SegmentRuleEditor
        projectSlug="my-app"
        segmentKey="beta-users"
        initialRules={rules}
        segmentName="Beta Users"
      />,
    );
    expect(screen.getByDisplayValue("plan")).toBeInTheDocument();
    expect(screen.getByDisplayValue("beta")).toBeInTheDocument();
  });

  it("adds a new condition row", async () => {
    const user = userEvent.setup();
    render(
      <SegmentRuleEditor
        projectSlug="my-app"
        segmentKey="beta-users"
        initialRules={{ attribute: "country", op: "eq", value: "AR" } as LeafCondition}
        segmentName="Beta Users"
      />,
    );
    await user.click(screen.getByRole("button", { name: /add condition/i }));
    const attributeInputs = screen.getAllByLabelText(/attribute/i);
    expect(attributeInputs).toHaveLength(2);
  });

  it("saves changes via API", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          segment: { id: "s1", key: "beta-users", rules: { attribute: "plan", op: "eq", value: "beta" } },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    const rules: RuleConditionNode = {
      attribute: "plan",
      op: "eq",
      value: "beta",
    } as LeafCondition;
    render(
      <SegmentRuleEditor
        projectSlug="my-app"
        segmentKey="beta-users"
        initialRules={rules}
        segmentName="Beta Users"
      />,
    );
    await user.type(screen.getByLabelText(/attribute/i), "new-attr");
    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(screen.getByText("Saved")).toBeInTheDocument());
  });
});
