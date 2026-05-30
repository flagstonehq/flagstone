import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectSettingsForm } from "@/components/settings/project-settings-form";
import { EnvironmentsList } from "@/components/settings/environments-list";
import { DangerZone } from "@/components/settings/danger-zone";
import type { Environment } from "@/lib/types";
const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
  usePathname: () => "/projects/my-app/settings",
}));
const envs: Environment[] = [
  { id: "e1", projectId: "p1", slug: "development", name: "Development" },
  { id: "e2", projectId: "p1", slug: "production", name: "Production" },
];
afterEach(() => {
  vi.clearAllMocks();
});
describe("ProjectSettingsForm", () => {
  it("shows current name and saves changes", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          project: { id: "p1", slug: "my-app", name: "Updated App" },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<ProjectSettingsForm projectSlug="my-app" name="My App" />);
    const input = screen.getByLabelText(/name/i);
    expect(input).toHaveValue("My App");
    await user.clear(input);
    await user.type(input, "Updated App");
    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(screen.getByText(/saved/i)).toBeInTheDocument());
  });
});
describe("EnvironmentsList", () => {
  it("renders environments and creates a new one", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          environment: {
            id: "e-new",
            project_id: "p1",
            slug: "staging",
            name: "Staging",
          },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<EnvironmentsList projectSlug="my-app" environments={envs} />);
    expect(screen.getByText("Development")).toBeInTheDocument();
    expect(screen.getByText("Production")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /add environment/i }));
    await user.type(await screen.findByLabelText(/name/i), "Staging");
    await user.click(screen.getByRole("button", { name: /add environment$/i }));
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });
  it("deletes environment with confirmation", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    render(<EnvironmentsList projectSlug="my-app" environments={envs} />);
    await user.click(screen.getByRole("button", { name: /delete development/i }));
    await screen.findByRole("dialog");
    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });
});
describe("DangerZone", () => {
  it("delete with wrong slug keeps button disabled", async () => {
    const user = userEvent.setup();
    render(<DangerZone projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /delete project/i }));
    await user.type(await screen.findByLabelText(/confirm/i), "wrong");
    expect(screen.getByRole("button", { name: /^delete project$/i })).toBeDisabled();
  });
  it("delete with correct slug redirects to /projects", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    render(<DangerZone projectSlug="my-app" />);
    await user.click(screen.getByRole("button", { name: /delete project/i }));
    await user.type(await screen.findByLabelText(/confirm/i), "my-app");
    await user.click(screen.getByRole("button", { name: /^delete project$/i }));
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/projects");
    });
    expect(mockRefresh).toHaveBeenCalled();
  });
});
