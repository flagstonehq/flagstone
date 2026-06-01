import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { LoginForm } from "@/components/login/login-form";

export default async function LoginPage() {
  const cookieStore = await cookies();
  if (cookieStore.get("access_token")) {
    redirect("/projects");
  }

  return (
    <main className="min-h-screen flex items-center justify-center bg-bg">
      <div className="w-full max-w-sm">
        <div className="bg-surface rounded-xl border border-border p-8 shadow-sm">
          <div className="mb-6 text-center">
            <div className="inline-flex items-center justify-center w-full mb-2">
              <span className="text-2xl font-bold text-primary">⚑ Flagstone</span>
            </div>
            <p className="text-sm text-text-secondary">Feature flags for your team</p>
          </div>
          <LoginForm />
        </div>
      </div>
    </main>
  );
}
