"use client";

import { useSyncExternalStore } from "react";
import { Languages, Monitor, Moon } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { useTheme } from "@/components/layout/theme-provider";

const LANG_OPTIONS = [
  { value: "en", label: "English" },
  { value: "es", label: "Español" },
];

function useIsClient() {
  return useSyncExternalStore(
    () => () => {},
    () => true,
    () => false,
  );
}

export function PreferencesForm() {
  const { theme, setTheme } = useTheme();
  const mounted = useIsClient();

  const isSystem = theme === "system";
  const isDark = theme === "dark";

  return (
    <section>
      <h2 className="mb-4 text-lg font-medium">Preferences</h2>

      <div className="space-y-5">
        <div className="space-y-3">
          <Label>Theme</Label>

          <label className="flex items-center justify-between rounded-lg border border-border px-4 py-3 transition-colors hover:bg-hover">
            <div className="flex items-center gap-3">
              <Monitor className="h-5 w-5 text-text-secondary" />
              <div>
                <p className="text-sm font-medium text-text">Use system setting</p>
                <p className="text-xs text-text-secondary">Automatically switch based on your system</p>
              </div>
            </div>
            <Switch
              checked={mounted && isSystem}
              onCheckedChange={(checked) => setTheme(checked ? "system" : "light")}
            />
          </label>

          <label className={`flex items-center justify-between rounded-lg border px-4 py-3 transition-colors ${
            mounted && isSystem ? "border-border opacity-50" : "border-border hover:bg-hover"
          }`}>
            <div className="flex items-center gap-3">
              <Moon className="h-5 w-5 text-text-secondary" />
              <div>
                <p className="text-sm font-medium text-text">Dark mode</p>
                <p className="text-xs text-text-secondary">Use dark theme</p>
              </div>
            </div>
            <Switch
              checked={mounted && isDark}
              disabled={mounted && isSystem ? true : undefined}
              onCheckedChange={(checked) => setTheme(checked ? "dark" : "light")}
            />
          </label>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="pref-language">Language</Label>
          <Select value="en">
            <SelectTrigger className="w-full" id="pref-language" aria-label="Language">
              <Languages className="h-4 w-4" />
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LANG_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </section>
  );
}
