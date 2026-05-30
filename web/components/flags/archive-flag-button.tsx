"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { Archive, Loader2 } from "lucide-react";
import { archiveFlag } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
interface ArchiveFlagButtonProps {
  projectSlug: string;
  flagKey: string;
  flagName: string;
}
export function ArchiveFlagButton({ projectSlug, flagKey, flagName }: ArchiveFlagButtonProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await archiveFlag(projectSlug, flagKey);
      setOpen(false);
      router.refresh();
    } catch {
      setError("Failed to archive flag. Please try again.");
    } finally {
      setLoading(false);
    }
  }
  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="text-text-secondary hover:text-danger hover:bg-danger-bg rounded p-1.5 transition-colors"
        aria-label={`Archive ${flagKey}`}
      >
        <Archive className="h-4 w-4" />
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Archive flag?</DialogTitle>
          </DialogHeader>
          <p className="text-text-secondary text-sm">
            <span className="text-text font-medium">{flagName}</span> will be archived and disabled
            in all environments. You can restore it later.
          </p>
          {error && (
            <p className="text-danger bg-danger-bg rounded-lg px-3 py-2 text-sm">{error}</p>
          )}
          <DialogFooter className="gap-2">
            {/* Cancel has focus by default per UX convention */}
            <Button variant="outline" onClick={() => setOpen(false)} disabled={loading} autoFocus>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleConfirm} disabled={loading}>
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Archive"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
