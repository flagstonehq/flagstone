"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { Trash2, Loader2 } from "lucide-react";
import { revokeApiKey } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
interface RevokeKeyButtonProps {
  projectSlug: string;
  envId: string;
  keyId: string;
  keyName: string;
}
export function RevokeKeyButton({ projectSlug, envId, keyId, keyName }: RevokeKeyButtonProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await revokeApiKey(projectSlug, envId, keyId);
      setOpen(false);
      router.refresh();
    } catch {
      setError("Failed to revoke key. Please try again.");
    } finally {
      setLoading(false);
    }
  }
  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="text-text-secondary hover:text-danger hover:bg-danger-bg rounded p-1.5 opacity-0 transition-colors group-hover:opacity-100"
        aria-label={`Revoke ${keyName}`}
      >
        <Trash2 className="h-4 w-4" />
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Revoke API key?</DialogTitle>
          </DialogHeader>
          <p className="text-text-secondary text-sm">
            <span className="text-text font-medium">{keyName}</span> will be permanently revoked.
            Any services using this key will lose access immediately.
          </p>
          {error && (
            <p className="text-danger bg-danger-bg rounded-lg px-3 py-2 text-sm">{error}</p>
          )}
          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={() => setOpen(false)} disabled={loading} autoFocus>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleConfirm} disabled={loading}>
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Revoke"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
