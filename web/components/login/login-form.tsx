"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { loginSchema } from "@/lib/schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";

type LoginFormValues = z.infer<typeof loginSchema>;

type LoginError =
  | { type: "invalid_credentials" }
  | { type: "account_locked"; retryAfter: number }
  | { type: "multiple_tenants"; tenants: { slug: string; name: string; role: string }[] }
  | { type: "network" };

export function LoginForm() {
  const router = useRouter();
  const [loginError, setLoginError] = useState<LoginError | null>(null);
  const [selectedTenantSlug, setSelectedTenantSlug] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
  });

  async function onSubmit(values: LoginFormValues) {
    setLoginError(null);

    try {
      const payload = selectedTenantSlug
        ? { ...values, tenant_slug: selectedTenantSlug }
        : values;

      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      const data = await res.json();

      if (res.ok) {
        router.push("/projects");
        router.refresh();
        return;
      }

      if (res.status === 409 && data.error?.code === "MULTIPLE_TENANTS") {
        setLoginError({
          type: "multiple_tenants",
          tenants: data.error.available_tenants,
        });
        return;
      }

      if (res.status === 423) {
        setLoginError({
          type: "account_locked",
          retryAfter: data.error?.retry_after ?? 900,
        });
        return;
      }

      setLoginError({ type: "invalid_credentials" });
    } catch {
      setLoginError({ type: "network" });
    }
  }

  if (loginError?.type === "multiple_tenants") {
    return (
      <div className="space-y-4">
        <p className="text-sm text-text-secondary text-center">
          Your account belongs to multiple organizations. Choose one to continue.
        </p>
        <div className="space-y-2">
          {loginError.tenants.map((t) => (
            <button
              key={t.slug}
              type="button"
              onClick={() => {
                setSelectedTenantSlug(t.slug);
                setLoginError(null);
                handleSubmit(onSubmit)();
              }}
              className="w-full flex items-center justify-between px-4 py-3 rounded-lg border border-border bg-surface hover:border-primary hover:bg-primary-bg transition-colors text-left"
            >
              <span className="font-medium text-text">{t.name}</span>
              <span className="text-xs text-text-secondary capitalize">{t.role}</span>
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={() => {
            setSelectedTenantSlug(null);
            setLoginError(null);
          }}
          className="w-full text-sm text-text-secondary hover:text-text transition-colors"
        >
          ← Back
        </button>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
      {loginError?.type === "invalid_credentials" && (
        <Alert variant="destructive">
          <AlertDescription>⚠ Invalid email or password.</AlertDescription>
        </Alert>
      )}
      {loginError?.type === "account_locked" && (
        <Alert variant="destructive">
          <AlertDescription>
            Account locked due to repeated failed logins. Try again in{" "}
            {Math.ceil(loginError.retryAfter / 60)} minutes.
          </AlertDescription>
        </Alert>
      )}
      {loginError?.type === "network" && (
        <Alert variant="destructive">
          <AlertDescription>
            Could not connect to the server. Please try again.
          </AlertDescription>
        </Alert>
      )}

      <div className="space-y-1.5">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          autoComplete="email"
          placeholder="admin@acme.com"
          aria-invalid={!!errors.email}
          disabled={isSubmitting}
          {...register("email")}
        />
        {errors.email && (
          <p className="text-xs text-danger">{errors.email.message}</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          placeholder="••••••••"
          aria-invalid={!!errors.password}
          disabled={isSubmitting}
          {...register("password")}
        />
        {errors.password && (
          <p className="text-xs text-danger">{errors.password.message}</p>
        )}
      </div>

      <Button
        type="submit"
        className="w-full bg-primary hover:bg-primary-dark text-white"
        disabled={isSubmitting}
      >
        {isSubmitting ? (
          <>
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Signing in…
          </>
        ) : (
          "Sign in"
        )}
      </Button>

      <div className="space-y-3 pt-1">
        <div className="text-center">
          <button
            type="button"
            className="text-sm text-primary hover:underline"
            onClick={() => alert("Password reset not available in MVP.")}
          >
            Forgot password?
          </button>
        </div>
        <div className="border-t border-border pt-3 text-center">
          <p className="text-xs text-text-secondary">
            Don&apos;t have an account?{" "}
            <span className="text-primary">Request access</span>
          </p>
        </div>
      </div>
    </form>
  );
}
