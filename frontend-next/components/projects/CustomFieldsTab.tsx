"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { customFields, type CustomField, type CustomFieldOption } from "@/lib/api";
import { useT } from "@/lib/i18n";

// Stored values; the visible labels come from projects.customFields.types.*.
const FIELD_TYPES = ["text", "number", "date", "select", "multiselect", "user"] as const;

function isFieldType(v: string): v is (typeof FIELD_TYPES)[number] {
  return (FIELD_TYPES as readonly string[]).includes(v);
}

function OptionsManager({ fieldId }: { fieldId: string }) {
  const qc = useQueryClient();
  const t = useT();
  const [value, setValue] = useState("");

  const options = useQuery({
    queryKey: ["custom-field-options", fieldId],
    queryFn: () => customFields.options(fieldId),
  });
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: ["custom-field-options", fieldId] });

  const add = useMutation({
    mutationFn: (v: string) => customFields.addOption(fieldId, v),
    onSuccess: () => {
      setValue("");
      invalidate();
    },
  });

  const remove = useMutation({
    mutationFn: (optionId: string) => customFields.removeOption(optionId),
    onSuccess: invalidate,
  });

  return (
    <div className="mt-2 pl-1">
      <ul className="space-y-1" data-testid={`options-${fieldId}`}>
        {(options.data ?? []).map((opt: CustomFieldOption) => (
          <li key={opt.id} className="flex items-center gap-2 text-xs">
            <span className="text-[#1a1f36]">{opt.value}</span>
            <button
              onClick={() => remove.mutate(opt.id)}
              className="text-red-600 hover:underline"
              aria-label={t("projects.customFields.removeOptionAria", { value: opt.value })}
            >
              {t("projects.customFields.remove")}
            </button>
          </li>
        ))}
        {options.data && options.data.length === 0 && (
          <li className="text-xs text-slate-400">{t("projects.customFields.noOptions")}</li>
        )}
      </ul>
      <div className="mt-1 flex items-center gap-2">
        <input
          aria-label={t("projects.customFields.newOptionAria")}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={t("projects.customFields.optionPlaceholder")}
          className="flex-1 rounded border border-slate-300 px-2 py-1 text-xs"
        />
        <button
          onClick={() => value && add.mutate(value)}
          disabled={!value || add.isPending}
          className="rounded bg-[#0052cc] px-2 py-1 text-xs text-white disabled:opacity-60"
        >
          {t("projects.customFields.addOption")}
        </button>
      </div>
    </div>
  );
}

export function CustomFieldsTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const t = useT();
  const [name, setName] = useState("");
  const [fieldType, setFieldType] = useState<CustomField["field_type"]>("text");
  const [required, setRequired] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);

  const fields = useQuery({
    queryKey: ["custom-fields", projectKey],
    queryFn: () => customFields.list(projectKey),
  });
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: ["custom-fields", projectKey] });

  const create = useMutation({
    mutationFn: () =>
      customFields.create(projectKey, {
        name,
        field_type: fieldType,
        required,
      }),
    onSuccess: () => {
      setName("");
      setFieldType("text");
      setRequired(false);
      invalidate();
    },
  });

  const del = useMutation({
    mutationFn: (fieldId: string) => customFields.remove(fieldId),
    onSuccess: invalidate,
  });

  return (
    <div className="space-y-6" data-testid="custom-fields-tab">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">{t("projects.customFields.title")}</h3>
        <ul className="space-y-1" data-testid="custom-fields-list">
          {(fields.data ?? []).map((field: CustomField) => (
            <li key={field.id} className="border-b border-slate-100 py-1.5 text-sm">
              <div className="flex items-center gap-2">
                <span className="text-[#1a1f36]">{field.name}</span>
                <span className="text-xs text-slate-400">
                  {isFieldType(field.field_type) ? t(`projects.customFields.types.${field.field_type}`) : field.field_type}
                </span>
                {field.required && (
                  <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700">
                    {t("projects.customFields.required")}
                  </span>
                )}
                <span className="ml-auto flex items-center gap-3">
                  {(field.field_type === "select" ||
                    field.field_type === "multiselect") && (
                    <button
                      onClick={() =>
                        setExpanded(expanded === field.id ? null : field.id)
                      }
                      className="text-xs text-[#0052cc] hover:underline"
                      aria-label={t("projects.customFields.manageOptionsAria", { name: field.name })}
                    >
                      {t("projects.customFields.options")}
                    </button>
                  )}
                  <button
                    onClick={() => del.mutate(field.id)}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={t("projects.customFields.deleteAria", { name: field.name })}
                  >
                    {t("projects.customFields.remove")}
                  </button>
                </span>
              </div>
              {expanded === field.id &&
                (field.field_type === "select" ||
                  field.field_type === "multiselect") && (
                  <OptionsManager fieldId={field.id} />
                )}
            </li>
          ))}
          {fields.data && fields.data.length === 0 && (
            <li className="py-2 text-sm text-slate-400">{t("projects.customFields.empty")}</li>
          )}
        </ul>
      </section>

      <section className="rounded-xl border border-slate-200 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-slate-700">{t("projects.customFields.newField")}</h3>
        <div className="flex flex-col gap-2 sm:flex-row">
          <input
            aria-label={t("projects.customFields.fieldName")}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("projects.customFields.fieldName")}
            className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <select
            aria-label={t("projects.customFields.fieldType")}
            value={fieldType}
            onChange={(e) =>
              setFieldType(e.target.value as CustomField["field_type"])
            }
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            {FIELD_TYPES.map((v) => (
              <option key={v} value={v}>
                {t(`projects.customFields.types.${v}`)}
              </option>
            ))}
          </select>
          <label className="flex items-center gap-1 text-sm text-slate-600">
            <input
              type="checkbox"
              aria-label={t("projects.customFields.required")}
              checked={required}
              onChange={(e) => setRequired(e.target.checked)}
            />
            {t("projects.customFields.required")}
          </label>
        </div>

        {create.isError && (
          <p className="text-sm text-red-600">
            {create.error instanceof Error
              ? create.error.message
              : t("projects.customFields.createFailed")}
          </p>
        )}

        <div className="flex justify-end">
          <button
            onClick={() => name && create.mutate()}
            disabled={!name || create.isPending}
            className="rounded bg-[#0052cc] px-4 py-1 text-sm text-white disabled:opacity-60"
          >
            {t("projects.customFields.addField")}
          </button>
        </div>
      </section>
    </div>
  );
}
