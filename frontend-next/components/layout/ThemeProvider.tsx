"use client";

import { createContext, useContext, useState } from "react";

type Theme = "light" | "dark";

interface ThemeContextValue {
  theme: Theme;
  setTheme: (t: Theme) => void;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function resolveInitialTheme(): Theme {
  // On the client during hydration the no-flash <head> script has already set the
  // correct `dark` class on <html> before React runs, so reading it back gives the
  // real theme without any post-mount re-render (which would re-assert every
  // controlled input in the tree). On the server there is no document → "light".
  if (typeof document === "undefined") return "light";
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function applyThemeClass(theme: Theme) {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("dark", theme === "dark");
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  // Lazy-init from the DOM class the no-flash script already applied. No mount
  // effect → no extra app-wide re-render after hydration.
  const [theme, setThemeState] = useState<Theme>(resolveInitialTheme);

  const setTheme = (t: Theme) => {
    setThemeState(t);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("theme", t);
    }
    applyThemeClass(t);
  };

  const toggleTheme = () => setTheme(theme === "dark" ? "light" : "dark");

  return (
    <ThemeContext.Provider value={{ theme, setTheme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return ctx;
}

export function ThemeToggle({ className }: { className?: string }) {
  const { toggleTheme } = useTheme();

  // Markup is intentionally theme-AGNOSTIC: both the moon/"Dark" and sun/"Light"
  // variants are always rendered, and CSS (keyed on the `.dark` class the no-flash
  // script puts on <html>) shows exactly one. Because the DOM is identical on the
  // server and on the client's first render, there is no hydration mismatch — and
  // no post-mount re-render (which could race controlled inputs elsewhere on the
  // page during fast programmatic input). The aria-label is static for the same
  // reason (a theme-branched attribute would itself be a mismatch).
  return (
    <button
      type="button"
      data-testid="theme-toggle"
      onClick={toggleTheme}
      title="Toggle light and dark theme"
      aria-label="Toggle light and dark theme"
      className={
        className ??
        "inline-flex items-center gap-2 rounded border border-slate-300 px-3 py-1.5 text-sm text-[#1a1f36] hover:bg-slate-50 dark:border-[#2a3142] dark:text-[#e6e8eb] dark:hover:bg-[#232a3a]"
      }
    >
      {/* Shown in light mode → offers switching to dark. */}
      <span className="inline-flex items-center gap-2 dark:hidden">
        <svg viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4">
          <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
        </svg>
        Dark
      </span>
      {/* Shown in dark mode → offers switching to light. */}
      <span className="hidden items-center gap-2 dark:inline-flex">
        <svg viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4">
          <path d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4.95 2.05a1 1 0 010 1.414l-.707.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM18 9a1 1 0 010 2h-1a1 1 0 110-2h1zM3 9a1 1 0 010 2H2a1 1 0 110-2h1zm2.464-4.243a1 1 0 011.414 0l.707.707A1 1 0 116.17 6.878l-.707-.707a1 1 0 010-1.414zM10 6a4 4 0 100 8 4 4 0 000-8zm-1 10a1 1 0 112 0v1a1 1 0 11-2 0v-1zm6.243-1.757a1 1 0 011.414 1.414l-.707.707a1 1 0 01-1.414-1.414l.707-.707zM4.464 14.243a1 1 0 011.414 1.414l-.707.707A1 1 0 013.757 14.95l.707-.707z" />
        </svg>
        Light
      </span>
    </button>
  );
}
