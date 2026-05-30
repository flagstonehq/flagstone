import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { API_BASE } from "../mocks/handlers";
import { CreateFlagDialog } from "@/components/flags/create-flag-dialog";
import { ArchiveFlagButton } from "@/components/flags/archive-flag-button";
import { EnvToggle } from "@/components/flags/env-toggle";
import type { Environment } from "@/lib/types";
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), refresh: mockRefresh }),
  usePathname: () => "/projects/my-app/flags",
}));
const envs: Environment[] = [{ id: "e1", projectId: "p1", slug: "production", name: "Production" }];
afterEach(() => {
  vi.clearAllMocks();
  server.resetHandlers();
});
describe("EnvToggle", () => {
  it("calls API and refreshes on toggle", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    render(
      <EnvToggle
        projectSlug="my-app"
        flagKey="new-checkout"
        envSlug="production"
        defaultEnabled={false}
      />,
    );
    const sw = screen.getByRole("switch");
    await user.click(sw);
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });
  it("reverts and shows error if API fails", async () => {
    server.use(
      http.patch(`${API_BASE}/api/v1/projects/:slug/flags/:key/environments/:env`, () =>
        HttpResponse.json({ error: { code: "SERVER_ERROR" } }, { status: 500 }),
      ),
    );
    const user = userEvent.setup();
    render(
      <EnvToggle
        projectSlug="my-app"
        flagKey="new-checkout"
        envSlug="production"
        defaultEnabled={false}
      />,
    );
    await user.click(screen.getByRole("switch"));
    expect(await screen.findByText(/failed/i)).toBeInTheDocument();
    expect(screen.getByRole("switch")).not.toBeChecked();
  });
});
describe("ArchiveFlagButton", () => {
  it("opens confirm dialog", async () => {
    const user = userEvent.setup();
    render(
      <ArchiveFlagButton
        projectSlug="my-app"
        flagKey="new-checkout"
        flagName="New Checkout Flow"
      />,
    );
    await user.click(screen.getByRole("button", { name: /archive new-checkout/i }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("New Checkout Flow")).toBeInTheDocument();
  });
  it("cancel button has focus by default", async () => {
    const user = userEvent.setup();
    render(
      <ArchiveFlagButton
        projectSlug="my-app"
        flagKey="new-checkout"
        flagName="New Checkout Flow"
      />,
    );
    await user.click(screen.getByRole("button", { name: /archive new-checkout/i }));
    await screen.findByRole("dialog");
    expect(screen.getByRole("button", { name: /cancel/i })).toHaveFocus();
  });
  it("calls refresh after archive", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    render(
      <ArchiveFlagButton
        projectSlug="my-app"
        flagKey="new-checkout"
        flagName="New Checkout Flow"
      />,
    );
    await user.click(screen.getByRole("button", { name: /archive new-checkout/i }));
    await user.click(await screen.findByRole("button", { name: /^archive$/i }));
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });
});
describe("CreateFlagDialog", () => {
  it("shows key conflict error on 409", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({ error: { code: "KEY_CONFLICT", message: "Flag key already exists" } }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<CreateFlagDialog projectSlug="my-app" environments={envs} />);
    await user.click(screen.getByRole("button", { name: /new flag/i }));
    await user.type(await screen.findByLabelText(/key/i), "duplicate-key");
    await user.type(screen.getByLabelText(/name/i), "Some Flag");
    await user.click(screen.getByRole("button", { name: /create flag/i }));
    expect(await screen.findByText(/already exists/i)).toBeInTheDocument();
  });
  it("resets form on cancel", async () => {
    const user = userEvent.setup();
    render(<CreateFlagDialog projectSlug="my-app" environments={envs} />);
    await user.click(screen.getByRole("button", { name: /new flag/i }));
    await user.type(await screen.findByLabelText(/key/i), "some-key");
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
