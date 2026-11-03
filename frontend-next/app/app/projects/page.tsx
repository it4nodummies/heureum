import type { Metadata } from "next";
import { cookies, headers } from "next/headers";
import ProjectsPage from "@/components/projects/ProjectsPage";
import { LOCALE_COOKIE, firstAcceptLanguage, resolveLocale } from "@/lib/i18n/resolve";
import { translate } from "@/lib/i18n/translate";

// Tab title in the request locale (cookie → Accept-Language → en), like the root layout.
export async function generateMetadata(): Promise<Metadata> {
  const [cookieStore, headerList] = await Promise.all([cookies(), headers()]);
  const locale = resolveLocale({
    cookie: cookieStore.get(LOCALE_COOKIE)?.value,
    browser: firstAcceptLanguage(headerList.get("accept-language")),
  });
  return { title: translate(locale, "projects.meta.title") };
}

export default function ProjectsRoute() {
  return <ProjectsPage />;
}
