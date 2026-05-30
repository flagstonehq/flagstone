"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Check } from "lucide-react";
import { updateProjectSchema } from "@/lib/schemas";
import { updateProject, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type FormValues = z.infer<typeof updateProjectSchema>;

interface ProjectSettingsFormProps {
  projectSlug: string;
  name: string;
}

export function ProjectSettingsForm({ projectSlug, name }: ProjectSettingsFormProps) {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [showSuccess, setShowSuccess] = useState(false);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<FormValues>({
    resolver: zodResolver(updateProjectSchema),
    defaultValues: { name },
  });

  async function onSubmit(values: FormValues) {
    setServerError(null);
    setShowSuccess(false);
    try {
      await updateProject(projectSlug, values);
      setShowSuccess(true);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(err.message);
      } else {
        setServerError("Something went wrong. Please try again.");
      }
    }
  }

  return (
    <section>
      <h2 className="mb-4 text-lg font-medium">Project name</h2>
      <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
        {serverError && (
          <p className="bg-danger-bg text-danger rounded-lg px-3 py-2 text-sm">{serverError}</p>
        )}
        <div className="space-y-1.5">
          <Label htmlFor="settings-name">Name</Label>
          <Input
            id="settings-name"
            disabled={isSubmitting}
            aria-invalid={!!errors.name}
            {...register("name")}
          />
          {errors.name && <p className="text-danger text-xs">{errors.name.message}</p>}
        </div>
        <div className="flex items-center gap-3">
          <Button
            type="submit"
            disabled={isSubmitting || !isDirty}
            className="bg-primary hover:bg-primary-dark text-white"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Saving…
              </>
            ) : (
              "Save"
            )}
          </Button>
          {showSuccess && (
            <span className="text-success flex items-center gap-1 text-sm">
              <Check className="h-4 w-4" />
              Saved
            </span>
          )}
        </div>
      </form>
    </section>
  );
}
