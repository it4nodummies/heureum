"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { projects as projectsApi, ProjectTypeKey } from "@/lib/api";

interface Props {
  onClose: () => void;
  onCreated: () => void;
}

type TemplateKey = "scrum" | "kanban" | "business";

const TEMPLATES: Record<
  TemplateKey,
  { projectTypeKey: ProjectTypeKey; projectTemplateKey: string; label: string; description: string; icon: React.ReactNode }
> = {
  scrum: {
    projectTypeKey: "software",
    projectTemplateKey: "com.pyxis.greenhopper.jira:gh-scrum-template",
    label: "Scrum",
    description: "Sprints, backlog, burndown charts",
    icon: (
      <svg viewBox="0 0 32 32" fill="none" className="w-8 h-8">
        <rect width="32" height="32" rx="6" fill="#E8F4FD" />
        <path d="M8 22l4-8 4 4 4-6 4 4" stroke="#0052cc" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    ),
  },
  kanban: {
    projectTypeKey: "software",
    projectTemplateKey: "com.pyxis.greenhopper.jira:gh-kanban-template",
    label: "Kanban",
    description: "Continuous flow, WIP limits",
    icon: (
      <svg viewBox="0 0 32 32" fill="none" className="w-8 h-8">
        <rect width="32" height="32" rx="6" fill="#F0FFF4" />
        <rect x="7" y="9" width="6" height="10" rx="2" fill="#22c55e" />
        <rect x="15" y="9" width="6" height="7" rx="2" fill="#22c55e" opacity=".6" />
        <rect x="23" y="9" width="2" height="14" rx="1" fill="#22c55e" opacity=".3" />
      </svg>
    ),
  },
  business: {
    projectTypeKey: "business",
    projectTemplateKey: "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control",
    label: "Business",
    description: "Task tracking, no sprints",
    icon: (
      <svg viewBox="0 0 32 32" fill="none" className="w-8 h-8">
        <rect width="32" height="32" rx="6" fill="#FFF7ED" />
        <rect x="8" y="10" width="16" height="2" rx="1" fill="#f97316" />
        <rect x="8" y="15" width="12" height="2" rx="1" fill="#f97316" opacity=".7" />
        <rect x="8" y="20" width="8" height="2" rx="1" fill="#f97316" opacity=".4" />
      </svg>
    ),
  },
};

const TEMPLATE_ORDER: TemplateKey[] = ["scrum", "kanban", "business"];

export default function CreateProjectModal({ onClose, onCreated }: Props) {
  const queryClient = useQueryClient();
  const [step, setStep] = useState<1 | 2>(1);
  const [template, setTemplate] = useState<TemplateKey>("scrum");
  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [description, setDescription] = useState("");
  const [keyTouched, setKeyTouched] = useState(false);

  const { mutate, isPending, error } = useMutation({
    mutationFn: () =>
      projectsApi.create({
        key,
        name,
        description,
        projectTypeKey: TEMPLATES[template].projectTypeKey,
        projectTemplateKey: TEMPLATES[template].projectTemplateKey,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
      onCreated();
    },
  });

  function derivedKey(n: string) {
    return n
      .toUpperCase()
      .replace(/[^A-Z0-9]/g, "")
      .slice(0, 10);
  }

  function handleNameChange(val: string) {
    setName(val);
    if (!keyTouched) setKey(derivedKey(val));
  }

  function handleCreate() {
    mutate();
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm">
      <div className="bg-white rounded-2xl shadow-2xl shadow-slate-300/40 w-full max-w-lg overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100">
          <h2 className="text-base font-semibold text-[#1a1f36]">
            {step === 1 ? "Select project type" : "Create project"}
          </h2>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
              <path
                fillRule="evenodd"
                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                clipRule="evenodd"
              />
            </svg>
          </button>
        </div>

        <div className="p-6">
          {step === 1 ? (
            <>
              <p className="text-sm text-slate-500 mb-4">
                Choose the methodology that fits your team's workflow.
              </p>
              <div className="space-y-2">
                {TEMPLATE_ORDER.map((tpl) => {
                  const pt = TEMPLATES[tpl];
                  return (
                    <button
                      key={tpl}
                      onClick={() => setTemplate(tpl)}
                      className={`flex items-center gap-4 w-full p-4 rounded-xl border-2 text-left transition-all ${
                        template === tpl
                          ? "border-[#0052cc] bg-[#e8f0fe]"
                          : "border-slate-100 hover:border-slate-200 hover:bg-slate-50"
                      }`}
                    >
                      {pt.icon}
                      <div>
                        <p className="text-sm font-semibold text-[#1a1f36]">{pt.label}</p>
                        <p className="text-xs text-slate-500">{pt.description}</p>
                      </div>
                      <div className="ml-auto">
                        <div
                          className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                            template === tpl
                              ? "border-[#0052cc] bg-[#0052cc]"
                              : "border-slate-300"
                          }`}
                        >
                          {template === tpl && (
                            <div className="w-1.5 h-1.5 rounded-full bg-white" />
                          )}
                        </div>
                      </div>
                    </button>
                  );
                })}
              </div>
              <div className="flex justify-end mt-6">
                <button
                  onClick={() => setStep(2)}
                  className="px-5 py-2 bg-[#0052cc] hover:bg-[#0065ff] text-white text-sm font-semibold rounded-lg transition-colors"
                >
                  Next
                </button>
              </div>
            </>
          ) : (
            <>
              {error && (
                <div className="mb-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-lg px-4 py-3">
                  {error instanceof Error ? error.message : "Failed to create project"}
                </div>
              )}
              <div className="space-y-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5">
                    Project name <span className="text-red-400">*</span>
                  </label>
                  <input
                    type="text"
                    value={name}
                    onChange={(e) => handleNameChange(e.target.value)}
                    placeholder="My awesome project"
                    className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all"
                    autoFocus
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5">
                    Project key <span className="text-red-400">*</span>
                  </label>
                  <input
                    type="text"
                    value={key}
                    onChange={(e) => {
                      setKeyTouched(true);
                      setKey(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 10));
                    }}
                    placeholder="MAP"
                    className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all font-mono"
                  />
                  <p className="text-xs text-slate-400 mt-1">
                    2–10 uppercase letters/numbers. Used in issue identifiers (e.g. MAP-1).
                  </p>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5">
                    Description
                  </label>
                  <textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="What is this project about?"
                    rows={3}
                    className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all resize-none"
                  />
                </div>
              </div>

              <div className="flex items-center justify-between mt-6">
                <button
                  onClick={() => setStep(1)}
                  className="px-4 py-2 text-sm font-medium text-slate-500 hover:text-slate-700 transition-colors"
                >
                  ← Back
                </button>
                <button
                  onClick={handleCreate}
                  disabled={!name || !key || isPending}
                  className="px-5 py-2 bg-[#0052cc] hover:bg-[#0065ff] disabled:opacity-50 text-white text-sm font-semibold rounded-lg transition-colors"
                >
                  {isPending ? "Creating…" : "Create project"}
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
