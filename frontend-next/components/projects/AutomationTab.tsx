"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { integrations, type AutomationRule, type AutomationRun } from "@/lib/api";

const TRIGGERS = [
  { value: "issue_created", label: "Issue created" },
  { value: "issue_updated", label: "Issue updated" },
  { value: "issue_transitioned", label: "Issue transitioned" },
] as const;

const CONDITION_FIELDS = [
  { value: "priority", label: "Priority" },
  { value: "title_contains", label: "Title contains" },
] as const;

const ACTION_TYPES = [
  { value: "set_assignee", label: "Set assignee" },
  { value: "add_label", label: "Add label" },
  { value: "transition_issue", label: "Transition issue" },
  { value: "add_comment", label: "Add comment" },
] as const;

interface ConditionRow {
  field: string;
  value: string;
}
interface ActionRow {
  type: string;
  value: string;
}

function RunsView({ ruleId }: { ruleId: string }) {
  const runs = useQuery({
    queryKey: ["automation-runs", ruleId],
    queryFn: () => integrations.automationRuns(ruleId),
  });

  if (runs.isLoading) {
    return <p className="py-1 text-xs text-slate-400">Loading runs…</p>;
  }

  const rows = runs.data ?? [];
  if (rows.length === 0) {
    return <p className="py-1 text-xs text-slate-400">No runs yet</p>;
  }

  return (
    <ul className="space-y-1" data-testid={`runs-${ruleId}`}>
      {rows.map((run: AutomationRun) => (
        <li key={run.id} className="flex flex-col gap-0.5 border-b border-slate-100 py-1 text-xs">
          <span className="flex items-center gap-2">
            <span className="font-medium text-[#1a1f36]">{run.status}</span>
            <span className="text-slate-400">{new Date(run.triggered_at).toLocaleString()}</span>
          </span>
          {run.log && <span className="text-slate-500">{run.log}</span>}
        </li>
      ))}
    </ul>
  );
}

export function AutomationTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [triggerType, setTriggerType] = useState<string>("issue_created");
  const [conditions, setConditions] = useState<ConditionRow[]>([]);
  const [actions, setActions] = useState<ActionRow[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);

  const rules = useQuery({
    queryKey: ["automation", projectKey],
    queryFn: () => integrations.automationRules(projectKey),
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["automation", projectKey] });

  const resetForm = () => {
    setName("");
    setTriggerType("issue_created");
    setConditions([]);
    setActions([]);
    setShowForm(false);
  };

  const create = useMutation({
    mutationFn: () => {
      const condObj: Record<string, string> = {};
      for (const c of conditions) {
        if (c.field) condObj[c.field] = c.value;
      }
      const actArr = actions
        .filter((a) => a.type)
        .map((a) => ({ type: a.type, value: a.value }));
      return integrations.automationCreate(projectKey, {
        name,
        trigger_type: triggerType,
        conditions_json: JSON.stringify(condObj),
        actions_json: JSON.stringify(actArr),
      });
    },
    onSuccess: () => {
      resetForm();
      invalidate();
    },
  });

  const toggle = useMutation({
    mutationFn: (rule: AutomationRule) =>
      integrations.automationUpdate(rule.id, { is_active: !rule.is_active }),
    onSuccess: invalidate,
  });

  const del = useMutation({
    mutationFn: (ruleId: string) => integrations.automationDelete(ruleId),
    onSuccess: invalidate,
  });

  return (
    <div className="space-y-6" data-testid="automation-tab">
      <section>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-700">Automation rules</h3>
          {!showForm && (
            <button
              onClick={() => setShowForm(true)}
              className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white"
            >
              New rule
            </button>
          )}
        </div>

        <ul className="space-y-1" data-testid="automation-list">
          {(rules.data ?? []).map((rule: AutomationRule) => (
            <li key={rule.id} className="border-b border-slate-100 py-1.5 text-sm">
              <div className="flex items-center gap-2">
                <span className="text-[#1a1f36]">{rule.name}</span>
                <span className="text-xs text-slate-400">{rule.trigger_type}</span>
                <span className="ml-auto flex items-center gap-3">
                  <label className="flex items-center gap-1 text-xs text-slate-600">
                    <input
                      type="checkbox"
                      aria-label={`Toggle rule ${rule.name}`}
                      checked={rule.is_active}
                      onChange={() => toggle.mutate(rule)}
                    />
                    Active
                  </label>
                  <button
                    onClick={() => setExpanded(expanded === rule.id ? null : rule.id)}
                    className="text-xs text-[#0052cc] hover:underline"
                    aria-label={`View runs for ${rule.name}`}
                  >
                    View runs
                  </button>
                  <button
                    onClick={() => del.mutate(rule.id)}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={`Delete rule ${rule.name}`}
                  >
                    Remove
                  </button>
                </span>
              </div>
              {expanded === rule.id && (
                <div className="mt-1 pl-1">
                  <RunsView ruleId={rule.id} />
                </div>
              )}
            </li>
          ))}
          {rules.data && rules.data.length === 0 && (
            <li className="py-2 text-sm text-slate-400">No automation rules</li>
          )}
        </ul>
      </section>

      {showForm && (
        <section className="rounded-xl border border-slate-200 p-4 space-y-4">
          <h3 className="text-sm font-semibold text-slate-700">New rule</h3>

          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              aria-label="Rule name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Rule name"
              className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
            />
            <select
              aria-label="Trigger"
              value={triggerType}
              onChange={(e) => setTriggerType(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              {TRIGGERS.map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <span className="text-xs font-semibold text-slate-500">Conditions</span>
              <button
                onClick={() => setConditions([...conditions, { field: "priority", value: "" }])}
                className="text-xs text-[#0052cc] hover:underline"
              >
                Add condition
              </button>
            </div>
            <ul className="space-y-1">
              {conditions.map((c, i) => (
                <li key={i} className="flex items-center gap-2">
                  <select
                    aria-label={`Condition field ${i + 1}`}
                    value={c.field}
                    onChange={(e) => {
                      const next = [...conditions];
                      next[i] = { ...next[i], field: e.target.value };
                      setConditions(next);
                    }}
                    className="rounded border border-slate-300 px-2 py-1 text-sm"
                  >
                    {CONDITION_FIELDS.map((f) => (
                      <option key={f.value} value={f.value}>{f.label}</option>
                    ))}
                  </select>
                  <input
                    aria-label={`Condition value ${i + 1}`}
                    value={c.value}
                    onChange={(e) => {
                      const next = [...conditions];
                      next[i] = { ...next[i], value: e.target.value };
                      setConditions(next);
                    }}
                    placeholder="Value"
                    className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
                  />
                  <button
                    onClick={() => setConditions(conditions.filter((_, j) => j !== i))}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={`Remove condition ${i + 1}`}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <span className="text-xs font-semibold text-slate-500">Actions</span>
              <button
                onClick={() => setActions([...actions, { type: "set_assignee", value: "" }])}
                className="text-xs text-[#0052cc] hover:underline"
              >
                Add action
              </button>
            </div>
            <ul className="space-y-1">
              {actions.map((a, i) => (
                <li key={i} className="flex items-center gap-2">
                  <select
                    aria-label={`Action type ${i + 1}`}
                    value={a.type}
                    onChange={(e) => {
                      const next = [...actions];
                      next[i] = { ...next[i], type: e.target.value };
                      setActions(next);
                    }}
                    className="rounded border border-slate-300 px-2 py-1 text-sm"
                  >
                    {ACTION_TYPES.map((t) => (
                      <option key={t.value} value={t.value}>{t.label}</option>
                    ))}
                  </select>
                  <input
                    aria-label={`Action value ${i + 1}`}
                    value={a.value}
                    onChange={(e) => {
                      const next = [...actions];
                      next[i] = { ...next[i], value: e.target.value };
                      setActions(next);
                    }}
                    placeholder="Value"
                    className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
                  />
                  <button
                    onClick={() => setActions(actions.filter((_, j) => j !== i))}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={`Remove action ${i + 1}`}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          </div>

          {create.isError && (
            <p className="text-sm text-red-600">
              {create.error instanceof Error ? create.error.message : "Failed to create rule"}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <button
              onClick={resetForm}
              className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600"
            >
              Cancel
            </button>
            <button
              onClick={() => name && create.mutate()}
              disabled={!name || create.isPending}
              className="rounded bg-[#0052cc] px-4 py-1 text-sm text-white disabled:opacity-60"
            >
              Create
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
