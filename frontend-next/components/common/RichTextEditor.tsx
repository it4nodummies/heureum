"use client";

import { useEffect, useRef, useState } from "react";
import { profile, users, type ADFNode } from "@/lib/api";

// ── Constrained ADF vocabulary ───────────────────────────────────────────────
//
// This editor is deliberately dependency-free: a contentEditable <div> plus a
// minimal toolbar. It hydrates from an ADF doc and serializes the DOM back to
// ADF using ONLY the node/mark types AdfRenderer renders, so a round-trip
// (stored ADF → editor DOM → serialized ADF) is stable:
//
//   blocks : doc / paragraph / bulletList / orderedList / listItem / heading(1-3)
//   inline : text (+ marks strong/em/code) / mention {attrs:{id,text}}
//
// @mention insertion is handled by an autocomplete: typing "@word" opens a
// positioned dropdown of user results (project-scoped assignable search when a
// projectKey is given, else global user search); selecting one replaces the
// "@word" token with a non-editable mention span that serializes back to a
// mention node, so the whole path is round-trip safe.

const EMPTY_DOC: ADFNode = { type: "doc", version: 1, content: [{ type: "paragraph", content: [] }] };

// Escapes for interpolation into innerHTML, including quotes so values placed
// inside double-quoted attributes (e.g. data-id="...") cannot break out of the
// attribute. Mention attrs.id/text are attacker-controllable (not validated to
// be UUIDs by the API), so every attribute value MUST pass through this.
function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// ── ADF doc → HTML (hydrate the contentEditable) ─────────────────────────────

function inlineToHtml(nodes: ADFNode[] | undefined): string {
  if (!nodes) return "";
  return nodes
    .map((n) => {
      if (n.type === "mention") {
        const id = String(n.attrs?.id ?? "");
        const text = String(n.attrs?.text ?? "");
        const label = text.startsWith("@") ? text : `@${text}`;
        return `<span data-mention data-id="${escapeHtml(id)}" contenteditable="false" class="rounded bg-[#0052cc]/10 px-1 py-0.5 font-medium text-[#0052cc]">${escapeHtml(label)}</span>`;
      }
      if (n.type !== "text") return "";
      let html = escapeHtml(n.text ?? "");
      for (const m of n.marks ?? []) {
        if (m.type === "strong") html = `<strong>${html}</strong>`;
        else if (m.type === "em") html = `<em>${html}</em>`;
        else if (m.type === "code") html = `<code>${html}</code>`;
      }
      return html;
    })
    .join("");
}

function blockToHtml(node: ADFNode): string {
  switch (node.type) {
    case "paragraph": {
      const inner = inlineToHtml(node.content);
      return `<p>${inner || "<br>"}</p>`;
    }
    case "heading": {
      const level = Math.min(3, Math.max(1, Number(node.attrs?.level ?? 3)));
      return `<h${level}>${inlineToHtml(node.content) || "<br>"}</h${level}>`;
    }
    case "bulletList":
      return `<ul>${(node.content ?? []).map(blockToHtml).join("")}</ul>`;
    case "orderedList":
      return `<ol>${(node.content ?? []).map(blockToHtml).join("")}</ol>`;
    case "listItem": {
      // A listItem holds block content (usually a single paragraph); the editor
      // renders it as a flat <li> so execCommand list handling stays sane.
      const inline = (node.content ?? [])
        .map((c) => (c.type === "paragraph" || c.type === "heading" ? inlineToHtml(c.content) : blockToHtml(c)))
        .join("");
      return `<li>${inline || "<br>"}</li>`;
    }
    default:
      return "";
  }
}

function adfToHtml(doc: ADFNode | null): string {
  if (!doc || !doc.content || doc.content.length === 0) return "<p><br></p>";
  return doc.content.map(blockToHtml).join("");
}

// ── DOM → ADF doc (serialize on input) ───────────────────────────────────────

type Mark = "strong" | "em" | "code";

function pushText(out: ADFNode[], text: string, marks: Mark[]) {
  if (text === "") return;
  const node: ADFNode = { type: "text", text };
  if (marks.length) node.marks = marks.map((m) => ({ type: m }));
  out.push(node);
}

function serializeInline(node: Node, marks: Mark[], out: ADFNode[]) {
  if (node.nodeType === Node.TEXT_NODE) {
    pushText(out, node.textContent ?? "", marks);
    return;
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return;
  const el = node as HTMLElement;
  const tag = el.tagName.toLowerCase();

  if (el.hasAttribute("data-mention")) {
    const id = el.getAttribute("data-id") ?? "";
    const text = el.textContent ?? "";
    out.push({ type: "mention", attrs: { id, text } });
    return;
  }
  if (tag === "br") return;

  const next: Mark[] = [...marks];
  const style = el.style;
  if (tag === "b" || tag === "strong" || /^(bold|[6-9]00)$/.test(style.fontWeight)) {
    if (!next.includes("strong")) next.push("strong");
  }
  if (tag === "i" || tag === "em" || style.fontStyle === "italic") {
    if (!next.includes("em")) next.push("em");
  }
  if (tag === "code") {
    if (!next.includes("code")) next.push("code");
  }
  el.childNodes.forEach((child) => serializeInline(child, next, out));
}

const BLOCK_TAGS = new Set(["p", "div", "h1", "h2", "h3", "ul", "ol"]);

function serializeBlock(el: HTMLElement): ADFNode | ADFNode[] {
  const tag = el.tagName.toLowerCase();
  if (tag === "ul" || tag === "ol") {
    const items: ADFNode[] = [];
    el.querySelectorAll(":scope > li").forEach((li) => {
      const inline: ADFNode[] = [];
      li.childNodes.forEach((child) => serializeInline(child, [], inline));
      items.push({ type: "listItem", content: [{ type: "paragraph", content: inline }] });
    });
    return { type: tag === "ul" ? "bulletList" : "orderedList", content: items };
  }
  if (tag === "h1" || tag === "h2" || tag === "h3") {
    const inline: ADFNode[] = [];
    el.childNodes.forEach((child) => serializeInline(child, [], inline));
    return { type: "heading", attrs: { level: Number(tag[1]) }, content: inline };
  }
  // p / div → paragraph
  const inline: ADFNode[] = [];
  el.childNodes.forEach((child) => serializeInline(child, [], inline));
  return { type: "paragraph", content: inline };
}

function serialize(root: HTMLElement): ADFNode {
  const content: ADFNode[] = [];
  let inlineBuffer: ADFNode[] = [];
  const flush = () => {
    if (inlineBuffer.length) {
      content.push({ type: "paragraph", content: inlineBuffer });
      inlineBuffer = [];
    }
  };
  root.childNodes.forEach((child) => {
    if (child.nodeType === Node.ELEMENT_NODE && BLOCK_TAGS.has((child as HTMLElement).tagName.toLowerCase())) {
      flush();
      const block = serializeBlock(child as HTMLElement);
      if (Array.isArray(block)) content.push(...block);
      else content.push(block);
    } else {
      serializeInline(child, [], inlineBuffer);
    }
  });
  flush();
  if (content.length === 0) content.push({ type: "paragraph", content: [] });
  return { type: "doc", version: 1, content };
}

function isEmptyDoc(doc: ADFNode): boolean {
  return (doc.content ?? []).every(
    (n) => n.type === "paragraph" && (n.content ?? []).length === 0
  );
}

// ── Component ────────────────────────────────────────────────────────────────

interface Props {
  valueAdf: ADFNode | null;
  onChangeAdf: (doc: ADFNode) => void;
  placeholder?: string;
  /** When set, @mention search is scoped to the project's assignable users. */
  projectKey?: string;
  ariaLabel?: string;
  testId?: string;
}

interface MentionUser {
  accountId: string;
  displayName: string;
}

// Detects an in-progress "@word" token immediately before a collapsed caret.
// The "@" must start the string or follow whitespace (so `foo@bar` mid-word
// doesn't trigger); the token may contain word chars only, so any space closes
// it. Returns the text node plus the char offsets of "@" and the caret.
function detectMention(root: HTMLElement): { node: Text; at: number; caret: number; query: string } | null {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || !sel.isCollapsed) return null;
  const range = sel.getRangeAt(0);
  const node = range.startContainer;
  if (node.nodeType !== Node.TEXT_NODE || !root.contains(node)) return null;
  const caret = range.startOffset;
  const before = (node.textContent ?? "").slice(0, caret);
  const m = before.match(/(?:^|\s)@(\w*)$/);
  if (!m) return null;
  const query = m[1];
  return { node: node as Text, at: caret - query.length - 1, caret, query };
}

export function RichTextEditor({ valueAdf, onChangeAdf, placeholder, projectKey, ariaLabel, testId }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [empty, setEmpty] = useState(true);

  // ── @mention autocomplete state ────────────────────────────────────────────
  const [mQuery, setMQuery] = useState<string | null>(null); // null = closed
  const [mResults, setMResults] = useState<MentionUser[]>([]);
  const [mActive, setMActive] = useState(0);
  const [mPos, setMPos] = useState<{ top: number; left: number }>({ top: 0, left: 0 });
  // The detected token location, captured at input time so a later click on the
  // dropdown (which doesn't move the caret) can replace exactly that token.
  const tokenRef = useRef<{ node: Text; at: number; caret: number } | null>(null);
  const searchSeq = useRef(0);

  function closeMention() {
    setMQuery(null);
    setMResults([]);
    setMActive(0);
    tokenRef.current = null;
  }

  // Debounced user search whenever the query changes.
  useEffect(() => {
    if (mQuery === null) return;
    const q = mQuery;
    const seq = ++searchSeq.current;
    const t = setTimeout(async () => {
      try {
        const raw = projectKey
          ? await users.assignableSearch(projectKey, q)
          : await profile.searchUsers(q);
        if (seq !== searchSeq.current) return; // stale response
        setMResults(raw.map((u) => ({ accountId: u.accountId, displayName: u.displayName })));
        setMActive(0);
      } catch {
        if (seq === searchSeq.current) setMResults([]);
      }
    }, 200);
    return () => clearTimeout(t);
  }, [mQuery, projectKey]);

  function refreshMention() {
    const el = ref.current;
    if (!el) return;
    const hit = detectMention(el);
    if (!hit) {
      if (mQuery !== null) closeMention();
      return;
    }
    tokenRef.current = { node: hit.node, at: hit.at, caret: hit.caret };
    setMQuery(hit.query);
    // Position the dropdown just under the "@word" token.
    try {
      const r = document.createRange();
      r.setStart(hit.node, hit.at);
      r.setEnd(hit.node, hit.caret);
      const rect = r.getBoundingClientRect();
      const base = el.getBoundingClientRect();
      setMPos({ top: rect.bottom - base.top, left: rect.left - base.left });
    } catch {
      /* positioning best-effort */
    }
  }

  function selectMention(user: MentionUser) {
    const el = ref.current;
    const tok = tokenRef.current;
    if (!el || !tok) {
      closeMention();
      return;
    }
    el.focus();
    // Replace the "@query" slice with a non-editable mention span + a space.
    const range = document.createRange();
    range.setStart(tok.node, tok.at);
    range.setEnd(tok.node, tok.caret);
    range.deleteContents();

    const span = document.createElement("span");
    span.setAttribute("data-mention", "");
    span.setAttribute("data-id", user.accountId);
    span.setAttribute("contenteditable", "false");
    span.className = "rounded bg-[#0052cc]/10 px-1 py-0.5 font-medium text-[#0052cc]";
    span.textContent = `@${user.displayName}`;
    const space = document.createTextNode(" ");
    // insertNode prepends at the range start, so insert space first then span.
    range.insertNode(space);
    range.insertNode(span);

    // Place the caret after the trailing space.
    const sel = window.getSelection();
    if (sel) {
      const after = document.createRange();
      after.setStartAfter(space);
      after.collapse(true);
      sel.removeAllRanges();
      sel.addRange(after);
    }
    closeMention();
    emit();
  }

  // Hydrate once on mount from the initial ADF. The editor is uncontrolled
  // afterwards (contentEditable owns its DOM); to re-seed, remount via a React
  // `key`. This avoids clobbering the caret on every keystroke.
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.innerHTML = adfToHtml(valueAdf ?? EMPTY_DOC);
    try {
      document.execCommand("styleWithCSS", false, String(false));
    } catch {
      /* not supported in some environments; serializer handles inline styles anyway */
    }
    setEmpty(isEmptyDoc(valueAdf ?? EMPTY_DOC));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function emit() {
    const el = ref.current;
    if (!el) return;
    const doc = serialize(el);
    setEmpty(isEmptyDoc(doc) && el.textContent?.trim() === "");
    onChangeAdf(doc);
  }

  function onInput() {
    emit();
    refreshMention();
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (mQuery === null) return;
    if (e.key === "Escape") {
      e.preventDefault();
      closeMention();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setMActive((i) => (mResults.length ? (i + 1) % mResults.length : 0));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setMActive((i) => (mResults.length ? (i - 1 + mResults.length) % mResults.length : 0));
    } else if (e.key === "Enter" || e.key === "Tab") {
      if (mResults.length) {
        e.preventDefault();
        selectMention(mResults[Math.min(mActive, mResults.length - 1)]);
      }
    }
  }

  function exec(command: string, arg?: string) {
    ref.current?.focus();
    document.execCommand(command, false, arg);
    emit();
  }

  function toggleCode() {
    const sel = window.getSelection();
    ref.current?.focus();
    if (!sel || sel.rangeCount === 0 || sel.isCollapsed) {
      emit();
      return;
    }
    const range = sel.getRangeAt(0);
    const code = document.createElement("code");
    try {
      range.surroundContents(code);
    } catch {
      // Selection spans multiple nodes: fall back to wrapping its text.
      const text = range.toString();
      range.deleteContents();
      code.textContent = text;
      range.insertNode(code);
    }
    sel.removeAllRanges();
    emit();
  }

  const btn =
    "rounded px-2 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100";

  return (
    <div className="rounded border border-slate-300 focus-within:border-[#0052cc] focus-within:ring-2 focus-within:ring-[#0052cc]/20">
      <div className="flex flex-wrap items-center gap-1 border-b border-slate-200 px-1.5 py-1" data-testid="rich-editor-toolbar">
        {/* preventDefault on mousedown keeps the editor selection while clicking. */}
        <button type="button" aria-label="Bold" title="Bold" className={btn} onMouseDown={(e) => e.preventDefault()} onClick={() => exec("bold")}>
          <b>B</b>
        </button>
        <button type="button" aria-label="Italic" title="Italic" className={btn} onMouseDown={(e) => e.preventDefault()} onClick={() => exec("italic")}>
          <i>I</i>
        </button>
        <button type="button" aria-label="Code" title="Code" className={`${btn} font-mono`} onMouseDown={(e) => e.preventDefault()} onClick={toggleCode}>
          {"</>"}
        </button>
        <button type="button" aria-label="Bullet list" title="Bullet list" className={btn} onMouseDown={(e) => e.preventDefault()} onClick={() => exec("insertUnorderedList")}>
          • List
        </button>
        <button type="button" aria-label="Heading" title="Heading" className={btn} onMouseDown={(e) => e.preventDefault()} onClick={() => exec("formatBlock", "h3")}>
          H3
        </button>
      </div>
      <div className="relative">
        {empty && placeholder && (
          <span className="pointer-events-none absolute left-3 top-2 text-sm text-slate-400">{placeholder}</span>
        )}
        <div
          ref={ref}
          role="textbox"
          aria-label={ariaLabel ?? placeholder}
          aria-multiline="true"
          data-testid={testId ?? "rich-editor"}
          contentEditable
          suppressContentEditableWarning
          onInput={onInput}
          onKeyDown={onKeyDown}
          // Closing on blur lets clicks outside dismiss the dropdown; dropdown
          // items preventDefault on mousedown so selecting one doesn't blur.
          onBlur={() => closeMention()}
          className="prose prose-sm min-h-[6rem] max-w-none px-3 py-2 text-sm focus:outline-none [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_code]:rounded [&_code]:bg-slate-100 [&_code]:px-1"
        />
        {mQuery !== null && mResults.length > 0 && (
          <ul
            data-testid="mention-autocomplete"
            className="absolute z-20 max-h-52 w-64 overflow-auto rounded border border-slate-200 bg-white py-1 text-sm shadow-lg"
            style={{ top: mPos.top, left: mPos.left }}
          >
            {mResults.map((u, i) => (
              <li key={u.accountId}>
                <button
                  type="button"
                  // Keep the editor's caret/selection while clicking.
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => selectMention(u)}
                  className={`flex w-full items-center px-3 py-1.5 text-left ${
                    i === mActive ? "bg-[#0052cc]/10 text-[#0052cc]" : "text-slate-700 hover:bg-slate-100"
                  }`}
                >
                  {u.displayName}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
