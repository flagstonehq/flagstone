import Link from "next/link";
import type { Project } from "@/lib/types";
import { formatDate } from "@/lib/utils";
import { ArrowRight } from "lucide-react";

interface ProjectCardProps {
  project: Project;
}

export function ProjectCard({ project }: ProjectCardProps) {
  return (
    <Link
      href={`/projects/${project.slug}/flags`}
      className="group block rounded-xl border border-border bg-surface p-5 transition-all hover:border-primary hover:shadow-sm"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h2 className="truncate font-semibold text-text transition-colors group-hover:text-primary">
            {project.name}
          </h2>
          <p className="mt-0.5 font-mono text-xs text-text-secondary">
            {project.slug}
          </p>
        </div>
        <ArrowRight className="mt-0.5 h-4 w-4 shrink-0 text-text-secondary transition-colors group-hover:text-primary" />
      </div>

      <div className="mt-4 border-t border-border pt-4">
        <p className="text-xs text-text-secondary">
          Created {formatDate(project.createdAt)}
        </p>
      </div>
    </Link>
  );
}
