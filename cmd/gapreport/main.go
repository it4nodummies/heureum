// gapreport confronta le route registrate in internal/api/router.go
// con i path dell'OpenAPI ufficiale e genera docs/contracts/gap-report.md.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Route struct {
	Method string
	Path   string
}

// methodPattern è l'unica definizione dei metodi HTTP riconosciuti:
// alimenta sia la regex di estrazione route sia il filtro sui path item OpenAPI.
const methodPattern = "GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS|TRACE"

var routeRe = regexp.MustCompile(`mux\.Handle(?:Func)?\(\s*"(` + methodPattern + `) ([^"]+)"`)
var paramRe = regexp.MustCompile(`\{[^}]+\}`)

var httpMethods = func() map[string]bool {
	m := map[string]bool{}
	for _, v := range strings.Split(methodPattern, "|") {
		m[v] = true
	}
	return m
}()

func extractRoutes(src string) []Route {
	var out []Route
	for _, m := range routeRe.FindAllStringSubmatch(src, -1) {
		out = append(out, Route{Method: m[1], Path: m[2]})
	}
	return out
}

func normalizePath(p string) string {
	return paramRe.ReplaceAllString(strings.TrimSuffix(p, "/"), "{}")
}

// normalizeKey normalizza la parte path di una chiave "METHOD /path".
func normalizeKey(k string) string {
	parts := strings.SplitN(k, " ", 2)
	if len(parts) != 2 {
		return k
	}
	return parts[0] + " " + normalizePath(parts[1])
}

type operation struct {
	Summary string `json:"summary"`
}

type spec struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func loadSpecRoutes(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := map[string]string{} // "METHOD /path" -> summary
	for p, ops := range s.Paths {
		for method, rawOp := range ops {
			m := strings.ToUpper(method)
			if !httpMethods[m] {
				// campi non-metodo a livello di path item, es. "parameters", "$ref", ecc.
				continue
			}
			var op operation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				fmt.Fprintf(os.Stderr, "warn: %s: cannot parse %s %s: %v\n", path, m, p, err)
				continue
			}
			out[m+" "+p] = op.Summary
		}
	}
	return out, nil
}

// diffRoutes confronta le route implementate con quelle del contratto.
// Le chiavi sono "METHOD /path"; i nomi dei parametri di path vengono
// normalizzati prima del confronto. Gli slice risultanti sono ordinati.
func diffRoutes(implemented map[string]bool, specs map[string]string) (matched, missing, extra []string) {
	implNorm := map[string]bool{}
	for k := range implemented {
		implNorm[normalizeKey(k)] = true
	}
	specNorm := map[string]bool{}
	for k := range specs {
		n := normalizeKey(k)
		specNorm[n] = true
		if implNorm[n] {
			matched = append(matched, k)
		} else {
			missing = append(missing, k)
		}
	}
	for k := range implemented {
		if !specNorm[normalizeKey(k)] {
			extra = append(extra, k)
		}
	}
	sort.Strings(matched)
	sort.Strings(missing)
	sort.Strings(extra)
	return matched, missing, extra
}

func main() {
	routerSrc, err := os.ReadFile("internal/api/router.go")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	implemented := map[string]bool{}
	for _, r := range extractRoutes(string(routerSrc)) {
		implemented[r.Method+" "+normalizePath(r.Path)] = true
	}

	specs := map[string]string{}
	for _, file := range []string{
		"docs/contracts/jira-platform-v3.json",
		"docs/contracts/jira-agile-1.0.json",
	} {
		m, err := loadSpecRoutes(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for k, v := range m {
			specs[k] = v
		}
	}

	matched, missing, extra := diffRoutes(implemented, specs)

	var b strings.Builder
	b.WriteString("# Gap report — endpoint vs OpenAPI ufficiale\n\n")
	b.WriteString("> Generato da `go run ./cmd/gapreport`. Non modificare a mano.\n\n")
	fmt.Fprintf(&b, "- Nel contratto e implementati (path match): **%d**\n", len(matched))
	fmt.Fprintf(&b, "- Nel contratto ma mancanti: **%d**\n", len(missing))
	fmt.Fprintf(&b, "- Implementati ma fuori contratto (estensioni): **%d**\n\n", len(extra))
	b.WriteString("## Mancanti (dal contratto)\n\n")
	for _, k := range missing {
		fmt.Fprintf(&b, "- `%s` — %s\n", k, specs[k])
	}
	b.WriteString("\n## Estensioni fuori contratto\n\n")
	for _, k := range extra {
		fmt.Fprintf(&b, "- `%s`\n", k)
	}
	if err := os.WriteFile("docs/contracts/gap-report.md", []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("matched=%d missing=%d extra=%d → docs/contracts/gap-report.md\n",
		len(matched), len(missing), len(extra))
}
