// Epic colour palette (Round 28). Mirrors internal/domain/issue/epiccolors.go
// EXACTLY — the backend stores one of these KEYS on the epic (`color.key` in
// the agile epic shape) and both sides must paint the same hex so an epic
// looks identical on cards, lozenges, the backlog panel and the timeline.
// Keep the two maps in sync when touching either.

export const EPIC_COLORS: Record<string, string> = {
  color_1: "#8B5CF6", // purple
  color_2: "#3B82F6", // blue
  color_3: "#10B981", // green
  color_4: "#F59E0B", // amber
  color_5: "#EF4444", // red
  color_6: "#EC4899", // pink
  color_7: "#14B8A6", // teal
  color_8: "#6366F1", // indigo
  color_9: "#84CC16", // lime
  color_10: "#F97316", // orange
  color_11: "#06B6D4", // cyan
  color_12: "#D946EF", // fuchsia
  color_13: "#EAB308", // yellow
  color_14: "#64748B", // slate
};

export const DEFAULT_EPIC_COLOR_KEY = "color_1";

// Palette keys in display order (color_1 … color_14).
export const EPIC_COLOR_KEYS: string[] = Object.keys(EPIC_COLORS);

// Hex for an epic colour key; falls back to color_1 when the key is missing or
// unknown (pre-R28 epics before the startup backfill, or a parent ref that
// doesn't carry a colour).
export function epicColorHex(key?: string | null): string {
  return (key && EPIC_COLORS[key]) || EPIC_COLORS[DEFAULT_EPIC_COLOR_KEY];
}
