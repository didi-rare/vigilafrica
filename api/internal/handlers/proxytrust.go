package handlers

import (
	"bufio"
	"encoding/hex"
	"log/slog"
	"net"
	"os"
	"strings"
)

// procNetRoute is the Linux routing table. Overridable so the parser can be
// tested without a container, and so a non-Linux dev host degrades quietly
// instead of pretending it measured something.
const procNetRoute = "/proc/net/route"

// DefaultGatewayIP returns the container's IPv4 default gateway, or "" when it
// cannot be determined (non-Linux host, unreadable file, no default route).
//
// This is the address the API actually sees as its peer for traffic arriving
// through a published port: the host-side proxy connects from the bridge
// gateway, so `r.RemoteAddr` is the gateway — measured on the production host
// on 2026-08-18 as "[::ffff:172.19.0.1]:38090", read from inside the API
// container's own network namespace.
func DefaultGatewayIP() string {
	return defaultGatewayIPFrom(procNetRoute)
}

func defaultGatewayIPFrom(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "" // header only
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// Iface Destination Gateway Flags RefCnt Use Metric Mask ...
		if len(fields) < 3 {
			continue
		}
		// The default route is the one whose destination is 0.0.0.0.
		if fields[1] != "00000000" {
			continue
		}
		ip := parseProcRouteAddr(fields[2])
		if ip == "" || ip == "0.0.0.0" {
			continue
		}
		return ip
	}
	return ""
}

// parseProcRouteAddr decodes one /proc/net/route address field. The kernel
// prints these as little-endian hex, so the four hex bytes appear in reverse
// order: "010013AC" is 172.19.0.1, not 1.0.19.172. Reading it the other way
// round yields a plausible-looking but wrong address — precisely the class of
// silent error this task exists to eliminate, so it is pinned by a test using
// the value measured on the production host.
func parseProcRouteAddr(field string) string {
	raw, err := hex.DecodeString(field)
	if err != nil || len(raw) != 4 {
		return ""
	}
	return net.IPv4(raw[3], raw[2], raw[1], raw[0]).String()
}

// VerifyGatewayTrusted checks that the container's own default gateway falls
// inside the configured trusted-proxy set, and reports what it found.
//
// ⚠️ This is the drift detector for TRUSTED_PROXY_CIDRS. The compose files pin
// each stack's subnet so the gateway is deterministic, but a pin can be removed
// or a network recreated onto a different pool range. If that happens the
// failure is otherwise SILENT: the API stops believing Caddy, falls back to the
// private peer address, fails to geolocate it, and returns a null location —
// while /health still reports ok and nothing is logged. This turns that into a
// loud ERROR at startup.
//
// It returns true when the gateway is trusted, false when it is not, and true
// when the gateway cannot be determined at all (nothing was proven, so do not
// raise a false alarm — that case is logged at debug).
func VerifyGatewayTrusted(cfg ProxyConfig, logger *slog.Logger) bool {
	return verifyGatewayTrusted(DefaultGatewayIP(), cfg, logger)
}

// verifyGatewayTrusted is the pure core, taking the gateway as an argument so
// it can be tested. Reading the real one inside the test container would read
// the *test* container's gateway, which is unrelated to the deployed value —
// an early version of this file asserted through that and reported a failure
// that meant nothing.
func verifyGatewayTrusted(gateway string, cfg ProxyConfig, logger *slog.Logger) bool {
	if logger == nil {
		logger = slog.Default()
	}

	configured := make([]string, 0, len(cfg.Trusted))
	for _, n := range cfg.Trusted {
		configured = append(configured, n.String())
	}

	if gateway == "" {
		logger.Debug("trusted-proxy check skipped: no default gateway could be read",
			"trusted_proxy_cidrs", configured)
		return true
	}

	if isTrustedProxy(gateway, cfg.Trusted) {
		logger.Info("trusted-proxy check passed: the default gateway is trusted",
			"gateway", gateway, "trusted_proxy_cidrs", configured)
		return true
	}

	logger.Error("trusted-proxy check FAILED: the default gateway is NOT in TRUSTED_PROXY_CIDRS — "+
		"forwarded headers from the reverse proxy will be ignored, so client geolocation will return "+
		"null and every caller will share one rate-limit bucket",
		"gateway", gateway, "trusted_proxy_cidrs", configured)
	return false
}
