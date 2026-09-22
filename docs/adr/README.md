# Architecture Decision Records

Decisioni difficili da annullare — contratto pubblico, schema persistito, modello di permessi,
licenza, fornitore. Un file per decisione, immutabile: una decisione superata non si riscrive,
si marca `sostituita da NNNN` e se ne aggiunge una nuova.

Le decisioni ordinarie stanno nei design document sotto `docs/superpowers/specs/`; il vocabolario
condiviso in `docs/GLOSSARIO.md`.

| N | Decisione | Stato |
|---|---|---|
| [0001](0001-superficie-jira-compat-congelata.md) | La superficie Jira-compat è un ponte di migrazione congelato | accettata |
| [0002](0002-nessuna-multi-tenancy-applicativa.md) | Il tenant è l'istanza: nessuna multi-tenancy applicativa | accettata |
| [0003](0003-promessa-di-stabilita-selettiva.md) | La promessa di stabilità è selettiva: quattro aree di migrazione | accettata |
