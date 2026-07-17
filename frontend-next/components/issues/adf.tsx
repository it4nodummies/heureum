"use client";

import type { ADFNode } from "@/lib/api";

export function AdfRenderer({ doc }: { doc: ADFNode | null }) {
  if (!doc || !doc.content || doc.content.length === 0) {
    return <p className="italic text-slate-400">No description</p>;
  }
  return (
    <div className="prose prose-sm max-w-none">
      {doc.content?.map((n, i) => (
        <AdfBlock key={i} node={n} />
      ))}
    </div>
  );
}

function AdfBlock({ node }: { node: ADFNode }) {
  if (node.type === "paragraph") {
    return (
      <p>
        {node.content?.map((c, i) => (
          <AdfInline key={i} node={c} />
        ))}
      </p>
    );
  }
  if (node.type === "bulletList") {
    return (
      <ul className="list-disc pl-5">
        {node.content?.map((c, i) => (
          <AdfBlock key={i} node={c} />
        ))}
      </ul>
    );
  }
  if (node.type === "orderedList") {
    return (
      <ol className="list-decimal pl-5">
        {node.content?.map((c, i) => (
          <AdfBlock key={i} node={c} />
        ))}
      </ol>
    );
  }
  if (node.type === "listItem") {
    return (
      <li>
        {node.content?.map((c, i) => (
          <AdfBlock key={i} node={c} />
        ))}
      </li>
    );
  }
  if (node.type === "heading") {
    const level = Number(node.attrs?.level ?? 3);
    const inner = node.content?.map((c, i) => <AdfInline key={i} node={c} />);
    if (level === 1) return <h1 className="text-xl font-bold">{inner}</h1>;
    if (level === 2) return <h2 className="text-lg font-semibold">{inner}</h2>;
    return <h3 className="font-semibold">{inner}</h3>;
  }
  return (
    <>
      {node.content?.map((c, i) => (
        <AdfBlock key={i} node={c} />
      ))}
    </>
  );
}

// ── Plain-text <-> ADF round trip (minimal, paragraphs-only) ────────────────
//
// Used by the issue "Edit" mode: the description is edited as plain text in a
// <textarea>, then converted back to a minimal ADF doc on save.

export function adfToText(doc: ADFNode | null): string {
  if (!doc || !doc.content) return "";
  const inlineText = (nodes: ADFNode[] | undefined): string =>
    (nodes ?? [])
      .map((c) => {
        if (c.type === "text") return c.text ?? "";
        if (c.type === "mention") return String(c.attrs?.text ?? "");
        return inlineText(c.content);
      })
      .join("");
  return doc.content
    .filter((n) => n.type === "paragraph")
    .map((n) => inlineText(n.content))
    .join("\n");
}

export function textToAdf(text: string): ADFNode {
  const lines = text.split("\n").filter((l) => l.trim() !== "");
  return {
    type: "doc",
    version: 1,
    content: lines.map((line) => ({
      type: "paragraph",
      content: [{ type: "text", text: line }],
    })),
  };
}

function AdfInline({ node }: { node: ADFNode }) {
  if (node.type === "mention") {
    const label = String(node.attrs?.text ?? "");
    return (
      <span
        data-mention
        className="rounded bg-[#0052cc]/10 px-1 py-0.5 font-medium text-[#0052cc]"
      >
        {label.startsWith("@") ? label : `@${label}`}
      </span>
    );
  }
  if (node.type !== "text") return null;
  let el: React.ReactNode = node.text;
  for (const m of node.marks ?? []) {
    if (m.type === "strong") el = <strong>{el}</strong>;
    if (m.type === "em") el = <em>{el}</em>;
    if (m.type === "code") el = <code className="rounded bg-slate-100 px-1">{el}</code>;
  }
  return <>{el}</>;
}
