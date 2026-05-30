"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Trash2, Loader2 } from "lucide-react";
import { createEnvironmentSchema } from "@/lib/schemas";
import { createEnvironment, deleteEnvironment, ApiError } from "@/lib/api";
import type { Environment } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";

interface EnvironmentsListProps {
  projectSlug: string;
  environments: Environment[];
}

function toSlug(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function EnvironmentsList({ projectSlug, environments }: EnvironmentsListProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setValue,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<z.infer<typeof createEnvironmentSchema>>({
    resolver: zodResolver(createEnvironmentSchema),
    defaultValues: { name: "", slug: "" },
  });
  function handleNameChange(e: React.ChangeEvent<HTMLInputElement>) {
    const name = e.target.value;
    setValue("name", name);
    setValue("slug", toSlug(name), { shouldValidate: false });
  }
  async function onCreateSubmit(values: z.infer<typeof createEnvironmentSchema>) {
    setServerError(null);
    try {
      await createEnvironment(projectSlug, values);
      reset();
      setOpen(false);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setServerError("An environment with that slug already exists.");
      } else {
        setServerError("Something went wrong. Please try again.");
      }
    }
  }
  async function handleDelete(env: Environment) {
    setDeleting(true);
    setDeleteError(null);
    try {
      await deleteEnvironment(projectSlug, env.id);
      setDeleteTarget(null);
      router.refresh();
    } catch {
      setDeleteError("Failed to delete environment. Please try again.");
    } finally {
      setDeleting(false);
    }
  }
  return (
    <section>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-medium">Environments</h2>
        <Dialog
          open={open}
          onOpenChange={(next) => {
            if (!next) {
              reset();
              setServerError(null);
            }
            setOpen(next);
          }}
        >
          <DialogTrigger render={<Button className="gap-1.5" />}>
            <Plus className="h-4 w-4" />
            Add environment
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Add environment</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleSubmit(onCreateSubmit)} noValidate className="space-y-4 pt-2">
              {serverError && (
                <p className="bg-danger-bg text-danger rounded-lg px-3 py-2 text-sm">
                  {serverError}
                </p>
              )}
              <div className="space-y-1.5">
                <Label htmlFor="env-name">Name</Label>
                <Input
                  id="env-name"
                  placeholder="Staging"
                  disabled={isSubmitting}
                  aria-invalid={!!errors.name}
                  {...register("name", { onChange: handleNameChange })}
                />
                {errors.name && <p className="text-danger text-xs">{errors.name.message}</p>}
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="env-slug">Slug</Label>
                <Input
                  id="env-slug"
                  placeholder="staging"
                  className="font-mono text-sm"
                  disabled={isSubmitting}
                  aria-invalid={!!errors.slug}
                  {...register("slug")}
                />
                {errors.slug && <p className="text-danger text-xs">{errors.slug.message}</p>}
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    reset();
                    setOpen(false);
                    setServerError(null);
                  }}
                  disabled={isSubmitting}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  className="bg-primary hover:bg-primary-dark text-white"
                  disabled={isSubmitting}
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Adding…
                    </>
                  ) : (
                    "Add environment"
                  )}
                </Button>
              </div>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      {environments.length === 0 ? (
        <p className="text-text-secondary text-sm">No environments yet.</p>
      ) : (
        <div className="space-y-2">
          {environments.map((env) => (
            <div
              key={env.id}
              className="border-border flex items-center justify-between rounded-lg border px-4 py-3"
            >
              <div>
                <p className="text-sm font-medium">{env.name}</p>
                <p className="text-text-secondary font-mono text-xs">{env.slug}</p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="text-text-secondary hover:text-danger"
                onClick={() => setDeleteTarget(env)}
                aria-label={`Delete ${env.name}`}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          ))}
        </div>
      )}
      {/* Delete confirmation dialog */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(next) => {
          if (!next) {
            setDeleteTarget(null);
            setDeleteError(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete environment?</DialogTitle>
          </DialogHeader>
          <p className="text-text-secondary text-sm">
            <span className="text-text font-medium">{deleteTarget?.name}</span> will be permanently
            removed. This action cannot be undone.
          </p>
          {deleteError && (
            <p className="bg-danger-bg text-danger rounded-lg px-3 py-2 text-sm">{deleteError}</p>
          )}
          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={() => setDeleteTarget(null)}
              disabled={deleting}
              autoFocus
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => deleteTarget && handleDelete(deleteTarget)}
              disabled={deleting}
            >
              {deleting ? <Loader2 className="h-4 w-4 animate-spin" /> : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
