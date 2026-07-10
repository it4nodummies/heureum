package project

import "testing"

func TestTemplateKeyForType(t *testing.T) {
	cases := map[Type]string{
		TypeScrum:    "com.pyxis.greenhopper.jira:gh-scrum-template",
		TypeKanban:   "com.pyxis.greenhopper.jira:gh-kanban-template",
		TypeBusiness: "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control",
	}
	for typ, want := range cases {
		if got := TemplateKeyForType(typ); got != want {
			t.Errorf("TemplateKeyForType(%q) = %q, want %q", typ, got, want)
		}
	}
}

func TestTypeForTemplateKey(t *testing.T) {
	if TypeForTemplateKey("com.pyxis.greenhopper.jira:gh-kanban-template") != TypeKanban {
		t.Error("kanban template must map to TypeKanban")
	}
	if TypeForTemplateKey("unknown") != TypeScrum {
		t.Error("unknown template must default to TypeScrum")
	}
}

func TestProjectTypeKeyForType(t *testing.T) {
	if ProjectTypeKeyForType(TypeScrum) != "software" || ProjectTypeKeyForType(TypeKanban) != "software" {
		t.Error("scrum/kanban must map to software")
	}
	if ProjectTypeKeyForType(TypeBusiness) != "business" {
		t.Error("business must map to business")
	}
}
