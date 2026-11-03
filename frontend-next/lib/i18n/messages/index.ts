import type { Catalog, Locale } from "../types";
import { en, type Messages } from "./en";
import { it } from "./it";

export const messages: { [L in Locale]: L extends "en" ? Messages : Catalog<Messages> } = { en, it };
