"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { integrations, type AutomationRule, type AutomationRun } from "@/lib/api";
import { useFormat, useT } from "@/lib/i18n";

// Stored values; the visible labels come from projects.automation.* in the catalogue.
const TRIGGERS = ["issue_created", "issue_updated", "issue_transitioned"] as const;
const CONDITION_FIELDS = ["priority", "title_contains"] as const;
const ACTION_TYPES = ["set_assignee", "add_label", "transition_issue", "add_comment"] as const;

const RUN_STATUSES = ["success", "skipped", "error", "test"] as const;

function isRunStatus(v: string): v is (typeof RUN_STATUSES)[number] {
  return (RUN_STATUSES as readonly string[]).includes(v);
}

function isTrigger(v: string): v is (typeof TRIGGERS)[number] {
  return (TRIGGERS as readonly string[]).includes(v);
}

interface ConditionRow {
  field: string;
  value: string;
}
interface ActionRow {
  type: string;
  value: string;
}

function RunsView({ ruleId }: { ruleId: string }) {
  const t = useT();
  const fmt = useFormat();
  const runs = useQuery({
    queryKey: ["automation-runs", ruleId],
    queryFn: () => integrations.automationRuns(ruleId),
  });

  if (runs.isLoading) {
    return <p className="py-1 text-xs text-slate-400">{t("projects.automation.loadingRuns")}</p>;
  }

  const rows = runs.data ?? [];
  if (rows.length === 0) {
    return <p className="py-1 text-xs text-slate-400">{t("projects.automation.noRuns")}</p>;
  }

  return (
    <ul className="space-y-1" data-testid={`runs-${ruleId}`}>
      {rows.map((run: AutomationRun) => (
        <li key={run.id} className="flex flex-col gap-0.5 border-b border-slate-100 py-1 text-xs">
          <span className="flex items-center gap-2">
            <span className="font-medium text-[#1a1f36]">
              {isRunStatus(run.status) ? t(`projects.automation.runStatus.${run.status}`) : run.status}
            </span>
            <span className="text-slate-400">{fmt.dateTime(run.triggered_at)}</span>
          </span>
          {run.log && <span className="text-slate-500">{run.log}</span>}
        </li>
      ))}
    </ul>
  );
}

export function AutomationTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const t = useT();
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
          <h3 className="text-sm font-semibold text-slate-700">{t("projects.automation.title")}</h3>
          {!showForm && (
            <button
              onClick={() => setShowForm(true)}
              className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white"
            >
              {t("projects.automation.newRule")}
            </button>
          )}
        </div>

        <ul className="space-y-1" data-testid="automation-list">
          {(rules.data ?? []).map((rule: AutomationRule) => (
            <li key={rule.id} className="border-b border-slate-100 py-1.5 text-sm">
              <div className="flex items-center gap-2">
                <span className="text-[#1a1f36]">{rule.name}</span>
                <span className="text-xs text-slate-400">
                  {isTrigger(rule.trigger_type) ? t(`projects.automation.triggers.${rule.trigger_type}`) : rule.trigger_type}
                </span>
                <span className="ml-auto flex items-center gap-3">
                  <label className="flex items-center gap-1 text-xs text-slate-600">
                    <input
                      type="checkbox"
                      aria-label={t("projects.automation.toggleAria", { name: rule.name })}
                      checked={rule.is_active}
                      onChange={() => toggle.mutate(rule)}
                    />
                    {t("projects.automation.active")}
                  </label>
                  <button
                    onClick={() => setExpanded(expanded === rule.id ? null : rule.id)}
                    className="text-xs text-[#0052cc] hover:underline"
                    aria-label={t("projects.automation.viewRunsAria", { name: rule.name })}
                  >
                    {t("projects.automation.viewRuns")}
                  </button>
                  <button
                    onClick={() => del.mutate(rule.id)}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={t("projects.automation.deleteAria", { name: rule.name })}
                  >
                    {t("projects.automation.remove")}
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
            <li className="py-2 text-sm text-slate-400">{t("projects.automation.empty")}</li>
          )}
        </ul>
      </section>

      {showForm && (
        <section className="rounded-xl border border-slate-200 p-4 space-y-4">
          <h3 className="text-sm font-semibold text-slate-700">{t("projects.automation.newRule")}</h3>

          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              aria-label={t("projects.automation.ruleName")}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("projects.automation.ruleName")}
              className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
            />
            <select
              aria-label={t("projects.automation.trigger")}
              value={triggerType}
              onChange={(e) => setTriggerType(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              {TRIGGERS.map((v) => (
                <option key={v} value={v}>{t(`projects.automation.triggers.${v}`)}</option>
              ))}
            </select>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <span className="text-xs font-semibold text-slate-500">{t("projects.automation.conditions")}</span>
              <button
                onClick={() => setConditions([...conditions, { field: "priority", value: "" }])}
                className="text-xs text-[#0052cc] hover:underline"
              >
                {t("projects.automation.addCondition")}
              </button>
            </div>
            <ul className="space-y-1">
              {conditions.map((c, i) => (
                <li key={i} className="flex items-center gap-2">
                  <select
                    aria-label={t("projects.automation.conditionField", { n: i + 1 })}
                    value={c.field}
                    onChange={(e) => {
                      const next = [...conditions];
                      next[i] = { ...next[i], field: e.target.value };
                      setConditions(next);
                    }}
                    className="rounded border border-slate-300 px-2 py-1 text-sm"
                  >
                    {CONDITION_FIELDS.map((v) => (
                      <option key={v} value={v}>{t(`projects.automation.conditionFields.${v}`)}</option>
                    ))}
                  </select>
                  <input
                    aria-label={t("projects.automation.conditionValue", { n: i + 1 })}
                    value={c.value}
                    onChange={(e) => {
                      const next = [...conditions];
                      next[i] = { ...next[i], value: e.target.value };
                      setConditions(next);
                    }}
                    placeholder={t("projects.automation.value")}
                    className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
                  />
                  <button
                    onClick={() => setConditions(conditions.filter((_, j) => j !== i))}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={t("projects.automation.removeCondition", { n: i + 1 })}
                  >
                    {t("projects.automation.remove")}
                  </button>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <span className="text-xs font-semibold text-slate-500">{t("projects.automation.actions")}</span>
              <button
                onClick={() => setActions([...actions, { type: "set_assignee", value: "" }])}
                className="text-xs text-[#0052cc] hover:underline"
              >
                {t("projects.automation.addAction")}
              </button>
            </div>
            <ul className="space-y-1">
              {actions.map((a, i) => (
                <li key={i} className="flex items-center gap-2">
                  <select
                    aria-label={t("projects.automation.actionType", { n: i + 1 })}
                    value={a.type}
                    onChange={(e) => {
                      const next = [...actions];
                      next[i] = { ...next[i], type: e.target.value };
                      setActions(next);
                    }}
                    className="rounded border border-slate-300 px-2 py-1 text-sm"
                  >
                    {ACTION_TYPES.map((v) => (
                      <option key={v} value={v}>{t(`projects.automation.actionTypes.${v}`)}</option>
                    ))}
                  </select>
                  <input
                    aria-label={t("projects.automation.actionValue", { n: i + 1 })}
                    value={a.value}
                    onChange={(e) => {
                      const next = [...actions];
                      next[i] = { ...next[i], value: e.target.value };
                      setActions(next);
                    }}
                    placeholder={t("projects.automation.value")}
                    className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
                  />
                  <button
                    onClick={() => setActions(actions.filter((_, j) => j !== i))}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={t("projects.automation.removeAction", { n: i + 1 })}
                  >
                    {t("projects.automation.remove")}
                  </button>
                </li>
              ))}
            </ul>
          </div>

          {create.isError && (
            <p className="text-sm text-red-600">
              {create.error instanceof Error ? create.error.message : t("projects.automation.createFailed")}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <button
              onClick={resetForm}
              className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600"
            >
              {t("projects.automation.cancel")}
            </button>
            <button
              onClick={() => name && create.mutate()}
              disabled={!name || create.isPending}
              className="rounded bg-[#0052cc] px-4 py-1 text-sm text-white disabled:opacity-60"
            >
              {t("projects.automation.create")}
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
