"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { changePassword as changePasswordApi, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Check, Loader2 } from "lucide-react";

const changePasswordSchema = z
  .object({
    currentPassword: z.string().min(1, "Current password is required."),
    newPassword: z.string().min(8, "New password must be at least 8 characters."),
    confirmPassword: z.string().min(1, "Please confirm your new password."),
  })
  .refine((data) => data.newPassword === data.confirmPassword, {
    message: "Passwords do not match.",
    path: ["confirmPassword"],
  });

type ChangePasswordFormData = z.infer<typeof changePasswordSchema>;

export function ChangePasswordForm() {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ChangePasswordFormData>({
    resolver: zodResolver(changePasswordSchema),
  });

  async function onSubmit(data: ChangePasswordFormData) {
    setServerError(null);
    setSuccess(false);
    try {
      await changePasswordApi({
        current_password: data.currentPassword,
        new_password: data.newPassword,
        confirm_password: data.confirmPassword,
      });
      setSuccess(true);
      setTimeout(() => {
        router.push("/login");
        router.refresh();
      }, 3000);
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(err.message);
      } else {
        setServerError("An unexpected error occurred.");
      }
    }
  }

  return (
    <section>
      <h2 className="mb-4 text-lg font-semibold">Change password</h2>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="currentPassword">Current password</Label>
          <Input id="currentPassword" type="password" {...register("currentPassword")} />
          {errors.currentPassword && (
            <p className="text-xs text-danger">{errors.currentPassword.message}</p>
          )}
        </div>
        <div className="space-y-2">
          <Label htmlFor="newPassword">New password</Label>
          <Input id="newPassword" type="password" {...register("newPassword")} />
          {errors.newPassword && (
            <p className="text-xs text-danger">{errors.newPassword.message}</p>
          )}
        </div>
        <div className="space-y-2">
          <Label htmlFor="confirmPassword">Confirm new password</Label>
          <Input id="confirmPassword" type="password" {...register("confirmPassword")} />
          {errors.confirmPassword && (
            <p className="text-xs text-danger">{errors.confirmPassword.message}</p>
          )}
        </div>
        {serverError && <p className="text-sm text-danger">{serverError}</p>}
        {success && (
          <p className="flex items-center gap-1.5 text-sm text-success">
            <Check className="h-4 w-4" />
            Password updated. Redirecting to sign in...
          </p>
        )}
        <Button type="submit" disabled={isSubmitting || success}>
          {isSubmitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
          {isSubmitting ? "Saving..." : "Update password"}
        </Button>
      </form>
    </section>
  );
}
