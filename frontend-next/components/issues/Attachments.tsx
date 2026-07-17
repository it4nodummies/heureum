"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { attachments as attachmentsApi, Attachment } from "@/lib/api";

interface Props {
  issueKey: string;
}

function humanSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function Attachments({ issueKey }: Props) {
  const qc = useQueryClient();
  const listKey = ["issue", issueKey, "attachments"];
  const { data, isLoading } = useQuery({
    queryKey: listKey,
    queryFn: () => attachmentsApi.list(issueKey),
  });

  const [isDragging, setIsDragging] = useState(false);
  const [pendingNames, setPendingNames] = useState<string[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const upload = useMutation({
    mutationFn: (file: File) => attachmentsApi.upload(issueKey, file),
    onMutate: (file) => setPendingNames((names) => [...names, file.name]),
    onSettled: (_data, _err, file) =>
      setPendingNames((names) => {
        const idx = names.indexOf(file.name);
        if (idx === -1) return names;
        const next = names.slice();
        next.splice(idx, 1);
        return next;
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: listKey }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => attachmentsApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: listKey }),
  });

  function uploadFiles(files: FileList | File[]) {
    Array.from(files).forEach((file) => upload.mutate(file));
  }

  function onDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files.length) uploadFiles(e.dataTransfer.files);
  }

  const items = data ?? [];

  return (
    <section className="mt-8" data-testid="attachments-section">
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">Attachments</h2>

      <div
        data-testid="attachments-dropzone"
        onDragOver={(e) => {
          e.preventDefault();
          setIsDragging(true);
        }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={onDrop}
        onClick={() => fileInputRef.current?.click()}
        role="button"
        tabIndex={0}
        className={`mb-4 flex cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed px-6 py-8 text-center transition-colors ${
          isDragging ? "border-[#0052cc] bg-[#0052cc]/5" : "border-slate-200 hover:border-slate-300"
        }`}
      >
        <p className="text-sm text-slate-500">
          Drag and drop files here, or <span className="font-semibold text-[#0052cc]">browse</span>
        </p>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          aria-label="Upload attachment"
          className="hidden"
          onChange={(e) => {
            if (e.target.files?.length) uploadFiles(e.target.files);
            e.target.value = "";
          }}
        />
      </div>

      {pendingNames.length > 0 && (
        <ul className="mb-3 space-y-1">
          {pendingNames.map((name, i) => (
            <li key={`${name}-${i}`} className="text-xs text-slate-400">
              Uploading {name}…
            </li>
          ))}
        </ul>
      )}

      {upload.isError && (
        <p className="mb-3 text-xs text-red-600">
          {upload.error instanceof Error ? upload.error.message : "Failed to upload file."}
        </p>
      )}

      {!isLoading && items.length === 0 && pendingNames.length === 0 && (
        <p className="text-sm text-slate-400">No attachments yet.</p>
      )}

      {items.length > 0 && (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
          {items.map((att) => (
            <AttachmentCard
              key={att.id}
              attachment={att}
              onDelete={() => remove.mutate(att.id)}
              deleting={remove.isPending && remove.variables === att.id}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

function AttachmentCard({
  attachment,
  onDelete,
  deleting,
}: {
  attachment: Attachment;
  onDelete: () => void;
  deleting: boolean;
}) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const isImage = attachment.mimeType.startsWith("image/");

  useEffect(() => {
    if (!isImage) return;
    let objectUrl: string | null = null;
    let cancelled = false;
    attachmentsApi
      .contentBlobUrl(attachment)
      .then((url) => {
        if (cancelled) {
          URL.revokeObjectURL(url);
          return;
        }
        objectUrl = url;
        setPreviewUrl(url);
      })
      .catch(() => {
        /* preview is best-effort — fall back to the generic file icon */
      });
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attachment.id, isImage]);

  async function download() {
    const url = await attachmentsApi.contentBlobUrl(attachment);
    const a = document.createElement("a");
    a.href = url;
    a.download = attachment.filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  return (
    <li
      data-testid={`attachment-card-${attachment.id}`}
      className="flex flex-col overflow-hidden rounded-xl border border-slate-200 bg-white"
    >
      <div className="flex h-24 items-center justify-center bg-slate-50">
        {isImage && previewUrl ? (
          // eslint-disable-next-line @next/next/no-img-element -- previewUrl is a blob: object URL, next/image can't optimize those
          <img src={previewUrl} alt={attachment.filename} className="h-full w-full object-cover" />
        ) : (
          <span className="text-3xl text-slate-300" aria-hidden>
            📄
          </span>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-0.5 p-2">
        <span className="truncate text-xs font-medium text-[#1a1f36]" title={attachment.filename}>
          {attachment.filename}
        </span>
        <span className="text-[11px] text-slate-400">
          {humanSize(attachment.size)} · {formatDate(attachment.created)}
        </span>
        <div className="mt-1 flex gap-2 text-xs">
          <button onClick={download} className="text-[#0052cc] hover:underline">
            Download
          </button>
          <button
            onClick={onDelete}
            disabled={deleting}
            aria-label={`Delete ${attachment.filename}`}
            className="text-slate-400 hover:text-red-600 disabled:opacity-60"
          >
            {deleting ? "Deleting…" : "Delete"}
          </button>
        </div>
      </div>
    </li>
  );
}
