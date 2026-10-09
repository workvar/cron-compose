package network

import (
	"context"
	"fmt"
	"strings"
)

func probeDualWifi(ctx context.Context) DualWifiInfo {
	if !has("iw") {
		return DualWifiInfo{Supported: false, Detail: "iw not installed"}
	}
	out, err := run(ctx, "iw", "list")
	if err != nil {
		return DualWifiInfo{Supported: false, Detail: err.Error()}
	}
	// Look for multiple interface combination slots that include two station modes,
	// or any indication the phy can host >1 managed interface.
	lower := strings.ToLower(out)
	if strings.Contains(lower, "interface combinations") {
		// Heuristic: station count >= 2 in combination lines.
		if strings.Contains(lower, "station <= 2") ||
			strings.Contains(lower, "station <= 3") ||
			strings.Contains(lower, "station <= 4") ||
			strings.Contains(lower, "#{ managed } <= 2") ||
			(strings.Contains(lower, "#{station") && strings.Contains(lower, "<= 2")) {
			return DualWifiInfo{Supported: true, Detail: "phy reports multi-station"}
		}
	}
	// Some Pi chipsets allow adding a second station even when combinations are vague;
	// we still surface unsupported until a live create succeeds during connect.
	return DualWifiInfo{Supported: false, Detail: "single-station phy (or unknown combinations)"}
}

// wifiConnectAlongside brings up a second Wi‑Fi connection on a virtual STA iface
// without taking down the primary association.
func wifiConnectAlongside(ctx context.Context, a Args) error {
	primary := nmWifiDevice(ctx)
	if primary == "" {
		return fmt.Errorf("no wifi device")
	}
	phy, err := wifiPhyForIface(ctx, primary)
	if err != nil {
		return err
	}
	virt := "wlan-cc1"
	// Clean up a stale virtual iface from a previous attempt.
	_, _ = mustPriv(ctx, "iw", "dev", virt, "del")
	if _, err := mustPriv(ctx, "iw", "phy", phy, "interface", "add", virt, "type", "station"); err != nil {
		return fmt.Errorf("dual_wifi_unsupported: %w", err)
	}
	name := a.Connection
	if name == "" {
		name = a.SSID
	}
	if name == "" {
		return fmt.Errorf("connection or ssid is required")
	}
	conName := name + "-alongside"
	if !nmConnExists(ctx, conName) {
		args := []string{
			"connection", "add", "type", "wifi", "con-name", conName,
			"ifname", virt, "ssid", a.SSID,
		}
		if a.SSID == "" {
			// Resolve SSID from the named connection.
			ssid := nmConnSSID(ctx, name)
			if ssid == "" {
				ssid = name
			}
			args = []string{
				"connection", "add", "type", "wifi", "con-name", conName,
				"ifname", virt, "ssid", ssid,
			}
			if a.PSK != "" {
				args = append(args, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", a.PSK)
			} else {
				// Copy PSK from existing profile when possible.
				psk, _ := run(ctx, "nmcli", "-s", "-g", "802-11-wireless-security.psk", "connection", "show", name)
				psk = strings.TrimSpace(psk)
				if psk != "" {
					args = append(args, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", psk)
				}
			}
		} else if a.PSK != "" {
			args = append(args, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", a.PSK)
		}
		args = append(args, "autoconnect", "no")
		if _, err := mustPriv(ctx, "nmcli", args...); err != nil {
			_, _ = mustPriv(ctx, "iw", "dev", virt, "del")
			return err
		}
	} else {
		_, _ = mustPriv(ctx, "nmcli", "connection", "modify", conName, "connection.interface-name", virt)
	}
	if _, err := mustPriv(ctx, "nmcli", "connection", "up", conName); err != nil {
		return fmt.Errorf("dual_wifi_unsupported: %w", err)
	}
	return nil
}

func wifiPhyForIface(ctx context.Context, iface string) (string, error) {
	out, err := run(ctx, "iw", "dev", iface, "info")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "wiphy") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return "phy" + fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("wiphy not found for %s", iface)
}
