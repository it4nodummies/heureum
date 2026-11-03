import { common } from "./common";
import { layout } from "./layout";
import { auth } from "./auth";
import { profile } from "./profile";
import { notifications } from "./notifications";
import { issues } from "./issues";
import { boards } from "./boards";
import { projects } from "./projects";
import { search } from "./search";
import { filters } from "./filters";
import { dashboards } from "./dashboards";
import { groups } from "./groups";

/** English catalogue — the source of truth for every message key. */
export const en = {
  common,
  layout,
  auth,
  profile,
  notifications,
  issues,
  boards,
  projects,
  search,
  filters,
  dashboards,
  groups,
} as const;

export type Messages = typeof en;
