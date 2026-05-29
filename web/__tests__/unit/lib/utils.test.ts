import { describe, it, expect } from "vitest";
import { cn, formatDate, formatRelative } from "@/lib/utils";

describe("cn", () => {
  it("merges class names", () => {
    expect(cn("px-4", "py-2")).toBe("px-4 py-2");
  });
  it("handles conditional classes", () => {
    const show = false
    expect(cn("px-4", show && "hidden", "py-2")).toBe("px-4 py-2");
  });
});

describe("formatDate", () => {
  it("formats ISO date with month, day, year", () => {
    const result = formatDate("2026-06-15T12:00:00Z");
    expect(result).toMatch(/Jun\s+\d{1,2},\s+2026/);
  });
});

describe("formatRelative", () => {
  it('returns "just now" for recent dates', () => {
    const result = formatRelative(new Date().toISOString());
    expect(result).toBe("just now");
  });
  it('returns "5m ago" for 5 minute old dates', () => {
    const past = new Date(Date.now() - 300_000).toISOString();
    const result = formatRelative(past);
    expect(result).toBe("5m ago");
  });
  it("returns formatted date for old dates", () => {
    const old = new Date("2026-01-15T12:00:00Z").toISOString();
    const result = formatRelative(old);
    expect(result).not.toMatch(/\d+[mhd] ago/);
  });
});
