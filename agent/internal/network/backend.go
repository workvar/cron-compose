package network

import (
	"context"
	"strings"
)

// DetectBackend prefers NetworkManager (Raspberry Pi OS default), then netplan.
func DetectBackend(ctx context.Context) string {
	if has("nmcli") {
		if out, err := run(ctx, "nmcli", "-t", "-f", "RUNNING", "general"); err == nil {
			if strings.EqualFold(strings.TrimSpace(out), "running") {
				return BackendNetworkManager
			}
		}
		// nmcli present but NM may still be usable for read-only queries.
		if _, err := run(ctx, "nmcli", "general"); err == nil {
			return BackendNetworkManager
		}
	}
	if has("netplan") {
		return BackendNetplan
	}
	return BackendUnavailable
}

// ProbeCapabilities returns Hello-style capability strings for the host.
func ProbeCapabilities(ctx context.Context) []string {
	caps := []string{}
	b := DetectBackend(ctx)
	if b == BackendUnavailable {
		return caps
	}
	caps = append(caps, "network")
	if has("bluetoothctl") {
		caps = append(caps, "network.bluetooth")
	}
	if has("mmcli") {
		caps = append(caps, "network.cellular")
	}
	if dual := probeDualWifi(ctx); dual.Supported {
		caps = append(caps, "network.dual_wifi")
	}
	return caps
}
