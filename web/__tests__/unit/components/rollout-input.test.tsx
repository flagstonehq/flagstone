import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RolloutInput } from "@/components/rules/rollout-input";

describe("RolloutInput", () => {
  it("renders the current value", () => {
    render(<RolloutInput value={50} onChange={() => {}} />);
    expect(screen.getByLabelText("Rollout percentage")).toHaveValue(50);
  });

  it("renders default 100 when no value", () => {
    render(<RolloutInput value={undefined} onChange={() => {}} />);
    expect(screen.getByLabelText("Rollout percentage")).toHaveValue(100);
  });

  it("calls onChange when user types a number", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<RolloutInput value={100} onChange={onChange} />);
    const input = screen.getByLabelText("Rollout percentage");
    await user.type(input, "5");
    expect(onChange).toHaveBeenCalled();
  });

  it("calls onChange with undefined when cleared", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<RolloutInput value={50} onChange={onChange} />);
    const input = screen.getByLabelText("Rollout percentage");
    await user.clear(input);
    expect(onChange).toHaveBeenCalledWith(undefined);
  });

  it("shows error message", () => {
    render(<RolloutInput value={150} onChange={() => {}} error="Must be 0-100" />);
    expect(screen.getByText("Must be 0-100")).toBeInTheDocument();
  });
});
