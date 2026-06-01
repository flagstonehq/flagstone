"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";

interface RawKeyModalProps {
  rawKey: string;
  onClose: () => void;
}

export function RawKeyModal({ rawKey, onClose }: RawKeyModalProps) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(rawKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      const input = document.getElementById("raw-key-input") as HTMLInputElement;
      input?.select();
    }
  }

  return (
    <Dialog
      open={true}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md" showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>API Key created</DialogTitle>
        </DialogHeader>
        <p className="text-danger bg-danger-bg rounded-lg px-3 py-2 text-sm">
          This key will not be shown again. Copy it now and store it securely.
        </p>
        <div className="flex gap-2">
          <Input
            id="raw-key-input"
            value={rawKey}
            readOnly
            className="flex-1 font-mono text-xs"
            onClick={(e) => (e.target as HTMLInputElement).select()}
          />
          <Button
            variant="outline"
            size="icon"
            onClick={handleCopy}
            aria-label={copied ? "Copied" : "Copy to clipboard"}
          >
            {copied ? <Check className="text-success h-4 w-4" /> : <Copy className="h-4 w-4" />}
          </Button>
        </div>
        <div className="flex justify-end">
          <Button
            variant="default"
            className="bg-primary hover:bg-primary-dark text-white"
            onClick={onClose}
            autoFocus
          >
            Done
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
