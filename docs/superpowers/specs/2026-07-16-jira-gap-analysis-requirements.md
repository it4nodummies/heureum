# Gap analysis vs Jira Cloud — Requirements per i prossimi round

> **Data**: 2026-07-16
> **Ambito**: aggiorna `docs/superpowers/specs/2026-07-10-jira-parity-roadmap-design.md` (Round 7 in poi) alla luce
> dello stato reale del codice oggi, dopo le release 1.0.0-1.0.2 e le fix di questa sessione
> (transizioni workflow, drag&drop backlog).

## Nota metodologica

Non ho accesso diretto alla sessione Chrome dell'utente su Jira Cloud in questa sessione (sono in
esecuzione come processo CLI in background, senza strumenti di controllo browser) — a differenza di
quanto previsto dal design roadmap originale ("riferimento visivo: istanza reale via sessione
Chrome"). Questo documento si basa invece su:

1. `jira-opensource-spec.md` — l'analisi diretta di Jira Cloud già fatta il 13/05/2026 (istanza
   `harpaitalia.atlassian.net`), con classificazione 🟢 FONDAMENTALE / 🟡 IMPORTANTE / 🔴 NON
   NECESSARIO già stabilita e approvata per questo progetto.
2. Un audit diretto del codice attuale (letto file per file, non solo verificato "esiste la
   directory") per ogni area 🟡 ancora aperta secondo quello spec, per determinare lo stato reale:
   non iniziato / solo backend / parziale / completo.

Questo documento **non** ridiscute le priorità 🟢/🟡/🔴 già decise — le usa come sono. Copre solo gli
scostamenti tra "quello che lo spec dice dovrebbe esserci" e "quello che c'è davvero nel codice oggi".

## Il pattern dominante: backend pronto, UI mai collegata

Il fatto più rilevante emerso dall'audit — e coerente con **tutti e tre** i bug reali sistemati in
questa sessione (transizioni workflow, drag&drop backlog, e altri trovati durante le review) — è che
**la maggior parte delle feature 🟡 mancanti non richiede nuovo lavoro di backend**: il dominio, il
service, l'handler e spesso persino il client API frontend esistono già, funzionano, e semplicemente
non sono mai stati collegati a una pagina o componente UI. Questo cambia radicalmente la stima di
sforzo rispetto a "costruire la feature da zero": il grosso del rischio/costo (schema dati, logica di
dominio, autorizzazione) è già stato pagato e verificato.

## Requirement 1 (Tier "cablaggio", rischio backend ≈ zero)

Ognuno di questi ha **già** dominio + service + handler + route funzionanti; manca solo la UI (o,
in un caso, una singola chiamata API frontend mai fatta).

### 1.1 Timeline / Roadmap
**Stato:** solo backend. `internal/domain/timeline/service.go:15-135` costruisce barre Gantt reali da
sprint ed epic (con calcolo progresso/colore) più header settimana/mese/trimestre; endpoint
`GET /rest/api/3/project/{projectID}/timeline` (`router.go:396`). **Zero** riferimenti a "timeline" in
tutto `frontend-next/` — nessuna pagina esiste.
**Requirement:** pagina Timeline sotto il progetto (tab accanto a Board/Backlog/Reports/Settings),
barre Gantt per epic/sprint, zoom settimana/mese/trimestre (i dati per lo zoom sono già calcolati
server-side).

### 1.2 Calendar
**Stato:** solo backend. `internal/domain/calendar/service.go:15-58` aggrega issue per giorno del
mese in base a due date; endpoint `GET /rest/api/3/project/{projectID}/calendar`
(`calendar_handler.go:19-30`). Zero riferimenti "calendar" nel frontend.
**Requirement:** vista calendario mensile per progetto, issue con due date visualizzate sui giorni
corrispondenti.

### 1.3 Burnup chart
**Stato:** parziale — unico caso di gap "quasi a costo zero". Backend completo e già registrato
(`report_handler.go:58-70`, `router.go:334`), ma `frontend-next/lib/api.ts`'s `reports` client
(righe 581-587) non chiama mai quell'endpoint e la pagina report non lo renderizza.
**Requirement:** aggiungere la chiamata client + il grafico Burnup alla pagina Reports esistente
(stesso pattern già usato per Burndown/Velocity/CFD — letteralmente copiare lo schema).

### 1.4 Automation rules — UI
**Stato:** solo backend, e persino il client frontend esiste già ma è morto. Rule engine completo:
CRUD regole, condizioni (priority/title-contains), azioni (set_assignee/add_label/transition/
comment), dispatcher su trigger, storico esecuzioni (`internal/domain/automation/service.go:18-223`).
`frontend-next/lib/api.ts:803-840` ha già `AutomationRule` + `automationRules()` — mai importato da
nessuna pagina.
**Requirement:** pagina "Automation" in Project Settings: lista regole, form per crearne (trigger →
condizione → azione), toggle attiva/disattiva, log delle esecuzioni.

### 1.5 Custom fields — UI
**Stato:** solo backend, e il frontend usa un finto custom field hardcoded al suo posto. CRUD field
completo con opzioni per select/multiselect e storage valori per issue
(`internal/domain/customfield/service.go:16-124`). Il frontend invece usa una chiave Jira-style
finta `customfield_10016` che in realtà è solo la colonna nativa `StoryPoints`
(`IssueView.tsx:49,94,223`, `lib/api.ts:84`) — non un vero custom field.
**Requirement:** tab "Custom fields" in Project Settings per creare/gestire field e opzioni; il form
di creazione/edit issue deve renderizzare i custom field configurati e permettere di impostarne il
valore.

### 1.6 Time tracking — UI
**Stato:** solo backend. Colonne `OriginalEstimate`/`TimeSpent` sull'issue
(`internal/domain/issue/model.go:43-44`) e sistema worklog completo (Add/List/Get/Delete,
`worklog_service.go:31-71` + handler). Zero riferimenti "worklog"/"time" nel frontend — `IssueView.tsx`
non ha nessuna UI per stima/log lavoro.
**Requirement:** sezione "Time tracking" nella vista issue: campi stima originale/rimanente, bottone
"Log work" con lista dei worklog registrati.

### 1.7 Filtri condivisi — sblocco API
**Stato:** parziale, un caso interessante — il dominio supporta già la condivisione a livello di
query (`SavedFilter.IsShared`, query `owner_id = ? OR is_shared = ?` in
`internal/domain/search/saved_filter.go:17,32,57,115-117`), ma l'handler HTTP **hardcoda** `false`/`nil`
per quel parametro sia in creazione che in update (`filter_handler.go:78,113`) — la UI non può mai
impostarlo perché l'API non lo accetta.
**Requirement:** aggiungere `is_shared` al body di `POST/PATCH /filter`, e un toggle "Condividi con
il team" nella UI dei filtri esistente.

### 1.8 CSV export — bottone mancante (+ bugfix)
**Stato:** solo backend, con un bug di qualità dati. Endpoint funzionante
(`issue_handler.go:129-166`, `router.go:374`) ma nessun bottone "Export" da nessuna parte nel
frontend. **Bug**: scrive gli UUID grezzi di `StatusID`/`TypeID` invece dei nomi risolti
(righe 141-148) — va corretto insieme al cablaggio, altrimenti il CSV esportato è illeggibile.
**Requirement:** bottone "Export CSV" nella vista lista/ricerca issue; fix del bug di risoluzione
nomi nell'handler esistente.

### 1.9 Notification preferences — creazione righe mancanti
**Stato:** parziale. La UI in `profile/page.tsx:67-90` permette di modificare via_app/via_email solo
per righe di preferenza **già esistenti**; un utente nuovo (zero righe) vede solo un testo statico
"Default preferences" senza nessun controllo (`profile/page.tsx:86-88`), e l'endpoint
`UpdateSettings` non ha modo di creare una riga per un nuovo tipo di evento
(`notification_handler.go:70-92`).
**Requirement:** permettere di aggiungere una preferenza per un tipo di evento/progetto non ancora
configurato, non solo modificare quelle esistenti.

## Requirement 2 (Tier "nuovo lavoro", backend + frontend da costruire)

### 2.1 Bulk edit issue
**Stato:** non iniziato al di fuori del drag&drop backlog (che serve solo per assegnazione
sprint/ordinamento, non per bulk-edit di campi — `backlog/page.tsx:34,187,229-230,276`). La vista
lista/ricerca (`SearchResults.tsx`) è una tabella di sola lettura: le uniche checkbox servono per la
visibilità colonne, non per selezionare righe (righe 18-28, 56-61); nessuna toolbar di azioni bulk.
**Requirement:** checkbox di selezione riga nella vista lista, toolbar "N selezionate" con azioni
bulk (cambia stato, assegnatario, aggiungi label), endpoint backend per applicare un cambiamento a
un set di issue key in una sola chiamata.

### 2.2 Inline edit nella vista lista
**Stato:** non iniziato. Ogni cella è testo statico (`SearchResults.tsx:30-49`); modificare un campo
richiede di navigare alla issue e usare l'edit-mode a form intero di `IssueView.tsx` — non è
possibile editare status/assignee/priority direttamente dalla riga della lista.
**Requirement:** dropdown inline per status/assignee/priority direttamente nella tabella lista,
senza navigare alla issue detail.

### 2.3 Fix versions / Release
**Stato:** non iniziato, peggio di uno stub. Non esiste nessun dominio `version`/`release`, nessun
handler, nessuna UI. La colonna `Issue.VersionID` esiste nello schema
(`internal/domain/issue/model.go:41`) ma non è mai letta né scritta da nessuna parte del codice —
è debris dello schema, non una feature parziale.
**Requirement:** dominio `internal/domain/version` (o `release`): CRUD versione per progetto
(nome, data di rilascio, flag "released"), assegnazione issue→versione, UI in Project Settings per
gestire le versioni e un selettore versione nel form issue. Decidere se riusare `VersionID` esistente
o introdurre una tabella pivot (per supportare più fix-version per issue, come Jira reale).

### 2.4 Teams (distinto da Groups)
**Stato:** `internal/domain/group` è solo un meccanismo di raggruppamento per permessi (nome +
membership piatta, `group/model.go:5-16`) — non esiste un concetto "Team" separato (nessuna vista
workload, nessuna distinzione team-vs-ruolo-progetto). Nessuna UI di gestione gruppi esiste nel
frontend nonostante l'handler admin-gated esista già (`group_handler.go`, `router.go:400-406`).
**Requirement (in due parti, valutare se prioritizzare solo la prima):**
  1. **UI mancante per Groups** (già costruito lato backend, stesso pattern del Tier 1 sopra) — bassa
     complessità.
  2. **Teams come concetto Jira-style** (workload view, capacity) — è 🔴 in `jira-opensource-spec.md`
     sezione 4.23 tranne "gestione team con membri" (🟡): richiede nuovo dominio. Consigliato:
     implementare solo la UI Groups (parte 1) in questo round, rinviare il concetto Team pieno.

## Requirement 3 (fuori scope, solo per completezza)

- **Components** (categorizzazione sub-progetto distinta dalle label): assente dal codice e non
  menzionato esplicitamente in `jira-opensource-spec.md` — non è né 🟢 né 🟡 nello spec originale.
  Non incluso nei requirement sopra; da considerare solo se l'utente lo richiede esplicitamente.
- Tutte le feature 🔴 di `jira-opensource-spec.md` (Advanced Roadmaps, LDAP/SAML, permessi granulari
  per singola operazione, Forms pubblici, Wiki/Confluence, Goals/OKR, Standups, ecc.) restano
  esplicitamente fuori scope, come da decisione originale del progetto.

## Priorità consigliata

1. **Tutto il Requirement 1** prima — sono 9 feature "quasi pronte", rischio bassissimo, alto valore
   percepito (l'utente probabilmente le userà tutte prima di notare cosa manca in Requirement 2), e
   lo stesso schema di lavoro già rodato in questa sessione (individua endpoint pronto → costruisci
   la UI → e2e test → review). Stimabile come 1-2 round nello stile già usato dal progetto.
2. **Requirement 2.1 + 2.2** (bulk edit + inline edit nella lista) — stesso valore percepito alto,
   ma richiede nuovo lavoro backend (endpoint bulk-update) oltre alla UI.
3. **Requirement 2.3** (Fix versions) — dominio nuovo di dimensioni contenute, utile per chi fa
   release tracking.
4. **Requirement 2.4 parte 1** (solo UI Groups) — piccolo, isolato, stesso pattern del Tier 1.
5. **Requirement 2.4 parte 2** (Teams pieno) — solo se richiesto esplicitamente, è ai margini tra 🟡
   e 🔴 nello spec originale.

## Prossimo passo

Questo documento non include ancora un piano di implementazione dettagliato (task-by-task) — per lo
stesso motivo per cui questo progetto lavora a round: 9+ feature insieme sono troppe per un singolo
piano eseguibile. Il passo successivo è scegliere **quale fetta** (round) pianificare in dettaglio
per primo — la raccomandazione è iniziare dal Requirement 1 nella sua interezza, dato il rischio
quasi nullo e il fatto che chiude esattamente il Round 7 già previsto dalla roadmap originale
(`docs/superpowers/specs/2026-07-10-jira-parity-roadmap-design.md`), mai completato sul fronte UI.
