import type { Catalog } from "../../types";
import type { filters as en } from "../en/filters";

export const filters: Catalog<typeof en> = {
  title: "Filtri",
  filterName: "Nome filtro",
  shareWithTeam: "Condividi con il team",
  save: "Salva filtro",
  invalidJql: "Query JQL non valida",
  saved: "Filtri salvati",
  noneYet: "Nessun filtro salvato",
  resultCount: { one: "{count} risultato", other: "{count} risultati" },
  prev: "Precedente",
  next: "Successiva",
  row: {
    editName: "Modifica nome filtro",
    shared: "Condiviso",
    sharedTitle: "Condiviso con il team",
    edit: "Modifica filtro",
    editTitle: "Modifica {name}",
    delete: "Elimina filtro",
    deleteTitle: "Elimina {name}",
  },
};
