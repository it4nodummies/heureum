package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ErrInvalidTargetURL: the URL is malformed, uses a scheme other than
// http/https, carries userinfo, or has no usable host.
var ErrInvalidTargetURL = errors.New("invalid webhook URL")

// ErrPrivateTarget: the host resolves to (or is) a loopback, private,
// link-local, multicast, unspecified or otherwise non-public address that is
// not covered by the operator allowlist. Outgoing webhooks are a blind SSRF
// vector against cluster-internal targets (k8s API, registries, metadata
// endpoints), so these are refused at create AND at delivery.
var ErrPrivateTarget = errors.New("webhook target resolves to a non-public address")

// LookupIP resolves a hostname to its addresses. Package-level seam so tests
// can exercise the SSRF policy without real DNS; production uses the default
// resolver. Literal IPs never go through it.
var LookupIP = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// resolveTimeout bounds the DNS lookup done by the policy.
const resolveTimeout = 5 * time.Second

// ValidateTargetURL applies the outgoing-webhook target policy to raw:
//   - scheme http or https, non-empty host, no userinfo;
//   - the host (literal or resolved via LookupIP) must not yield ANY
//     non-public address (see isDisallowedIP) — unless the host itself is in
//     allow, or every such address falls inside an allowed CIDR / equals an
//     allowed IP.
//
// allow is the operator escape hatch (APP_WEBHOOK_ALLOW_PRIVATE_HOSTS): a list
// of hostnames, IPs and/or CIDRs. Errors wrap ErrInvalidTargetURL or
// ErrPrivateTarget; callers facing clients should map both to a generic
// message rather than echoing resolution details.
func ValidateTargetURL(raw string, allow []string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTargetURL, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("%w: scheme %q not allowed", ErrInvalidTargetURL, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: userinfo not allowed", ErrInvalidTargetURL)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrInvalidTargetURL)
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	_, err = resolveAllowed(ctx, host, parseAllowlist(allow))
	return err
}

// resolveAllowed resolves host and applies the policy, returning the
// addresses the caller may connect to. It is shared by ValidateTargetURL
// (pre-flight) and the delivery client's dialer (connect time), so the policy
// that was checked is the policy that is dialed.
func resolveAllowed(ctx context.Context, host string, allow allowlist) ([]net.IP, error) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return nil, fmt.Errorf("%w: empty host", ErrInvalidTargetURL)
	}
	// IPv6 zone identifiers (fe80::1%eth0) name a local interface: never a
	// legitimate webhook target and they would bypass the literal-IP parse.
	if strings.Contains(host, "%") {
		return nil, fmt.Errorf("%w: zone identifier not allowed", ErrInvalidTargetURL)
	}

	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			if !allow.hostAllowed(host) {
				return nil, fmt.Errorf("%w (host %q)", ErrPrivateTarget, host)
			}
		}
		addrs, err := LookupIP(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("%w: resolve %q: %v", ErrInvalidTargetURL, host, err)
		}
		for _, a := range addrs {
			ips = append(ips, a.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: %q resolves to no address", ErrInvalidTargetURL, host)
	}
	// An allowlisted hostname is trusted wherever it points (homelab DNS →
	// 192.168.x.x). Otherwise every address must be public or inside an
	// allowed CIDR: one private answer among public ones is a rejection.
	if allow.hostAllowed(host) {
		return ips, nil
	}
	for _, ip := range ips {
		if isDisallowedIP(ip) && !allow.ipAllowed(ip) {
			return nil, fmt.Errorf("%w (host %q)", ErrPrivateTarget, host)
		}
	}
	return ips, nil
}

// extraDisallowed covers non-public ranges that net.IP's predicates do not:
// CGNAT, IETF protocol assignments, benchmarking, reserved/broadcast, and the
// NAT64 well-known prefix (which embeds an IPv4 address and could reach
// 127.0.0.1 through a translator).
var extraDisallowed = mustCIDRs(
	"100.64.0.0/10",
	"192.0.0.0/24",
	"198.18.0.0/15",
	"240.0.0.0/4",
	"64:ff9b::/96",
)

// isDisallowedIP reports whether ip is loopback, private, link-local,
// multicast, unspecified or in extraDisallowed. IPv4-mapped IPv6 addresses
// are checked as their IPv4 form.
func isDisallowedIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, n := range extraDisallowed {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// allowlist is the parsed form of APP_WEBHOOK_ALLOW_PRIVATE_HOSTS: hostnames
// (matched case-insensitively against the URL host) and IP networks (a bare
// IP is a /32 or /128).
type allowlist struct {
	hosts map[string]struct{}
	nets  []*net.IPNet
}

func parseAllowlist(entries []string) allowlist {
	a := allowlist{hosts: map[string]struct{}{}}
	for _, e := range entries {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(e); err == nil {
			a.nets = append(a.nets, n)
			continue
		}
		if ip := net.ParseIP(e); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			} else {
				ip = ip.To4()
			}
			a.nets = append(a.nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		a.hosts[strings.TrimSuffix(e, ".")] = struct{}{}
	}
	return a
}

func (a allowlist) hostAllowed(host string) bool {
	_, ok := a.hosts[strings.ToLower(strings.TrimSuffix(host, "."))]
	return ok
}

func (a allowlist) ipAllowed(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, n := range a.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func mustCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic("webhook: bad built-in CIDR " + c)
		}
		out = append(out, n)
	}
	return out
}
