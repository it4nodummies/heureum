"use client";

import { use, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, sprints, type AgileSprint, type SearchIssue } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

function IssueRow({ issue }: { issue: SearchIssue }) {
  return (
    <div className="flex items-center gap-2 border-b border-slate-100 py-1 text-sm" data-testid={`row-${issue.key}`}>
      <span className="font-mono text-xs text-slate-500">{issue.key}</span>
      <span className="text-[#1a1f36]">{issue.fields.summary}</span>
    </div>
  );
}

export default function BacklogPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();
  const [newSprint, setNewSprint] = useState("");

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const backlog = useQuery({ queryKey: ["board", id, "backlog"], queryFn: () => boards.backlog(id) });
  const sprintList = useQuery({ queryKey: ["board", id, "sprints"], queryFn: () => boards.sprints(id) });
  const projectKey = board.data?.location?.projectKey;

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["board", id, "backlog"] });
    qc.invalidateQueries({ queryKey: ["board", id, "sprints"] });
  };

  const createSprint = useMutation({
    mutationFn: (name: string) => sprints.create(name, id),
    onSuccess: () => {
      setNewSprint("");
      invalidate();
    },
  });
  const setState = useMutation({
    mutationFn: ({ sprintId, state }: { sprintId: number; state: "active" | "closed" }) =>
      sprints.setState(sprintId, state),
    onSuccess: invalidate,
  });

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="backlog" />}
      <div className="mx-auto max-w-3xl p-4">
        {/* sprint */}
        {sprintList.data?.values.map((sp: AgileSprint) => (
          <SprintSection
            key={sp.id}
            sprint={sp}
            onState={(state) => setState.mutate({ sprintId: sp.id, state })}
          />
        ))}

        {/* crea sprint */}
        <div className="my-3 flex gap-2">
          <input
            aria-label="New sprint name"
            value={newSprint}
            onChange={(e) => setNewSprint(e.target.value)}
            placeholder="Sprint name"
            className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
          />
          <button
            onClick={() => newSprint && createSprint.mutate(newSprint)}
            className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
            disabled={createSprint.isPending}
          >
            Create sprint
          </button>
        </div>

        {/* backlog: usiamo un div (non un heading) per il conteggio così l'unico
            heading con testo "Backlog" resta il tab link nell'header condiviso
            (evita ambiguità con getByRole("heading", { name: /Backlog/i }) nell'E2E). */}
        <div className="mb-1 mt-4 text-sm font-semibold text-slate-500">
          Backlog ({backlog.data?.issues.length ?? 0})
        </div>
        <div>
          {backlog.data?.issues.map((iss) => <IssueRow key={iss.key} issue={iss} />)}
          {backlog.data && backlog.data.issues.length === 0 && (
            <p className="py-2 text-sm text-slate-400">Backlog is empty</p>
          )}
        </div>
      </div>
    </div>
  );
}

function SprintSection({
  sprint,
  onState,
}: {
  sprint: AgileSprint;
  onState: (state: "active" | "closed") => void;
}) {
  const issues = useQuery({ queryKey: ["sprint", sprint.id, "issues"], queryFn: () => sprints.issues(sprint.id) });
  return (
    <div className="mb-3 rounded border border-slate-200 p-2" data-testid={`sprint-${sprint.id}`}>
      <div className="mb-1 flex items-center justify-between">
        <span className="text-sm font-semibold text-[#1a1f36]">
          {sprint.name} <span className="text-xs font-normal text-slate-400">({sprint.state})</span>
        </span>
        <span className="flex gap-2">
          {sprint.state === "future" && (
            <button
              onClick={() => onState("active")}
              className="rounded border border-slate-300 px-2 py-0.5 text-xs"
            >
              Start sprint
            </button>
          )}
          {sprint.state === "active" && (
            <button
              onClick={() => onState("closed")}
              className="rounded border border-slate-300 px-2 py-0.5 text-xs"
            >
              Complete sprint
            </button>
          )}
        </span>
      </div>
      {issues.data?.issues.map((iss) => <IssueRow key={iss.key} issue={iss} />)}
      {issues.data && issues.data.issues.length === 0 && (
        <p className="py-1 text-xs text-slate-400">No issues in this sprint</p>
      )}
    </div>
  );
}
