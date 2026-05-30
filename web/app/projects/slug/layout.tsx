import { Sidebar } from "@/components/layout/sidebar";

interface ProjectLayoutProps {
  children: React.ReactNode;
  params: Promise<{ slug: string }>;
}

export default async function ProjectLayout({ children, params }: ProjectLayoutProps) {
  const { slug } = await params;

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar projectSlug={slug} />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">{children}</div>
    </div>
  );
}
