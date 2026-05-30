import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProjectCard } from "@/components/projects/project-card";
import type { Project } from "@/lib/types";

const project: Project = {
  id: "p1",
  tenantId: "t1",
  slug: "my-app",
  name: "My App",
  createdAt: "2026-06-15T12:00:00Z",
  updatedAt: "2026-06-15T12:00:00Z",
};

describe("ProjectCard", () => {
  it("renders project name and slug", () => {
    render(<ProjectCard project={project} />);
    expect(screen.getByText("My App")).toBeInTheDocument();
    expect(screen.getByText("my-app")).toBeInTheDocument();
  });

  it("links to the project flags page", () => {
    render(<ProjectCard project={project} />);
    expect(screen.getByRole("link")).toHaveAttribute(
      "href",
      "/projects/my-app/flags",
    );
  });

  it("renders the creation date", () => {
    render(<ProjectCard project={project} />);
    expect(screen.getByText(/jun 15, 2026/i)).toBeInTheDocument();
  });
});
