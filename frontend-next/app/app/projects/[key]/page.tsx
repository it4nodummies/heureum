import { ProjectOverview } from "@/components/projects/ProjectOverview";

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return <ProjectOverview projectKey={key} />;
}
