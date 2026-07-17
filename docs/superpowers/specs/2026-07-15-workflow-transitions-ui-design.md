# Creazione manuale delle transizioni di workflow + feedback su drag fallito — Design Document

> **Data:** 2026-07-15

---

## Problema

Aggiungere un nuovo status tramite il pannello Workflow ("Add status") crea solo la riga
`WorkflowStatus` — non crea nessuna `WorkflowTransition` verso o dagli status vicini. Il drag&drop
sulla board chiama `POST /issue/{key}/transitions {status_id}` (`app/app/boards/[boardId]/page.tsx:39-43`
→ `lib/api.ts` `issues.transition`), che il backend valida contro le transizioni esistenti
(`internal/api/handlers/workflow_handler.go:293-304` → `GetAvailableTransitions`,
`internal/domain/workflow/service.go:115-121`). Se non esiste una `WorkflowTransition` dallo status
corrente verso quello di destinazione, il backend risponde `400 {"invalid transition"}`.

La mutation `move` in `page.tsx` non ha un `onError`, quindi l'errore viene ingoiato silenziosamente:
la card torna semplicemente indietro senza nessun messaggio.

Il backend ha già pronto tutto il necessario per risolvere lato dati:
- `POST /rest/api/3/project/{key}/workflow/transitions` (`router.go:249`, handler `AddTransition`)
- `DELETE /rest/api/3/project/{key}/workflow/transitions/{id}` (`router.go:252`, handler `DeleteTransition`)
- Client frontend già scritti e mai chiamati: `workflow.addTransition` e `workflow.deleteTransition`
  (`lib/api.ts:523-540`)

Ma `WorkflowEditor.tsx` (righe 154-167) mostra le transizioni solo in sola lettura — nessun form per
crearle o rimuoverle.

## Decisioni (confermate con l'utente)

- Creazione transizioni **manuale** tramite form (no creazione automatica tra status adiacenti).
- Aggiungere anche il **feedback visibile** quando un drag fallisce per transizione mancante.
- Aggiungere anche la possibilità di **rimuovere** una transizione esistente (simmetrico agli status).

## Design

### 1. Form "Add transition" in `WorkflowEditor.tsx`

Nella sezione "Transitions", sotto la lista esistente:
- Due `<select>`: "From status" e "To status", entrambi popolati da `statuses` (value = id, label =
  nome), con `aria-label` distinti (`"From status"`, `"To status"`).
- Un `<input>` opzionale per il nome (`aria-label="Transition name"`).
- Due checkbox: "Require assignee" e "Set resolution" (`aria-label` uguali al testo).
- Bottone "Add transition" → mutation che chiama
  `workflow.addTransition(projectKey, { from_status_id, to_status_id, name, require_assignee, set_resolution })`,
  poi invalida la query `["workflow", projectKey]`.
- Ogni riga della lista transizioni esistente guadagna un bottone "Remove"
  (`aria-label="Delete transition {computed label}"`) → `workflow.deleteTransition(projectKey, id)`,
  poi invalida la stessa query.

Nessuna modifica backend: gli endpoint esistono e sono già testati a livello di routing/contract.

### 2. Feedback su drag fallito (`app/app/boards/[boardId]/page.tsx`)

Aggiungere `onError` alla mutation `move`, che imposta uno stato locale `moveError` col messaggio
d'errore (già estratto da `apiFetch`, es. `"invalid transition"`). Renderizzare un banner
(`data-testid="move-error"`, `role="alert"`) sopra `BoardColumns` quando `moveError` è impostato, con
testo `Move failed: {moveError}` e un bottone di chiusura (`aria-label="Dismiss error"`) che lo
azzera. `onSuccess` azzera `moveError` a `null` (nel caso un drag riuscito segua uno fallito).

Nessuna modifica backend: il messaggio arriva già formattato da `apiFetch`.

### 3. Test

- **E2E** (`workflow.spec.ts`): nuovo test che aggiunge due status custom, crea una transizione tra
  loro tramite il nuovo form, verifica che compaia nella lista transizioni, poi la rimuove e verifica
  che scompaia.
- **E2E** (`board.spec.ts`): nuovo test che aggiunge un nuovo status via API/setup (o tramite UI) senza
  creare transizioni verso di esso, tenta un drag di una card verso quella colonna sulla board, e
  verifica che compaia il banner d'errore (`data-testid="move-error"`) con testo che include
  `"invalid transition"`.

## Fuori scope

- Creazione automatica di transizioni tra status adiacenti alla creazione/riordino.
- Modifica del messaggio d'errore backend (resta il testo grezzo `"invalid transition"`, solo
  reso visibile invece che ingoiato).
- Editing dei campi `require_assignee`/`set_resolution` di una transizione esistente (endpoint
  `UpdateTransition` già esiste ma non richiesto ora — solo add/remove).
