package contract

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/webhook"
)

// TestMain pins the outgoing-webhook resolver for the whole package: contract
// tests register hooks on example.com-style hosts and must not depend on real
// DNS (CI may be offline). Every hostname resolves to a public address, so the
// SSRF policy accepts it the way it would in production.
func TestMain(m *testing.M) {
	webhook.LookupIP = func(_ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	os.Exit(m.Run())
}
