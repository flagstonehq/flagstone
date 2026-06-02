"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Loader2 } from "lucide-react";
import { createSegmentSchema } from "@/lib/schemas";
import { createSegment, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

type FormValues = z.infer<typeof createSegmentSchema>;

interface CreateSegmentDialogProps {
  projectSlug: string;
}

export function CreateSegmentDialog({ projectSlug }: CreateSegmentDialogProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(createSegmentSchema),
  });

  async function onSubmit(values: FormValues) {
    setServerError(null);
    try {
      await createSegment(projectSlug, values);
      reset();
      setOpen(false);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setServerError("A segment with that key already exists.");
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
        render={<Button className="bg-primary hover:bg-primary-dark gap-1.5 text-white" />}
      >
        <Plus className="h-4 w-4" />
        New segment
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create segment</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4 pt-2">
          {serverError && (
            <p className="text-danger bg-danger-bg rounded-lg px-3 py-2 text-sm">{serverError}</p>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="segment-key">Key</Label>
            <Input
              id="segment-key"
              placeholder="beta-users"
              className="font-mono text-sm"
              disabled={isSubmitting}
              aria-invalid={!!errors.key}
              {...register("key")}
            />
            {errors.key && <p className="text-danger text-xs">{errors.key.message}</p>}
            <p className="text-text-secondary text-xs">
              Lowercase letters, numbers, hyphens, underscores.
            </p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="segment-name">Name</Label>
            <Input
              id="segment-name"
              placeholder="Beta Users"
              disabled={isSubmitting}
              aria-invalid={!!errors.name}
              {...register("name")}
            />
            {errors.name && <p className="text-danger text-xs">{errors.name.message}</p>}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="segment-desc">
              Description <span className="text-text-secondary font-normal">(optional)</span>
            </Label>
            <Textarea
              id="segment-desc"
              placeholder="Users enrolled in the beta program"
              rows={2}
              disabled={isSubmitting}
              {...register("description")}
            />
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
              className="bg-primary hover:bg-primary-dark text-white"
              disabled={isSubmitting}
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating…
                </>
              ) : (
                "Create segment"
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
