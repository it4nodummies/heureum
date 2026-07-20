"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import Link from "next/link";
import {
  issues,
  meta,
  watchers,
  votes,
  parseJiraDuration,
  formatSeconds,
  customFields,
  versions,
  type ADFNode,
  type CustomField,
  type IssueCustomValue,
} from "@/lib/api";
import { AdfRenderer } from "./adf";
import { RichTextEditor } from "@/components/common/RichTextEditor";
import { Activity } from "./Activity";
import { Attachments } from "./Attachments";
import { DevelopmentPanel } from "./DevelopmentPanel";
import { LinkedWorkItems } from "./LinkedWorkItems";
import { Subtasks } from "./Subtasks";
import { TimeTracking } from "./TimeTracking";
import { UserPicker } from "@/components/common/UserPicker";

interface Props {
  issueKey: string;
}

export function IssueView({ issueKey }: Props) {
  const { data: issue, isLoading, isError, error } = useQuery({
    queryKey: ["issue", issueKey],
    queryFn: () => issues.get(issueKey),
  });

  const qc = useQueryClient();

  // Single "Edit" mode covers summary, description, priority, assignee and
  // labels — they're saved together via one PUT /rest/api/3/issue/{key}.
  const [editMode, setEditMode] = useState(false);
  const [draftSummary, setDraftSummary] = useState("");
  const [draftDescriptionAdf, setDraftDescriptionAdf] = useState<ADFNode | null>(null);
  // Bumped on each entry into edit mode so the (uncontrolled) RichTextEditor
  // remounts and re-hydrates from the current description.
  const [editSession, setEditSession] = useState(0);
  const [draftPriorityId, setDraftPriorityId] = useState("");
  const [draftAssigneeId, setDraftAssigneeId] = useState<string | null>(null);
  const [draftAssigneeLabel, setDraftAssigneeLabel] = useState<string | null>(null);
  const [draftLabels, setDraftLabels] = useState("");
  const [draftFixVersionIds, setDraftFixVersionIds] = useState<string[]>([]);
  const [draftStoryPoints, setDraftStoryPoints] = useState("");
  const [draftOriginalEstimate, setDraftOriginalEstimate] = useState("");
  const [draftRemainingEstimate, setDraftRemainingEstimate] = useState("");

  // Dynamic custom fields. projectKey is derived from the issue key prefix so
  // the query can run before the issue payload loads (hooks must be
  // unconditional). Story points (customfield_10016) are NATIVE and handled
  // separately below — they are NOT part of this system.
  const cfProjectKey = issueKey.split("-")[0];
  const { data: customFieldDefs } = useQuery({
    queryKey: ["customFields", cfProjectKey],
    queryFn: () => customFields.list(cfProjectKey),
  });
  const { data: customFieldValues } = useQuery({
    queryKey: ["customValues", issueKey],
    queryFn: () => customFields.values(issueKey),
  });
  // Drafts for edit mode: scalar values (text/number/date/user-accountId/
  // select-option-id) plus multiselect option-id arrays.
  const [draftCustom, setDraftCustom] = useState<Record<string, string>>({});
  const [draftCustomMulti, setDraftCustomMulti] = useState<Record<string, string[]>>({});
  const [draftCustomUserLabels, setDraftCustomUserLabels] = useState<Record<string, string | null>>({});
  // Names of custom fields whose value failed to persist after the issue itself
  // was already updated (non-blocking).
  const [cfCustomWarnings, setCfCustomWarnings] = useState<string[]>([]);

  // Re-seed custom-field drafts whenever we enter edit mode OR the definitions/
  // values finish loading. Only seeds fields the user hasn't already typed into
  // (non-empty draft), so late-arriving query data fills blank inputs without
  // clobbering in-progress edits. startEdit resets the drafts to {} first, so a
  // fresh edit always reflects current server values.
  useEffect(() => {
    if (!editMode || !customFieldDefs) return;
    setDraftCustom((prev) => {
      const next = { ...prev };
      for (const def of customFieldDefs) {
        if (def.field_type === "multiselect") continue;
        if ((next[def.id] ?? "") !== "") continue;
        const val = (customFieldValues ?? []).find((cv) => cv.field_id === def.id);
        if (def.field_type === "select") next[def.id] = val?.option_id ?? "";
        else if (def.field_type === "number") next[def.id] = val?.value_number != null ? String(val.value_number) : "";
        else if (def.field_type === "date") next[def.id] = val?.value_date ? val.value_date.slice(0, 10) : "";
        else next[def.id] = val?.value_text ?? "";
      }
      return next;
    });
    setDraftCustomMulti((prev) => {
      const next = { ...prev };
      for (const def of customFieldDefs) {
        if (def.field_type !== "multiselect") continue;
        if ((next[def.id] ?? []).length > 0) continue;
        const val = (customFieldValues ?? []).find((cv) => cv.field_id === def.id);
        next[def.id] = val?.option_id ? [val.option_id] : [];
      }
      return next;
    });
  }, [editMode, customFieldDefs, customFieldValues]);

  function cfIsEmpty(cf: CustomField) {
    if (cf.field_type === "multiselect") return (draftCustomMulti[cf.id] ?? []).length === 0;
    return !(draftCustom[cf.id] ?? "").trim();
  }
  const requiredCustomMissing = (customFieldDefs ?? []).some((cf) => cf.required && cfIsEmpty(cf));

  const { data: priorities } = useQuery({
    queryKey: ["priorities"],
    queryFn: () => meta.priorities(),
    enabled: editMode,
  });

  // Project versions used as the fix-versions option set (only fetched in edit
  // mode). Keyed on the project prefix so it mirrors the custom-field query.
  const { data: projectVersions } = useQuery({
    queryKey: ["versions", cfProjectKey],
    queryFn: () => versions.list(cfProjectKey),
    enabled: editMode,
  });

  const save = useMutation({
    mutationFn: async () => {
      await issues.update(issueKey, {
        summary: draftSummary,
        description: draftDescriptionAdf ?? { type: "doc", version: 1, content: [] },
        ...(draftPriorityId ? { priority: { id: draftPriorityId } } : {}),
        // Always sent (not conditional on truthy) so picking "Unassigned"
        // (draftAssigneeId === null) actually clears the assignee rather than
        // being silently dropped from the payload.
        assignee: { accountId: draftAssigneeId ?? "" },
        labels: draftLabels
          .split(",")
          .map((l) => l.trim())
          .filter(Boolean),
        // Always sent (even when empty) so deselecting every version clears the
        // fix-versions rather than being dropped; [] tells the PUT handler to
        // reconcile the pivot to empty.
        fixVersions: draftFixVersionIds.map((id) => ({ id })),
        customfield_10016: draftStoryPoints.trim() === "" ? 0 : Number(draftStoryPoints),
        timetracking: {
          originalEstimateSeconds: parseJiraDuration(draftOriginalEstimate),
          remainingEstimateSeconds: parseJiraDuration(draftRemainingEstimate),
        },
      });
      // Custom-value failures do NOT reject the save — the issue update already
      // succeeded; they surface as a non-blocking warning instead.
      return persistCustomValues();
    },
    onSuccess: (failed) => {
      qc.invalidateQueries({ queryKey: ["issue", issueKey] });
      qc.invalidateQueries({ queryKey: ["customValues", issueKey] });
      setCfCustomWarnings(failed);
      setEditMode(false);
    },
  });

  // Persists each filled custom field, isolating each setValue in try/catch so
  // one failure never rejects the save (which would leave the issue updated but
  // strand the user in edit mode). Returns the names of fields that failed.
  async function persistCustomValues(): Promise<string[]> {
    const failed: string[] = [];
    for (const cf of customFieldDefs ?? []) {
      try {
        if (cf.field_type === "multiselect") {
          const ids = draftCustomMulti[cf.id] ?? [];
          if (ids.length) await customFields.setValue(issueKey, cf.id, { option_id: ids[0] });
          continue;
        }
        const raw = (draftCustom[cf.id] ?? "").trim();
        if (!raw) continue;
        if (cf.field_type === "select") {
          await customFields.setValue(issueKey, cf.id, { option_id: raw });
        } else if (cf.field_type === "number") {
          await customFields.setValue(issueKey, cf.id, { value: Number(raw) });
        } else if (cf.field_type === "date") {
          await customFields.setValue(issueKey, cf.id, { value: `${raw}T00:00:00Z` });
        } else {
          await customFields.setValue(issueKey, cf.id, { value: raw });
        }
      } catch {
        failed.push(cf.name);
      }
    }
    return failed;
  }

  const { data: w } = useQuery({ queryKey: ["watchers", issueKey], queryFn: () => watchers.get(issueKey) });
  const { data: v } = useQuery({ queryKey: ["votes", issueKey], queryFn: () => votes.get(issueKey) });
  const toggleWatch = useMutation({
    mutationFn: () => (w?.isWatching ? watchers.unwatch(issueKey) : watchers.watch(issueKey)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["watchers", issueKey] }),
  });
  const toggleVote = useMutation({
    mutationFn: () => (v?.hasVoted ? votes.unvote(issueKey) : votes.vote(issueKey)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["votes", issueKey] }),
  });

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <div className="w-7 h-7 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
        <span className="text-sm text-slate-400">Loading issue…</span>
      </div>
    );
  }

  if (isError || !issue) {
    return (
      <div className="px-8 py-8">
        <div className="p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
          {error instanceof Error ? error.message : "Issue not found."}
        </div>
      </div>
    );
  }

  const f = issue.fields;

  function startEdit() {
    setDraftSummary(f.summary);
    setDraftDescriptionAdf(f.description);
    setEditSession((n) => n + 1);
    setDraftPriorityId(f.priority?.id ?? "");
    setDraftAssigneeId(f.assignee?.accountId ?? null);
    setDraftAssigneeLabel(f.assignee?.displayName ?? null);
    setDraftLabels(f.labels.join(", "));
    setDraftFixVersionIds((f.fixVersions ?? []).map((v) => v.id));
    setDraftStoryPoints(f.customfield_10016 != null ? String(f.customfield_10016) : "");
    setDraftOriginalEstimate(
      f.timetracking?.originalEstimateSeconds ? formatSeconds(f.timetracking.originalEstimateSeconds) : ""
    );
    setDraftRemainingEstimate(
      f.timetracking?.remainingEstimateSeconds ? formatSeconds(f.timetracking.remainingEstimateSeconds) : ""
    );
    // Reset custom-field drafts; the [editMode, customFieldDefs,
    // customFieldValues] effect seeds them from current values (and re-seeds if
    // those queries are still loading when Edit is clicked).
    setDraftCustom({});
    setDraftCustomMulti({});
    setDraftCustomUserLabels({});
    setCfCustomWarnings([]);
    setEditMode(true);
  }

  const projectKey = f.project?.key ?? issue.key.split("-")[0];
  const projectName = f.project?.name ?? projectKey;

  return (
    <div className="max-w-5xl px-8 py-8">
      <nav data-testid="issue-breadcrumb" className="mb-3 flex items-center gap-1.5 text-xs text-slate-400">
        <Link href="/app/projects" className="text-[#0052cc] hover:underline">
          Projects
        </Link>
        <span>›</span>
        <Link href={`/app/projects/${projectKey}`} className="text-[#0052cc] hover:underline">
          {projectName}
        </Link>
        <span>›</span>
        <span className="text-slate-400">{issue.key}</span>
      </nav>

      <div className="mb-2 text-xs font-mono text-slate-400">{issue.key}</div>

      {editMode ? (
        <input
          autoFocus
          value={draftSummary}
          onChange={(e) => setDraftSummary(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") setEditMode(false);
          }}
          className="mb-6 w-full rounded border border-[#0052cc] px-2 py-1 text-2xl font-bold text-[#1a1f36] tracking-tight"
        />
      ) : (
        <div className="mb-6 flex items-center gap-2">
          <h1 className="text-2xl font-bold text-[#1a1f36] tracking-tight">{f.summary}</h1>
          <button
            type="button"
            onClick={startEdit}
            aria-label="Edit"
            data-testid="issue-edit-button"
            className="rounded p-1 text-slate-400 hover:text-[#0052cc] focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M12 20h9" />
              <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" />
            </svg>
          </button>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-[1fr_260px] gap-8">
        <div className="bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-6">
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
            Description
          </h2>
          {editMode ? (
            <RichTextEditor
              key={editSession}
              valueAdf={draftDescriptionAdf}
              onChangeAdf={setDraftDescriptionAdf}
              placeholder="Add a description…"
              projectKey={projectKey}
              ariaLabel="Description"
              testId="description-editor"
            />
          ) : (
            <AdfRenderer doc={f.description} />
          )}
        </div>

        <aside className="space-y-4 bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-5 h-fit">
          <button onClick={() => toggleWatch.mutate()} disabled={toggleWatch.isPending}
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-60">
            {w?.isWatching ? "Stop watching" : "Watch"} ({w?.watchCount ?? 0})
          </button>
          <button onClick={() => toggleVote.mutate()} disabled={toggleVote.isPending}
            className="mt-2 w-full rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-60">
            {v?.hasVoted ? "Unvote" : "Vote"} ({v?.votes ?? 0})
          </button>
          <Field label="Status" value={f.status?.name} />
          <Field label="Type" value={f.issuetype?.name} />

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Priority</div>
              <select
                value={draftPriorityId}
                onChange={(e) => setDraftPriorityId(e.target.value)}
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              >
                {!draftPriorityId && <option value="">Select…</option>}
                {(priorities ?? []).map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <Field label="Priority" value={f.priority?.name} />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Assignee</div>
              <UserPicker
                projectKey={projectKey}
                value={draftAssigneeId}
                valueLabel={draftAssigneeLabel}
                onChange={(accountId, user) => {
                  setDraftAssigneeId(accountId);
                  setDraftAssigneeLabel(user?.displayName ?? null);
                }}
              />
            </div>
          ) : (
            <Field label="Assignee" value={f.assignee?.displayName ?? "Unassigned"} />
          )}
          <Field label="Reporter" value={f.reporter?.displayName ?? "—"} />

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Labels</div>
              <input
                value={draftLabels}
                onChange={(e) => setDraftLabels(e.target.value)}
                placeholder="comma, separated, labels"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field label="Labels" value={f.labels.length ? f.labels.join(", ") : "None"} />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Fix versions</div>
              <select
                multiple
                data-testid="issue-fixversions-select"
                aria-label="Fix versions"
                value={draftFixVersionIds}
                onChange={(e) =>
                  setDraftFixVersionIds(Array.from(e.target.selectedOptions, (o) => o.value))
                }
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              >
                {(projectVersions ?? []).map((ver) => (
                  <option key={ver.id} value={ver.id}>
                    {ver.name}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Fix versions</div>
              <div data-testid="issue-fixversions" className="mt-1 flex flex-wrap gap-1">
                {(f.fixVersions ?? []).length ? (
                  (f.fixVersions ?? []).map((ver) => (
                    <span
                      key={ver.id}
                      className="rounded bg-slate-100 px-2 py-0.5 text-xs text-[#1a1f36]"
                    >
                      {ver.name}
                    </span>
                  ))
                ) : (
                  <span className="text-sm text-[#1a1f36]">None</span>
                )}
              </div>
            </div>
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                Story points
              </div>
              <input
                type="number"
                min={0}
                value={draftStoryPoints}
                onChange={(e) => setDraftStoryPoints(e.target.value)}
                placeholder="0"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field label="Story points" value={f.customfield_10016 != null ? String(f.customfield_10016) : undefined} />
          )}

          {(customFieldDefs ?? []).map((cf) => (
            <CustomFieldRow
              key={cf.id}
              field={cf}
              value={(customFieldValues ?? []).find((v) => v.field_id === cf.id)}
              editMode={editMode}
              projectKey={projectKey}
              scalar={draftCustom[cf.id] ?? ""}
              multi={draftCustomMulti[cf.id] ?? []}
              userLabel={draftCustomUserLabels[cf.id] ?? null}
              onScalar={(val) => setDraftCustom((p) => ({ ...p, [cf.id]: val }))}
              onMulti={(vals) => setDraftCustomMulti((p) => ({ ...p, [cf.id]: vals }))}
              onUser={(accountId, label) => {
                setDraftCustom((p) => ({ ...p, [cf.id]: accountId ?? "" }));
                setDraftCustomUserLabels((p) => ({ ...p, [cf.id]: label }));
              }}
            />
          ))}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                Original estimate
              </div>
              <input
                value={draftOriginalEstimate}
                onChange={(e) => setDraftOriginalEstimate(e.target.value)}
                placeholder="e.g. 1d 4h"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field
              label="Original estimate"
              value={f.timetracking?.originalEstimateSeconds ? formatSeconds(f.timetracking.originalEstimateSeconds) : "—"}
            />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                Remaining estimate
              </div>
              <input
                value={draftRemainingEstimate}
                onChange={(e) => setDraftRemainingEstimate(e.target.value)}
                placeholder="e.g. 4h"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field
              label="Remaining estimate"
              value={f.timetracking?.remainingEstimateSeconds ? formatSeconds(f.timetracking.remainingEstimateSeconds) : "—"}
            />
          )}

          {editMode && (
            <div className="flex gap-2 pt-2">
              <button
                onClick={() => save.mutate()}
                disabled={save.isPending || requiredCustomMissing}
                className="flex-1 rounded bg-[#0052cc] px-3 py-2 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
              >
                {save.isPending ? "Saving…" : "Save"}
              </button>
              <button
                onClick={() => setEditMode(false)}
                className="flex-1 rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
              >
                Cancel
              </button>
            </div>
          )}

          {editMode && requiredCustomMissing && (
            <p className="text-xs text-amber-700">Fill all required (*) custom fields to save.</p>
          )}

          {save.isError && (
            <p className="text-xs text-red-600">
              {save.error instanceof Error ? save.error.message : "Failed to save changes."}
            </p>
          )}

          {cfCustomWarnings.length > 0 && (
            <p className="text-xs text-amber-700">
              Saved, but these custom fields could not be updated: {cfCustomWarnings.join(", ")}.
            </p>
          )}
        </aside>
      </div>

      <DevelopmentPanel issueKey={issue.key} />

      <Subtasks issueKey={issue.key} projectKey={projectKey} />

      <LinkedWorkItems issueKey={issue.key} />

      <Attachments issueKey={issue.key} />

      <TimeTracking issueKey={issue.key} />

      <Activity issueKey={issue.key} projectKey={projectKey} />
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{label}</div>
      <div data-testid={`field-${label.toLowerCase()}`} className="text-sm text-[#1a1f36] mt-0.5">
        {value ?? "—"}
      </div>
    </div>
  );
}

const cfEditClass =
  "mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]";

function CustomFieldRow({
  field,
  value,
  editMode,
  projectKey,
  scalar,
  multi,
  userLabel,
  onScalar,
  onMulti,
  onUser,
}: {
  field: CustomField;
  value?: IssueCustomValue;
  editMode: boolean;
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

  function displayValue(): string {
    if (!value) return "—";
    switch (field.field_type) {
      case "number":
        return value.value_number != null ? String(value.value_number) : "—";
      case "date":
        return value.value_date ? value.value_date.slice(0, 10) : "—";
      case "select":
      case "multiselect": {
        const opt = (options ?? []).find((o) => o.id === value.option_id);
        return opt?.value ?? (value.option_id ? "…" : "—");
      }
      default:
        return value.value_text || "—";
    }
  }

  return (
    <div data-testid={`custom-field-${field.name}`}>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
        {field.name}
        {field.required && <span className="text-red-500"> *</span>}
      </div>
      {!editMode ? (
        <div className="text-sm text-[#1a1f36] mt-0.5">{displayValue()}</div>
      ) : field.field_type === "number" ? (
        <input type="number" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfEditClass} />
      ) : field.field_type === "date" ? (
        <input type="date" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfEditClass} />
      ) : field.field_type === "select" ? (
        <select value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfEditClass}>
          <option value="">Select…</option>
          {(options ?? []).map((o) => (
            <option key={o.id} value={o.id}>
              {o.value}
            </option>
          ))}
        </select>
      ) : field.field_type === "multiselect" ? (
        <select
          multiple
          value={multi}
          onChange={(e) => onMulti(Array.from(e.target.selectedOptions, (o) => o.value))}
          className={cfEditClass}
        >
          {(options ?? []).map((o) => (
            <option key={o.id} value={o.id}>
              {o.value}
            </option>
          ))}
        </select>
      ) : field.field_type === "user" ? (
        <UserPicker
          projectKey={projectKey}
          value={scalar || null}
          valueLabel={userLabel ?? (value?.value_text ? "Assigned" : null)}
          label={field.name}
          onChange={(accountId, user) => onUser(accountId, user?.displayName ?? null)}
        />
      ) : (
        <input type="text" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfEditClass} />
      )}
    </div>
  );
}
