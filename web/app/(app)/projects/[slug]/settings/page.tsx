import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { Project, Environment } from "@/lib/types";
import { Topbar } from "@/components/layout/topbar";
import { ProjectSettingsForm } from "@/components/settings/project-settings-form";
import { EnvironmentsList } from "@/components/settings/environments-list";
import { DangerZone } from "@/components/settings/danger-zone";

interface SettingsPageProps {
  params: Promise<{ slug: string }>;
}

export default async function SettingsPage({ params }: SettingsPageProps) {
  const { slug } = await params;

  const cookieStore = await cookies();

  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const [projectRes, envRes] = await Promise.all([
    serverFetch<{ project: Project }>(`/api/v1/projects/${slug}`, token),
    serverFetch<{ environments: Environment[] }>(`/api/v1/projects/${slug}/environments`, token),
  ]);

  const { project } = projectRes;
  const { environments } = envRes;

  return (
    <div className="flex h-full flex-col">
      <Topbar title="Settings" />
      <main className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-2xl space-y-10 p-6">
          <ProjectSettingsForm projectSlug={slug} name={project.name} />
          <EnvironmentsList projectSlug={slug} environments={environments} />
          <DangerZone projectSlug={slug} />
        </div>
      </main>
    </div>
  );
}
