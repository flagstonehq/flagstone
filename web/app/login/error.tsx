"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export default function LoginError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main className="min-h-screen flex items-center justify-center bg-bg">
      <div className="w-full max-w-sm text-center space-y-4">
        <div className="flex justify-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-danger-bg">
            <AlertTriangle className="h-8 w-8 text-danger" />
          </div>
        </div>
        <div>
          <h2 className="text-lg font-semibold text-text">Something went wrong</h2>
          <p className="mt-1 text-sm text-text-secondary">
            {error.message ?? "An unexpected error occurred."}
          </p>
        </div>
        <Button onClick={reset} variant="outline">
          Try again
        </Button>
      </div>
    </main>
  );
}
