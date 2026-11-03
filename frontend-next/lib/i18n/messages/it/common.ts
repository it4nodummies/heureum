import type { Catalog } from "../../types";
import type { common as en } from "../en/common";

export const common: Catalog<typeof en> = {
  actions: {
    save: "Salva",
    cancel: "Annulla",
    delete: "Elimina",
    create: "Crea",
    edit: "Modifica",
    close: "Chiudi",
    add: "Aggiungi",
    remove: "Rimuovi",
    confirm: "Conferma",
    search: "Cerca",
  },
  loading: "Caricamento…",
  never: "mai",
  error: "Si è verificato un errore",
  comingSoon: "Presto disponibile",
  issueCount: { one: "{count} ticket", other: "{count} ticket" },
};
