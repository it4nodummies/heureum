"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { comments, type ADFNode } from "@/lib/api";
import { AdfRenderer } from "./adf";
import { RichTextEditor } from "@/components/common/RichTextEditor";
import { useFormat, useT } from "@/lib/i18n";

const EMPTY_DOC: ADFNode = { type: "doc", version: 1, content: [{ type: "paragraph", content: [] }] };

function isEmpty(doc: ADFNode): boolean {
  return (doc.content ?? []).every((n) => n.type === "paragraph" && (n.content ?? []).length === 0);
}

export function Comments({ issueKey, projectKey }: { issueKey: string; projectKey?: string }) {
  const t = useT();
  const fmt = useFormat();
  const qc = useQueryClient();
  const { data } = useQuery({ queryKey: ["comments", issueKey], queryFn: () => comments.list(issueKey) });
  const [draft, setDraft] = useState<ADFNode>(EMPTY_DOC);
  // Remounts the (uncontrolled) editor to clear it after a successful submit.
  const [composerKey, setComposerKey] = useState(0);
  const add = useMutation({
    mutationFn: () => comments.add(issueKey, draft),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["comments", issueKey] });
      setDraft(EMPTY_DOC);
      setComposerKey((n) => n + 1);
    },
  });
  const list = data?.comments ?? [];
  return (
    <section>
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">{t("issues.comments.title")}</h2>
      <div className="space-y-4">
        {list.map((c) => (
          <div key={c.id} className="rounded-lg border border-slate-200 p-3">
            <div className="mb-1 text-sm font-semibold text-[#1a1f36]">
              {c.author?.displayName ?? t("issues.unknownUser")}
              <span className="ml-2 text-xs font-normal text-slate-400">{c.created ? fmt.date(c.created) : null}</span>
            </div>
            <AdfRenderer doc={c.body} />
          </div>
        ))}
        {list.length === 0 && <p className="text-sm text-slate-400">{t("issues.comments.empty")}</p>}
      </div>
      <div className="mt-4">
        <RichTextEditor
          key={composerKey}
          valueAdf={EMPTY_DOC}
          onChangeAdf={setDraft}
          placeholder={t("issues.comments.placeholder")}
          projectKey={projectKey}
          ariaLabel={t("issues.comments.ariaLabel")}
          testId="comment-editor"
        />
        <button
          onClick={() => add.mutate()}
          disabled={isEmpty(draft) || add.isPending}
          className="mt-2 rounded bg-[#0052cc] px-4 py-2 font-semibold text-white disabled:opacity-60"
        >
          {t("issues.comments.add")}
        </button>
      </div>
    </section>
  );
}
