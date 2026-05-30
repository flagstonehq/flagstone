import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateProjectDialog } from "@/components/projects/create-project-dialog";
import { createProject } from "@/lib/api";

const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), refresh: mockRefresh }),
  usePathname: () => "/projects",
}));

afterEach(() => {
  vi.clearAllMocks();
});

describe("CreateProjectDialog — integration", () => {
  it("opens when button is clicked", async () => {
    const user = userEvent.setup();
    render(<CreateProjectDialog />);
    await user.click(screen.getByRole("button", { name: /new project/i }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("auto-generates slug from name", async () => {
    const user = userEvent.setup();
    render(<CreateProjectDialog />);
    await user.click(screen.getByRole("button", { name: /new project/i }));
    await user.type(await screen.findByLabelText(/name/i), "My Cool App");
    expect(screen.getByLabelText(/slug/i)).toHaveValue("my-cool-app");
  });

  it("closes and resets on cancel", async () => {
    const user = userEvent.setup();
    render(<CreateProjectDialog />);
    await user.click(screen.getByRole("button", { name: /new project/i }));
    await user.type(await screen.findByLabelText(/name/i), "Some Project");
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("submits successfully and calls router.refresh()", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          project: { id: "p-new", slug: "new-project", name: "New Project" },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<CreateProjectDialog />);
    await user.click(screen.getByRole("button", { name: /new project/i }));
    await user.type(await screen.findByLabelText(/name/i), "New Project");
    await user.clear(screen.getByLabelText(/slug/i));
    await user.type(screen.getByLabelText(/slug/i), "new-project");
    await user.click(screen.getByRole("button", { name: /create project/i }));
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });

  it("shows server error on 409 slug conflict", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: { code: "SLUG_CONFLICT", message: "Slug already exists" },
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<CreateProjectDialog />);
    await user.click(screen.getByRole("button", { name: /new project/i }));
    await user.type(await screen.findByLabelText(/name/i), "Dupe");
    await user.clear(screen.getByLabelText(/slug/i));
    await user.type(screen.getByLabelText(/slug/i), "duplicate-slug");
    await user.click(screen.getByRole("button", { name: /create project/i }));
    expect(await screen.findByText(/slug already exists/i)).toBeInTheDocument();
  });
});
