package v3

import "testing"

func TestMakeTransition_Shape(t *testing.T) {
	tr := MakeTransition(TransitionInput{
		ID: "tr-1", Name: "Done", ToID: "st-done", ToName: "Done", ToCategory: "done",
		Available: true, BaseURL: "http://x",
	})
	if tr.ID != "tr-1" || tr.Name != "Done" {
		t.Errorf("campi base errati: %+v", tr)
	}
	if tr.To.ID != "st-done" || tr.To.Name != "Done" {
		t.Errorf("to errato: %+v", tr.To)
	}
	if tr.To.StatusCategory.Key != "done" {
		t.Errorf("statusCategory errata: %+v", tr.To.StatusCategory)
	}
	if !tr.IsAvailable {
		t.Error("isAvailable atteso true")
	}
	// campi booleani conformi presenti (default false)
	if tr.HasScreen || tr.IsGlobal || tr.IsInitial || tr.IsConditional || tr.Looped {
		t.Error("flag booleani non di default")
	}
}

func TestTransitions_Wrapper(t *testing.T) {
	ts := Transitions{Transitions: []IssueTransition{}}
	if ts.Transitions == nil {
		t.Error("transitions deve essere slice non-nil (anche vuoto)")
	}
}
