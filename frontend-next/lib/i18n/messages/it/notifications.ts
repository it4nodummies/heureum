import type { Catalog } from "../../types";
import type { notifications as en } from "../en/notifications";

export const notifications: Catalog<typeof en> = {
  title: "Notifiche",
  markAllRead: "Segna tutte come lette",
  direct: "Dirette",
  watching: "In osservazione",
  withCount: "{label} ({count})",
  other: "Altro",
  read: "Segna come letta",
  markRead: "Segna come letta: {title}",
  empty: "Nessuna notifica",
  types: {
    assignment: "Ti è stato assegnato un ticket",
    comment: "Nuovo commento su {key}",
    mention: "Ti hanno menzionato in {key}",
    sprint_started: "Sprint avviato",
    sprint_completed: "Sprint completato",
    status_change: "Stato cambiato su {key}",
  },
  bodies: {
    assignment: "Ti è stato assegnato {key}: {summary}",
    mention: "Qualcuno ti ha menzionato nei commenti di {key}: {summary}",
    sprint_started: "Lo sprint \"{name}\" è stato avviato",
    sprint_completed: "Lo sprint \"{name}\" è stato completato",
    status_change: "{summary}: stato cambiato in {status}",
  },
};
