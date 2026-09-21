# 0002 — Il tenant è l'istanza: nessuna multi-tenancy applicativa

Data: 2026-09-21 · Stato: accettata

## Contesto

Lo schema contiene tabelle `organizations` e colonne `org_id` mai realmente usate: l'audit del
Round 12 aveva già emesso il verdetto "niente multi-tenancy applicativa, `organizations`
vestigiale". Nessuna query filtra per organizzazione; le key dei progetti sono uniche a livello
di istanza, non di organizzazione.

Il progetto è distribuito con licenza AGPL v3 in modalità self-hosted (Docker Compose e chart
Helm in `deploy/`), e la direzione decisa è la validazione tramite uso interno prima di
qualunque ambizione di adozione esterna. Non esiste oggi un'offerta SaaS gestita né un piano
per averne una.

## Decisione

**Porta aperta senza costruirci nulla.** Le tabelle e le colonne vestigiali **restano dove
sono**: non si rimuovono e non si attivano. L'unità di isolamento è il **deployment**: una
istanza = un tenant.

## Alternative scartate

- **Rimozione dello schema vestigiale con una migrazione**: fa guadagnare pulizia, ma è
  irreversibile, mentre il costo di tenere due tabelle inerti è zero. Una migrazione di
  rimozione andrebbe poi disfatta a mano se la decisione cambiasse.
- **Multi-tenancy applicativa completa** (claim `org` nel JWT, namespace delle key per
  organizzazione, filtro per `org_id` su ogni query): è un round grosso che tocca ogni query
  del progetto e introduce una classe di bug — la fuga di dati fra tenant — che oggi non può
  esistere per costruzione.

## Conseguenze

- Nessuna query filtra per `org_id`; chi legge il codice non deve chiedersi se una query è
  "tenant-safe". Le key dei progetti restano globali all'istanza.
- L'isolamento fra clienti, se servisse, è a livello di deployment: un'istanza e un database
  per cliente, con backup/restore per istanza (vedi il minimo operativo nel design document
  `docs/superpowers/specs/2026-09-21-direzione-prodotto-design.md`).
- `organizations` e `org_id` sono **codice inerte dichiarato**: non vanno usate, estese né
  citate come se fossero attive. Questa ADR è la risposta a chiunque le trovi e si chieda
  perché ci sono.
- La licenza resta AGPL v3: la decisione non la tocca, ma la presuppone — è coerente con il
  self-hosting e non con un SaaS gestito proprietario.
- Se un domani si vuole un SaaS multi-tenant, questa ADR va **sostituita** da una nuova, e il
  lavoro comprende JWT claim, namespace delle key e un audit di ogni query esistente. Le
  tabelle già presenti non riducono quel lavoro in modo significativo: risparmiano una
  migrazione, non l'audit.
