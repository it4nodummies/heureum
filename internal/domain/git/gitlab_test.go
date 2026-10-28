package git

import (
	"os"
	"strings"
	"testing"
)

// TestGitLab_ValidateWebhook_ConstantTime: GitLab authenticates with a shared
// token in X-Gitlab-Token. The comparison must be constant-time (a plain ==
// short-circuits on the first differing byte and leaks the token prefix) and
// must reject empty values on either side. The static check on the source
// pins the mechanism, since timing can't be asserted reliably in a unit test.
func TestGitLab_ValidateWebhook_ConstantTime(t *testing.T) {
	p := NewGitLabProvider()
	const secret = "gitlab-shared-token-0123456789"

	if valid, err := p.ValidateWebhook(nil, secret, secret); err != nil || !valid {
		t.Errorf("right token: valid=%v err=%v, want accepted", valid, err)
	}
	for _, wrong := range []string{"gitlab-shared-token-012345678X", "gitlab-shared-token-", "g", "", secret + "x"} {
		if valid, _ := p.ValidateWebhook(nil, secret, wrong); valid {
			t.Errorf("wrong token %q accepted; want reject", wrong)
		}
	}
	if valid, _ := p.ValidateWebhook(nil, "", ""); valid {
		t.Errorf("empty secret and empty token accepted; want reject")
	}

	src, err := os.ReadFile("gitlab.go")
	if err != nil {
		t.Fatalf("read gitlab.go: %v", err)
	}
	if !strings.Contains(string(src), "subtle.ConstantTimeCompare") {
		t.Errorf("gitlab.go does not use subtle.ConstantTimeCompare for the token check")
	}
	if strings.Contains(string(src), "signature == secret") {
		t.Errorf("gitlab.go still compares the token with == (not constant-time)")
	}
}
