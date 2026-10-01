# Requisiti guidati da Harpa — Design / Requirements Document

> **Data:** 2026-10-01 · **Stato: BOZZA — ipotesi da validare col tour del Jira di Harpa.**
> Seguito operativo del design doc [2026-09-21](2026-09-21-direzione-prodotto-design.md) e degli
> ADR in `docs/adr/`. Questo documento NON riapre la parità con Jira: la incornicia dentro i
> bisogni di un utente reale.

---

## Provenienza (perché questo documento esiste)

Il 2026-10-01 la strategia «compat Jira congelata / zero-config» è stata **riaperta su richiesta
dell'utente e riconfermata**, con un fatto nuovo che la rafforza invece di contraddirla:

- **Harpa Italia adotterà davvero Heureum.** Utente interno reale — **sviluppo software**,
  **10–50 persone**, più team. È l'evento di validazione che la tesi zero-config aspettava (il
  design doc 2026-09-21 lo prevedeva esplicitamente: tesi «validata dall'uso interno in Harpa»).
- **Harpa parte da zero** (progetti nuovi, nessuna migrazione dello storico) → **l'import da Jira
  non serve**: la decisione #8 «niente import» regge, e l'**ADR 0001 (compat congelata) regge**.

**Conseguenza:** la roadmap non è più «zero-config in astratto», ma **«zero-config + ciò che un
team dev di 10–50 persone tocca ogni giorno, misurato su Harpa»**.

## Vincoli ereditati (non negoziabili senza riaprire un ADR)

- **ADR 0001** — superficie Jira-compat congelata: non si rincorrono le 602 rotte mancanti; le
  feature nuove sono **native**, non si piegano alle shape v3.
- **ADR 0002** — nessuna multi-tenancy applicativa: un'istanza = un tenant.
- **ADR 0003** — promessa di stabilità solo su issue/progetti/ricerca-JQL/agile.
- **Decisione #8 (2026-09-21)** — niente import da Jira, niente SSO *di default*. (L'SSO è
  l'unica di queste che l'adozione reale di Harpa può sfiorare — vedi R8.)

## Legenda stato

- **✅ DECISO** — già confermato da una decisione registrata; non serve il tour.
- **🔎 IPOTESI** — plausibile per un team dev 10–50, **da confermare col tour** del Jira di Harpa
  (cosa tocca ogni giorno vs cosa non apre mai). Nessuna priorità è definitiva finché non è
  confermata.
- Priorità provvisorie: **P0** (blocca l'adozione) · **P1** (attrito quotidiano forte) ·
  **P2** (utile) · **P3** (nice-to-have).

---

## Requisiti

### R1 — Onboarding zero-config · ✅ DECISO · P0
**Bisogno Harpa:** 10–50 persone creano progetti; un progetto deve essere usabile **subito**.
**Heureum oggi:** `POST /project` crea il progetto e aggiunge il creatore come admin, **ma non**
crea board, issue type di default né resolution; `CreateDefaultWorkflow` fallisce in silenzio
(`log.Printf`). Un progetto nuovo è inutilizzabile finché non si apre Settings.
**Gap → lavoro:** è esattamente **R25** (già progettato): `POST /project` crea board + issue type
default + resolution; i template scrum/kanban/process-control generano config davvero diverse
(oggi mappano solo `project.Type`); il fallimento del workflow smette di essere silenzioso; E2E di
accettazione «crea progetto → trascina una issue sulla board senza mai aprire Settings».
**Validazione tour:** confermare quali template Harpa userebbe (scrum? kanban?) e quali issue type
si aspetta di default.

### R2 — Wizard di primo avvio · 🔎 IPOTESI · P1
**Bisogno Harpa:** istanza vuota → primo utente e primo progetto senza smanettare con env/seed.
**Heureum oggi:** nessun wizard; il primo utente nasce da signup aperto o seed demo.
**Gap → lavoro:** schermata guidata «benvenuto → crea admin → crea primo progetto». Decisione già
«aperta» nel design doc, candidata subito dopo R25.
**Validazione tour:** come verrebbe fatto il bootstrap in Harpa (chi crea l'istanza, quanti admin).

### R3 — Components · 🔎 IPOTESI · priorità da confermare
**Bisogno Harpa (ipotesi):** categorizzare il lavoro per modulo/servizio/area — uso comune nei
team dev.
**Heureum oggi:** **non esistono** (dominio/rotta assenti; C.2 escluso in R21 su richiesta).
**Gap → lavoro:** dominio `component` + associazione issue↔component + filtro/board per component.
Costo medio; **da fare solo se Harpa li usa davvero** — è la domanda-chiave del tour, non un dato.
**Validazione tour:** aprire 3–4 progetti di Harpa su Jira e vedere se il campo Components è
popolato e usato nei filtri/board, o ignorato.

### R4 — Pannello Development / integrazione con il repo · 🔎 IPOTESI · P1–P2
**Bisogno Harpa:** vedere branch/commit/PR collegati a una issue — pane quotidiano di un team dev.
**Heureum oggi:** **esiste** un pannello Development e il dominio `git` con provider
**GitHub/GitLab/Forgejo**, ma **inbound via webhook** (commit/branch/PR arrivano da eventi del
provider), non una app OAuth con stato build/CI e sincronizzazione bidirezionale.
**Gap → lavoro (da confermare):** quanto Harpa ha bisogno — basta il link commit→issue via
webhook (già c'è) o serve l'integrazione OAuth «vera» (stato PR live, checks CI)? Scelta di
ambito, non di parità.
**Validazione tour:** quale provider usa Harpa, e se nel loro Jira il pannello Development mostra
solo commit o anche stato PR/build.

### R5 — Operazioni bulk / triage del backlog · 🔎 IPOTESI · P2
**Bisogno Harpa:** triage di backlog grandi (riassegnare, ri-etichettare, spostare in sprint in
massa).
**Heureum oggi:** `POST /issues/bulk` esiste ma **limitato** (set parziale di campi; es. non
esprime l'unassign — limite noto del decode a puntatore).
**Gap → lavoro:** estendere bulk (campi coperti, selezione da board/ricerca) + UI di selezione
multipla.
**Validazione tour:** osservare se Harpa fa triage massivo o lavora issue-per-issue.

### R6 — Pianificazione sprint / capacity · 🔎 IPOTESI · P3
**Bisogno Harpa (ipotesi):** stimare capacità del team per sprint.
**Heureum oggi:** story points sì; **capacity assente** («Teams con capacity» escluso in FASE D).
**Gap → lavoro:** campo capacity per sprint/team + indicatore di carico. Alta incertezza: molti
team 10–50 non usano capacity formale.
**Validazione tour:** verificare se Harpa usa la capacity di Jira o solo i punti.

### R7 — Difetti UX quotidiani (già noti) · ✅ DIFETTO CONFERMATO · P1
**Bisogno Harpa:** affidabilità nell'uso di tutti i giorni.
**Heureum oggi (bug reali registrati in STATE):** `WorklogService.Delete` **non** scala
`issue.TimeSpent` (il tempo totale resta gonfiato); la history logga anche i **campi invariati**
(il controllo è `title != nil`, non `*title != issue.Title`, e il form rimanda ogni campo a ogni
salvataggio → changelog rumoroso). Non sono Harpa-specifici, ma si sentono ogni giorno.
**Gap → lavoro:** due fix mirati + test; piccoli, alto rapporto valore/costo.

### R8 — SSO (Google Workspace) · 🔎 IPOTESI · P2 — tocca una decisione registrata
**Bisogno Harpa (ipotesi):** login aziendale senza password dedicate.
**Heureum oggi:** email/password + API token; **SSO escluso** dalla decisione #8.
**Gap → decisione:** è **l'unica** cosa che l'adozione reale di Harpa può far riaprire della #8.
Non blocca un fresh-start. Se Harpa lo vuole, va deciso esplicitamente (nota in `docs/adr/`), non
introdotto per inerzia.
**Validazione tour / colloquio:** chiedere a Harpa se l'SSO è un requisito d'adozione o un
desiderata.

---

## Esplicitamente FUORI ambito

- Le **602 rotte Jira** mancanti e in particolare scheme / security level / field configuration /
  properties: è la complessità che «easy for product owners» esiste apposta per non avere.
- **Import da Jira** (Harpa parte da zero — decisione #8).
- **Multi-tenancy** applicativa (ADR 0002).
- **Advanced Roadmaps, Goals/OKR, team workload/annidati** (FASE D), salvo emergano dal tour come
  bisogno quotidiano reale di Harpa (improbabile a 10–50).

## Domande aperte — da risolvere NEL TOUR

Sono le risposte che fissano le priorità vere (oggi sono ipotesi):
1. Scrum, kanban o misto? Quanti progetti e come sono strutturati?
2. Quali **issue type** e **custom field** sono davvero popolati e usati nei filtri?
3. **Components**: usati o ignorati?
4. Pannello **Development**: solo commit, o stato PR/build? Quale provider?
5. Fanno **triage bulk**? Usano **capacity** di sprint?
6. Quali **report/dashboard** guardano davvero (burndown? velocity? altro)?
7. **SSO**: requisito o desiderata?
8. Cosa in Jira **non aprono mai** (→ conferma di cosa NON ci serve).

## Prossimi passi

1. **Tour del Jira reale di Harpa** (serve il connettore Chrome ri-registrato: riavvio sessione).
   Inventario sistematico → risposte alle 8 domande sopra.
2. **Confermare / ripriorizzare** i requisiti R1–R8 con le evidenze.
3. **Registrare l'esito**: nota in `docs/adr/` o STATE.md — «strategia riaperta il 2026-10-01 su
   *Harpa adotta Heureum*; ADR 0001 e no-import confermati; roadmap = R25 + priorità dev di
   Harpa», con la lista confermata.
4. **Piani di round** (metodo Subagent-Driven): R25 zero-config per primo (P0, già progettato),
   poi i requisiti confermati in ordine di priorità.
