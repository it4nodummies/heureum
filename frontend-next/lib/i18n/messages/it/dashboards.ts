import type { Catalog } from "../../types";
import type { dashboards as en } from "../en/dashboards";

export const dashboards: Catalog<typeof en> = {
  title: "Dashboard",
  newName: "Nome della nuova dashboard",
  namePlaceholder: "Nome dashboard",
  empty: "Nessuna dashboard",
  detail: {
    fallbackTitle: "Dashboard",
    gadgetType: "Tipo di gadget",
    addGadget: "Aggiungi gadget",
    noGadgets: "Nessun gadget",
  },
  gadgets: {
    assignedToMe: "Assegnati a me",
    activityStream: "Attività recenti",
    noAssigned: "Nessun ticket assegnato",
    noActivity: "Nessuna attività recente",
    changedOn: "ha modificato {field} in",
  },
};
