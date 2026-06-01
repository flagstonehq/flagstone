import { cookies } from "next/headers";
import { redirect, notFound } from "next/navigation";
import { serverFetch } from "@/lib/api";
import { getEnvironments, getFlagEnvironment } from "@/lib/api";
import type { Flag } from "@/lib/types";
import { RuleEditor } from "@/components/rules/rule-editor";
import { Topbar } from "@/components/layout/topbar";

interface FlagDetailPageProps {
  params: Promise<{ slug: string; key: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export default async function FlagDetailPage({ params, searchParams }: FlagDetailPageProps) {
  const { slug, key } = await params;
  const sp = await searchParams;

  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const flags = await serverFetch<Flag[]>(
    `/api/v1/projects/${slug}/flags`,
    token,
  );

  const flag = flags.find((f) => f.key === key);
  if (!flag) notFound();

  const environments = await getEnvironments(slug);

  const envSlug = (Array.isArray(sp.env) ? sp.env[0] : sp.env) ?? environments[0]?.slug;
  const targetEnv = environments.find((e) => e.slug === envSlug);

  let envConfig;
  if (targetEnv) {
    try {
      envConfig = await getFlagEnvironment(slug, key, envSlug);
    } catch {
      envConfig = {
        flagId: flag.id,
        environmentId: targetEnv.id,
        enabled: false,
        rules: [],
        defaultValue: flag.defaultValue,
        version: 0,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
    }
  }

  return (
    <div className="flex h-full flex-col">
      <Topbar
        title={flag.name}
        action={
          <span className="text-xs text-text-secondary font-mono">{key}</span>
        }
      />
      <main className="flex-1 overflow-y-auto p-6">
        <RuleEditor
          projectSlug={slug}
          flagKey={key}
          flagType={flag.type}
          initialRules={envConfig?.rules ?? []}
          initialEnabled={envConfig?.enabled ?? false}
          initialVersion={envConfig?.version ?? 0}
          environments={environments}
          currentEnvSlug={envSlug}
        />
      </main>
    </div>
  );
}
