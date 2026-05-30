import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DangerZone } from "@/components/settings/danger-zone";
const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));
describe("DangerZone", () => {
  it("confirm button is disabled until correct slug is typed", async () => {
    const user = userEvent.setup();
    render(<DangerZone projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /delete project/i }));
    const confirmBtn = screen.getByRole("button", { name: /^delete project$/i });
    expect(confirmBtn).toBeDisabled();
    await user.type(screen.getByLabelText(/confirm/i), "wrong-slug");
    expect(confirmBtn).toBeDisabled();
    await user.clear(screen.getByLabelText(/confirm/i));
    await user.type(screen.getByLabelText(/confirm/i), "my-app");
    expect(confirmBtn).toBeEnabled();
  });
  it("calls router.push on successful delete", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    render(<DangerZone projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /delete project/i }));
    await user.type(screen.getByLabelText(/confirm/i), "my-app");
    await user.click(screen.getByRole("button", { name: /^delete project$/i }));
    await vi.waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/projects");
    });
    expect(mockRefresh).toHaveBeenCalled();
  });
});
