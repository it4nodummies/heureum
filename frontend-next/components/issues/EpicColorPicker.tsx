"use client";

// 14-swatch colour picker shown on an epic's detail. Saves straight away via
// POST /rest/agile/1.0/epic/{key} { color: { key } } and invalidates the epic
// + issue queries so every lozenge pointing at this epic repaints.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { epics } from "@/lib/api";
import { DEFAULT_EPIC_COLOR_KEY, EPIC_COLORS, EPIC_COLOR_KEYS } from "@/lib/epicColors";

interface Props {
  epicKey: string;
}

export function EpicColorPicker({ epicKey }: Props) {
  const qc = useQueryClient();
  const { data: epic } = useQuery({
    queryKey: ["epic", epicKey],
    queryFn: () => epics.get(epicKey),
  });
  const current = epic?.color.key ?? DEFAULT_EPIC_COLOR_KEY;

  const setColor = useMutation({
    mutationFn: (key: string) => epics.update(epicKey, { color: { key } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["epic", epicKey] });
      qc.invalidateQueries({ queryKey: ["epics"] });
      qc.invalidateQueries({ queryKey: ["issue", epicKey] });
    },
  });

  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Epic colour</div>
      <div
        role="radiogroup"
        aria-label="Epic colour"
        data-testid="epic-color-picker"
        className="mt-1.5 grid grid-cols-7 gap-1.5"
      >
        {EPIC_COLOR_KEYS.map((key) => {
          const selected = key === current;
          return (
            <button
              key={key}
              type="button"
              role="radio"
              aria-checked={selected}
              aria-label={key.replace("_", " ")}
              title={key.replace("_", " ")}
              data-testid={`epic-color-${key}`}
              data-selected={selected ? "true" : undefined}
              disabled={setColor.isPending}
              onClick={() => {
                if (!selected) setColor.mutate(key);
              }}
              style={{ backgroundColor: EPIC_COLORS[key] }}
              className={`h-6 w-6 rounded-md transition focus:outline-none focus-visible:ring-2 focus-visible:ring-[#0052cc]/60 disabled:opacity-60 ${
                selected ? "ring-2 ring-offset-2 ring-[#1a1f36]" : "hover:scale-110"
              }`}
            />
          );
        })}
      </div>
      {setColor.isError && (
        <p className="mt-1 text-xs text-red-600">
          {setColor.error instanceof Error ? setColor.error.message : "Could not change the colour."}
        </p>
      )}
    </div>
  );
}
