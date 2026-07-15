"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issues, meta, projects as projectsApi } from "@/lib/api";

interface Props {
  projectKey?: string;
  onClose: () => void;
  onCreated: (key: string) => void;
}

export function CreateIssueModal({ projectKey, onClose, onCreated }: Props) {
  const qc = useQueryClient();
  const { data: types } = useQuery({
    queryKey: ["issuetypes"],
    queryFn: () => meta.issueTypes(),
  });

  // Quando projectKey non è passato (es. dal menu "Create" della topbar),
  // mostriamo un selettore di progetto alimentato dagli stessi dati usati
  // dalla pagina progetti (projects.search).
  const showProjectPicker = !projectKey;
  const projectsList = useQuery({
    queryKey: ["projects", ""],
    queryFn: () => projectsApi.search({ maxResults: 50 }),
    enabled: showProjectPicker,
  });

  const [selectedProjectKey, setSelectedProjectKey] = useState("");
  const [summary, setSummary] = useState("");
  const [typeName, setTypeName] = useState("Task");

  const effectiveProjectKey = projectKey ?? selectedProjectKey;

  const create = useMutation({
    mutationFn: () =>
      issues.create({ projectKey: effectiveProjectKey, summary, issueTypeName: typeName }),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["projectIssues", effectiveProjectKey] });
      onCreated(res.key);
      onClose();
    },
  });

  const canSubmit = !!effectiveProjectKey && !!summary && !create.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
      <div className="w-[480px] rounded-lg bg-white p-6 shadow-xl">
        <h2 className="mb-4 text-lg font-semibold text-[#1a1f36]">Create issue</h2>

        {showProjectPicker && (
          <>
            <label
              htmlFor="issue-project"
              className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
            >
              Project
            </label>
            <select
              id="issue-project"
              value={selectedProjectKey}
              onChange={(e) => setSelectedProjectKey(e.target.value)}
              className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
            >
              <option value="">Select a project…</option>
              {(projectsList.data?.values ?? []).map((p) => (
                <option key={p.key} value={p.key}>
                  {p.name} ({p.key})
                </option>
              ))}
            </select>
          </>
        )}

        <label
          htmlFor="issue-type"
          className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
        >
          Type
        </label>
        <select
          id="issue-type"
          value={typeName}
          onChange={(e) => setTypeName(e.target.value)}
          className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        >
          {(types ?? [{ id: "0", name: "Task", subtask: false }]).map((t) => (
            <option key={t.id} value={t.name}>
              {t.name}
            </option>
          ))}
        </select>

        <label
          htmlFor="issue-summary"
          className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
        >
          Summary
        </label>
        <input
          id="issue-summary"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          className="mb-4 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        />

        {create.isError && (
          <p className="mb-3 text-sm text-red-600">
            {create.error instanceof Error ? create.error.message : "Failed to create issue"}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="rounded px-4 py-2 text-sm text-slate-600 hover:text-slate-800">
            Cancel
          </button>
          <button
            onClick={() => create.mutate()}
            disabled={!canSubmit}
            className="rounded bg-[#0052cc] px-4 py-2 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
          >
            {create.isPending ? "Creating…" : "Create"}
          </button>
        </div>
      </div>
    </div>
  );
}
