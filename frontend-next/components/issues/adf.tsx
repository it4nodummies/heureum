"use client";

import type { ADFNode } from "@/lib/api";

export function AdfRenderer({ doc }: { doc: ADFNode | null }) {
  if (!doc) return <p className="italic text-slate-400">No description</p>;
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
    return (
      <h3 className="font-semibold">
        {node.content?.map((c, i) => (
          <AdfInline key={i} node={c} />
        ))}
      </h3>
    );
  }
  return (
    <>
      {node.content?.map((c, i) => (
        <AdfBlock key={i} node={c} />
      ))}
    </>
  );
}

function AdfInline({ node }: { node: ADFNode }) {
  if (node.type !== "text") return null;
  let el: React.ReactNode = node.text;
  for (const m of node.marks ?? []) {
    if (m.type === "strong") el = <strong>{el}</strong>;
    if (m.type === "em") el = <em>{el}</em>;
    if (m.type === "code") el = <code className="rounded bg-slate-100 px-1">{el}</code>;
  }
  return <>{el}</>;
}
