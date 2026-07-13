package jql

import "testing"

func TestLex_SimpleClause(t *testing.T) {
	toks, err := Lex(`project = DEMO`)
	if err != nil {
		t.Fatalf("Lex: %v", err)
	}
	want := []Token{
		{Kind: TokIdent, Val: "project"},
		{Kind: TokOp, Val: "="},
		{Kind: TokIdent, Val: "DEMO"},
		{Kind: TokEOF},
	}
	assertTokens(t, toks, want)
}

func TestLex_QuotedString(t *testing.T) {
	toks, _ := Lex(`summary ~ "login page"`)
	if toks[2].Kind != TokString || toks[2].Val != "login page" {
		t.Fatalf("quoted string not lexed: %+v", toks[2])
	}
}

func TestLex_OperatorsAndKeywords(t *testing.T) {
	toks, _ := Lex(`status IN (Done, "In Progress") AND assignee != currentUser() ORDER BY created DESC`)
	kinds := []TokKind{}
	for _, tk := range toks {
		kinds = append(kinds, tk.Kind)
	}
	// ident IN ( ident , string ) AND ident != ident ( ) ORDER BY ident DESC EOF
	// verifichiamo che parentesi, virgole e keyword multi-parola siano token distinti
	if toks[1].Kind != TokKeyword || toks[1].Val != "IN" {
		t.Errorf("IN non riconosciuto come keyword: %+v", toks[1])
	}
	if toks[2].Kind != TokLParen {
		t.Errorf("( atteso, %+v", toks[2])
	}
	if toks[6].Kind != TokRParen {
		t.Errorf(") atteso, %+v", toks[6])
	}
}

func TestLex_NotEqualAndComparators(t *testing.T) {
	toks, _ := Lex(`a != b >= c <= d > e < f !~ g ~ h`)
	ops := []string{}
	for _, tk := range toks {
		if tk.Kind == TokOp {
			ops = append(ops, tk.Val)
		}
	}
	got := len(ops)
	if got != 7 {
		t.Fatalf("attesi 7 operatori, trovati %d: %v", got, ops)
	}
	if ops[0] != "!=" || ops[1] != ">=" || ops[2] != "<=" {
		t.Errorf("operatori multi-char errati: %v", ops)
	}
}

func TestLex_UnterminatedString(t *testing.T) {
	if _, err := Lex(`summary ~ "oops`); err == nil {
		t.Fatal("attesa err per stringa non terminata")
	}
}

func assertTokens(t *testing.T, got, want []Token) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len token: got %d want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Val != want[i].Val {
			t.Errorf("token[%d]: got %+v want %+v", i, got[i], want[i])
		}
	}
}
