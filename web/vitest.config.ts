import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "happy-dom",
    environmentOptions: {
      happyDOM: { url: "http://localhost" },
    },
    globals: true,
    setupFiles: ["__tests__/setup.ts"],
    exclude: ["__tests__/e2e/**", "node_modules/**"],
    coverage: {
      provider: "v8",
      thresholds: { lines: 80, functions: 80, branches: 80 },
      exclude: [
        "components/ui/**",
        "**/*.config.*",
        "__tests__/**",
        "next.config.ts",
        "postcss.config.mjs",
      ],
    },
  },
  resolve: {
    alias: { "@": path.resolve(__dirname, ".") },
  },
});
