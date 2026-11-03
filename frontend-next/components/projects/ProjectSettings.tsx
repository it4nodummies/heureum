"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { projects as projectsApi } from "@/lib/api";
import { WorkflowEditor } from "@/components/workflow/WorkflowEditor";
import { ProjectSummary } from "@/components/projects/ProjectSummary";
import { IntegrationsTab } from "@/components/projects/IntegrationsTab";
import { AutomationTab } from "@/components/projects/AutomationTab";
import { CustomFieldsTab } from "@/components/projects/CustomFieldsTab";
import { AccessTab } from "@/components/projects/AccessTab";
import { useT } from "@/lib/i18n";

interface Props {
  projectKey: string;
}

export function ProjectSettings({ projectKey }: Props) {
  const queryClient = useQueryClient();
  const router = useRouter();
  const t = useT();

  const { data: project, isLoading, isError, error } = useQuery({
    queryKey: ["project", projectKey],
    queryFn: () => projectsApi.get(projectKey),
  });

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [tab, setTab] = useState<"general" | "workflow" | "summary" | "integrations" | "automation" | "fields" | "access">("general");

  useEffect(() => {
    if (project) {
      setName(project.name);
      setDescription(project.description ?? "");
    }
  }, [project]);

  const save = useMutation({
    mutationFn: () => projectsApi.update(projectKey, { name, description }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["project", projectKey] });
      queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  const archive = useMutation({
    mutationFn: () => projectsApi.archive(projectKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
      router.push("/app/projects");
    },
  });

  function handleArchive() {
    if (!project) return;
    if (!confirm(t("projects.list.archiveConfirm", { name: project.name }))) return;
    archive.mutate();
  }

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <div className="w-7 h-7 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
        <span className="text-sm text-slate-400">{t("projects.header.loading")}</span>
      </div>
    );
  }

  if (isError || !project) {
    return (
      <div className="px-8 py-8">
        <div className="p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
          {error instanceof Error ? error.message : t("projects.header.notFound")}
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      <div className="px-8 pt-8 pb-0 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-[#1a1f36] tracking-tight">
            {t("projects.settings.title", { name: project.name })}
          </h1>
          <p className="text-sm text-slate-400 font-mono mt-1">{project.key}</p>
        </div>
      </div>

      <div className="px-8 py-6 max-w-lg">
        <div className="mb-4 flex gap-4 border-b">
          <button
            onClick={() => setTab("general")}
            className={tab === "general" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.general")}
          </button>
          <button
            onClick={() => setTab("workflow")}
            className={tab === "workflow" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.workflow")}
          </button>
          <button
            onClick={() => setTab("summary")}
            className={tab === "summary" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.summary")}
          </button>
          <button
            onClick={() => setTab("integrations")}
            className={tab === "integrations" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.integrations")}
          </button>
          <button
            onClick={() => setTab("automation")}
            className={tab === "automation" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.automation")}
          </button>
          <button
            onClick={() => setTab("fields")}
            className={tab === "fields" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.fields")}
          </button>
          <button
            onClick={() => setTab("access")}
            className={tab === "access" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}
          >
            {t("projects.settings.tabs.access")}
          </button>
        </div>

        {tab === "general" && (
          <>
            <div className="bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-6 space-y-4">
              <div>
                <label
                  htmlFor="proj-name"
                  className="block text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5"
                >
                  {t("projects.settings.name")}
                </label>
                <input
                  id="proj-name"
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all"
                />
              </div>
              <div>
                <label
                  htmlFor="proj-desc"
                  className="block text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5"
                >
                  {t("projects.settings.description")}
                </label>
                <textarea
                  id="proj-desc"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={4}
                  className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all resize-none"
                />
              </div>

              {save.isError && (
                <div className="p-3 bg-red-50 border border-red-100 text-red-600 text-sm rounded-lg">
                  {save.error instanceof Error ? save.error.message : t("projects.settings.saveFailed")}
                </div>
              )}
              {save.isSuccess && (
                <p className="text-sm text-green-600">{t("projects.settings.saved")}</p>
              )}

              <div className="flex justify-end pt-1">
                <button
                  onClick={() => save.mutate()}
                  disabled={save.isPending || !name}
                  className="px-5 py-2 bg-[#0052cc] hover:bg-[#0065ff] disabled:opacity-50 text-white text-sm font-semibold rounded-lg transition-colors"
                >
                  {save.isPending ? t("projects.settings.saving") : t("projects.settings.save")}
                </button>
              </div>
            </div>

            <div className="bg-white border border-red-100 rounded-2xl shadow-sm shadow-slate-100/80 p-6 mt-6">
              <h2 className="text-sm font-semibold text-[#1a1f36] mb-1">{t("projects.settings.archiveTitle")}</h2>
              <p className="text-xs text-slate-500 mb-4">
                {t("projects.settings.archiveHelp")}
              </p>
              <button
                onClick={handleArchive}
                disabled={archive.isPending}
                className="px-4 py-2 border border-red-200 text-red-500 hover:bg-red-50 disabled:opacity-50 text-sm font-semibold rounded-lg transition-colors"
              >
                {archive.isPending ? t("projects.settings.archiving") : t("projects.settings.archive")}
              </button>
              {archive.isError && (
                <p className="mt-3 text-sm text-red-600">
                  {archive.error instanceof Error ? archive.error.message : t("projects.settings.archiveFailed")}
                </p>
              )}
            </div>
          </>
        )}

        {tab === "workflow" && <WorkflowEditor projectKey={projectKey} />}

        {tab === "summary" && (
          <div className="space-y-3">
            <ProjectSummary projectKey={projectKey} />
            <a
              href={`/app/projects/${projectKey}/reports`}
              className="text-[#0052cc] hover:underline text-sm"
            >
              {t("projects.settings.openReports")}
            </a>
          </div>
        )}

        {tab === "integrations" && <IntegrationsTab projectKey={projectKey} />}

        {tab === "automation" && <AutomationTab projectKey={projectKey} />}

        {tab === "fields" && <CustomFieldsTab projectKey={projectKey} />}

        {tab === "access" && <AccessTab projectKey={projectKey} />}
      </div>
    </div>
  );
}
