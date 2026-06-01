import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConditionRow } from "@/components/rules/condition-row";
import type { LeafCondition } from "@/lib/types";

function leaf(overrides: Partial<LeafCondition> = {}): LeafCondition {
  return {
    attribute: "country",
    operator: "eq",
    value: "AR",
    ...overrides,
  };
}

describe("ConditionRow", () => {
  it("renders attribute, operator and value", () => {
    render(<ConditionRow condition={leaf()} onChange={() => {}} onDelete={() => {}} />);
    expect(screen.getByLabelText("Attribute")).toHaveValue("country");
    expect(screen.getByLabelText("Value")).toHaveValue("AR");
  });

  it("calls onChange when attribute changes", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<ConditionRow condition={leaf()} onChange={onChange} onDelete={() => {}} />);
    await user.type(screen.getByLabelText("Attribute"), "x");
    expect(onChange).toHaveBeenCalled();
  });

  it("calls onChange when value changes", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<ConditionRow condition={leaf()} onChange={onChange} onDelete={() => {}} />);
    await user.type(screen.getByLabelText("Value"), "x");
    expect(onChange).toHaveBeenCalled();
  });

  it("calls onDelete when delete button is clicked", async () => {
    const onDelete = vi.fn();
    const user = userEvent.setup();
    render(<ConditionRow condition={leaf()} onChange={() => {}} onDelete={onDelete} />);
    await user.click(screen.getByRole("button", { name: /remove condition/i }));
    expect(onDelete).toHaveBeenCalledOnce();
  });
});
