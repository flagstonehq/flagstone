import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { User, Session } from "@/lib/types";
import { ChangePasswordForm } from "@/components/account/change-password-form";
import { SessionsList } from "@/components/account/sessions-list";

export default async function AccountPage() {
  const token = (await cookies()).get("access_token")?.value;
  if (!token) redirect("/login");

  let user: User | null = null;
  let sessions: Session[] = [];

  try {
    user = await serverFetch<User>("/api/v1/auth/me", token);
  } catch {
    redirect("/login");
  }

  try {
    sessions = await serverFetch<Session[]>("/api/v1/auth/sessions", token);
  } catch {
    // sessions list is optional
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-8 p-6">
      <h1 className="text-2xl font-bold">Account</h1>

      <section>
        <h2 className="mb-4 text-lg font-semibold">Profile</h2>
        <div className="space-y-2 rounded-lg border border-border bg-surface p-4">
          <div className="flex justify-between text-sm">
            <span className="text-text-secondary">Email</span>
            <span>{user.email}</span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-text-secondary">Role</span>
            <span className="capitalize">{user.role}</span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-text-secondary">Member since</span>
            <span>{new Date(user.createdAt).toLocaleDateString()}</span>
          </div>
          {user.lastLoginAt && (
            <div className="flex justify-between text-sm">
              <span className="text-text-secondary">Last sign in</span>
              <span>{new Date(user.lastLoginAt).toLocaleString()}</span>
            </div>
          )}
        </div>
      </section>

      <ChangePasswordForm />

      <SessionsList initial={sessions} />
    </div>
  );
}
