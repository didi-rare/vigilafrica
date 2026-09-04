package handlers

import (
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// TestParseProcRouteAddrDecodesLittleEndian pins the byte order against the
// address actually measured on the production host on 2026-08-18. Big-endian
// decoding returns 1.0.19.172 here — a valid-looking address that would make
// the gateway check silently wrong rather than fail.
func TestParseProcRouteAddrDecodesLittleEndian(t *testing.T) {
	tests := map[string]string{
		"010013AC": "172.19.0.1", // production bridge gateway, measured
		"010012AC": "172.18.0.1", // staging bridge gateway, measured
		"00000000": "0.0.0.0",
		"0101A8C0": "192.168.1.1",
		"":         "",
		"ZZ":       "",
		"010013":   "",
	}
	for field, want := range tests {
		if got := parseProcRouteAddr(field); got != want {
			t.Errorf("parseProcRouteAddr(%q) = %q, want %q", field, got, want)
		}
	}
}

func writeRoute(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "route")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestDefaultGatewayIPFrom(t *testing.T) {
	const header = "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n"

	t.Run("reads the default route, skipping non-default entries", func(t *testing.T) {
		// The on-link subnet route comes first and has gateway 00000000; taking
		// the first row rather than the default row would return "".
		body := header +
			"eth0\t000013AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n" +
			"eth0\t00000000\t010013AC\t0003\t0\t0\t0\t00000000\t0\t0\t0\n"
		if got := defaultGatewayIPFrom(writeRoute(t, body)); got != "172.19.0.1" {
			t.Errorf("got %q, want 172.19.0.1", got)
		}
	})

	t.Run("returns empty when there is no default route", func(t *testing.T) {
		body := header + "eth0\t000013AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n"
		if got := defaultGatewayIPFrom(writeRoute(t, body)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns empty on a missing file rather than panicking", func(t *testing.T) {
		if got := defaultGatewayIPFrom(filepath.Join(t.TempDir(), "absent")); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns empty on a header-only file", func(t *testing.T) {
		if got := defaultGatewayIPFrom(writeRoute(t, header)); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func mustCIDRs(t *testing.T, cidrs ...string) ProxyConfig {
	t.Helper()
	var nets []*net.IPNet
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatalf("bad CIDR %q: %v", c, err)
		}
		nets = append(nets, n)
	}
	return ProxyConfig{Trusted: nets}
}

// TestIPv4MappedPeerMatchesIPv4CIDR is the regression test for the form the
// peer address ACTUALLY arrives in. Measured from inside the production API
// container, the peer is "[::ffff:172.19.0.1]:38090" — IPv4-mapped IPv6, not
// dotted quad. A narrowed /32 is only safe because net.IPNet.Contains
// normalizes that; any refactor that stops doing so would break geolocation in
// production while every unit test using plain IPv4 still passed.
func TestIPv4MappedPeerMatchesIPv4CIDR(t *testing.T) {
	cfg := mustCIDRs(t, "127.0.0.1/8", "::1/128", "172.19.0.1/32")

	trusted := []string{"::ffff:172.19.0.1", "172.19.0.1"}
	for _, peer := range trusted {
		if !isTrustedProxy(peer, cfg.Trusted) {
			t.Errorf("peer %q should be trusted by 172.19.0.1/32", peer)
		}
	}

	// The point of narrowing: everything else on the same bridge is excluded.
	// These are the real neighbours — prod-umami is publicly reachable through
	// Caddy, and under the previous 172.16.0.0/12 all of them were trusted.
	untrusted := []string{
		"::ffff:172.19.0.3", // prod-umami
		"172.19.0.4",        // prod-api itself
		"172.19.0.5",        // prod-db
		"172.18.0.1",        // the *staging* gateway
		"172.16.0.1",
	}
	for _, peer := range untrusted {
		if isTrustedProxy(peer, cfg.Trusted) {
			t.Errorf("peer %q must NOT be trusted by 172.19.0.1/32", peer)
		}
	}
}

func TestVerifyGatewayTrusted(t *testing.T) {
	discard := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	cfg := mustCIDRs(t, "127.0.0.1/8", "::1/128", "172.19.0.1/32")

	t.Run("passes when the gateway is the trusted one", func(t *testing.T) {
		if !verifyGatewayTrusted("172.19.0.1", cfg, discard) {
			t.Error("the measured production gateway should be trusted")
		}
	})

	t.Run("FAILS when the subnet has drifted", func(t *testing.T) {
		// The exact scenario the check exists for: the network is recreated on a
		// different pool range, the pin is gone, and nothing else would notice.
		if verifyGatewayTrusted("172.20.0.1", cfg, discard) {
			t.Error("a drifted gateway must be reported as untrusted")
		}
	})

	t.Run("FAILS for a bridge neighbour rather than the gateway", func(t *testing.T) {
		if verifyGatewayTrusted("172.19.0.3", cfg, discard) {
			t.Error("prod-umami's address must not pass the gateway check")
		}
	})

	t.Run("unreadable gateway is not treated as a failure", func(t *testing.T) {
		// Nothing was proven, so do not raise a false alarm.
		if !verifyGatewayTrusted("", cfg, discard) {
			t.Error("expected true when the gateway cannot be determined")
		}
	})
}
