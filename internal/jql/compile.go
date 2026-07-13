package jql

import (
	"fmt"
	"strings"
)

// Resolver traduce i nomi Jira in id interni. Implementato dal dominio
// (search.DBResolver) per tenere questo package disaccoppiato dal DB.
type Resolver interface {
	ProjectID(keyOrID string) (string, bool)
	UserID(login string) (string, bool)
	CurrentUserID() string
}

// Compiled è il risultato: clausola WHERE con placeholder ? e relativi
// argomenti, più la clausola ORDER BY (senza "ORDER BY").
type Compiled struct {
	Where string
	Args  []any
	Order string
}

// priorityNames normalizza i nomi di priorità JQL all'enum interno.
var priorityNames = map[string]string{
	"highest": "highest", "high": "high", "medium": "medium",
	"low": "low", "lowest": "lowest",
}

// orderColumns mappa i campi JQL ordinabili alle colonne SQL.
var orderColumns = map[string]string{
	"priority": "priority", "created": "created_at", "updated": "updated_at",
	"summary": "title", "status": "status_id", "key": "seq_id",
	"assignee": "assignee_id", "duedate": "due_date",
}

// Compile trasforma un *Query in una Compiled. Restituisce errore se un campo
// o un valore non è risolvibile.
func Compile(q *Query, r Resolver) (*Compiled, error) {
	out := &Compiled{}
	if q.Where != nil {
		where, args, err := compileNode(q.Where, r)
		if err != nil {
			return nil, err
		}
		out.Where = where
		out.Args = args
	}
	if len(q.Order) > 0 {
		parts := make([]string, 0, len(q.Order))
		for _, k := range q.Order {
			col, ok := orderColumns[k.Field]
			if !ok {
				return nil, fmt.Errorf("campo di ordinamento non supportato: %s", k.Field)
			}
			dir := "ASC"
			if k.Desc {
				dir = "DESC"
			}
			parts = append(parts, col+" "+dir)
		}
		out.Order = strings.Join(parts, ", ")
	}
	return out, nil
}

func compileNode(n Node, r Resolver) (string, []any, error) {
	switch t := n.(type) {
	case And:
		return compileBinary(t.Left, t.Right, "AND", r)
	case Or:
		return compileBinary(t.Left, t.Right, "OR", r)
	case Not:
		w, a, err := compileNode(t.Inner, r)
		if err != nil {
			return "", nil, err
		}
		return "NOT (" + w + ")", a, nil
	case *Clause:
		return compileClause(t, r)
	default:
		return "", nil, fmt.Errorf("nodo sconosciuto %T", n)
	}
}

func compileBinary(l, rt Node, op string, r Resolver) (string, []any, error) {
	lw, la, err := compileNode(l, r)
	if err != nil {
		return "", nil, err
	}
	rw, ra, err := compileNode(rt, r)
	if err != nil {
		return "", nil, err
	}
	return "(" + lw + " " + op + " " + rw + ")", append(la, ra...), nil
}

func compileClause(c *Clause, r Resolver) (string, []any, error) {
	switch c.Field {
	case "project":
		return resolvedEq("project_id", c, r.ProjectID)
	case "status":
		return nameSubqueryClause("status_id", "workflow_statuses", c)
	case "type", "issuetype":
		return nameSubqueryClause("type_id", "issue_types", c)
	case "assignee":
		return userClause("assignee_id", c, r)
	case "reporter":
		return userClause("reporter_id", c, r)
	case "priority":
		return priorityClause(c)
	case "resolution":
		// solo IS EMPTY / IS NOT EMPTY (risolte vs non risolte)
		return nullableClause("resolution_id", c)
	case "labels":
		return labelsClause(c)
	case "summary":
		return textClause("title", c)
	case "text":
		return freeTextClause(c)
	case "key":
		return keyClause(c)
	case "created", "updated":
		return dateClause(c)
	default:
		return "", nil, fmt.Errorf("campo non supportato: %s", c.Field)
	}
}

// resolvedEq gestisce =, !=, IN, NOT IN, IS EMPTY per un campo id risolto per nome.
func resolvedEq(col string, c *Clause, resolve func(string) (string, bool)) (string, []any, error) {
	if c.IsEmpty {
		return nullableClause(col, c)
	}
	switch c.Op {
	case "=", "!=":
		id, ok := resolve(c.Value)
		if !ok {
			return "", nil, fmt.Errorf("valore non trovato per %s: %q", col, c.Value)
		}
		sqlOp := "="
		if c.Op == "!=" {
			sqlOp = "!="
		}
		return col + " " + sqlOp + " ?", []any{id}, nil
	case "IN", "NOT IN":
		ids := make([]any, 0, len(c.Values))
		for _, v := range c.Values {
			id, ok := resolve(v)
			if !ok {
				return "", nil, fmt.Errorf("valore non trovato per %s: %q", col, v)
			}
			ids = append(ids, id)
		}
		return inClause(col, c.Op, ids), ids, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per %s", c.Op, col)
	}
}

func userClause(col string, c *Clause, r Resolver) (string, []any, error) {
	if c.IsEmpty {
		return nullableClause(col, c)
	}
	if c.Func == "currentUser" {
		op := "="
		if c.Op == "!=" {
			op = "!="
		}
		return col + " " + op + " ?", []any{r.CurrentUserID()}, nil
	}
	return resolvedEq(col, c, r.UserID)
}

func priorityClause(c *Clause) (string, []any, error) {
	norm := func(v string) (string, bool) {
		n, ok := priorityNames[strings.ToLower(v)]
		return n, ok
	}
	switch c.Op {
	case "=", "!=":
		p, ok := norm(c.Value)
		if !ok {
			return "", nil, fmt.Errorf("priorità sconosciuta: %q", c.Value)
		}
		return "priority " + c.Op + " ?", []any{p}, nil
	case "IN", "NOT IN":
		args := make([]any, 0, len(c.Values))
		for _, v := range c.Values {
			p, ok := norm(v)
			if !ok {
				return "", nil, fmt.Errorf("priorità sconosciuta: %q", v)
			}
			args = append(args, p)
		}
		return inClause("priority", c.Op, args), args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per priority", c.Op)
	}
}

func nullableClause(col string, c *Clause) (string, []any, error) {
	// IS EMPTY => IS NULL; IS NOT EMPTY => IS NOT NULL
	if c.Op == "IS NOT" || (c.Op == "!=" && c.IsEmpty) {
		return col + " IS NOT NULL", nil, nil
	}
	return col + " IS NULL", nil, nil
}

// nameSubqueryClause gestisce campi il cui valore è un id risolto per NOME in una
// tabella per-progetto (stati, tipi). Usa una subquery che abbraccia tutti i
// progetti: i nomi di stato/tipo non sono globalmente unici, quindi risolverli a
// un singolo id darebbe risultati errati in ambienti multi-progetto.
func nameSubqueryClause(col, table string, c *Clause) (string, []any, error) {
	if c.IsEmpty {
		return nullableClause(col, c)
	}
	switch c.Op {
	case "=":
		return col + " IN (SELECT id FROM " + table + " WHERE name = ?)", []any{c.Value}, nil
	case "!=":
		return col + " NOT IN (SELECT id FROM " + table + " WHERE name = ?)", []any{c.Value}, nil
	case "IN", "NOT IN":
		ph := strings.TrimSuffix(strings.Repeat("?,", len(c.Values)), ",")
		args := make([]any, len(c.Values))
		for i, v := range c.Values {
			args[i] = v
		}
		op := "IN"
		if c.Op == "NOT IN" {
			op = "NOT IN"
		}
		return col + " " + op + " (SELECT id FROM " + table + " WHERE name IN (" + ph + "))", args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per %s", c.Op, c.Field)
	}
}

func labelsClause(c *Clause) (string, []any, error) {
	sub := "EXISTS (SELECT 1 FROM issue_labels il JOIN labels l ON l.id = il.label_id " +
		"WHERE il.issue_id = issues.id AND l.name = ?)"
	switch c.Op {
	case "=":
		return sub, []any{c.Value}, nil
	case "!=":
		return "NOT " + sub, []any{c.Value}, nil
	case "IN":
		clauses := make([]string, 0, len(c.Values))
		args := make([]any, 0, len(c.Values))
		for _, v := range c.Values {
			clauses = append(clauses, sub)
			args = append(args, v)
		}
		return "(" + strings.Join(clauses, " OR ") + ")", args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per labels", c.Op)
	}
}

func textClause(col string, c *Clause) (string, []any, error) {
	if c.Op != "~" && c.Op != "!~" {
		return "", nil, fmt.Errorf("summary supporta solo ~ e !~")
	}
	neg := ""
	if c.Op == "!~" {
		neg = "NOT "
	}
	return neg + col + " LIKE ?", []any{"%" + c.Value + "%"}, nil
}

func freeTextClause(c *Clause) (string, []any, error) {
	if c.Op != "~" && c.Op != "!~" {
		return "", nil, fmt.Errorf("text supporta solo ~ e !~")
	}
	like := "%" + c.Value + "%"
	base := "(title LIKE ? OR description_json LIKE ?)"
	if c.Op == "!~" {
		return "NOT " + base, []any{like, like}, nil
	}
	return base, []any{like, like}, nil
}

func keyClause(c *Clause) (string, []any, error) {
	switch c.Op {
	case "=", "!=":
		return "key " + c.Op + " ?", []any{c.Value}, nil
	case "IN", "NOT IN":
		args := make([]any, len(c.Values))
		for i, v := range c.Values {
			args[i] = v
		}
		return inClause("key", c.Op, args), args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per key", c.Op)
	}
}

func dateClause(c *Clause) (string, []any, error) {
	col := "created_at"
	if c.Field == "updated" {
		col = "updated_at"
	}
	switch c.Op {
	case "=", "!=", ">", ">=", "<", "<=":
		return col + " " + c.Op + " ?", []any{c.Value}, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per %s", c.Op, c.Field)
	}
}

func inClause(col, op string, args []any) string {
	ph := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	if op == "NOT IN" {
		return col + " NOT IN (" + ph + ")"
	}
	return col + " IN (" + ph + ")"
}
