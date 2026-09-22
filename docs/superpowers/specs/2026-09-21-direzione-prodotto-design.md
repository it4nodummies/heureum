# Direzione del prodotto e sequenza R23-R25 — Design Document

> **Data:** 2026-09-21

---

## Problema

Il progetto è fermo dal 2026-07-20. Ventidue round di parità con Jira hanno prodotto 37k righe
di Go, 15k di TypeScript, 109 file di test e 30 spec Playwright — e **zero utenti**: il
repository è pubblico dal 2026-07-14 con 0 stelle, 0 fork, 0 issue.

Nel frattempo il README porta **due tesi di prodotto in conflitto**. La prima, perseguita da
tutti i round, è "clone drop-in di Jira": misurata dal gap report, è al 17% (119 path su ~700) e
le 602 rotte mancanti sono in larga parte scheme e configurazione. La seconda, dichiarata nella
tagline e mai perseguita, è *"issue & project tracking that's easy for product owners"* — che è
l'opposto di replicare la configurabilità di Jira.

Si è accumulato inoltre debito aperto che nessuna direzione futura rende meno urgente:

- **PR #24** (associazione team↔progetti, 14 commit, 1598 righe) è aperta dal 2026-07-20 con il
  solo job `e2e` rosso — 3 fallimenti su 3, non flakiness.
- **11 PR dependabot** aperte, la più vecchia dal 2026-07-21.
- **v1.1.1 non rilasciata**: `[Unreleased]` contiene tre fix Postgres user-facing gravi (salvare
  una issue non assegnata dava HTTP 500; le notifiche non si inserivano; la History restava
  vuota). Chi ha fatto `docker pull 1.1.0` ha ancora quei bug.

Questo documento registra la direzione scelta e le decisioni ordinarie che ne discendono. Le
decisioni difficili da annullare stanno in `docs/adr/`; il vocabolario in `docs/GLOSSARIO.md`.

## Decisioni (confermate con l'utente)

### Difficili da annullare — registrate come ADR

- **[ADR 0001](../../adr/0001-superficie-jira-compat-congelata.md)** — la superficie Jira-compat
  è congelata e riclassificata come ponte di migrazione.
- **[ADR 0002](../../adr/0002-nessuna-multi-tenancy-applicativa.md)** — il tenant è l'istanza;
  `organizations`/`org_id` restano inerti, né rimosse né attivate.
- **[ADR 0003](../../adr/0003-promessa-di-stabilita-selettiva.md)** — la promessa di stabilità
  copre solo issue, progetti, ricerca/JQL e agile.

### Ordinarie

| # | Decisione | Scelta | Motivo |
|---|---|---|---|
| 1 | Direzione | Prodotto distinto «easy for product owners», validato dall'uso interno in Harpa | 22 round di parità non hanno prodotto un utente; il fossato competitivo è la compat già fatta, non la completezza |
| 2 | Priorità immediata | Bonifica del debito prima di qualunque round nuovo | I fix Postgres in `[Unreleased]` sono user-facing gravi e fermi da due mesi |
| 3 | PR #24 (Teams) | Diagnosi di `EffectiveRole` → test deterministico → merge. Non si archivia | 1598 righe per il resto pulite; Teams↔progetti è raggruppamento di permessi, non il "Teams con capacity" escluso in FASE D |
| 4 | Gate Postgres | Ibrido: helper di test su **migrazioni reali**, applicato subito ai 4 domini che scrivono FK nullable | Cambiare solo driver non catturerebbe il bug FK: lo schema dei test non ha vincoli |
| 5 | Isolamento e2e | DB per worker Playwright, come conseguenza di #4 | Sblocca il parallelismo e toglie la flakiness storica da write-contention SQLite |
| 6 | Tesi di prodotto | Zero-config, prima e sola ipotesi | È un difetto documentato dai commenti del seed, non una scommessa da validare |
| 7 | Ambito zero-config | «Progetto utilizzabile in 60 secondi», niente di più | Ipotesi minima falsificabile: se non basta, la tesi è sbagliata e costa un round scoprirlo |
| 8 | Uso interno | Minimo operativo: backup/restore provato. Niente SSO, niente import da Jira | Il backup è l'unica assenza che trasforma un esperimento in perdita di dati; l'import contraddice la tesi da validare |
| 9 | Follow-up di STATE.md | Verifica completa dentro R23 | Si consegna a colleghi veri: bisogna sapere quali difetti noti si stanno consegnando |
| 10 | Ordine dei round | R24 prima di R25 | R25 scrive in `boards`/`issue_types`/`resolutions`: la rete di sicurezza va tesa prima |

## Sequenza dei round

### R23 — Riapertura cantiere

- Diagnosi del costo di `EffectiveRole` sull'authz → `board.spec.ts` reso deterministico →
  merge di PR #24.
- 11 PR dependabot in lotti; **`@dnd-kit/sortable` 8→10 per ultimo**, dopo la stabilizzazione
  del test che usa proprio quella libreria.
- Tag **v1.1.1** (push a carico dell'utente, vedi `docs/RELEASE.md`).
- `.gitignore` per `data/uploads`.
- Backup/restore di Postgres + volume allegati: documentato e provato.
- Verifica delle ~35 voci di follow-up in `STATE.md`: ciascuna marcata chiusa, aperta o
  archiviata.
- README: sezione sulla promessa selettiva (conseguenza obbligata dell'ADR 0003).

### R24 — Gate reale

- Helper di test condiviso che applica le **migrazioni reali** invece di `AutoMigrate` + stub.
- Applicato ai 4 domini che scrivono FK nullable: `issue`, `comment`, `notification`,
  `issue_history`.
- Matrice CI: la suite Go gira su SQLite **e** su Postgres.
- DB isolato per worker Playwright.

### R25 — Zero-config

- `POST /rest/api/3/project` crea anche board, issue type di default e resolution.
- I template scrum/kanban/process-control generano configurazioni davvero diverse (oggi mappano
  solo `project.Type`).
- Il fallimento di `CreateDefaultWorkflow` smette di essere silenzioso.
- E2E di accettazione: creare un progetto e trascinare una issue sulla board **senza mai aprire
  Settings**.

## Fatti verificati

Accertati leggendo il repository il 2026-09-18/21, non assunti:

- **Compat reale**: 119 path implementati / 602 mancanti / 93 estensioni —
  `docs/contracts/gap-report.md`.
- **PR #24**: tutti i job verdi tranne `e2e`, che fallisce 3 volte su 3 (con `retries: 2`) su
  `board.spec.ts:262` — un test drag&drop che contiene già un commento di nove righe a
  documentare una race fra stato ottimistico e refetch, mitigata con `waitForLoadState`.
- **I test non usano le migrazioni**: costruiscono lo schema con `db.AutoMigrate(&Model{})` più
  `CREATE TABLE` scritti a mano **senza alcun vincolo di foreign key** — 24 file, nessun helper
  condiviso (`newTestDB` duplicato in 2 file). Esempio: `internal/domain/version/service_test.go:11`.
- **Creazione progetto**: il workflow di default viene creato dall'handler HTTP, ma l'errore è
  **solo loggato** (`log.Printf` in `internal/api/handlers/project_handler.go`); il creatore
  **viene** aggiunto come project admin (`project.Service.CreateProject`). Non vengono creati
  board, issue type né resolution. I tre template mappano solo `project.Type`.
- **Issue type get-or-create per nome** — `internal/domain/issue/service.go:427`: un progetto
  nuovo non ne ha nessuno finché qualcuno non crea una issue.
- **Allegati**: `APP_UPLOADS_DIR` default `./data/uploads`, montato come volume nominato
  `uploads:/data/uploads` in `deploy/docker/docker-compose.yml` — non si perdono al redeploy.
  La cartella locale `data/uploads/` (140 file) è però untracked e fuori da `.gitignore`.
- **Follow-up parzialmente obsoleti**: la voce «nessun controllo di ownership su
  `GET/PUT/DELETE /filter/{id}`» è **già chiusa** — `FilterHandler.requireOwner` restituisce 403
  al non-proprietario non-admin. Restano invece reali: `WorklogService.Delete` non decrementa
  `TimeSpent`; `WorkflowHandler.ListStatuses` è dead code; la history logga i campi invariati
  (il controllo è `title != nil`, non `*title != issue.Title`, e il form del frontend rimanda
  ogni campo a ogni salvataggio).
- **CI su `main` verde**, ultimo run 2026-07-20.

## Assunzioni residue

- **Il rosso di PR #24 è timing o stato condiviso fra spec, non un difetto di `EffectiveRole`.**
  Non verificata: è il primo compito di R23. Se è falsa, R23 si allunga per correggere l'authz.
- **Circa 8-10 voci fantasma su 35** nei follow-up. Estrapolazione da 4 campioni: potrebbero
  essere molte di più o molte meno.
- **Ci lavora una sola persona.** Se entrano altre mani, cambia la struttura dei round e serve
  un CONTRIBUTING vivo, non solo il file.
- **I due mesi di fermo sono una pausa.** Se il progetto rientra fra sei mesi, R23 va rifatto:
  i dependabot saranno venticinque.
- **Harpa Italia è un utilizzatore reale possibile**, non solo il contesto in cui il progetto
  esiste.

## Decisioni ancora aperte

- **Wizard di primo avvio** (istanza vuota → primo progetto e primo utente): candidato per il
  round successivo a R25.
- **Sfoltimento dell'interfaccia** / modalità avanzata: deliberatamente rinviato a dopo aver
  visto qualcuno usare il prodotto. Sfoltire prima di avere un utente è indovinare.
- **Object storage S3** per allegati e avatar: serve solo con più repliche o per il backup
  fuori-host.
