package v3

import "testing"

func TestJiraFilter_Shape(t *testing.T) {
	owner := &User{AccountID: "u1", DisplayName: "Ada"}
	f := JiraFilter(FilterInput{
		ID: "f1", Name: "My open", Description: "desc", JQL: "project = DEMO",
		Favourite: true, Owner: owner, BaseURL: "http://x",
	})
	if f.ID != "f1" || f.Name != "My open" || f.JQL != "project = DEMO" {
		t.Errorf("campi base errati: %+v", f)
	}
	if !f.Favourite {
		t.Error("favourite deve essere true")
	}
	if f.Self == "" || f.SearchURL == "" || f.ViewURL == "" {
		t.Error("self/searchUrl/viewUrl devono essere valorizzati")
	}
	if f.SharePermissions == nil || f.EditPermissions == nil {
		t.Error("sharePermissions/editPermissions devono essere array non-nil (anche vuoti)")
	}
}
