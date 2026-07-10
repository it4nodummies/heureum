package v3

import (
	"net/http"
	"strconv"
	"strings"
)

// ParsePagination legge startAt/maxResults con default e cap in stile Jira.
// Esempio: ParsePagination(r, 50, 100) — default 50, cap 100.
func ParsePagination(r *http.Request, defaultMax, capMax int) (startAt, maxResults int) {
	q := r.URL.Query()
	startAt, _ = strconv.Atoi(q.Get("startAt"))
	if startAt < 0 {
		startAt = 0
	}
	maxResults, err := strconv.Atoi(q.Get("maxResults"))
	if err != nil || maxResults <= 0 {
		maxResults = defaultMax
	}
	if maxResults > capMax {
		maxResults = capMax
	}
	return startAt, maxResults
}

// Expand è l'insieme dei valori richiesti nel query param expand.
type Expand struct {
	raw   string
	items map[string]bool
}

func ParseExpand(r *http.Request) Expand {
	raw := r.URL.Query().Get("expand")
	items := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			items[p] = true
		}
	}
	return Expand{raw: raw, items: items}
}

func (e Expand) Has(name string) bool { return e.items[name] }
func (e Expand) String() string       { return e.raw }

// Fields modella il query param fields. Lo zero value (nessun parametro)
// include tutti i campi, come Jira.
type Fields struct {
	limited  bool // true = solo i campi elencati in include
	include  map[string]bool
	excluded map[string]bool
}

func ParseFields(r *http.Request) Fields {
	raw := r.URL.Query().Get("fields")
	if raw == "" {
		return Fields{}
	}
	f := Fields{include: map[string]bool{}, excluded: map[string]bool{}}
	f.limited = true
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		switch {
		case p == "":
		case p == "*all" || p == "*navigable":
			f.limited = false
		case strings.HasPrefix(p, "-"):
			f.excluded[strings.TrimSpace(p[1:])] = true
		default:
			f.include[p] = true
		}
	}
	return f
}

func (f Fields) Include(name string) bool {
	if f.excluded[name] {
		return false
	}
	if !f.limited {
		return true
	}
	return f.include[name]
}
