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

var routeRe = regexp.MustCompile(`mux\.Handle(?:Func)?\(\s*"(GET|POST|PUT|PATCH|DELETE) ([^"]+)"`)
var paramRe = regexp.MustCompile(`\{[^}]+\}`)

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

type operation struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

type spec struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

var httpMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "TRACE": true,
}

func loadSpecRoutes(path, prefix string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	out := map[string]string{} // "METHOD /path" -> summary
	for p, ops := range s.Paths {
		for method, raw := range ops {
			m := strings.ToUpper(method)
			if !httpMethods[m] {
				// campi non-metodo a livello di path item, es. "parameters", "$ref", ecc.
				continue
			}
			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				continue
			}
			out[m+" "+prefix+p] = op.Summary
		}
	}
	return out, nil
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
	for _, s := range []struct{ file, prefix string }{
		{"docs/contracts/jira-platform-v3.json", ""},
		{"docs/contracts/jira-agile-1.0.json", ""},
	} {
		m, err := loadSpecRoutes(s.file, s.prefix)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for k, v := range m {
			specs[k] = v
		}
	}

	var matched, missing, extra []string
	for k := range specs {
		parts := strings.SplitN(k, " ", 2)
		if implemented[parts[0]+" "+normalizePath(parts[1])] {
			matched = append(matched, k)
		} else {
			missing = append(missing, k)
		}
	}
	specNorm := map[string]bool{}
	for k := range specs {
		parts := strings.SplitN(k, " ", 2)
		specNorm[parts[0]+" "+normalizePath(parts[1])] = true
	}
	for k := range implemented {
		if !specNorm[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(matched)
	sort.Strings(missing)
	sort.Strings(extra)

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
