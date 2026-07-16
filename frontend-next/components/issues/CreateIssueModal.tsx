"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issues, meta, projects as projectsApi } from "@/lib/api";
import { textToAdf } from "./adf";
import { UserPicker } from "@/components/common/UserPicker";

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
  const { data: priorities } = useQuery({
    queryKey: ["priorities"],
    queryFn: () => meta.priorities(),
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
  const [description, setDescription] = useState("");
  const [priorityId, setPriorityId] = useState("");
  const [assigneeId, setAssigneeId] = useState<string | null>(null);
  const [assigneeLabel, setAssigneeLabel] = useState<string | null>(null);
  const [parentKey, setParentKey] = useState("");
  const [createAnother, setCreateAnother] = useState(false);
  const [justCreatedKey, setJustCreatedKey] = useState<string | null>(null);

  const effectiveProjectKey = projectKey ?? selectedProjectKey;
  const isSubtask = typeName === "Subtask";

  const create = useMutation({
    mutationFn: () =>
      issues.create({
        projectKey: effectiveProjectKey,
        summary,
        issueTypeName: typeName,
        ...(description.trim() ? { description: textToAdf(description) } : {}),
        ...(priorityId ? { priorityId } : {}),
        ...(assigneeId ? { assigneeId } : {}),
        ...(isSubtask && parentKey.trim() ? { parentKey: parentKey.trim() } : {}),
      }),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["projectIssues", effectiveProjectKey] });
      if (createAnother) {
        // Real Jira Cloud's "Create another" keeps project/type/priority/
        // assignee (the fields you're likely to want unchanged across a
        // batch) and only clears the per-issue summary/description/parent.
        setSummary("");
        setDescription("");
        setParentKey("");
        setJustCreatedKey(res.key);
      } else {
        onCreated(res.key);
        onClose();
      }
    },
  });

  function submit() {
    setJustCreatedKey(null);
    create.mutate();
  }

  const canSubmit = !!effectiveProjectKey && !!summary && !create.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
      <div className="w-[480px] max-h-[90vh] overflow-y-auto rounded-lg bg-white p-6 shadow-xl">
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
          className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        />

        <label
          htmlFor="issue-description"
          className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
        >
          Description
        </label>
        <textarea
          id="issue-description"
          rows={4}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Add a description…"
          className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        />

        <div className="mb-3 grid grid-cols-2 gap-3">
          <div>
            <label
              htmlFor="issue-priority"
              className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
            >
              Priority
            </label>
            <select
              id="issue-priority"
              value={priorityId}
              onChange={(e) => setPriorityId(e.target.value)}
              className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
            >
              <option value="">Default</option>
              {(priorities ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <div className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500">
              Assignee
            </div>
            <UserPicker
              projectKey={effectiveProjectKey}
              value={assigneeId}
              valueLabel={assigneeLabel}
              disabled={!effectiveProjectKey}
              onChange={(accountId, user) => {
                setAssigneeId(accountId);
                setAssigneeLabel(user?.displayName ?? null);
              }}
            />
          </div>
        </div>

        {isSubtask && (
          <>
            <label
              htmlFor="issue-parent"
              className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
            >
              Parent issue key
            </label>
            <input
              id="issue-parent"
              value={parentKey}
              onChange={(e) => setParentKey(e.target.value)}
              placeholder="e.g. DEMO-1"
              className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
            />
          </>
        )}

        <label className="mb-4 flex items-center gap-2 text-sm text-slate-600">
          <input
            type="checkbox"
            checked={createAnother}
            onChange={(e) => setCreateAnother(e.target.checked)}
            className="rounded border-slate-300"
          />
          Create another
        </label>

        {justCreatedKey && (
          <p className="mb-3 text-sm text-green-700">{justCreatedKey} created. Add the next one below.</p>
        )}

        {create.isError && (
          <p className="mb-3 text-sm text-red-600">
            {create.error instanceof Error ? create.error.message : "Failed to create issue"}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="rounded px-4 py-2 text-sm text-slate-600 hover:text-slate-800">
            {createAnother ? "Done" : "Cancel"}
          </button>
          <button
            onClick={submit}
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
