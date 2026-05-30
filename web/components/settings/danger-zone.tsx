"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { AlertTriangle, Loader2 } from "lucide-react";
import { deleteProject, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
interface DangerZoneProps {
  projectSlug: string;
}
export function DangerZone({ projectSlug }: DangerZoneProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [typedSlug, setTypedSlug] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const slugMatches = typedSlug === projectSlug;
  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await deleteProject(projectSlug);
      setOpen(false);
      router.push("/projects");
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to delete project. Please try again.");
      }
    } finally {
      setLoading(false);
    }
  }
  function handleOpenChange(next: boolean) {
    if (!next) {
      setTypedSlug("");
      setError(null);
    }
    setOpen(next);
  }
  return (
    <section className="border-danger/40 bg-danger-bg/20 rounded-lg border p-4">
      <div className="text-danger flex items-center gap-2">
        <AlertTriangle className="h-5 w-5" />
        <h2 className="text-lg font-medium">Danger Zone</h2>
      </div>
      <p className="text-text-secondary mt-1 text-sm">
        Deleting a project will permanently remove all flags, environments, and settings. This
        action cannot be undone.
      </p>
      <Button variant="destructive" className="mt-4" onClick={() => setOpen(true)}>
        Delete project
      </Button>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete project?</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <p className="text-text-secondary text-sm">
              This will permanently delete{" "}
              <span className="text-text font-medium">{projectSlug}</span> and all its data. Type{" "}
              <span className="text-text font-mono font-medium">{projectSlug}</span> to confirm.
            </p>
            {error && (
              <p className="bg-danger-bg text-danger rounded-lg px-3 py-2 text-sm">{error}</p>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="confirm-slug">
                Type <span className="font-mono">{projectSlug}</span> to confirm
              </Label>
              <Input
                id="confirm-slug"
                value={typedSlug}
                onChange={(e) => setTypedSlug(e.target.value)}
                placeholder={projectSlug}
                className="font-mono text-sm"
                disabled={loading}
              />
            </div>
          </div>
          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={loading}
              autoFocus
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirm}
              disabled={!slugMatches || loading}
            >
              {loading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Deleting…
                </>
              ) : (
                "Delete project"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
