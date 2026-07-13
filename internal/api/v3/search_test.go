package v3

import "testing"

func TestCursor_RoundTrip(t *testing.T) {
	tok := EncodeCursor(40)
	off, err := DecodeCursor(tok)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if off != 40 {
		t.Errorf("offset roundtrip: got %d", off)
	}
}

func TestCursor_EmptyIsZero(t *testing.T) {
	off, err := DecodeCursor("")
	if err != nil || off != 0 {
		t.Errorf("token vuoto deve dare offset 0, got %d err %v", off, err)
	}
}

func TestCursor_Invalid(t *testing.T) {
	if _, err := DecodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("atteso errore per token non valido")
	}
}
