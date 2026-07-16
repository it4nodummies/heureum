# Backlog: drag & drop + selezione multipla verso gli sprint — Design Document

> **Data:** 2026-07-16

---

## Problema

La pagina Backlog (`frontend-next/app/app/boards/[boardId]/backlog/page.tsx`) non ha nessuna
interattività sulle issue: `IssueRow` (righe 8-15) renderizza solo chiave e summary in un `<div>` —
niente checkbox, niente bottone "sposta in sprint", niente drag handle, nessun `DndContext`. Non è
possibile spostare una issue dal backlog a uno sprint (o viceversa, o tra sprint diversi) né
riordinarla, se non manualmente via API.

Il backend e il client API sono invece già pronti e funzionanti (mai chiamati dal frontend):
- `sprints.moveIssues(sprintId, issues[])` → `POST /rest/agile/1.0/sprint/{sprintId}/issue` →
  `AgileSprintHandler.MoveToSprint` (`agile_sprint_handler.go:184-208`) → `sprintSvc.AddIssue` —
  accetta già un array (bulk).
- `agileIssues.moveToBacklog(issues[])` → `POST /rest/agile/1.0/backlog/issue` →
  `AgileMiscHandler.MoveToBacklog` (`agile_misc_handler.go:92-121`) → `sprintSvc.RemoveIssue`.
- `agileIssues.rank(issues[], rankBeforeIssue?, rankAfterIssue?)` → `PUT /rest/agile/1.0/issue/rank`
  → `AgileMiscHandler.Rank` (`agile_misc_handler.go:43-89`) → `issueSvc.Rank` (posizione a
  midpoint, non LexoRank — `internal/domain/issue/service.go:298-...`).

## Decisioni (confermate con l'utente)

- Drag & drop **e** selezione multipla (checkbox), non solo l'uno o l'altro.
- Spostamenti supportati: Backlog ↔ Sprint, **e** Sprint ↔ Sprint direttamente (non a due passaggi
  via backlog).
- Riordino interno a una lista (backlog o sprint) incluso in questo giro, via `agileIssues.rank`.
- Trascinando una issue che fa parte della selezione corrente, **tutte** le issue selezionate si
  spostano insieme nella stessa destinazione (un'unica chiamata API per tipo di operazione).

## Design

### 1. Stato condiviso nel componente padre

`SprintSection` oggi incapsula il proprio `useQuery` per le issue dello sprint. Per un drag
cross-container serve che backlog + tutti gli sprint vivano in un'unica fonte dati nel padre
(`BacklogPage`): `useQueries` (TanStack Query) per il fetch dinamico delle issue di ogni sprint,
insieme al fetch del backlog. Da questi dati deriviamo (via `useMemo`):
- `itemsByContainer: Record<string, string[]>` — mappa containerId → array ordinato di issue key
  (containerId = `"backlog"` o `` `sprint-${sprintId}` ``).
- `issuesByKey: Record<string, SearchIssue>` — dati di rendering per ogni issue key.
- `keyToContainer: Record<string, string>` — dove si trova oggi ogni issue key.

### 2. Drag & drop — nota implementativa importante

**Niente anteprima live cross-container durante il drag** (niente `onDragOver` che sposta
otticamente gli item tra le liste mentre trascini). Le liste sono renderizzate direttamente dai
dati server (via i memo sopra); un `DragOverlay` mostra la card trascinata (con contatore se sono
selezionate più issue) mentre il cursore si muove. Il calcolo di destinazione (quale container,
quale posizione) avviene **solo in `onDragEnd`**, leggendo `over.id`: se corrisponde a un
containerId è un drop su area vuota/fine lista; se corrisponde a una issue key, il container e
l'indice si derivano da `keyToContainer`/`itemsByContainer`. Dopo il drop si chiamano le mutation
(vedi sotto) e si invalida la query — le liste si "assestano" nella posizione finale quando i dati
freschi arrivano, invece di riordinarsi live durante il trascinamento.

Perché: un'anteprima live multi-container richiederebbe uno stato locale mutabile sincronizzato
con tre-o-più query dinamiche (rischio concreto di bug di sincronizzazione, specialmente con
`useQueries` su un array di sprint di lunghezza variabile) per un beneficio puramente estetico. Il
comportamento risultante (drop → breve fetch → assestamento) è comunque fluido e resta pienamente
funzionale su tutte le capacità richieste (spostamento cross-container, riordino interno,
spostamento di gruppo). Ogni lista resta comunque una `SortableContext` (per l'affordance visiva di
drag/animazione dei singoli item) più un `useDroppable` sul container (necessario perché una lista
vuota non ha item su cui agganciare il drop).

Ogni riga issue (`IssueRow`) guadagna un drag handle dedicato (pattern identico a
`StatusRow`/`⠿` in `WorkflowEditor.tsx`) separato dalla checkbox, così cliccare la checkbox non
avvia un drag.

### 3. Selezione multipla

Stato `selected: Set<string>` nel padre. Al drop: se `active.id` (la issue trascinata) è nella
selezione corrente, il payload sono tutte le key selezionate; altrimenti solo quella issue.

### 4. Commit del drop (`onDragEnd`)

1. Calcola `sourceContainer` (da `keyToContainer`) e `targetContainer`/`targetIndex` (da `over`).
2. Se `sourceContainer !== targetContainer`: chiama `sprints.moveIssues` (destinazione = sprint) o
   `agileIssues.moveToBacklog` (destinazione = backlog).
3. Calcola i vicini (`rankBeforeIssue`/`rankAfterIssue`) nella lista di destinazione all'indice di
   drop (escludendo le key trascinate se già presenti) e chiama `agileIssues.rank` se almeno uno dei
   due vicini esiste (lista di destinazione non vuota).
4. Invalida le query di backlog + sprint issues coinvolte; azzera la selezione.

### 5. Test

- E2E in `frontend-next/e2e/board.spec.ts`: drag singola issue da Backlog a Sprint 1 (già seedato,
  vuoto); creazione di un secondo sprint via UI esistente + drag Sprint 1 → Sprint 2; selezione
  multipla (2 issue) + drag di gruppo verso uno sprint; riordino interno al backlog via drag tra due
  posizioni.

## Fuori scope

- Anteprima live del riordino durante il drag (vedi nota sopra).
- Un bottone "Move to..." alternativo al drag (la selezione multipla serve solo ad accompagnare il
  drag, non introduce un'azione bulk separata da menu).
- Modifiche backend: tutti gli endpoint necessari esistono già e funzionano.
