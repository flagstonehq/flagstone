import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RawKeyModal } from "@/components/api-keys/raw-key-modal";
describe("RawKeyModal", () => {
  it("displays the raw key in a readonly input", () => {
    render(<RawKeyModal rawKey="fs_live_secret123" onClose={vi.fn()} />);
    const input = screen.getByDisplayValue("fs_live_secret123");
    expect(input).toBeInTheDocument();
    expect(input).toHaveAttribute("readonly");
  });
  it("copy button does not crash when clicked", async () => {
    const user = userEvent.setup();
    render(<RawKeyModal rawKey="fs_live_secret123" onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /copy/i }));
    expect(screen.getByRole("button", { name: /done/i })).toBeInTheDocument();
  });
  it("calls onClose when dialog is dismissed via Done button", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<RawKeyModal rawKey="fs_live_secret123" onClose={onClose} />);
    await user.click(screen.getByRole("button", { name: /done/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
