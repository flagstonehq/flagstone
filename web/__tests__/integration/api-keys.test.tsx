import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { z } from "zod";
import { server } from "../mocks/server";
import { CreateKeyDialog } from "@/components/api-keys/create-key-dialog";
import { ApiKeysTable } from "@/components/api-keys/api-keys-table";
import { RevokeKeyButton } from "@/components/api-keys/revoke-key-button";
import type { Environment, APIKey } from "@/lib/types";
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), refresh: mockRefresh }),
  usePathname: () => "/projects/my-app/api-keys",
}));
vi.mock("@/lib/schemas", async (importOriginal) => {
  const actual: Record<string, unknown> = (await importOriginal()) ?? {};
  return {
    ...actual,
    createApiKeySchema: z.object({
      name: z.string().min(1, "Name is required"),
      environment_id: z.string(),
      expires_at: z.string().optional(),
    }),
  };
});
const uuid = (s: string) => "00000000-0000-4000-a000-" + s.padStart(12, "0");
const envs: Environment[] = [
  { id: uuid("e1"), projectId: "p1", slug: "development", name: "Development" },
  { id: uuid("e3"), projectId: "p1", slug: "production", name: "Production" },
];
const keys: APIKey[] = [
  {
    id: "ak1",
    environmentId: uuid("e1"),
    name: "Dev Key",
    keyPrefix: "fs_dev_abc",
    lastUsedAt: new Date().toISOString(),
    expiresAt: null,
    createdAt: new Date().toISOString(),
  },
];
afterEach(() => {
  vi.clearAllMocks();
  server.resetHandlers();
});
describe("CreateKeyDialog", () => {
  it("creates a key and shows RawKeyModal", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          api_key: {
            id: "ak-new",
            environment_id: "e1",
            name: "My Key",
            key_prefix: "fs_live_new",
            last_used_at: null,
            expires_at: null,
            created_at: new Date().toISOString(),
            raw_key: "fs_live_new_secret123",
          },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );
    const user = userEvent.setup();
    render(<CreateKeyDialog projectSlug="my-app" environments={envs} />);
    await user.click(screen.getByRole("button", { name: /new api key/i }));
    await user.type(await screen.findByLabelText(/name/i), "My Key");
    await user.click(screen.getByRole("button", { name: /create api key/i }));
    expect(await screen.findByDisplayValue("fs_live_new_secret123")).toBeInTheDocument();
  });
  it("clears rawKey state when dialog is closed and reopened", async () => {
    const user = userEvent.setup();
    render(<CreateKeyDialog projectSlug="my-app" environments={envs} />);
    await user.click(screen.getByRole("button", { name: /new api key/i }));
    await screen.findByRole("dialog");
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /new api key/i }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByDisplayValue(/fs_live/)).not.toBeInTheDocument();
  });
});
describe("RevokeKeyButton", () => {
  it("calls refresh after revoking", async () => {
    vi.spyOn(global, "fetch").mockResolvedValueOnce(new Response(null, { status: 204 }));
    const user = userEvent.setup();
    render(<RevokeKeyButton projectSlug="my-app" envId="e1" keyId="ak1" keyName="Dev Key" />);
    await user.click(screen.getByRole("button", { name: /revoke dev key/i }));
    await user.click(await screen.findByRole("button", { name: /revoke$/i }));
    await waitFor(() => expect(mockRefresh).toHaveBeenCalled());
  });
});
describe("ApiKeysTable", () => {
  it("shows empty state when no keys", () => {
    render(<ApiKeysTable apiKeys={[]} environments={envs} projectSlug="my-app" />);
    expect(screen.getByText(/no api keys/i)).toBeInTheDocument();
  });
  it("renders keys and shows revoke buttons", () => {
    render(<ApiKeysTable apiKeys={keys} environments={envs} projectSlug="my-app" />);
    expect(screen.getByText("Dev Key")).toBeInTheDocument();
    expect(screen.getByText(/fs_dev_abc/)).toBeInTheDocument();
    expect(screen.getByText("Development")).toBeInTheDocument();
  });
});
