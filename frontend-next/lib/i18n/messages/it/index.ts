import type { Catalog } from "../../types";
import type { Messages } from "../en";
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

/** Italian catalogue — typed against English, so a missing/extra key fails the build. */
export const it: Catalog<Messages> = {
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
};
