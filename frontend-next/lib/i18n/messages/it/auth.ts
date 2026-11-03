import type { Catalog } from "../../types";
import type { auth as en } from "../en/auth";

export const auth: Catalog<typeof en> = {
  tagline: "La gestione progetti, rinnovata",
  signIn: "Accedi",
  createAccount: "Crea account",
  email: "Email",
  emailPlaceholder: "nome@esempio.it",
  username: "Nome utente",
  usernamePlaceholder: "mariorossi",
  password: "Password",
  pleaseWait: "Attendi…",
  genericError: "Si è verificato un errore",
};
