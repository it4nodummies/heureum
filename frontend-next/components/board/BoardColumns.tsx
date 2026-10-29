"use client";

import { DndContext, DragEndEvent, useDraggable, useDroppable } from "@dnd-kit/core";
import type { SearchIssue } from "@/lib/api";
import type { EpicRef } from "@/lib/epicHierarchy";
import { epicColorHex } from "@/lib/epicColors";
import { EpicLozenge } from "@/components/issues/EpicLozenge";

// Una colonna mappa un SET di status (statusIds). `id` è lo status PRIMARIO
// (statuses[0]) usato come target della transizione quando ci si droppa sopra.
export interface Column {
  id: string;
  name: string;
  statusIds: string[];
}

export type Swimlane = "none" | "assignee" | "epic";

// The epic an issue belongs to (lib/epicHierarchy.ts epicOf: a story's parent,
// a subtask's story's parent), used for the card lozenge and the epic swimlane.
export type EpicResolver = (issue: SearchIssue) => EpicRef | null;
const noEpic: EpicResolver = () => null;

// Separatore per gli id dei droppable nelle swimlane: `${bandKey}␞${col.id}`.
// @dnd-kit richiede id droppable univoci, e la stessa colonna compare in ogni
// banda; l'id colonna (=status primario) va comunque estratto per la transizione.
const SEP = "␞";

// Bucketizza le issue nelle colonne per status.id (match per ID, non per nome):
// una issue finisce nella colonna il cui set di status contiene il suo status.id.
function bucketByColumn(issues: SearchIssue[], columns: Column[]): Record<string, SearchIssue[]> {
  const map: Record<string, SearchIssue[]> = {};
  for (const col of columns) map[col.id] = [];
  for (const iss of issues) {
    const sid = iss.fields.status?.id;
    if (!sid) continue;
    const col = columns.find((c) => c.statusIds.includes(sid));
    if (col) map[col.id].push(iss);
  }
  return map;
}

// Card issue trascinabile. The epic lozenge is a plain chip (no link): the
// whole card carries the drag listeners, so a link would fight the drag.
function IssueCard({ issue, epic }: { issue: SearchIssue; epic: EpicRef | null }) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: issue.key });
  const style = transform ? { transform: `translate(${transform.x}px, ${transform.y}px)` } : undefined;
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className="mb-2 cursor-grab rounded border border-slate-200 bg-white p-2 text-sm shadow-sm"
      data-testid={`card-${issue.key}`}
    >
      <div className="font-mono text-xs text-slate-500">{issue.key}</div>
      <div className="text-[#1a1f36]">{issue.fields.summary}</div>
      {epic && (
        <div className="mt-1">
          <EpicLozenge name={epic.name} colorKey={epic.colorKey} issueKey={epic.key} testId={`epic-lozenge-${issue.key}`} />
        </div>
      )}
    </div>
  );
}

// Colonna droppabile. `dropId` è l'id @dnd-kit del droppable (col.id in modalità
// flat, band-scoped nelle swimlane); `col.name` resta il testid stabile.
function ColumnBox({
  col,
  dropId,
  issues,
  resolveEpic,
}: {
  col: Column;
  dropId: string;
  issues: SearchIssue[];
  resolveEpic: EpicResolver;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: dropId });
  return (
    <div
      ref={setNodeRef}
      className={`w-64 shrink-0 rounded bg-slate-100 p-2 ${isOver ? "ring-2 ring-[#0052cc]" : ""}`}
      data-testid={`column-${col.name}`}
    >
      <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500">
        {col.name} <span className="text-slate-400">({issues.length})</span>
      </h3>
      {issues.map((iss) => (
        <IssueCard key={iss.key} issue={iss} epic={resolveEpic(iss)} />
      ))}
    </div>
  );
}

// Una riga di colonne (il layout classico del board). In modalità flat gli id
// droppable sono `col.id`; con `bandKey` sono `${bandKey}␞${col.id}`.
function ColumnRow({
  columns,
  issuesByColumn,
  bandKey,
  resolveEpic,
}: {
  columns: Column[];
  issuesByColumn: Record<string, SearchIssue[]>;
  bandKey?: string;
  resolveEpic: EpicResolver;
}) {
  return (
    <div className="flex gap-3 overflow-x-auto p-2">
      {columns.map((col) => (
        <ColumnBox
          key={col.id}
          col={col}
          dropId={bandKey !== undefined ? `${bandKey}${SEP}${col.id}` : col.id}
          issues={issuesByColumn[col.id] ?? []}
          resolveEpic={resolveEpic}
        />
      ))}
    </div>
  );
}

interface Swimband {
  key: string;
  label: string;
  // Epic swimlane only: the epic's palette key for the band header swatch.
  colorKey?: string;
}

// Band key of an issue: assignee accountId, or the resolved epic key (a
// subtask lands in its story's epic band, not in a band keyed by the story).
function bandKeyOf(iss: SearchIssue, mode: Swimlane, resolveEpic: EpicResolver): string {
  if (mode === "assignee") return iss.fields.assignee?.accountId ?? "none";
  return resolveEpic(iss)?.key ?? "none";
}

// Calcola le bande di swimlane a partire dalle issue visibili. La banda
// "senza valore" (key "none") va in fondo; le altre ordinate per etichetta.
function computeBands(issues: SearchIssue[], mode: Swimlane, resolveEpic: EpicResolver): Swimband[] {
  const bands = new Map<string, Swimband>();
  for (const iss of issues) {
    const key = bandKeyOf(iss, mode, resolveEpic);
    if (bands.has(key)) continue;
    if (mode === "assignee") {
      bands.set(key, { key, label: iss.fields.assignee?.displayName ?? "No assignee" });
    } else {
      const epic = resolveEpic(iss);
      // "KEY · Name" keeps the key visible next to the epic name.
      bands.set(key, epic ? { key, label: `${epic.key} · ${epic.name}`, colorKey: epic.colorKey } : { key, label: "No epic" });
    }
  }
  const out = Array.from(bands.values());
  out.sort((a, b) => {
    if (a.key === "none") return 1;
    if (b.key === "none") return -1;
    return a.label.localeCompare(b.label);
  });
  return out;
}

function issuesForBand(issues: SearchIssue[], mode: Swimlane, bandKey: string, resolveEpic: EpicResolver): SearchIssue[] {
  return issues.filter((iss) => bandKeyOf(iss, mode, resolveEpic) === bandKey);
}

export function BoardColumns({
  columns,
  issues,
  swimlane = "none",
  onMove,
  resolveEpic = noEpic,
}: {
  columns: Column[];
  issues: SearchIssue[];
  swimlane?: Swimlane;
  onMove: (issueKey: string, toStatusId: string) => void;
  // Optional: the issue's epic (card lozenge + epic swimlane); omitted → none.
  resolveEpic?: EpicResolver;
}) {
  // In entrambe le modalità l'id droppable termina con l'id colonna (= status
  // primario): in flat è l'id stesso, nelle swimlane è la parte dopo il SEP.
  const handleDragEnd = (e: DragEndEvent) => {
    const issueKey = String(e.active.id);
    if (!e.over) return;
    const raw = String(e.over.id);
    const sepIdx = raw.indexOf(SEP);
    const toStatusId = sepIdx >= 0 ? raw.slice(sepIdx + SEP.length) : raw;
    onMove(issueKey, toStatusId);
  };

  if (swimlane === "none") {
    const issuesByColumn = bucketByColumn(issues, columns);
    return (
      <DndContext onDragEnd={handleDragEnd}>
        <ColumnRow columns={columns} issuesByColumn={issuesByColumn} resolveEpic={resolveEpic} />
      </DndContext>
    );
  }

  const bands = computeBands(issues, swimlane, resolveEpic);
  return (
    <DndContext onDragEnd={handleDragEnd}>
      <div className="space-y-4">
        {bands.map((band) => {
          const bandIssues = issuesForBand(issues, swimlane, band.key, resolveEpic);
          const issuesByColumn = bucketByColumn(bandIssues, columns);
          return (
            <section
              key={band.key}
              data-testid={`swimlane-${band.key}`}
              className="rounded border border-slate-200"
            >
              <div className="flex items-center gap-2 border-b border-slate-200 bg-slate-50 px-3 py-1.5 text-xs font-semibold text-slate-600">
                {swimlane === "epic" && band.key !== "none" && (
                  <span
                    aria-hidden
                    className="h-2.5 w-2.5 shrink-0 rounded-sm"
                    style={{ backgroundColor: epicColorHex(band.colorKey) }}
                  />
                )}
                <span>
                  {band.label} <span className="text-slate-400">({bandIssues.length})</span>
                </span>
              </div>
              <ColumnRow columns={columns} issuesByColumn={issuesByColumn} bandKey={band.key} resolveEpic={resolveEpic} />
            </section>
          );
        })}
      </div>
    </DndContext>
  );
}
