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
  hierarchyLevelOf,
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
import { ChildIssues } from "./Subtasks";
import { EpicColorPicker } from "./EpicColorPicker";
import { ParentField } from "./ParentField";
import { TimeTracking } from "./TimeTracking";
import { UserPicker } from "@/components/common/UserPicker";
import { useFormat, useT } from "@/lib/i18n";

interface Props {
  issueKey: string;
}

export function IssueView({ issueKey }: Props) {
  const t = useT();
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
        <span className="text-sm text-slate-400">{t("issues.detail.loading")}</span>
      </div>
    );
  }

  if (isError || !issue) {
    return (
      <div className="px-8 py-8">
        <div className="p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
          {error instanceof Error ? error.message : t("issues.detail.notFound")}
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
  // Jira hierarchy level of this issue (1 epic / 0 story-task-bug / -1
  // subtask): drives the Epic/Parent field, the colour picker and the
  // children section.
  const level = hierarchyLevelOf(f.issuetype);

  return (
    <div className="max-w-5xl px-8 py-8">
      <nav data-testid="issue-breadcrumb" className="mb-3 flex items-center gap-1.5 text-xs text-slate-400">
        <Link href="/app/projects" className="text-[#0052cc] hover:underline">
          {t("issues.detail.breadcrumbProjects")}
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
            aria-label={t("issues.actions.edit")}
            data-testid="issue-edit-button"
            className="rounded p-1 text-slate-400 hover:text-[#0052cc] focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 focus-visible:ring-[#0052cc]/60"
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
            {t("issues.detail.description")}
          </h2>
          {editMode ? (
            <RichTextEditor
              key={editSession}
              valueAdf={draftDescriptionAdf}
              onChangeAdf={setDraftDescriptionAdf}
              placeholder={t("issues.detail.descriptionPlaceholder")}
              projectKey={projectKey}
              ariaLabel={t("issues.detail.description")}
              testId="description-editor"
            />
          ) : (
            <AdfRenderer doc={f.description} />
          )}
        </div>

        <aside className="space-y-4 bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-5 h-fit">
          <button onClick={() => toggleWatch.mutate()} disabled={toggleWatch.isPending}
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-60">
            {w?.isWatching ? t("issues.detail.stopWatching") : t("issues.detail.watch")} ({w?.watchCount ?? 0})
          </button>
          <button onClick={() => toggleVote.mutate()} disabled={toggleVote.isPending}
            className="mt-2 w-full rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-60">
            {v?.hasVoted ? t("issues.detail.unvote") : t("issues.detail.vote")} ({v?.votes ?? 0})
          </button>
          <Field id="status" label={t("issues.detail.status")} value={f.status?.name} />
          <Field id="type" label={t("issues.detail.type")} value={f.issuetype?.name} />
          {level === 1 ? (
            <EpicColorPicker epicKey={issue.key} />
          ) : (
            <ParentField
              issueKey={issue.key}
              projectKey={projectKey}
              childLevel={level}
              parent={f.parent ?? null}
            />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{t("issues.detail.priority")}</div>
              <select
                value={draftPriorityId}
                onChange={(e) => setDraftPriorityId(e.target.value)}
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              >
                {!draftPriorityId && <option value="">{t("issues.actions.select")}</option>}
                {(priorities ?? []).map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <Field id="priority" label={t("issues.detail.priority")} value={f.priority?.name} />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{t("issues.detail.assignee")}</div>
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
            <Field id="assignee" label={t("issues.detail.assignee")} value={f.assignee?.displayName ?? t("issues.unassigned")} />
          )}
          <Field id="reporter" label={t("issues.detail.reporter")} value={f.reporter?.displayName ?? "—"} />

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{t("issues.detail.labels")}</div>
              <input
                value={draftLabels}
                onChange={(e) => setDraftLabels(e.target.value)}
                placeholder={t("issues.detail.labelsPlaceholder")}
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field id="labels" label={t("issues.detail.labels")} value={f.labels.length ? f.labels.join(", ") : t("issues.none")} />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{t("issues.detail.fixVersions")}</div>
              <select
                multiple
                data-testid="issue-fixversions-select"
                aria-label={t("issues.detail.fixVersions")}
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
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{t("issues.detail.fixVersions")}</div>
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
                  <span className="text-sm text-[#1a1f36]">{t("issues.none")}</span>
                )}
              </div>
            </div>
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                {t("issues.detail.storyPoints")}
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
            <Field id="story points" label={t("issues.detail.storyPoints")} value={f.customfield_10016 != null ? String(f.customfield_10016) : undefined} />
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
                {t("issues.detail.originalEstimate")}
              </div>
              <input
                value={draftOriginalEstimate}
                onChange={(e) => setDraftOriginalEstimate(e.target.value)}
                placeholder={t("issues.detail.originalEstimatePlaceholder")}
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field
              id="original estimate"
              label={t("issues.detail.originalEstimate")}
              value={f.timetracking?.originalEstimateSeconds ? formatSeconds(f.timetracking.originalEstimateSeconds) : undefined}
            />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                {t("issues.detail.remainingEstimate")}
              </div>
              <input
                value={draftRemainingEstimate}
                onChange={(e) => setDraftRemainingEstimate(e.target.value)}
                placeholder={t("issues.detail.remainingEstimatePlaceholder")}
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field
              id="remaining estimate"
              label={t("issues.detail.remainingEstimate")}
              value={f.timetracking?.remainingEstimateSeconds ? formatSeconds(f.timetracking.remainingEstimateSeconds) : undefined}
            />
          )}

          {editMode && (
            <div className="flex gap-2 pt-2">
              <button
                onClick={() => save.mutate()}
                disabled={save.isPending || requiredCustomMissing}
                className="flex-1 rounded bg-[#0052cc] px-3 py-2 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
              >
                {save.isPending ? t("issues.actions.saving") : t("issues.actions.save")}
              </button>
              <button
                onClick={() => setEditMode(false)}
                className="flex-1 rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
              >
                {t("issues.actions.cancel")}
              </button>
            </div>
          )}

          {editMode && requiredCustomMissing && (
            <p className="text-xs text-amber-700">{t("issues.detail.requiredCustomMissing")}</p>
          )}

          {save.isError && (
            <p className="text-xs text-red-600">
              {save.error instanceof Error ? save.error.message : t("issues.detail.saveFailed")}
            </p>
          )}

          {cfCustomWarnings.length > 0 && (
            <p className="text-xs text-amber-700">
              {t("issues.detail.customFieldsWarning", { fields: cfCustomWarnings.join(", ") })}
            </p>
          )}
        </aside>
      </div>

      <DevelopmentPanel issueKey={issue.key} />

      <ChildIssues issueKey={issue.key} projectKey={projectKey} parentLevel={level} />

      <LinkedWorkItems issueKey={issue.key} />

      <Attachments issueKey={issue.key} />

      <TimeTracking issueKey={issue.key} />

      <Activity issueKey={issue.key} projectKey={projectKey} />
    </div>
  );
}

// `id` keeps the pre-i18n English testid (`field-assignee`, `field-story points`…)
// independent of the translated label.
function Field({ id, label, value }: { id: string; label: string; value?: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{label}</div>
      <div data-testid={`field-${id}`} className="text-sm text-[#1a1f36] mt-0.5">
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
  const t = useT();
  const fmt = useFormat();
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
        // A date-only value stored at UTC midnight: format it in UTC so it
        // never shifts to the previous day west of Greenwich.
        return value.value_date
          ? fmt.date(`${value.value_date.slice(0, 10)}T00:00:00Z`, {
              day: "numeric",
              month: "short",
              year: "numeric",
              timeZone: "UTC",
            })
          : "—";
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
          <option value="">{t("issues.actions.select")}</option>
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
          valueLabel={userLabel ?? (value?.value_text ? t("issues.assigned") : null)}
          label={field.name}
          onChange={(accountId, user) => onUser(accountId, user?.displayName ?? null)}
        />
      ) : (
        <input type="text" value={scalar} onChange={(e) => onScalar(e.target.value)} className={cfEditClass} />
      )}
    </div>
  );
}
