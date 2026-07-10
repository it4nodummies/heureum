# Contratti API (docs/contracts)

- `jira-platform-v3.json` — OpenAPI ufficiale Jira Cloud REST API v3 (fonte Atlassian).
- `jira-agile-1.0.json` — OpenAPI ufficiale Jira Software (Agile) 1.0.

Questi file sono la **fonte di verità** per la compatibilità drop-in.
Aggiornali con `scripts/update-contracts.sh` e committa il diff.
I contract test in `internal/contract` validano le risposte del nostro server contro questi schemi.
