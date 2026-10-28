"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import { boards } from "@/lib/api";
import { ScrumBacklog } from "./ScrumBacklog";
import { KanbanBacklog } from "./KanbanBacklog";

// Branch on the board type (boardsApi.get returns `type`): a scrum board keeps
// the sprint-based backlog; a kanban board gets a flat, rank-ordered to-do
// backlog with no sprint UI (sprint creation on a kanban board is rejected by
// the API — R26 Task 2).
export default function BacklogPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });

  // Wait for the board before choosing a layout, so we never briefly mount the
  // scrum view (and fire its sprint queries) for a kanban board.
  if (!board.data) {
    return (
      <div className="mx-auto max-w-3xl p-8 text-sm text-slate-400">Loading backlog…</div>
    );
  }

  return board.data.type === "kanban" ? <KanbanBacklog boardId={id} /> : <ScrumBacklog boardId={id} />;
}
