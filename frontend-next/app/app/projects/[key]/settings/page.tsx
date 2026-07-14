import { ProjectSettings } from "@/components/projects/ProjectSettings";

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return <ProjectSettings projectKey={key} />;
}
