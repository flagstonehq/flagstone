"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Loader2 } from "lucide-react";
import { createApiKeySchema } from "@/lib/schemas";
import { createApiKey, ApiError } from "@/lib/api";
import type { Environment } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { RawKeyModal } from "@/components/api-keys/raw-key-modal";
type FormValues = z.infer<typeof createApiKeySchema>;
interface CreateKeyDialogProps {
  projectSlug: string;
  environments: Environment[];
}
export function CreateKeyDialog({ projectSlug, environments }: CreateKeyDialogProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [rawKey, setRawKey] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setValue,
    reset,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(createApiKeySchema),
    defaultValues: { name: "", environment_id: "", expires_at: undefined },
  });
  const selectedEnv = watch("environment_id") ?? "";
  async function onSubmit(values: FormValues) {
    setServerError(null);
    try {
      const result = await createApiKey(projectSlug, {
        name: values.name,
        environment_id: values.environment_id,
        expires_at: values.expires_at || undefined,
      });
      setRawKey(result.rawKey);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(err.message);
      } else {
        setServerError("Something went wrong. Please try again.");
      }
    }
  }
  function handleOpenChange(next: boolean) {
    if (!next) {
      reset();
      setServerError(null);
      setRawKey(null);
    }
    setOpen(next);
  }
  if (rawKey) {
    return (
      <RawKeyModal
        rawKey={rawKey}
        onClose={() => {
          setRawKey(null);
          setOpen(false);
          reset();
        }}
      />
    );
  }
  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger
        render={<Button className="bg-primary hover:bg-primary-dark gap-1.5 text-white" />}
      >
        <Plus className="h-4 w-4" />
        New API key
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create API key</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4 pt-2">
          {serverError && (
            <p className="bg-danger-bg text-danger rounded-lg px-3 py-2 text-sm">{serverError}</p>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="ak-name">Name</Label>
            <Input
              id="ak-name"
              placeholder="Production Key"
              disabled={isSubmitting}
              aria-invalid={!!errors.name}
              {...register("name")}
            />
            {errors.name && <p className="text-danger text-xs">{errors.name.message}</p>}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ak-env">Environment</Label>
            <Select
              value={selectedEnv}
              onValueChange={(v) => setValue("environment_id", v ?? "", { shouldValidate: true })}
              disabled={isSubmitting}
            >
              <SelectTrigger id="ak-env" aria-label="Environment" className="w-full">
                <SelectValue placeholder="Select environment" />
              </SelectTrigger>
              <SelectContent>
                {environments.map((env) => (
                  <SelectItem key={env.id} value={env.id}>
                    {env.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {errors.environment_id && (
              <p className="text-danger text-xs">{errors.environment_id.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ak-expires">
              Expires <span className="text-text-secondary">(optional)</span>
            </Label>
            <Input
              id="ak-expires"
              type="datetime-local"
              disabled={isSubmitting}
              {...register("expires_at")}
            />
            {errors.expires_at && (
              <p className="text-danger text-xs">{errors.expires_at.message}</p>
            )}
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
                "Create API key"
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
