package v3

import "testing"

func TestAutocompleteData_HasCoreFields(t *testing.T) {
	d := AutocompleteData()
	names := map[string]bool{}
	for _, f := range d.VisibleFieldNames {
		names[f.Value] = true
	}
	for _, want := range []string{"project", "status", "assignee", "priority", "summary", "labels"} {
		if !names[want] {
			t.Errorf("campo %q mancante in autocomplete", want)
		}
	}
	// orderable/searchable devono essere stringhe "true"/"false" come da contratto
	for _, f := range d.VisibleFieldNames {
		if f.Orderable != "true" && f.Orderable != "false" {
			t.Errorf("orderable non stringa-booleana per %s: %q", f.Value, f.Orderable)
		}
	}
	found := false
	for _, fn := range d.VisibleFunctionNames {
		if fn.Value == "currentUser()" {
			found = true
		}
	}
	if !found {
		t.Error("funzione currentUser() mancante")
	}
}
