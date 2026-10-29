"use client";

// Small coloured chip naming an epic — used wherever an issue's epic is shown
// (issue detail parent field, backlog/board cards, the backlog epic panel).
// The colour comes from the epic's `color.key` (lib/epicColors.ts); callers
// that only have a fields.parent ref (no colour) get the default swatch.

import Link from "next/link";
import { epicColorHex } from "@/lib/epicColors";

interface Props {
  name: string;
  colorKey?: string | null;
  // When set, the chip is a link (e.g. to /app/browse/{key}).
  href?: string;
  // Shown as a tooltip and as the prefix on hover-less surfaces (cards).
  issueKey?: string;
  className?: string;
  testId?: string;
}

export function EpicLozenge({ name, colorKey, href, issueKey, className = "", testId = "epic-lozenge" }: Props) {
  const hex = epicColorHex(colorKey);
  const classes = `inline-flex max-w-full items-center gap-1.5 rounded px-1.5 py-0.5 text-xs font-medium leading-4 ${className}`;
  const body = (
    <>
      <span aria-hidden className="h-2 w-2 shrink-0 rounded-sm" style={{ backgroundColor: hex }} />
      <span className="truncate">{name}</span>
    </>
  );
  const style = { backgroundColor: `${hex}1f`, color: "#1a1f36" };
  const title = issueKey ? `${issueKey} · ${name}` : name;

  if (href) {
    return (
      <Link href={href} title={title} data-testid={testId} className={`${classes} hover:underline`} style={style}>
        {body}
      </Link>
    );
  }
  return (
    <span title={title} data-testid={testId} className={classes} style={style}>
      {body}
    </span>
  );
}
