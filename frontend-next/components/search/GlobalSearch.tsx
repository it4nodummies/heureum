"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

// GlobalSearch: la barra in top nav. Invio => vai alla pagina filtri con la
// query come JQL testuale (text ~ "...") oppure JQL grezza se contiene un operatore.
export function GlobalSearch() {
  const router = useRouter();
  const [q, setQ] = useState("");

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = q.trim();
    if (!trimmed) return;
    const isJql = /[=~<>]|\b(AND|OR|ORDER BY)\b/i.test(trimmed);
    const jql = isJql ? trimmed : `text ~ "${trimmed.replace(/"/g, "")}"`;
    router.push(`/app/filters?jql=${encodeURIComponent(jql)}`);
  };

  return (
    <form onSubmit={submit} className="relative">
      <svg
        viewBox="0 0 20 20"
        fill="currentColor"
        className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
      >
        <path
          fillRule="evenodd"
          d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z"
          clipRule="evenodd"
        />
      </svg>
      <input
        type="text"
        aria-label="Search"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Search"
        className="w-full pl-9 pr-4 py-1.5 text-sm bg-slate-50 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] focus:bg-white transition-all placeholder:text-slate-400"
      />
      <kbd className="absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-slate-300 font-mono bg-slate-100 px-1.5 py-0.5 rounded">
        /
      </kbd>
    </form>
  );
}
