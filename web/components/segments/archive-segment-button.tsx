"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { Archive, Loader2 } from "lucide-react";
import { archiveSegment } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";

interface ArchiveSegmentButtonProps {
  projectSlug: string;
  segmentKey: string;
  segmentName: string;
}

export function ArchiveSegmentButton({ projectSlug, segmentKey, segmentName }: ArchiveSegmentButtonProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await archiveSegment(projectSlug, segmentKey);
      setOpen(false);
      router.refresh();
    } catch {
      setError("Failed to archive segment. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="text-text-secondary hover:text-danger hover:bg-danger-bg rounded p-1.5 transition-colors"
        aria-label={`Archive ${segmentKey}`}
      >
        <Archive className="h-4 w-4" />
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Archive segment?</DialogTitle>
          </DialogHeader>
          <p className="text-text-secondary text-sm">
            <span className="text-text font-medium">{segmentName}</span> will be archived. Flags
            referencing this segment will skip it during evaluation.
          </p>
          {error && (
            <p className="text-danger bg-danger-bg rounded-lg px-3 py-2 text-sm">{error}</p>
          )}
          <DialogFooter className="gap-2">
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
