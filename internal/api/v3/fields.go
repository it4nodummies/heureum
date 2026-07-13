package v3

import "encoding/json"

// ProjectIssue serializza un IssueBean in map[string]any applicando la
// selezione dei fields. id/key/self restano sempre a top-level; la selezione
// agisce solo sul sotto-oggetto "fields". Fields vuoto o con "*all" => tutti.
func ProjectIssue(bean IssueBean, f Fields) (map[string]any, error) {
	raw, err := json.Marshal(bean)
	if err != nil {
		return nil, err
	}
	var full map[string]any
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, err
	}
	if f.includeAll() {
		return full, nil
	}
	fieldsMap, _ := full["fields"].(map[string]any)
	if fieldsMap == nil {
		return full, nil
	}
	pruned := make(map[string]any, len(fieldsMap))
	for k, v := range fieldsMap {
		if f.Include(k) {
			pruned[k] = v
		}
	}
	full["fields"] = pruned
	return full, nil
}

// ParseFieldsFromList costruisce un Fields da una lista esplicita (per i body
// POST /search che passano fields come []string).
func ParseFieldsFromList(list []string) Fields {
	return newFields(list)
}
