"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issues, meta, projects as projectsApi, customFields, versions, type CustomField } from "@/lib/api";
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
  const [fixVersionIds, setFixVersionIds] = useState<string[]>([]);
  const [createAnother, setCreateAnother] = useState(false);
  const [justCreatedKey, setJustCreatedKey] = useState<string | null>(null);

  // Dynamic custom-field values keyed by field id. Scalar (text/number/date/
  // user-accountId/select-option-id) live in cfValues; multiselect option ids
  // live in cfMulti. Story points (customfield_10016) are NATIVE and are not
  // part of this system — never rendered here.
  const [cfValues, setCfValues] = useState<Record<string, string>>({});
  const [cfMulti, setCfMulti] = useState<Record<string, string[]>>({});
  const [cfUserLabels, setCfUserLabels] = useState<Record<string, string | null>>({});
  // Names of custom fields whose value failed to persist after the issue was
  // already created (non-blocking — the issue exists regardless).
  const [cfWarnings, setCfWarnings] = useState<string[]>([]);

  const effectiveProjectKey = projectKey ?? selectedProjectKey;
  const isSubtask = typeName === "Subtask";

  const { data: customFieldDefs } = useQuery({
    queryKey: ["customFields", effectiveProjectKey],
    queryFn: () => customFields.list(effectiveProjectKey),
    enabled: !!effectiveProjectKey,
  });

  // Fix-versions option set for the selected project.
  const { data: projectVersions } = useQuery({
    queryKey: ["versions", effectiveProjectKey],
    queryFn: () => versions.list(effectiveProjectKey),
    enabled: !!effectiveProjectKey,
  });

  function cfIsEmpty(cf: CustomField) {
    if (cf.field_type === "multiselect") return (cfMulti[cf.id] ?? []).length === 0;
    return !(cfValues[cf.id] ?? "").trim();
  }
  const requiredMissing = (customFieldDefs ?? []).some((cf) => cf.required && cfIsEmpty(cf));

  // Persists each filled custom field. Each setValue is isolated in its own
  // try/catch so one failure never rejects the mutation (which would skip the
  // success path and tempt the user into re-submitting = a DUPLICATE issue).
  // Returns the names of fields that failed to save.
  async function persistCustomValues(newKey: string): Promise<string[]> {
    const failed: string[] = [];
    for (const cf of customFieldDefs ?? []) {
      try {
        if (cf.field_type === "multiselect") {
          const ids = cfMulti[cf.id] ?? [];
          // The value model stores a single option per field; persist the first
          // selection (multiselect breadth is a known follow-up).
          if (ids.length) await customFields.setValue(newKey, cf.id, { option_id: ids[0] });
          continue;
        }
        const raw = (cfValues[cf.id] ?? "").trim();
        if (!raw) continue;
        if (cf.field_type === "select") {
          await customFields.setValue(newKey, cf.id, { option_id: raw });
        } else if (cf.field_type === "number") {
          await customFields.setValue(newKey, cf.id, { value: Number(raw) });
        } else if (cf.field_type === "date") {
          await customFields.setValue(newKey, cf.id, { value: `${raw}T00:00:00Z` });
        } else {
          // text and user (accountId) both go through value_text.
          await customFields.setValue(newKey, cf.id, { value: raw });
        }
      } catch {
        failed.push(cf.name);
      }
    }
    return failed;
  }

  const create = useMutation({
    mutationFn: async () => {
      setCfWarnings([]);
      const res = await issues.create({
        projectKey: effectiveProjectKey,
        summary,
        issueTypeName: typeName,
        ...(description.trim() ? { description: textToAdf(description) } : {}),
        ...(priorityId ? { priorityId } : {}),
        ...(assigneeId ? { assigneeId } : {}),
        ...(isSubtask && parentKey.trim() ? { parentKey: parentKey.trim() } : {}),
        ...(fixVersionIds.length ? { fixVersions: fixVersionIds.map((id) => ({ id })) } : {}),
      });
      // Custom-field values require the freshly-created issue key. Failures here
      // do NOT reject — they come back as a list to surface non-blockingly.
      const failed = await persistCustomValues(res.key);
      return { res, failed };
    },
    onSuccess: ({ res, failed }) => {
      qc.invalidateQueries({ queryKey: ["projectIssues", effectiveProjectKey] });
      if (failed.length) setCfWarnings(failed);
      if (createAnother) {
        // Real Jira Cloud's "Create another" keeps project/type/priority/
        // assignee (the fields you're likely to want unchanged across a
        // batch) and only clears the per-issue summary/description/parent.
        setSummary("");
        setDescription("");
        setParentKey("");
        setFixVersionIds([]);
        setCfValues({});
        setCfMulti({});
        setCfUserLabels({});
        setJustCreatedKey(res.key);
      } else if (failed.length) {
        // Issue WAS created; keep the modal open so the warning is visible and
        // clear the per-issue inputs so a re-click can't create a duplicate.
        setSummary("");
        setDescription("");
        setParentKey("");
        setFixVersionIds([]);
        setCfValues({});
        setCfMulti({});
        setCfUserLabels({});
        setJustCreatedKey(res.key);
      } else {
        onCreated(res.key);
        onClose();
      }
    },
  });

  function submit() {
    setJustCreatedKey(null);
    setCfWarnings([]);
    create.mutate();
  }

  const canSubmit = !!effectiveProjectKey && !!summary && !requiredMissing && !create.isPending;

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

        <label
          htmlFor="issue-fixversions"
          className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
        >
          Fix versions
        </label>
        <select
          id="issue-fixversions"
          multiple
          value={fixVersionIds}
          onChange={(e) => setFixVersionIds(Array.from(e.target.selectedOptions, (o) => o.value))}
          disabled={!effectiveProjectKey}
          className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] disabled:opacity-60"
        >
          {(projectVersions ?? []).map((ver) => (
            <option key={ver.id} value={ver.id}>
              {ver.name}
            </option>
          ))}
        </select>

        {(customFieldDefs ?? []).map((cf) => (
          <div key={cf.id} className="mb-3">
            <label
              htmlFor={`cf-${cf.id}`}
              className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
            >
              {cf.name}
              {cf.required && <span className="text-red-500"> *</span>}
            </label>
            <CustomFieldCreateInput
              field={cf}
              projectKey={effectiveProjectKey}
              scalar={cfValues[cf.id] ?? ""}
              multi={cfMulti[cf.id] ?? []}
              userLabel={cfUserLabels[cf.id] ?? null}
              onScalar={(val) => setCfValues((p) => ({ ...p, [cf.id]: val }))}
              onMulti={(vals) => setCfMulti((p) => ({ ...p, [cf.id]: vals }))}
              onUser={(accountId, label) => {
                setCfValues((p) => ({ ...p, [cf.id]: accountId ?? "" }));
                setCfUserLabels((p) => ({ ...p, [cf.id]: label }));
              }}
            />
          </div>
        ))}

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

        {cfWarnings.length > 0 && (
          <p className="mb-3 text-sm text-amber-700">
            Issue created, but these custom fields could not be saved:{" "}
            {cfWarnings.join(", ")}. You can set them from the issue’s Details.
          </p>
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

const cfInputClass =
  "w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]";

function CustomFieldCreateInput({
  field,
  projectKey,
  scalar,
  multi,
  userLabel,
  onScalar,
  onMulti,
  onUser,
}: {
  field: CustomField;
  projectKey: string;
  scalar: string;
  multi: string[];
  userLabel: string | null;
  onScalar: (val: string) => void;
  onMulti: (vals: string[]) => void;
  onUser: (accountId: string | null, label: string | null) => void;
}) {
  const isOption = field.field_type === "select" || field.field_type === "multiselect";
  const { data: options } = useQuery({
    queryKey: ["customFieldOptions", field.id],
    queryFn: () => customFields.options(field.id),
    enabled: isOption,
  });

  switch (field.field_type) {
    case "number":
      return (
        <input id={`cf-${field.id}`} type="number" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfInputClass} />
      );
    case "date":
      return (
        <input id={`cf-${field.id}`} type="date" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfInputClass} />
      );
    case "select":
      return (
        <select id={`cf-${field.id}`} value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfInputClass}>
          <option value="">Select…</option>
          {(options ?? []).map((o) => (
            <option key={o.id} value={o.id}>
              {o.value}
            </option>
          ))}
        </select>
      );
    case "multiselect":
      return (
        <select
          id={`cf-${field.id}`}
          multiple
          value={multi}
          onChange={(e) => onMulti(Array.from(e.target.selectedOptions, (o) => o.value))}
          className={cfInputClass}
        >
          {(options ?? []).map((o) => (
            <option key={o.id} value={o.id}>
              {o.value}
            </option>
          ))}
        </select>
      );
    case "user":
      return (
        <UserPicker
          projectKey={projectKey}
          value={scalar || null}
          valueLabel={userLabel}
          disabled={!projectKey}
          label={field.name}
          onChange={(accountId, user) => onUser(accountId, user?.displayName ?? null)}
        />
      );
    default:
      return (
        <input id={`cf-${field.id}`} type="text" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfInputClass} />
      );
  }
}
