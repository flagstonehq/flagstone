"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Loader2 } from "lucide-react";
import { createProjectSchema } from "@/lib/schemas";
import { createProject, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

type FormValues = z.infer<typeof createProjectSchema>;

function toSlug(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

interface CreateProjectDialogProps {
  label?: string;
}

export function CreateProjectDialog({
  label = "New project",
}: CreateProjectDialogProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setValue,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: { name: "", slug: "" },
  });

  function handleNameChange(e: React.ChangeEvent<HTMLInputElement>) {
    const name = e.target.value;
    setValue("name", name);
    setValue("slug", toSlug(name), { shouldValidate: false });
  }

  async function onSubmit(values: FormValues) {
    setServerError(null);
    try {
      await createProject(values);
      reset();
      setOpen(false);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setServerError("A project with that slug already exists.");
      } else {
        setServerError("Something went wrong. Please try again.");
      }
    }
  }

  function handleOpenChange(next: boolean) {
    if (!next) {
      reset();
      setServerError(null);
    }
    setOpen(next);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger
        render={
          <Button className="gap-1.5 bg-primary text-white hover:bg-primary-dark" />
        }
      >
        <Plus className="h-4 w-4" />
        {label}
      </DialogTrigger>

      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create project</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4 pt-2">
          {serverError && (
            <p className="rounded-lg bg-danger-bg px-3 py-2 text-sm text-danger">
              {serverError}
            </p>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="proj-name">Name</Label>
            <Input
              id="proj-name"
              placeholder="My App"
              disabled={isSubmitting}
              aria-invalid={!!errors.name}
              {...register("name", { onChange: handleNameChange })}
            />
            {errors.name && (
              <p className="text-xs text-danger">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="proj-slug">Slug</Label>
            <Input
              id="proj-slug"
              placeholder="my-app"
              className="font-mono text-sm"
              disabled={isSubmitting}
              aria-invalid={!!errors.slug}
              {...register("slug")}
            />
            {errors.slug && (
              <p className="text-xs text-danger">{errors.slug.message}</p>
            )}
            <p className="text-xs text-text-secondary">
              Used in API calls. Only lowercase letters, numbers, and hyphens.
            </p>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              className="bg-primary text-white hover:bg-primary-dark"
              disabled={isSubmitting}
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating…
                </>
              ) : (
                "Create project"
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
