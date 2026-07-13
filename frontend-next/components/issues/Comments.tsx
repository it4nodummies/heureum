"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { comments, textToADF } from "@/lib/api";
import { AdfRenderer } from "./adf";

export function Comments({ issueKey }: { issueKey: string }) {
  const qc = useQueryClient();
  const { data } = useQuery({ queryKey: ["comments", issueKey], queryFn: () => comments.list(issueKey) });
  const [text, setText] = useState("");
  const add = useMutation({
    mutationFn: () => comments.add(issueKey, textToADF(text)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["comments", issueKey] });
      setText("");
    },
  });
  const list = data?.comments ?? [];
  return (
    <section className="mt-8">
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">Comments</h2>
      <div className="space-y-4">
        {list.map((c) => (
          <div key={c.id} className="rounded-lg border border-slate-200 p-3">
            <div className="mb-1 text-sm font-semibold text-[#1a1f36]">
              {c.author?.displayName ?? "Unknown"}
              <span className="ml-2 text-xs font-normal text-slate-400">{c.created?.slice(0, 10)}</span>
            </div>
            <AdfRenderer doc={c.body} />
          </div>
        ))}
        {list.length === 0 && <p className="text-sm text-slate-400">No comments yet.</p>}
      </div>
      <div className="mt-4">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Add a comment…"
          rows={3}
          aria-label="Add a comment"
          className="w-full rounded border border-slate-300 px-3 py-2"
        />
        <button
          onClick={() => add.mutate()}
          disabled={!text.trim() || add.isPending}
          className="mt-2 rounded bg-[#0052cc] px-4 py-2 font-semibold text-white disabled:opacity-60"
        >
          Add comment
        </button>
      </div>
    </section>
  );
}
