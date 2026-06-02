"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { setupSchema } from "@/lib/schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";

type SetupFormValues = z.infer<typeof setupSchema>;

export function SetupForm() {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<SetupFormValues>({
    resolver: zodResolver(setupSchema),
  });

  async function onSubmit(values: SetupFormValues) {
    setServerError(null);
    try {
      const res = await fetch("/api/auth/setup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tenant_name: values.tenant_name,
          admin_email: values.admin_email,
          admin_password: values.admin_password,
        }),
      });

      const data = await res.json();

      if (res.ok) {
        router.push("/projects");
        router.refresh();
        return;
      }

      if (data?.error?.code === "ALREADY_INITIALIZED") {
        setServerError("This instance is already set up. Please log in instead.");
        return;
      }

      setServerError(data?.error?.message ?? "Setup failed. Please try again.");
    } catch {
      setServerError("Could not connect to the server. Please try again.");
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
      {serverError && (
        <Alert variant="destructive">
          <AlertDescription>{serverError}</AlertDescription>
        </Alert>
      )}

      {serverError?.includes("log in instead") && (
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={() => router.push("/login")}
        >
          Go to Login
        </Button>
      )}

      <div className="space-y-1.5">
        <Label htmlFor="tenant_name">Organization name</Label>
        <Input
          id="tenant_name"
          placeholder="Acme Corp"
          aria-invalid={!!errors.tenant_name}
          disabled={isSubmitting}
          {...register("tenant_name")}
        />
        {errors.tenant_name && (
          <p className="text-xs text-danger">{errors.tenant_name.message}</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="admin_email">Admin email</Label>
        <Input
          id="admin_email"
          type="email"
          autoComplete="email"
          placeholder="admin@acme.com"
          aria-invalid={!!errors.admin_email}
          disabled={isSubmitting}
          {...register("admin_email")}
        />
        {errors.admin_email && (
          <p className="text-xs text-danger">{errors.admin_email.message}</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="admin_password">Password</Label>
        <Input
          id="admin_password"
          type="password"
          autoComplete="new-password"
          placeholder="At least 8 characters"
          aria-invalid={!!errors.admin_password}
          disabled={isSubmitting}
          {...register("admin_password")}
        />
        {errors.admin_password && (
          <p className="text-xs text-danger">{errors.admin_password.message}</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="confirm_password">Confirm password</Label>
        <Input
          id="confirm_password"
          type="password"
          autoComplete="new-password"
          placeholder="Re-enter password"
          aria-invalid={!!errors.confirm_password}
          disabled={isSubmitting}
          {...register("confirm_password")}
        />
        {errors.confirm_password && (
          <p className="text-xs text-danger">{errors.confirm_password.message}</p>
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
            Setting up…
          </>
        ) : (
          "Create instance"
        )}
      </Button>

      <p className="text-center text-xs text-text-secondary">
        Already have an account?{" "}
        <a href="/login" className="text-primary hover:underline">
          Sign in
        </a>
      </p>
    </form>
  );
}
