"use client";

import { useState } from "react";
import { Comments } from "./Comments";
import { History } from "./History";

type ActivityTab = "comments" | "history";

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  const className = active
    ? "pb-2 text-sm font-semibold border-b-2 border-[#0052cc] text-[#0052cc] transition-colors"
    : "pb-2 text-sm font-medium border-b-2 border-transparent text-slate-500 hover:text-[#1a1f36] hover:border-slate-300 transition-colors";
  return (
    <button type="button" onClick={onClick} aria-current={active ? "true" : undefined} className={className}>
      {children}
    </button>
  );
}

// Activity area at the bottom of the issue view: a small Comments | History
// tab bar. "Comments" stays the default tab so the existing comment
// add/list flow (Comments.tsx) isn't hidden behind an extra click; "History"
// renders the per-issue changelog (see History.tsx).
export function Activity({ issueKey }: { issueKey: string }) {
  const [tab, setTab] = useState<ActivityTab>("comments");

  return (
    <section className="mt-8">
      <div data-testid="activity-tabs" className="mb-4 flex items-center gap-5 border-b border-slate-200">
        <TabButton active={tab === "comments"} onClick={() => setTab("comments")}>
          Comments
        </TabButton>
        <TabButton active={tab === "history"} onClick={() => setTab("history")}>
          History
        </TabButton>
      </div>

      {tab === "comments" ? <Comments issueKey={issueKey} /> : <History issueKey={issueKey} />}
    </section>
  );
}
