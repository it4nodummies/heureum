import type { Catalog } from "../../types";
import type { layout as en } from "../en/layout";

export const layout: Catalog<typeof en> = {
  meta: {
    description: "Gestione progetti moderna",
  },
  nav: {
    forYou: "Per te",
    recent: "Recenti",
    starred: "Preferiti",
    apps: "App",
    plans: "Piani",
    projects: "Progetti",
    filters: "Filtri",
    dashboards: "Dashboard",
    teams: "Team",
    goals: "Obiettivi",
  },
  sidebar: {
    collapse: "Comprimi la barra laterale",
    expand: "Espandi la barra laterale",
    customize: "Personalizza la barra laterale",
    comingSoon: "Presto disponibile",
    comingSoonItem: "{label} (presto disponibile)",
    soonBadge: "Presto",
  },
  topbar: {
    create: "Crea",
    createIssue: "Ticket",
    createProject: "Progetto",
    profile: "Profilo",
    settings: "Impostazioni",
    signOut: "Esci",
  },
  theme: {
    toggle: "Alterna tema chiaro e scuro",
    dark: "Scuro",
    light: "Chiaro",
  },
};
