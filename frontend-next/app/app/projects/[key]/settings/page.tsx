import { ProjectSettings } from "@/components/projects/ProjectSettings";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return (
    <div>
      <ProjectHeader projectKey={key} active="settings" />
      <ProjectSettings projectKey={key} />
    </div>
  );
}
