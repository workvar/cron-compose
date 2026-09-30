package transport

import (
	"net"
	"strings"
)

// DefaultPort is used when an address is a bare hostname such as "grpc.example.com".
// The hostname's edge (a tunnel or proxy) decides which backend port it reaches, so
// the agent does not need that port spelled out.
const DefaultPort = "443"

// NormalizeAddr returns addr as host:port, adding DefaultPort to a bare host or IP.
// An empty address, or one with a port but no host (":9077"), is returned unchanged
// so callers can still reject it.
func NormalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return addr
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	if strings.HasPrefix(addr, ":") && net.ParseIP(addr) == nil {
		return addr
	}
	return net.JoinHostPort(strings.Trim(addr, "[]"), DefaultPort)
}

// HasHost reports whether addr names a host, with or without a port.
func HasHost(addr string) bool {
	if strings.ContainsAny(strings.TrimSpace(addr), " \t/") {
		return false
	}
	host, _, err := net.SplitHostPort(NormalizeAddr(addr))
	return err == nil && host != ""
}
