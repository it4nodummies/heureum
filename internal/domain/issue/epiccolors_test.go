package issue

import "testing"

func TestEpicColorHex_PaletteCoversEveryKeyAndIsDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, k := range EpicColorKeys {
		hex, ok := EpicColorHexes[k]
		if !ok {
			t.Fatalf("no hex for %s", k)
		}
		if prev, dup := seen[hex]; dup {
			t.Errorf("%s and %s share %s", prev, k, hex)
		}
		seen[hex] = k
		if got := EpicColorHex(&k); got != hex {
			t.Errorf("EpicColorHex(%s) = %s, want %s", k, got, hex)
		}
	}
	if len(EpicColorHexes) != len(EpicColorKeys) {
		t.Errorf("palette has %d entries, keys %d", len(EpicColorHexes), len(EpicColorKeys))
	}
	unknown := "color_99"
	if EpicColorHex(nil) != "#8B5CF6" || EpicColorHex(&unknown) != "#8B5CF6" {
		t.Errorf("nil/unknown must fall back to color_1 purple")
	}
}
