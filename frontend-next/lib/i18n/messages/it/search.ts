import type { Catalog } from "../../types";
import type { search as en } from "../en/search";

export const search: Catalog<typeof en> = {
  global: {
    label: "Cerca",
    placeholder: "Cerca",
  },
  columns: {
    label: "Colonne",
    key: "Chiave",
    summary: "Riepilogo",
    status: "Stato",
    priority: "Priorità",
    assignee: "Assegnatario",
    updated: "Aggiornato",
  },
  bulk: {
    selected: { one: "{count} selezionato", other: "{count} selezionati" },
    priority: "Priorità in blocco",
    priorityPlaceholder: "Priorità…",
    assignee: "Assegnatario in blocco",
    assigneePlaceholder: "Assegnatario…",
    labels: "Imposta etichette",
    labelsPlaceholder: "Imposta etichette, separate da virgola",
    labelsHint: "Sostituisce le etichette esistenti dei ticket selezionati",
    apply: "Applica",
    clear: "Annulla selezione",
    failed: "Non è stato possibile aggiornare alcuni ticket: {keys}",
  },
  table: {
    selectAll: "Seleziona tutti",
    selectRow: "Seleziona {key}",
    noResults: "Nessun risultato",
    priorityFor: "Priorità di {key}",
    statusFor: "Stato di {key}",
    assigneeFor: "Assegnatario di {key}",
    noTransitions: "Nessuna transizione disponibile",
    unassigned: "Non assegnato",
  },
  people: {
    search: "Cerca persone",
    searchFor: "Cerca persone per {key}",
    placeholder: "Cerca persone…",
    noMatches: "Nessuna persona trovata",
  },
};
