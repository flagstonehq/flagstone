"use client";

import { Input } from "@/components/ui/input";

interface RolloutInputProps {
  value: number | undefined;
  onChange: (value: number | undefined) => void;
  error?: string;
}

export function RolloutInput({ value, onChange, error }: RolloutInputProps) {
  return (
    <div className="flex items-center gap-2">
      <label className="text-xs text-text-secondary">Rollout</label>
      <div className="relative w-20">
        <Input
          type="number"
          min={0}
          max={100}
          step={1}
          value={value ?? 100}
          onChange={(e) => {
            const v = e.target.value === "" ? undefined : Number(e.target.value);
            onChange(v);
          }}
          aria-label="Rollout percentage"
          className="pr-6"
        />
        <span className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-xs text-text-secondary">
          %
        </span>
      </div>
      {error && <span className="text-xs text-danger">{error}</span>}
    </div>
  );
}
