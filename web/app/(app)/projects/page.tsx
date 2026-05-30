import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { Project } from "@/lib/types";
import { ProjectCard } from "@/components/projects/project-card";
import { CreateProjectDialog } from "@/components/projects/create-project-dialog";
import { Topbar } from "@/components/layout/topbar";
import { FolderOpen } from "lucide-react";

export default async function ProjectsPage() {
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const { projects } = await serverFetch<{ projects: Project[] }>(
    "/api/v1/projects",
    token,
  );

  return (
    <div className="flex flex-col h-full">
      <Topbar
        title="Projects"
        action={<CreateProjectDialog />}
      />

      <main className="flex-1 overflow-y-auto p-6">
        {projects.length === 0 ? (
          <div className="flex min-h-[400px] flex-col items-center justify-center gap-4 text-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary-bg">
              <FolderOpen className="h-8 w-8 text-primary" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-text">No projects yet</h2>
              <p className="mt-1 text-sm text-text-secondary">
                Create your first project to start managing feature flags.
              </p>
            </div>
            <CreateProjectDialog label="Create your first project" />
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {projects.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
