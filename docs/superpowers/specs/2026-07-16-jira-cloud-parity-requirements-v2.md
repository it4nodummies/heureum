# Heureum ↔ Jira Cloud — Requirements di parità (v2, dettagliati e azionabili)

> **Data:** 2026-07-16
> **Metodo:** esplorazione **live** di Jira Cloud (istanza `harpaitalia.atlassian.net`, sessione Chrome dell'utente — navigazione in sola lettura di: home "For you", board scrum, backlog, vista issue, List, Timeline, Calendar, Releases, Reports, Summary, Filters, Dashboards, Space settings, modale Create) **incrociata** con un audit file-per-file dello stato reale di Heureum (post 1.0.2). Sostituisce/estende `2026-07-16-jira-gap-analysis-requirements.md`.
> **Obiettivo:** replicare le funzionalità di Jira Cloud su Heureum. Questo documento è la base da cui derivare i piani di sviluppo per round.
> **Perché "v2":** la v1 era troppo generica ("aggiungi la pagina Timeline") e questo ha portato a feature realizzate a metà. Qui **ogni feature ha acceptance criteria numerati, concreti e testabili**, il gap backend/UI esatto con evidenza `file:line`, e gli endpoint/model già esistenti da riusare. Un piano di sviluppo deve poter nascere da una sezione senza altre indagini.

---

## 0. Come leggere questo documento

Ogni feature è classificata per **stato reale** (non "esiste la cartella"):
- **CABLAGGIO** 🔌 — dominio + service + handler + route backend già esistenti e funzionanti; manca SOLO la UI (o una singola chiamata client). Rischio backend ≈ 0.
- **ESTENDI** 🧩 — parzialmente presente; va completato (backend + UI).
- **NUOVO** 🏗️ — da costruire lato backend e frontend.
- **FUORI SCOPO** ⛔ — 🔴 nello spec originale, non replicato.

E per **priorità** ereditata da `jira-opensource-spec.md`: 🟢 FONDAMENTALE / 🟡 IMPORTANTE / 🔴 NON NECESSARIO.

**Regola trasversale (vale per ogni requisito UI):** ogni nuova pagina/sezione va (a) integrata nell'header/navigazione coerente già esistente (`ProjectHeader` per le viste progetto, sidebar per le globali), (b) coperta da almeno un test E2E Playwright che esercita il flusso end-to-end contro il backend reale, (c) soggetta all'enforcement permessi del Round 11 (le nuove rotte mutanti passano da `chk.Enforce`, le letture da `EnforceNotFound`).

---

## FASE A — Tier CABLAGGIO 🔌 (backend pronto, UI mai collegata)

Questi sono il grosso del gap percepito e il rischio più basso. Ordine consigliato di implementazione = questa sezione.

### A.1 — Vista issue completa: sotto-sezioni mancanti 🟢

La vista dettaglio issue di Jira è il cuore del prodotto. Heureum ha già l'header/breadcrumb, il pannello Details editabile (summary/description/priority/labels/story points) e i commenti, ma mancano **quattro sotto-sezioni** i cui backend sono completi.

#### A.1.1 Subtask (sottotask)
- **Stato:** BACKEND-ONLY. `issue.Service.Create` accetta `parentID`, `GetChildren` esiste (`internal/domain/issue/service.go:40,268`); model `ParentID`/`IsSubtask` (`model.go:24,39`); v3 serializza `parent`. Nessuna UI.
- **Jira osservato:** nella issue, sezione "Subtasks" con "Add subtask" inline (riga rapida: titolo → invio crea un subtask figlio); ogni subtask mostra key, checkbox stato, titolo, assignee; barra di progresso "N di M completati".
- **Requisiti (acceptance criteria):**
  1. La vista issue mostra una sezione **Subtasks** sotto la Description, sempre visibile (con empty state "Add subtask" se nessuno).
  2. Un campo inline permette di digitare un titolo e creare un subtask (tipo `Subtask`, `parentID` = issue corrente) con una sola chiamata `POST /issue`.
  3. Ogni subtask in lista mostra: icona tipo, key (link a `/app/browse/KEY`), titolo, dropdown stato inline (transizione), avatar assignee.
  4. Barra/contatore progresso "X di Y completati" (Y = totale subtask, X = subtask in status-category `done`).
  5. Il client `issues.create` deve accettare `parentId`; la lista subtask usa `GET /issue/{key}` figli (o una query JQL `parent = KEY`).
- **UI:** nuova sezione in `IssueView.tsx` + `issues.create({parentId})` nel client.
- **Backend:** nessun lavoro (verificare solo che il create via body accetti `fields.parent.key`).

#### A.1.2 Linked work items (collegamenti tra issue) 🟢
- **Stato:** BACKEND-ONLY. `IssueLink`/`LinkType` (`model.go:68-82`), handler + `POST /issueLink`, `DELETE /issueLink/{id}` (`router.go:237-243`). Nessun metodo in `api.ts`, nessuna UI.
- **Jira osservato:** sezione "Linked work items", "Add linked work item" → si sceglie il **tipo di relazione** (is blocked by / blocks / relates to / duplicates / is duplicated by / clones …) e la issue target (autocomplete per key/titolo); i link si raggruppano per tipo di relazione; ogni riga mostra la issue linkata (key, stato, titolo) con "x" per rimuovere.
- **Requisiti:**
  1. Sezione **Linked work items** nella issue, con "Add linked work item".
  2. Form: dropdown tipo relazione (i `LinkType` disponibili dal backend), campo target con autocomplete (ricerca issue per key/summary, riusa `/user/assignable` style o `/search/jql`).
  3. I link esistenti sono raggruppati per tipo relazione, con la direzione corretta (inward/outward label distinte, es. "blocks" vs "is blocked by").
  4. Ogni link mostra key+stato+titolo (link cliccabile) e un'azione rimuovi (`DELETE /issueLink/{id}`).
  5. Client `issueLinks.create({type, inwardKey, outwardKey})` + `issueLinks.delete(id)`; la GET issue deve esporre `fields.issuelinks` (verificare il mapping v3 — se manca è un piccolo fix backend, altrimenti serve una fetch dedicata).
- **UI:** sezione in `IssueView.tsx` + metodi client.
- **Backend:** verificare che `GET /issue/{key}` renda `fields.issuelinks`; se no, aggiungerlo al mapper (piccolo).

#### A.1.3 Attachments (allegati) 🟡
- **Stato:** BACKEND-ONLY. `IssueAttachment` (`model.go:111`), `attachment_service.go`, upload/get/serve/delete (`router.go:227-235`), storage su disco (`APP_UPLOADS_DIR`). Nessun metodo client, nessuna UI.
- **Jira osservato:** area "Attachments" con drag&drop e bottone upload; griglia di thumbnail/file con nome, dimensione, data, autore; click per aprire/scaricare; menu per eliminare. "+" nella toolbar issue include "Attachment".
- **Requisiti:**
  1. Sezione **Attachments** nella issue (o integrata nel "+"): area drag&drop + input file.
  2. Upload via `POST /issue/{key}/attachments` (multipart); durante l'upload, indicatore di avanzamento.
  3. Lista allegati: nome file, dimensione leggibile, data, autore, con link di download (`GET /attachment/content/{id}`) e azione elimina (`DELETE /attachment/{id}`).
  4. Anteprima immagine inline per i tipi image/*; icona generica per gli altri.
  5. Client `attachments.upload(key, file)` / `list` / `delete`.
- **UI:** sezione in `IssueView.tsx` + client multipart.
- **Backend:** nessun lavoro (già implementato in R12).

#### A.1.4 Time tracking / Worklog 🟡
- **Stato:** BACKEND-ONLY. `worklog_service.go` (Add/List/Get/Delete), routes (`router.go:215-217`), issue `OriginalEstimate`/`TimeSpent` (`model.go:43-44`), v3 emette `timetracking`. Nessuna UI.
- **Jira osservato:** pannello Details → "Time tracking": barra stima vs speso, "No time logged" iniziale; azione "Log work" → dialog con tempo speso (formato `2w 3d 4h 30m`), data inizio, tempo rimanente (auto/manuale), descrizione; campi "Original estimate" e "Remaining estimate" editabili.
- **Requisiti:**
  1. Nel pannello Details della issue, blocco **Time tracking**: barra di avanzamento speso/stima + testo (es. "3h logged / 8h estimated"), o "No time logged".
  2. Campi **Original estimate** e **Remaining estimate** editabili (formato Jira `Xw Yd Zh Wm`; parser/formatter lato client o backend — verificare se il backend già parsa il formato o accetta secondi).
  3. Bottone **Log work** → dialog: tempo speso (obbligatorio), data, descrizione → `POST /issue/{key}/worklog`.
  4. Lista worklog registrati (autore, tempo, data, descrizione) con azione elimina (`DELETE .../worklog/{id}`).
  5. Client `worklogs.add/list/delete`; verificare l'unità attesa dal backend (secondi vs stringa) e adattare il parser.
- **UI:** blocco in `IssueView.tsx` + dialog + client.
- **Backend:** verificare formato tempo accettato; eventuale helper di parsing `Xw Yd Zh Wm` ↔ secondi se assente.

#### A.1.5 Per-issue changelog / History 🟡
- **Stato:** BACKEND-ONLY. `IssueHistory` (`model.go:84`), `GET /issue/{key}/changelog` (`router.go:202`). Nessuna UI.
- **Jira osservato:** nella issue, tab **Activity** con selettore "Comments | History | Work log"; History = elenco cronologico "X ha cambiato Campo da A a B" con timestamp.
- **Requisiti:**
  1. Nella sezione Activity della issue, aggiungere il tab **History** accanto a Comments.
  2. Render cronologico degli eventi da `GET /issue/{key}/changelog`: autore, campo, da→a, timestamp relativo.
  3. (Se worklog implementato) tab **Work log** che mostra i worklog.
  4. Client `issues.changelog(key)`.
- **UI:** estendere la barra tab Activity in `IssueView.tsx`/`Comments.tsx`.

### A.2 — Timeline / Roadmap (vista progetto) 🟡 🔌
- **Stato:** BACKEND-ONLY. `internal/domain/timeline` costruisce barre Gantt (epic/sprint, progresso, header settimana/mese/trimestre), `GET /project/{projectID}/timeline` (`router.go:396`). Zero UI.
- **Jira osservato:** righe = epic (espandibili ai figli, checkbox); corsie in alto **Sprints** (pill Sprint N sull'asse temporale) e **Releases**; asse mesi con linea "oggi"; zoom **Today / Weeks / Months / Quarters**; filtri search/avatars/Epic/Type/More; drag delle barre per cambiare date (avanzato).
- **Requisiti:**
  1. Tab **Timeline** nell'header di progetto (accanto a Board/Backlog/Reports/Settings).
  2. Colonna sinistra "Work" con le epic del progetto, espandibili per mostrare le issue figlie.
  3. Area destra: barre Gantt per ogni epic/issue posizionate sull'asse temporale in base alle date; corsia **Sprints** con i marker degli sprint; corsia **Releases** (se versioni implementate — vedi C.1).
  4. Selettore zoom **Weeks / Months / Quarters** (i dati per lo zoom sono già calcolati server-side — verificare i parametri dell'endpoint).
  5. Linea verticale "oggi".
  6. Read-only nella prima versione (il drag-per-cambiare-date è un'estensione successiva, richiede update date via `PUT /issue`).
  7. Client `timeline.get(projectId)`.
- **UI:** nuova pagina `app/app/projects/[key]/timeline` (o sotto board) + rendering SVG/CSS delle barre (dependency-free, come i grafici report).
- **Backend:** nessun lavoro per la versione read-only.

### A.3 — Calendar (vista progetto) 🟡 🔌
- **Stato:** BACKEND-ONLY. `internal/domain/calendar` aggrega issue per giorno su due date, `GET /project/{projectID}/calendar` (`router.go:397`). Zero UI.
- **Jira osservato:** griglia mensile; sprint come bande colorate multi-giorno; issue con due date mostrate sui giorni; pannello destro **Unscheduled work** con lista draggabile sui giorni per schedulare (imposta due date); filtri Assignee/Type/Status/More; nav Today/‹mese›; selettore Month.
- **Requisiti:**
  1. Tab **Calendar** nell'header di progetto.
  2. Griglia mensile con navigazione mese precedente/successivo + "Today".
  3. Issue con due date renderizzate sul giorno corrispondente (chip con key+titolo, colore per tipo/stato); sprint come bande orizzontali multi-giorno.
  4. Pannello laterale **Unscheduled work** (issue senza due date) — nella prima versione può essere read-only; il drag-to-schedule (imposta due date via `PUT /issue`) è estensione successiva.
  5. Filtri per assignee/type/status.
  6. Client `calendar.get(projectId, month)`.
- **UI:** nuova pagina `app/app/projects/[key]/calendar` + griglia calendario.
- **Backend:** nessun lavoro per read-only.

### A.4 — Automation rules (UI) 🟡 🔌
- **Stato:** BACKEND-ONLY. Rule engine completo: trigger (`issue_created/updated/transitioned`), condizioni (priority/title_contains), azioni (set_assignee/add_label/transition/add_comment), dispatcher event-driven, storico esecuzioni (`internal/domain/automation`). `api.ts:840` ha `automationRules()` mai chiamato. Nessuna UI.
- **Jira osservato:** Project settings → Automation: lista regole (nome, stato attivo, ultimo run), "Create rule" con builder trigger→condizione→azione, toggle enable/disable, audit log delle esecuzioni.
- **Requisiti:**
  1. Tab **Automation** in Project Settings (accanto a General/Workflow/Summary/Integrations).
  2. Lista regole: nome, tipo trigger, stato (attiva/disattiva con toggle → `PATCH /automation/{id}`), azione elimina.
  3. Form crea/modifica regola: selettore trigger, blocco condizioni (campo+operatore+valore, i tipi supportati dal backend), blocco azioni (tipo azione + parametri) → `POST /project/{projectID}/automation`.
  4. Vista **audit/log esecuzioni** per regola (`GET /automation/{id}/runs`): timestamp, esito, issue coinvolta.
  5. Client `integrations.automation*` (list già presente → aggiungere create/update/delete/runs).
- **UI:** nuovo componente tab in `ProjectSettings.tsx` + form builder + client.
- **Backend:** nessun lavoro (verificare che condizioni/azioni disponibili siano esposte via un endpoint meta o hardcodabili).

### A.5 — Custom fields (UI) 🟡 🔌
- **Stato:** BACKEND-ONLY. CRUD field + 6 tipi (text/number/date/select/multiselect/user), opzioni, valori per-issue (`internal/domain/customfield`), routes (`router.go:376-386`). Il frontend hardcoda solo `customfield_10016`=story points.
- **Jira osservato (istanza reale):** custom field in uso ("ID PROGETTO" required, "Posizione"); i field appaiono sia nel modale Create (con `*` se obbligatori) sia nel pannello Details della issue; gestiti da Project settings → Fields (per work type).
- **Requisiti:**
  1. Tab **Custom fields** in Project Settings: lista field del progetto; crea field (nome, tipo tra i 6, obbligatorio sì/no, e per select/multiselect la gestione opzioni) → CRUD via routes esistenti.
  2. Il **modale Create issue** rende dinamicamente i custom field configurati per il progetto/tipo, con validazione dei required, e li invia nel create.
  3. La **vista issue** (pannello Details) mostra i custom field valorizzati ed editabili (`PUT /issue/{id}/custom-values/{fieldId}` o il canale esistente).
  4. Rendering per tipo: text→input, number→input numerico, date→date picker, select→dropdown (opzioni), multiselect→multi, user→user picker.
  5. Client `customFields.list/create/delete/options` + lettura/scrittura valori.
- **UI:** tab settings + rendering dinamico in CreateIssueModal e IssueView.
- **Backend:** nessun lavoro; distinguere chiaramente lo story-point nativo dal sistema custom-field reale (oggi confusi).

### A.6 — Burnup report 🟡 🔌
- **Stato:** BACKEND-ONLY (unico gap quasi a costo zero). Endpoint `burnup` registrato (`report_handler.go`, `router.go:334`); il client `reports` non lo chiama, la pagina non lo rende.
- **Requisiti:**
  1. Aggiungere `reports.burnup(...)` al client (stesso schema di burndown).
  2. Renderizzare il grafico Burnup nella pagina Reports esistente (riusare la primitiva SVG StackedArea/Line).
  3. Fix collaterale: la pagina reports hardcoda `boards.sprints(1)` (`reports/page.tsx:26`) — deve usare lo sprint/board del progetto corrente.
- **UI/Backend:** solo cablaggio client + grafico.

### A.7 — CSV export (bottone) 🟡 🔌 + bugfix
- **Stato:** BACKEND-ONLY. `GET /project/{key}/issues/export` (`router.go:374`), nessun bottone UI. **Bug** noto (v1): scriveva UUID grezzi di status/type invece dei nomi.
- **Requisiti:**
  1. Bottone **Export CSV** nella vista List/ricerca issue e/o nella pagina Filters.
  2. Verificare/fixare che il CSV contenga i **nomi** risolti di status/type/priority/assignee, non gli UUID.
  3. Client `issues.exportCsv(key/jql)` (download file).
- **UI:** bottone + download; **Backend:** fix risoluzione nomi nell'handler export.

### A.8 — Gestione membri/ruoli progetto (UI) 🟢 🔌
- **Stato:** BACKEND-ONLY per la gestione. Enforcement attivo (R11); endpoint membri esistono (`router.go:178-181`: list/add/remove/invite) ma **nessuna UI**. Anche i gruppi (admin) hanno handler ma nessuna UI.
- **Jira osservato:** Project settings → Access/People: lista membri con ruolo, aggiungi persona (ruolo), rimuovi; a livello globale gestione gruppi.
- **Requisiti:**
  1. Tab **Access / People** in Project Settings: lista membri (avatar, nome, ruolo admin/member/viewer), cambia ruolo, rimuovi (`DELETE /project/{key}/members/{userId}`), aggiungi membro (user search + ruolo → `POST /project/{key}/members`).
  2. (Globale, admin) pagina **Groups**: lista gruppi, crea/elimina, aggiungi/rimuovi utenti (`/group*`).
  3. Client `projects.members.*` + `groups.*`.
- **UI:** tab settings + pagina groups; **Backend:** nessun lavoro.

### A.9 — Filtri condivisi (sblocco) 🟡 🔌
- **Stato:** BACKEND-ONLY. `SavedFilter.IsShared` supportato in query, ma il create/update non lo espone (v1 hardcodava false) e la UI non ha toggle.
- **Jira osservato:** i filtri hanno **Viewers** ed **Editors** separati (Private / progetto+ruolo / gruppo).
- **Requisiti (versione pragmatica al modello attuale):**
  1. `POST/PUT /filter` accettano `is_shared` nel body (rimuovere l'hardcode).
  2. Toggle **"Condividi con il team"** nella UI dei filtri; nella lista, colonna che indica shared/privato.
  3. (Estensione, se si vuole parità piena) introdurre viewers/editors granulari — attualmente il backend ha solo un bool `is_shared`; documentare come gap se non prioritario.
- **UI:** toggle + colonna; **Backend:** togliere hardcode `is_shared`.

### A.10 — Dashboard gadget: aggiungi/rimuovi/disponi 🟡 🔌
- **Stato:** PARTIAL. Dashboard list/create/view ok; solo 2 gadget hydratati (`assigned_to_me`, `activity_stream`); le route add/remove widget esistono (`router.go:345-355`) ma il client non le chiama e non c'è UI per aggiungere gadget.
- **Jira osservato:** dashboard con gadget configurabili (Assigned to me, Activity stream, Filter results, Pie chart, Created vs Resolved, Sprint burndown…), layout a colonne, aggiungi gadget da catalogo, rimuovi, riordina.
- **Requisiti:**
  1. Nella vista dashboard: bottone **Add gadget** → catalogo dei gadget supportati; aggiunta via `POST /dashboard/{id}/gadget`.
  2. Rimozione gadget (`DELETE .../gadget/{gadgetId}`); (estensione) riordino/layout.
  3. Rendering tipizzato per ogni gadget supportato (non il dump JSON attuale per i tipi non gestiti).
  4. Client `dashboards.addGadget/removeGadget`.
- **UI:** estendere `dashboards/[id]/page.tsx`; **Backend:** verificare i tipi gadget disponibili (estendere se servono Filter results / Pie / Created-vs-Resolved come gadget).

---

## FASE B — Tier ESTENDI 🧩 (parziale, completare backend+UI)

### B.1 — Vista List: bulk edit + inline edit 🟢 🏗️(backend)+🧩(UI)
- **Stato:** NOT-STARTED per bulk/inline. `SearchResults.tsx` è tabella read-only (checkbox solo per visibilità colonne).
- **Jira osservato:** List con gerarchia espandibile (epic→figli), checkbox riga per selezione multipla, toolbar bulk ("N selezionate": cambia stato/assegnatario/label/sprint, elimina), **inline edit** delle celle (status/assignee/priority dropdown direttamente in riga), aggiungi colonna, "N of M" + paginazione.
- **Requisiti:**
  1. **Inline edit**: nelle celle status/assignee/priority della lista, dropdown che applica la modifica senza aprire la issue (`PUT /issue` o transizione per lo status).
  2. **Selezione riga**: checkbox per riga + "seleziona tutto"; barra azioni "N selezionate".
  3. **Bulk action**: cambia stato/assignee/label/sprint su tutte le selezionate; endpoint backend **nuovo** `POST /issues/bulk` che applica un set di modifiche a una lista di key in una transazione (o loop server-side con report esiti parziali).
  4. **Gerarchia** espandibile epic→figli nella lista (come Jira).
  5. Colonne configurabili (già presenti in parte via filters page) + "N of M" con paginazione reale.
- **Backend:** nuovo endpoint bulk-update; **UI:** rework `SearchResults.tsx`.

### B.2 — Sprint: goal + date + gestione completa 🟢 🧩
- **Stato:** PARTIAL. Create/start/complete/multiple/drag ok; il model sprint ha `Goal/StartDate/EndDate` ma la UI di creazione manda solo il nome.
- **Jira osservato:** creazione/edit sprint con **nome, goal, data inizio, data fine, durata**; "Complete sprint" chiede dove spostare le issue non completate (backlog o sprint successivo).
- **Requisiti:**
  1. Form crea/edit sprint con nome, **goal**, **start/end date** (date picker) → `PATCH /sprints/{id}`.
  2. All'avvio sprint, impostazione date; alla chiusura, **dialog "sposta le N issue non completate"** (backlog o prossimo sprint).
  3. Il goal dello sprint mostrato sulla board (header sprint).
- **Backend:** verificare che start/complete gestiscano lo spostamento issue; **UI:** form esteso in backlog page.

### B.3 — Board: configurazione colonne/swimlane/quick filter 🟢 🏗️(backend)+🧩(UI)
- **Stato:** PARTIAL/read-only. Colonne derivate 1:1 dagli status del workflow; il model Board non memorizza config colonne/swimlane/quickfilter.
- **Jira osservato:** Board settings: **mappare più status in una colonna**, aggiungere/rimuovere/riordinare colonne, impostare min/max per colonna; **swimlane** (per epic/assignee/nessuna); **quick filters** (JQL salvate come chip sopra la board); **card layout** (campi mostrati sulle card).
- **Requisiti:**
  1. Backend: estendere il dominio board con configurazione colonne (colonna = nome + set di status), swimlane, quick filters.
  2. UI Board settings (o tab): CRUD colonne + mapping status→colonna (drag), scelta swimlane, definizione quick filter (nome + JQL).
  3. Board rende le colonne configurate (non 1:1 status), le swimlane e i chip quick-filter.
- **Backend:** estensione modello + migrazione; **UI:** editor board + rendering. (Il più corposo del tier B.)

### B.4 — Editor issue: campi completi nel Create e nel Details 🟢 🧩
- **Stato:** PARTIAL. Create modale = project+type+summary (+picker); Details editabile = summary/description/priority/labels/story points. Mancano nel create: description, priority, assignee, parent, custom field; manca assignee-edit user-picker nella issue.
- **Jira osservato:** modale Create ricco (status, custom required, story points, assignee "Automatic", labels, parent, sprint, description slash-commands); nella issue tutti i campi editabili.
- **Requisiti:**
  1. Modale Create: aggiungere description, priority, assignee (user picker), parent (per subtask/story sotto epic), sprint, e i custom field (vedi A.5).
  2. Vista issue: **assignee editabile** con user-picker (serve endpoint `/user/assignable/search` — già esistente, R12 lo ha reso membership-scoped) e "Assign to me".
  3. "Create another" checkbox nel modale (crea e riapre pulito), come Jira.
- **UI:** estendere CreateIssueModal + IssueView; **Backend:** nessun lavoro (assignable search già c'è).

### B.5 — Rich text editor (ADF) + @mentions 🟡 🧩
- **Stato:** PARTIAL. ADF **render-only**; input = textarea con conversione testo→ADF; niente @mentions.
- **Jira osservato:** editor WYSIWYG per description/commenti (bold/italic/liste/heading/code/link/quote/tabelle/immagini), **@mention** utenti (autocomplete → notifica), slash-commands.
- **Requisiti:**
  1. Editor rich (toolbar minima: bold/italic/code/liste/heading/link/quote) per description e commenti, con output ADF valido.
  2. **@mention**: autocomplete utenti nel testo → salva menzione in ADF → genera notifica al menzionato (il dominio commenti già gestiva @menzioni in R3 — verificare e ricablare in UI).
  3. (Estensione) slash-commands.
- **UI:** componente editor (valutare libreria leggera vs custom); **Backend:** le @menzioni-commento esistono dal R3, verificare il parsing lato server.

### B.6 — Notification hub + email 🟡 🧩/🏗️
- **Stato:** PARTIAL. Bell in-app completa; preferenze per (utente,progetto,evento) modificabili solo se la riga esiste. **Email = non implementata** (esiste solo il flag `via_email`, nessun mailer).
- **Jira osservato:** campanella con tab **Direct / Watching**, raggruppamento, mark-read; **email** per eventi (assegnazione, commento, menzione…) con preferenze.
- **Requisiti:**
  1. Bell: tab **Direct / Watching**, "mark all read" (già presente), raggruppamento per issue.
  2. Preferenze notifiche: poter **aggiungere** una preferenza per un tipo evento/progetto non ancora configurato (oggi solo modifica righe esistenti — vedi v1 §1.9).
  3. **Email delivery** (🏗️ nuovo backend): wiring SMTP nel worker (config `SMTP_*` già documentata ma non letta), invio email sugli eventi con rispetto delle preferenze `via_email`. Richiede coda/worker (il worker esiste).
- **Backend:** implementare il mailer SMTP nel worker; **UI:** tab bell + gestione prefs.

### B.7 — Workflow editor: regole di transizione nel form 🟡 🧩
- **Stato:** PARTIAL. Editor add/delete/reorder status + add/delete transizioni ok; le regole `require_assignee`/`set_resolution` esistono nel tipo ma non sono esposte nel form.
- **Requisiti:**
  1. Nel form transizione dell'editor workflow, esporre i toggle **require_assignee** e **set_resolution** (backend già li supporta).
  2. (Estensione) condizioni/validatori/post-function aggiuntivi.
- **UI:** estendere WorkflowEditor; **Backend:** nessun lavoro per i due flag base.

### B.8 — Profilo utente: tema, lingua, avatar 🟡 🧩
- **Stato:** PARTIAL. Display name + timezone + prefs; `locale` supportato dal client ma non in UI; niente tema/lingua/avatar.
- **Requisiti:**
  1. Selettore **lingua/locale** (già supportato da `profile.update`).
  2. Selettore **tema** (light/dark) — richiede supporto tema nel frontend.
  3. **Avatar upload** (richiede storage, riusa il sistema attachment/uploads).
- **UI:** estendere profile page; **Backend:** avatar upload nuovo (piccolo, riusa uploads).

---

## FASE C — Tier NUOVO 🏗️ (da costruire, backend+frontend)

### C.1 — Releases / Versions (Fix versions) 🟡 🏗️
- **Stato:** NOT-STARTED. `VersionID` è colonna morta; nessun dominio/handler/UI.
- **Jira osservato:** tab **Releases**: tabella version (nome, status, progress bar, start date, release date, description), "Create release", filtro released/unreleased; sulla issue campo **Fix versions** (multi); Release report.
- **Requisiti:**
  1. Dominio `internal/domain/version`: CRUD versione per progetto (nome, descrizione, start date, release date, flag released), + associazione issue↔versione (valutare pivot per multi-fix-version come Jira).
  2. Endpoint v3 versioni (`/project/{key}/versions`, `/version/{id}`) e assegnazione su issue.
  3. Tab **Releases** in project header: tabella con progress (issue done/totali per versione), create/edit/release.
  4. Campo **Fix versions** nel pannello issue + nel create.
  5. La corsia **Releases** della Timeline (A.2) usa queste versioni.
- **Backend + UI + migrazione**: round dedicato.

### C.2 — Components 🟡 🏗️
- **Stato:** NOT-STARTED (né codice né menzione esplicita nello spec originale).
- **Jira osservato:** Components = categorizzazione sub-progetto (nome, descrizione, lead, default assignee); campo Components (multi) sulla issue; filtrabile.
- **Requisiti:**
  1. Dominio `component` per progetto (nome, descrizione, lead opzionale, default assignee opzionale).
  2. Campo **Components** (multi) su issue + create.
  3. Gestione in Project settings.
- **Nota priorità:** non esplicitamente 🟢/🟡 nello spec originale — **da confermare con l'utente** se includere.

### C.3 — Board estese / Company-managed vs Team-managed ⛔→🟡 (da valutare)
- Jira distingue progetti team-managed e company-managed con schemi condivisi (workflow/field/permission scheme riusabili tra progetti). Heureum è di fatto team-managed per progetto. La parità piena (schemi condivisi) è 🔴/estensione enterprise — **fuori scope** salvo richiesta.

---

## FASE D — FUORI SCOPO ⛔ (confermato, non replicato)

Dallo spec originale `jira-opensource-spec.md` e confermato: Advanced Roadmaps (plans multi-progetto), Goals/OKR, Teams con capacity/workload, LDAP/SAML/SSO, permission scheme granulari per singola operazione, Forms pubblici, Confluence/Docs/Whiteboards, Code/Security/Deployments (dev panel integrato con provider esterni oltre l'auto-commento già fatto), Rovo/AI, App marketplace. La **home "For you"** personale multi-progetto (worked-on/assigned/starred) è 🟡 opzionale — valutare una versione minima.

---

## Riepilogo priorità e proposta di round

| # | Round proposto | Contenuto | Tier | Priorità |
|---|---|---|---|---|
| R13 | **Issue view completa** | A.1 (subtask, links, attachments, worklog, history) + B.4 (campi create/assignee) | 🔌 + 🧩 | 🟢 |
| R14 | **Viste progetto mancanti** | A.2 Timeline + A.3 Calendar + A.6 Burnup + A.7 CSV export | 🔌 | 🟡 |
| R15 | **Configurabilità** | A.4 Automation UI + A.5 Custom fields UI + B.7 workflow rules | 🔌 + 🧩 | 🟡 |
| R16 | **Amministrazione & condivisione** | A.8 membri/ruoli/gruppi UI + A.9 filtri condivisi + A.10 gadget dashboard | 🔌 | 🟢/🟡 |
| R17 | **List produttiva** | B.1 bulk + inline edit + gerarchia lista | 🏗️+🧩 | 🟢 |
| R18 | **Board & Sprint pro** | B.3 board config (colonne/swimlane/quickfilter) + B.2 sprint goal/date | 🏗️+🧩 | 🟢 |
| R19 | **Releases** | C.1 versions/fix-versions + corsia Timeline | 🏗️ | 🟡 |
| R20 | **Editor & Notifiche** | B.5 rich editor + @mentions + B.6 email SMTP + notification hub | 🧩+🏗️ | 🟡 |
| R21 | **Rifiniture** | B.8 profilo (tema/lingua/avatar) + C.2 Components (se confermato) | 🧩+🏗️ | 🟡 |

**Ordine consigliato:** R13 → R14 → R15 → R16 prima (massimo valore, rischio quasi nullo: sono per la maggior parte cablaggi di backend già pronto e testato). R17-R18 poi (nuovo backend contenuto, alto valore). R19-R21 in coda.

**Metodo per ogni round:** `superpowers:writing-plans` (un piano dettagliato per round, non più di 1 fetta) → `superpowers:subagent-driven-development` con gate a tre livelli. Ogni feature del piano deve avere gli acceptance criteria di questo documento come definizione di "fatto", + un E2E che li verifica — è questo che evita le realizzazioni parziali del passato.

## Prossimo passo
Scegliere il primo round da pianificare in dettaglio (raccomandato **R13 — Issue view completa**, perché la vista issue è la più usata e i backend sono tutti pronti) e lanciare `writing-plans` su quella fetta.
