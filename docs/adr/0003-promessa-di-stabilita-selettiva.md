# 0003 — La promessa di stabilità è selettiva: quattro aree di migrazione

Data: 2026-09-21 · Stato: accettata

## Contesto

L'ADR 0001 congela la superficie Jira-compat e la riclassifica come ponte di migrazione. Quella
decisione dice cosa facciamo internamente — non aggiungere rotte v3 — ma non dice nulla su cosa
garantiamo a chi quella superficie la usa. Oggi il README non promette niente: rimanda al gap
report e lascia al lettore il compito di dedurre cosa può considerare stabile.

I 119 path implementati non hanno però tutti lo stesso valore. Alcuni esistono perché un
contract test li richiedeva o perché completavano una famiglia di endpoint; altri sono il
percorso che una migrazione da Jira attraversa davvero. Promettere stabilità su tutti e 119
equivale a vincolarsi su rotte che nessuno usa; non promettere nulla nasconde il fossato
competitivo — JQL vero e conformità verificata contro l'OpenAPI ufficiale — che è la ragione
per cui la superficie esiste.

## Decisione

Si dichiarano **stabili entro la major** soltanto le quattro aree che una migrazione da Jira
attraversa:

1. **Issue** — CRUD, campi, transizioni, commenti, worklog, link, watcher.
2. **Progetti** — CRUD, ricerca, categorie.
3. **Ricerca / JQL** — `/search/jql` e il linguaggio accettato dal parser.
4. **Agile** — board, sprint, backlog (`/rest/agile/1.0/*`).

Tutto il resto della superficie resta **best-effort**, col rimando al gap report.

## Alternative scartate

- **Promessa piena sui 119 path**: vincola anche le rotte scritte per far passare un contract
  test, a fronte di zero consumatori esterni che la richiedano.
- **Nessuna promessa** (lo stato attuale): non comunica nulla a chi valuta una migrazione, che
  è esattamente il lettore per cui la superficie è stata congelata invece che rimossa.

## Conseguenze

- Il README acquisisce una sezione che nomina le quattro aree come stabili e dichiara il resto
  best-effort. Senza quella sezione questa ADR non ha effetto.
- Una modifica incompatibile dentro le quattro aree — rimozione di una rotta, cambio di shape,
  restringimento di un campo — richiede una **major** e un periodo di deprecazione annunciato.
- I contract test che coprono quelle quattro aree diventano **non negoziabili**: cancellarne uno
  per far passare la CI è una rottura di promessa, non una scorciatoia di manutenzione.
- Fuori dalle quattro aree non si garantisce nulla: quelle rotte possono cambiare o sparire in
  una minor, e il gap report resta l'unica fonte su cosa esiste.
- Estendere la promessa ad altre aree richiede una **nuova ADR**, non un allargamento
  silenzioso: la promessa cresce solo quando un consumatore reale la giustifica.
- Rimando incrociato: questa ADR presuppone l'**ADR 0001** e ne è la faccia esterna.
