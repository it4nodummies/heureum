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
insieme al fetch del backlog. Da questi dati deriviamo (via `useMemo`) un `serverItems:
Record<string, string[]>` (containerId → array ordinato di issue key, containerId = `"backlog"` o
`` `sprint-${sprintId}` ``) e un `issuesByKey: Record<string, SearchIssue>` (dati di rendering per
ogni issue key). `serverItems` seeda/risincronizza lo stato locale mutabile `items` (sezione 2),
che è la fonte di verità usata per il rendering e per il drag.

### 2. Drag & drop — anteprima live cross-container (pattern "multiple containers" di dnd-kit)

Lo stato `items: Record<string, string[]>` (containerId → issue key) nel padre è la **fonte di
verità durante l'interazione**: viene risincronizzato dai dati server via `useEffect` ogni volta che
le query cambiano, ma **solo quando non è in corso un drag** (guardia su `activeId === null`), così
il riordino live non viene sovrascritto a metà trascinamento.

Le card trascinate sono trattate sempre come un **blocco** (`draggedKeys`): se l'item sotto il
cursore fa parte della selezione corrente, `draggedKeys` = tutte le key selezionate; altrimenti solo
quell'item. Questo permette di generalizzare la logica di anteprima a singolo-item e multi-item con
lo stesso codice, invece di gestirli come due casi separati.

`onDragOver(event)`: determina il container di destinazione (`overContainer`, da `over.id` — se è
un containerId è drop su area vuota/fine lista, altrimenti si risale al container che contiene
quella issue key) e l'indice di inserimento al suo interno; rimuove `draggedKeys` da ovunque si
trovino in `items` e li reinserisce come blocco contiguo nella posizione calcolata in
`overContainer`. Questo fa sì che le altre card si spostino visivamente per fare spazio mentre
trascini, sia dentro la stessa lista sia tra liste diverse.

`onDragEnd(event)`: a questo punto `items` riflette già la disposizione finale (grazie agli
aggiornamenti live di `onDragOver`) — non serve ricalcolare nulla da zero:
1. `targetContainer` = il container che ora contiene `draggedKeys` in `items`.
2. `sourceContainer` = il container di partenza, catturato in un ref a `onDragStart`.
3. Se `sourceContainer !== targetContainer`: chiama `sprints.moveIssues` (destinazione sprint) o
   `agileIssues.moveToBacklog` (destinazione backlog).
4. I vicini immediati del blocco in `items[targetContainer]` (esclusi i `draggedKeys` stessi)
   diventano `rankBeforeIssue`/`rankAfterIssue`; chiama `agileIssues.rank` se almeno uno dei due
   esiste (lista di destinazione non vuota oltre al blocco).
5. Invalida le query di backlog + sprint issues coinvolte (l'effetto di risincronizzazione
   riconcilia `items` con i dati server, che ora coincidono con quanto appena inviato); azzera
   `activeId` e la selezione.

Un `DragOverlay` mostra la card trascinata (con contatore se sono selezionate più issue) mentre il
cursore si muove. Ogni lista è una `SortableContext` (i cui `items` sono `items[containerId]`, la
stessa fonte di verità locale — non i dati server grezzi) più un `useDroppable` sul container
(necessario perché una lista vuota non ha item su cui agganciare il drop).

Ogni riga issue (`IssueRow`) guadagna un drag handle dedicato (pattern identico a
`StatusRow`/`⠿` in `WorkflowEditor.tsx`) separato dalla checkbox, così cliccare la checkbox non
avvia un drag.

### 3. Selezione multipla

Stato `selected: Set<string>` nel padre. Al drop: se `active.id` (la issue trascinata) è nella
selezione corrente, il payload sono tutte le key selezionate; altrimenti solo quella issue.

### 4. Test

- E2E in `frontend-next/e2e/board.spec.ts`: drag singola issue da Backlog a Sprint 1 (già seedato,
  vuoto); creazione di un secondo sprint via UI esistente + drag Sprint 1 → Sprint 2; selezione
  multipla (2 issue) + drag di gruppo verso uno sprint; riordino interno al backlog via drag tra due
  posizioni.

## Fuori scope

- Un bottone "Move to..." alternativo al drag (la selezione multipla serve solo ad accompagnare il
  drag, non introduce un'azione bulk separata da menu).
- Modifiche backend: tutti gli endpoint necessari esistono già e funzionano.
