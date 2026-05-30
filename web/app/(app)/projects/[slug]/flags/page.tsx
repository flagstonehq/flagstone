import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { Flag, Environment } from "@/lib/types";
import { FlagsTable } from "@/components/flags/flags-table";
import { CreateFlagDialog } from "@/components/flags/create-flag-dialog";
import { Topbar } from "@/components/layout/topbar";

interface FlagsPageProps {
  params: Promise<{ slug: string }>;
}

export default async function FlagsPage({ params }: FlagsPageProps) {
  const { slug } = await params;

  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const [{ flags }, { environments }] = await Promise.all([
    serverFetch<{ flags: Flag[] }>(`/api/v1/projects/${slug}/flags`, token),
    serverFetch<{ environments: Environment[] }>(
      `/api/v1/projects/${slug}/environments`,
      token,
    ),
  ]);

  const activeFlags = flags.filter((f) => !f.archivedAt);

  return (
    <div className="flex h-full flex-col">
      <Topbar
        title="Flags"
        action={
          <CreateFlagDialog projectSlug={slug} environments={environments} />
        }
      />
      <main className="flex-1 overflow-y-auto p-6">
        <FlagsTable
          flags={activeFlags}
          environments={environments}
          projectSlug={slug}
        />
      </main>
    </div>
  );
}
