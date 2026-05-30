"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Loader2 } from "lucide-react";
import { createFlagSchema } from "@/lib/schemas";
import { createFlag, ApiError } from "@/lib/api";
import type { Environment } from "@/lib/types";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

type FormValues = z.infer<typeof createFlagSchema>;

interface CreateFlagDialogProps {
  projectSlug: string;
  environments: Environment[];
}

export function CreateFlagDialog({ projectSlug, environments }: CreateFlagDialogProps) {
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
    resolver: zodResolver(createFlagSchema),
    defaultValues: { type: "boolean" },
  });

  async function onSubmit(values: FormValues) {
    setServerError(null);
    try {
      await createFlag(projectSlug, values);
      reset();
      setOpen(false);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setServerError("A flag with that key already exists.");
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
        New flag
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create flag</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4 pt-2">
          {serverError && (
            <p className="text-danger bg-danger-bg rounded-lg px-3 py-2 text-sm">{serverError}</p>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="flag-key">Key</Label>
            <Input
              id="flag-key"
              placeholder="new-checkout-flow"
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
            <Label htmlFor="flag-name">Name</Label>
            <Input
              id="flag-name"
              placeholder="New Checkout Flow"
              disabled={isSubmitting}
              aria-invalid={!!errors.name}
              {...register("name")}
            />
            {errors.name && <p className="text-danger text-xs">{errors.name.message}</p>}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="flag-type">Type</Label>
            <Select
              defaultValue="boolean"
              onValueChange={(v) =>
                setValue("type", v as FormValues["type"], { shouldValidate: true })
              }
              disabled={isSubmitting}
            >
              <SelectTrigger id="flag-type" aria-invalid={!!errors.type}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="boolean">Boolean</SelectItem>
                <SelectItem value="string">String</SelectItem>
                <SelectItem value="number">Number</SelectItem>
                <SelectItem value="json">JSON</SelectItem>
              </SelectContent>
            </Select>
            {errors.type && <p className="text-danger text-xs">{errors.type.message}</p>}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="flag-desc">
              Description <span className="text-text-secondary font-normal">(optional)</span>
            </Label>
            <Textarea
              id="flag-desc"
              placeholder="What does this flag control?"
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
                "Create flag"
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
