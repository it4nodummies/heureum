package issue

// Epic colour palette (Round 28). Epics store one of Jira's colour KEYS
// (EpicColorKeys, color_1..color_14, see hierarchy.go); this file is the
// single key → hex mapping shared by every renderer (timeline bars, agile
// epic shapes, the frontend swatches), so an epic looks the same everywhere.
// The palette is Jira-like: 14 distinct hues, color_1 = purple (the colour
// the timeline hardcoded for every epic before R28).

// EpicColorHexes maps each key of EpicColorKeys to its hex colour.
var EpicColorHexes = map[string]string{
	"color_1":  "#8B5CF6", // purple
	"color_2":  "#3B82F6", // blue
	"color_3":  "#10B981", // green
	"color_4":  "#F59E0B", // amber
	"color_5":  "#EF4444", // red
	"color_6":  "#EC4899", // pink
	"color_7":  "#14B8A6", // teal
	"color_8":  "#6366F1", // indigo
	"color_9":  "#84CC16", // lime
	"color_10": "#F97316", // orange
	"color_11": "#06B6D4", // cyan
	"color_12": "#D946EF", // fuchsia
	"color_13": "#EAB308", // yellow
	"color_14": "#64748B", // slate
}

// DefaultEpicColorKey is the colour of an epic whose key is missing or
// unknown (pre-R28 rows before the startup backfill ran).
const DefaultEpicColorKey = "color_1"

// EpicColorHex returns the hex colour for an epic colour key, falling back
// to DefaultEpicColorKey's hex when key is nil, empty or not in the palette.
func EpicColorHex(key *string) string {
	if key != nil {
		if hex, ok := EpicColorHexes[*key]; ok {
			return hex
		}
	}
	return EpicColorHexes[DefaultEpicColorKey]
}
