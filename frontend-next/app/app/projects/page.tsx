import type { Metadata } from "next";
import ProjectsPage from "@/components/projects/ProjectsPage";
import { requestLocale } from "@/lib/i18n/server";
import { translate } from "@/lib/i18n/translate";

// Tab title in the request locale (cookie → Accept-Language → en), like the root layout.
export async function generateMetadata(): Promise<Metadata> {
  return { title: translate(await requestLocale(), "projects.meta.title") };
}

export default function ProjectsRoute() {
  return <ProjectsPage />;
}
