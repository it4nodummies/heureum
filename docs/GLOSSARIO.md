# Glossario

Termini usati nei design document (`docs/superpowers/specs/`), nei piani
(`docs/superpowers/plans/`), negli ADR (`docs/adr/`) e in `docs/superpowers/STATE.md`. Una riga
per termine; se un termine cambia significato, si aggiorna qui e non altrove.

- **Round** — slice verticale di lavoro (API + UI + test) con un piano dedicato in
  `docs/superpowers/plans/` e il gate a tre livelli verde prima di dirsi concluso.
- **Gate a tre livelli** — (1) `go build && go vet && go test`; (2) build frontend + Playwright;
  (3) `go run ./cmd/gapreport` senza diff. Tutti e tre, o il round non è finito.
- **Superficie compat** — l'insieme dei path `/rest/api/3/*` e `/rest/agile/1.0/*` implementati
  e presidiati dai contract test. Congelata: vedi ADR 0001.
- **Ponte di migrazione** — il ruolo della superficie compat dopo il congelamento: serve a far
  entrare chi arriva da Jira, non a replicare Jira.
- **Estensione** — rotta implementata fuori dal contratto ufficiale Jira (93 nel gap report).
  Legittima, ma non gode della promessa di stabilità (ADR 0003).
- **Rotta nativa** — rotta nuova, non modellata su Jira, che le feature post-congelamento usano
  al posto di inventarsi una shape v3.
- **Zero-config** — proprietà bersaglio: un progetto appena creato è utilizzabile **senza mai
  aprire Settings**. Misurata dall'E2E dei "60 secondi".
- **Minimo operativo** — ciò che deve esistere prima di mettere dati veri in un'istanza:
  backup/restore di Postgres e del volume allegati, documentato e **provato**.
- **Follow-up fantasma** — voce di `STATE.md` già chiusa nel codice ma mai tolta dal file.
  Circa 8-10 su 35 stimate; la verifica è un compito del Round 23.
- **Tenant** — l'istanza di deployment. Non esiste un tenant applicativo: vedi ADR 0002.
- **`seq_id`** — id sequenziale in stile Jira (progetti e issue da 10000, board e sprint da 1),
  esposto come `id`/`key` pubblico; la PK interna resta un UUID.
