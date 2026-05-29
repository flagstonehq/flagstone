import { describe, it, expect, vi, afterEach, beforeAll, afterAll } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LoginForm } from "@/components/login/login-form";

const mockPush = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

afterEach(() => {
  vi.clearAllMocks();
});

function fillAndSubmit(email = "admin@acme.com", password = "password123") {
  const user = userEvent.setup();
  render(<LoginForm />);
  return {
    user,
    submit: async () => {
      await user.type(screen.getByLabelText(/email/i), email);
      await user.type(screen.getByLabelText(/password/i), password);
      await user.click(screen.getByRole("button", { name: /sign in/i }));
    },
  };
}

describe("LoginForm — integration", () => {
  it("redirects to /projects on successful login", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true, tenant: { slug: "acme" } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const { submit } = fillAndSubmit();
    await submit();
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/projects"));
  });

  it("shows invalid credentials error on 401", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({ error: { code: "INVALID_CREDENTIALS", message: "Invalid email or password" } }),
        { status: 401, headers: { "Content-Type": "application/json" } },
      ),
    );
    const { submit } = fillAndSubmit();
    await submit();
    expect(await screen.findByText(/invalid email or password/i)).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows account locked message on 423", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({ error: { code: "ACCOUNT_LOCKED", retry_after: 900 } }),
        { status: 423, headers: { "Content-Type": "application/json" } },
      ),
    );
    const { submit } = fillAndSubmit();
    await submit();
    expect(await screen.findByText(/account locked/i)).toBeInTheDocument();
    expect(await screen.findByText(/15 minutes/i)).toBeInTheDocument();
  });

  it("shows tenant picker on 409 MULTIPLE_TENANTS", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: "MULTIPLE_TENANTS",
            available_tenants: [
              { slug: "acme", name: "Acme Corp", role: "owner" },
              { slug: "beta-labs", name: "Beta Labs", role: "viewer" },
            ],
          },
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    const { submit } = fillAndSubmit();
    await submit();
    expect(await screen.findByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("Beta Labs")).toBeInTheDocument();
  });

  it("shows network error when fetch throws", async () => {
    vi.spyOn(global, "fetch").mockRejectedValueOnce(new Error("Network error"));
    const { submit } = fillAndSubmit();
    await submit();
    expect(await screen.findByText(/could not connect/i)).toBeInTheDocument();
  });
});
