package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

// withResolver stubs LookupIP for the test with a fixed host → addresses map.
// A host not in the map is a hard error, so a test can never fall through to
// real DNS unnoticed.
func withResolver(t *testing.T, table map[string][]string) {
	t.Helper()
	prev := LookupIP
	LookupIP = func(_ context.Context, host string) ([]net.IPAddr, error) {
		ips, ok := table[host]
		if !ok {
			return nil, fmt.Errorf("stub resolver: unexpected lookup of %q", host)
		}
		out := make([]net.IPAddr, 0, len(ips))
		for _, s := range ips {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out, nil
	}
	t.Cleanup(func() { LookupIP = prev })
}

func TestValidateTargetURL(t *testing.T) {
	withResolver(t, map[string][]string{
		"example.com":       {"93.184.216.34"},
		"forgejo.home.arpa": {"192.168.1.58"},
		"mixed.example.com": {"93.184.216.34", "10.0.0.5"},
		"v6.example.com":    {"2606:2800:220:1:248:1893:25c8:1946"},
		"mapped.example":    {"::ffff:10.1.2.3"},
	})

	cases := []struct {
		name    string
		raw     string
		allow   []string
		wantErr error // nil = accepted; ErrPrivateTarget / ErrInvalidTargetURL otherwise
	}{
		// rejected targets
		{"loopback v4", "http://127.0.0.1:8080", nil, ErrPrivateTarget},
		{"link-local metadata", "http://169.254.169.254/latest/meta-data", nil, ErrPrivateTarget},
		{"private 10/8", "http://10.0.0.1", nil, ErrPrivateTarget},
		{"private 192.168/16", "http://192.168.1.58:3000", nil, ErrPrivateTarget},
		{"private 172.16/12", "http://172.31.255.254", nil, ErrPrivateTarget},
		{"loopback v6", "http://[::1]", nil, ErrPrivateTarget},
		{"link-local v6", "http://[fe80::1]", nil, ErrPrivateTarget},
		{"unique-local v6", "http://[fd00::1]", nil, ErrPrivateTarget},
		{"unspecified v4", "http://0.0.0.0", nil, ErrPrivateTarget},
		{"unspecified v6", "http://[::]", nil, ErrPrivateTarget},
		{"multicast v4", "http://224.0.0.1", nil, ErrPrivateTarget},
		{"ipv4-mapped v6 literal", "http://[::ffff:127.0.0.1]", nil, ErrPrivateTarget},
		{"ipv4-mapped v6 via dns", "http://mapped.example", nil, ErrPrivateTarget},
		{"cgnat 100.64/10", "http://100.64.0.1", nil, ErrPrivateTarget},
		{"ipv4-translated v6 (::ffff:0:0/96) embedding loopback", "http://[::ffff:0:7f00:1]/", nil, ErrPrivateTarget},
		{"this-network 0.0.0.0/8 (not just 0.0.0.0)", "http://0.0.0.1/", nil, ErrPrivateTarget},
		{"ipv4-compatible v6 (::/96) embedding loopback", "http://[::127.0.0.1]/", nil, ErrPrivateTarget},
		{"ipv4-compatible v6 (::/96) embedding private", "http://[::10.0.0.1]/", nil, ErrPrivateTarget},
		{"site-local v6 (fec0::/10)", "http://[fec0::1]/", nil, ErrPrivateTarget},
		{"localhost by name", "http://localhost:9000", nil, ErrPrivateTarget},
		{"localhost subdomain", "http://api.localhost", nil, ErrPrivateTarget},
		{"any private among resolved", "http://mixed.example.com", nil, ErrPrivateTarget},
		{"userinfo", "http://user:pw@example.com", nil, ErrInvalidTargetURL},
		{"ftp scheme", "ftp://example.com", nil, ErrInvalidTargetURL},
		{"file scheme", "file:///etc/passwd", nil, ErrInvalidTargetURL},
		{"no scheme", "localhost", nil, ErrInvalidTargetURL},
		{"empty", "", nil, ErrInvalidTargetURL},
		{"empty host", "http:///path", nil, ErrInvalidTargetURL},
		{"ipv6 zone", "http://[fe80::1%25eth0]", nil, ErrInvalidTargetURL},
		{"private despite unrelated allow", "http://10.0.0.1", []string{"192.168.1.0/24", "forgejo.home.arpa"}, ErrPrivateTarget},

		// accepted targets
		{"public https", "https://example.com", nil, nil},
		{"public v6", "https://v6.example.com", nil, nil},
		{"public literal", "https://93.184.216.34/hook", nil, nil},
		{"private ip in allowed cidr", "http://192.168.1.58:3000", []string{"192.168.1.0/24"}, nil},
		{"private ip equals allowed ip", "http://192.168.1.58:3000", []string{"192.168.1.58"}, nil},
		{"allowed hostname resolving private", "http://forgejo.home.arpa", []string{"forgejo.home.arpa"}, nil},
		{"allowed hostname case-insensitive", "http://Forgejo.Home.Arpa/x", []string{"forgejo.home.arpa"}, nil},
		{"cidr covers resolved private ip", "http://forgejo.home.arpa", []string{"192.168.0.0/16"}, nil},
		{"loopback allowed by cidr", "http://127.0.0.1:8080", []string{"127.0.0.0/8"}, nil},
		{"allowlist entries trimmed", "http://10.0.0.1", []string{" 10.0.0.0/8 "}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTargetURL(tc.raw, tc.allow)
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("ValidateTargetURL(%q, %v) = %v, want accepted", tc.raw, tc.allow, err)
			case tc.wantErr != nil && err == nil:
				t.Fatalf("ValidateTargetURL(%q, %v) = nil, want %v", tc.raw, tc.allow, tc.wantErr)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("ValidateTargetURL(%q, %v) = %v, want errors.Is %v", tc.raw, tc.allow, err, tc.wantErr)
			}
		})
	}
}

// TestValidateTargetURL_ResolverFailureRejects: a hostname that cannot be
// resolved is not accepted "by default".
func TestValidateTargetURL_ResolverFailureRejects(t *testing.T) {
	withResolver(t, map[string][]string{})
	if err := ValidateTargetURL("https://nxdomain.example", nil); err == nil {
		t.Fatal("unresolvable host accepted")
	}
}
