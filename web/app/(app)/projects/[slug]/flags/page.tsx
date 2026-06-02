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

  const [flags, environments] = await Promise.all([
    serverFetch<Flag[]>(`/api/v1/projects/${slug}/flags`, token),
    serverFetch<Environment[]>(`/api/v1/projects/${slug}/environments`, token),
  ]);

  // Fetch enabled states for all environments in parallel, then build a lookup
  // map: envSlug -> flagKey -> enabled. This lets each EnvToggle initialise
  // with the real state instead of always showing OFF.
  const flagStateResults = await Promise.allSettled(
    environments.map((env) =>
      serverFetch<{ flagKey: string; enabled: boolean }[]>(
        `/api/v1/projects/${slug}/environments/${env.slug}/flag-states`,
        token,
      ),
    ),
  );

  const enabledMap: Record<string, Record<string, boolean>> = {};
  environments.forEach((env, i) => {
    const result = flagStateResults[i];
    if (result.status === "fulfilled") {
      enabledMap[env.slug] = {};
      result.value.forEach((s) => {
        enabledMap[env.slug][s.flagKey] = s.enabled;
      });
    }
  });

  const activeFlags = flags.filter((f) => !f.archivedAt);

  return (
    <div className="flex h-full flex-col">
      <Topbar
        title="Flags"
        action={<CreateFlagDialog projectSlug={slug} environments={environments} />}
      />
      <main className="flex-1 overflow-y-auto p-4 sm:p-6">
        <FlagsTable
          flags={activeFlags}
          environments={environments}
          projectSlug={slug}
          enabledMap={enabledMap}
        />
      </main>
    </div>
  );
}
