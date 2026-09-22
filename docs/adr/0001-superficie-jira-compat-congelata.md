# 0001 — La superficie Jira-compat è un ponte di migrazione congelato

Data: 2026-09-21 · Stato: accettata

## Contesto

Heureum espone `/rest/api/3/*` e `/rest/agile/1.0/*` con l'intento dichiarato di essere
"drop-in compatibile" con Jira Cloud. Dopo 22 round di parità il gap report misura
**119 path implementati, 602 mancanti, 93 estensioni fuori contratto**: circa il 17% del
contratto ufficiale. Le 602 mancanti sono in larga parte scheme, property e configurazione
(`fieldconfigurationscheme`, `issuesecurityschemes`, `*/properties/{key}`) — la parte di Jira
che un prodotto "facile per i product owner" esiste apposta per non avere.

Il repository è pubblico dal 2026-07-14 con 0 stelle, 0 fork e 0 issue: non esiste oggi un
consumatore esterno della superficie compat. Il README porta però già una seconda tesi di
prodotto — *"issue & project tracking that's easy for product owners"* — che nessun round ha
finora perseguito.

## Decisione

La superficie Jira-compat è **congelata** e riclassificata come **ponte di migrazione**, non
come obiettivo da completare:

- i 119 path esistenti si mantengono e restano presidiati dai contract test in
  `internal/contract/`;
- non si aggiungono nuove rotte v3 se non quando una migrazione reale lo richiede;
- le feature nuove usano **rotte native**, senza piegarsi alle shape v3.

## Alternative scartate

- **Crescita opportunistica** (ogni feature nuova prende anche una shape v3): raddoppia il
  costo di ogni round per un consumatore che non esiste, e forza le feature native dentro
  modelli progettati per un altro prodotto.
- **Doppia superficie esplicita** (API nativa + `/rest/api/3` come adapter): raddoppia la
  superficie da mantenere prima di avere un solo utente.

## Conseguenze

- Il gap report resta stabile intorno al 17%: **quella cifra smette di essere un debito e
  diventa una scelta registrata**. Va detto nel README, altrimenti si legge come incompletezza.
- `go run ./cmd/gapreport` resta nel gate a tre livelli come rilevatore di regressioni, non
  come misura di avanzamento.
- Una feature nuova che non ha una controparte in Jira **non deve** inventarsi una shape v3:
  usa una rotta nativa. Il vincolo di `additionalProperties:false` di molti schemi Jira rende
  questo non solo lecito ma necessario (aggiungere campi custom ai DTO v3 rompe i contract test).
- Un effetto collaterale su un endpoint compat resta ammesso quando **Jira stesso lo ha**:
  `POST /rest/api/3/project` che crea anche una board è allineato al comportamento di Jira, non
  una violazione del congelamento (vedi la decisione zero-config nel design document
  `docs/superpowers/specs/2026-09-21-direzione-prodotto-design.md`).
- La promessa pubblica di stabilità che ne discende è oggetto dell'**ADR 0003**: congelare dice
  cosa facciamo, non cosa garantiamo a chi ci usa.
- Se un domani compare un consumatore esterno reale con un'esigenza documentata, questa ADR va
  riaperta e sostituita: non è una decisione per sempre, è una decisione per "a zero utenti".
