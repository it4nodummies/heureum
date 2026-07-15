# Riordino status/colonne board — Design Document

> **Data:** 2026-07-15

---

## Problema

Il pannello "Workflow" nelle impostazioni di progetto (`WorkflowEditor.tsx`, raggiungibile da
Project Settings → tab Workflow) è l'unico punto in cui si gestiscono gli status del workflow, e
le colonne della board sono generate 1:1 da quegli status ordinati per `position`
(`agile_board_handler.go` `Configuration()`). Oggi:

1. Un nuovo status va sempre in coda (`AddStatus` in `internal/domain/workflow/service.go` imposta
   `Position = MAX(position) + 1`), quindi non si può inserire una colonna prima di "Done".
2. Il backend supporta già il riordino (`ReorderStatuses`, endpoint
   `PUT /rest/api/3/project/{key}/workflow/statuses/order`, client già pronto in
   `lib/api.ts` come `workflow.reorderStatuses`) ma **nessuna UI lo chiama** — la lista status è una
   `<ul>` statica senza drag&drop né altri controlli d'ordine.
3. `GetWorkflow` (`service.go`) fa `Preload("Statuses")` senza `Order`, quindi l'ordine mostrato nel
   pannello non è garantito coincidere con l'ordine reale delle colonne board.
4. La combobox "New status category" (`todo`/`inprogress`/`done`) viene scambiata per un
   selettore di posizione/colonna, ma imposta in realtà solo un tag semantico (`Category`) usato
   altrove (es. auto-set della resolution quando si entra in uno status categoria "done").

## Decisioni (confermate con l'utente)

- Riordino via **drag & drop** (non frecce su/giù).
- Nessuna modifica al form di creazione status: un nuovo status nasce sempre in coda; per
  posizionarlo prima di "Done" lo si trascina dopo la creazione. Nessun cambiamento a `AddStatus`
  o al suo endpoint.
- La combobox categoria resta con la logica attuale; si chiarisce solo label/testo d'aiuto.

## Design

### 1. Fix backend — ordinamento stabile (`internal/domain/workflow/service.go`)

`GetWorkflow` usa `Preload("Statuses", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") })`
invece di `Preload("Statuses")` semplice, così l'ordine restituito da
`GET /rest/api/3/project/{key}/workflow` coincide sempre con `position` (e quindi con l'ordine
delle colonne board). Nessun altro chiamante di `GetWorkflow` viene impattato (uso confermato via
grep: solo risposte in lettura).

### 2. Frontend — drag & drop nel pannello Workflow (`WorkflowEditor.tsx`)

`@dnd-kit/sortable` è già una dipendenza del frontend (`package.json`) ma non ancora usata da
nessun componente — è il pacchetto adatto per una lista verticale riordinabile (diverso da
`@dnd-kit/core` puro già usato in `BoardColumns.tsx` per il drag delle card tra colonne).

- Sostituire la `<ul>` statica (righe 40-55 di `WorkflowEditor.tsx`) con `DndContext` +
  `SortableContext` (strategy verticale) e un sotto-componente riga che usa `useSortable`.
- `onDragEnd` calcola il nuovo array ordinato di ID status e chiama una nuova mutation che invoca
  `workflow.reorderStatuses(projectKey, newOrderIds)` (client già presente in `lib/api.ts`, mai
  chiamato finora).
- Optimistic update: aggiornare subito l'ordine nella cache di React Query
  (`qc.setQueryData(["workflow", projectKey], ...)`) per un riordino percepito come istantaneo,
  poi invalidare/rifare fetch per allinearsi al server; su errore, rollback all'ordine precedente.
- Mantenere il bottone "Remove" esistente sulla riga.

### 3. Chiarire la combobox categoria

- Rinominare la label da "New status category" a qualcosa come "Category (reporting only)".
- Aggiungere una riga di testo d'aiuto sotto il form, tipo: *"La categoria è usata per report e
  per impostare automaticamente la resolution — non determina l'ordine della colonna. Trascina gli
  status nella lista per riordinare le colonne."*
- Nessuna modifica di logica/dati.

### 4. Test

- **Backend**: nuovo test in `internal/domain/workflow/service_test.go` che crea più status, chiama
  `ReorderStatuses` con un ordine diverso da quello di creazione, e verifica che `GetWorkflow`
  restituisca gli status nel nuovo ordine.
- **E2E**: estendere `frontend-next/e2e/workflow.spec.ts` con un test che trascina uno status in una
  nuova posizione (drag di un elemento con `data-testid` dedicato) e verifica, dopo un reload della
  pagina, che l'ordine sia persistito (via testid o contenuto testuale della lista in ordine).

## Fuori scope

- Inserimento di un nuovo status in una posizione arbitraria già in fase di creazione (deciso:
  fuori scope, si copre con drag&drop post-creazione).
- Colonne board indipendenti dagli status del workflow (mapping N:1, `ConstraintType`, ecc.) — non
  richiesto, resta il modello 1 status = 1 colonna.
- Modifiche alla logica di `Category` (transizioni, auto-resolution): solo chiarimento testuale in
  UI.
