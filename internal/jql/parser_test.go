package jql

import "testing"

func TestParse_SingleClause(t *testing.T) {
	q, err := Parse(`project = DEMO`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	c, ok := q.Where.(*Clause)
	if !ok {
		t.Fatalf("atteso *Clause, got %T", q.Where)
	}
	if c.Field != "project" || c.Op != "=" || c.Value != "DEMO" {
		t.Errorf("clause errata: %+v", c)
	}
}

func TestParse_AndOrPrecedence(t *testing.T) {
	// AND lega più forte di OR: a=1 OR b=2 AND c=3  =>  Or(a=1, And(b=2, c=3))
	q, err := Parse(`a = 1 OR b = 2 AND c = 3`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	or, ok := q.Where.(Or)
	if !ok {
		t.Fatalf("atteso Or alla radice, got %T", q.Where)
	}
	if _, ok := or.Right.(And); !ok {
		t.Errorf("atteso And come figlio destro di Or, got %T", or.Right)
	}
}

func TestParse_Parens(t *testing.T) {
	q, _ := Parse(`(a = 1 OR b = 2) AND c = 3`)
	and, ok := q.Where.(And)
	if !ok {
		t.Fatalf("atteso And alla radice, got %T", q.Where)
	}
	if _, ok := and.Left.(Or); !ok {
		t.Errorf("atteso Or a sinistra, got %T", and.Left)
	}
}

func TestParse_InList(t *testing.T) {
	q, _ := Parse(`status IN (Done, "In Progress", Blocked)`)
	c := q.Where.(*Clause)
	if c.Op != "IN" {
		t.Fatalf("op atteso IN, got %q", c.Op)
	}
	if len(c.Values) != 3 || c.Values[1] != "In Progress" {
		t.Errorf("values errati: %v", c.Values)
	}
}

func TestParse_NotIn(t *testing.T) {
	q, _ := Parse(`status NOT IN (Done)`)
	c := q.Where.(*Clause)
	if c.Op != "NOT IN" || len(c.Values) != 1 {
		t.Errorf("NOT IN errato: %+v", c)
	}
}

func TestParse_IsEmpty(t *testing.T) {
	q, _ := Parse(`assignee IS EMPTY`)
	c := q.Where.(*Clause)
	if c.Op != "IS" || !c.IsEmpty {
		t.Errorf("IS EMPTY errato: %+v", c)
	}
	q2, _ := Parse(`resolution IS NOT EMPTY`)
	c2 := q2.Where.(*Clause)
	if c2.Op != "IS NOT" || !c2.IsEmpty {
		t.Errorf("IS NOT EMPTY errato: %+v", c2)
	}
}

func TestParse_Function(t *testing.T) {
	q, _ := Parse(`assignee = currentUser()`)
	c := q.Where.(*Clause)
	if c.Func != "currentUser" {
		t.Errorf("funzione non riconosciuta: %+v", c)
	}
}

func TestParse_OrderBy(t *testing.T) {
	q, _ := Parse(`project = DEMO ORDER BY priority DESC, created ASC`)
	if len(q.Order) != 2 {
		t.Fatalf("attese 2 chiavi order, %d", len(q.Order))
	}
	if q.Order[0].Field != "priority" || !q.Order[0].Desc {
		t.Errorf("order[0] errata: %+v", q.Order[0])
	}
	if q.Order[1].Field != "created" || q.Order[1].Desc {
		t.Errorf("order[1] errata: %+v", q.Order[1])
	}
}

func TestParse_OrderByOnly(t *testing.T) {
	q, err := Parse(`ORDER BY created DESC`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if q.Where != nil {
		t.Errorf("Where deve essere nil, got %T", q.Where)
	}
	if len(q.Order) != 1 {
		t.Errorf("attesa 1 chiave order")
	}
}

func TestParse_EmptyQuery(t *testing.T) {
	q, err := Parse(``)
	if err != nil {
		t.Fatalf("Parse vuota: %v", err)
	}
	if q.Where != nil || len(q.Order) != 0 {
		t.Errorf("query vuota deve dare Where nil e nessun order")
	}
}

func TestParse_Errors(t *testing.T) {
	for _, bad := range []string{`project =`, `= DEMO`, `project == DEMO`, `status IN Done`, `(a = 1`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("attesa err per %q", bad)
		}
	}
}
