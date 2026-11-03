import type { Catalog } from "../../types";
import type { groups as en } from "../en/groups";

export const groups: Catalog<typeof en> = {
  title: "Team",
  subtitle: "Gestisci i team e i loro membri. Richiede un amministratore globale.",
  create: {
    title: "Crea team",
    name: "Nome del team",
    submit: "Crea team",
    creating: "Creazione…",
    failed: "Impossibile creare il team",
  },
  list: {
    title: "Tutti i team",
    loading: "Caricamento dei team…",
    failed: "Impossibile caricare i team",
    empty: "Nessun team",
    confirmDelete: 'Eliminare il team "{name}"?',
    deleteAria: "Elimina {name}",
  },
  members: {
    loading: "Caricamento dei membri…",
    failed: "Impossibile caricare i membri",
    empty: "Nessun membro",
    removeAria: "Rimuovi {user} da {group}",
    addAria: "Aggiungi utente a {group}",
    addPlaceholder: "Cerca persone da aggiungere…",
    searching: "Ricerca in corso…",
    noMatches: "Nessuna persona trovata",
    addFailed: "Impossibile aggiungere l'utente",
  },
};
