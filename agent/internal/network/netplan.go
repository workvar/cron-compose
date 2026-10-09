package network

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func netplanStatus(ctx context.Context) (Status, error) {
	st := Status{
		Backend:      BackendNetplan,
		Interfaces:   []Interface{},
		Wifi:         WifiStatus{Saved: []WifiNetwork{}},
		Bluetooth:    btStatus(ctx),
		Cellular:     CellularStatus{Modems: []Modem{}},
		Capabilities: ProbeCapabilities(ctx),
		DualWifi:     DualWifiInfo{Supported: false, Detail: "dual wifi requires NetworkManager"},
	}
	ifaces, err := ipJSONIfaces(ctx)
	if err != nil {
		st.Error = err.Error()
		return st, err
	}
	st.Interfaces = ifaces
	st.DefaultRoute, st.ControlPlane = nmDefaultRoute(ctx, ifaces)
	st.Cellular = cellStatus(ctx, ifaces)
	return st, nil
}

func ipJSONIfaces(ctx context.Context) ([]Interface, error) {
	out, err := run(ctx, "ip", "-j", "addr")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Ifname   string `json:"ifname"`
		Operstate string `json:"operstate"`
		Address  string `json:"address"`
		LinkType string `json:"link_type"`
		AddrInfo []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Prefix int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, err
	}
	var ifaces []Interface
	for _, r := range raw {
		if r.Ifname == "lo" {
			continue
		}
		iface := Interface{
			Name:  r.Ifname,
			State: r.Operstate,
			Type:  guessType(r.Ifname, r.LinkType),
			MAC:   r.Address,
		}
		for _, a := range r.AddrInfo {
			cidr := fmt.Sprintf("%s/%d", a.Local, a.Prefix)
			if a.Family == "inet" {
				iface.IPv4 = append(iface.IPv4, cidr)
			} else if a.Family == "inet6" {
				iface.IPv6 = append(iface.IPv6, cidr)
			}
		}
		ifaces = append(ifaces, iface)
	}
	return ifaces, nil
}

func guessType(name, linkType string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "wlan") || strings.HasPrefix(n, "wlp") || strings.HasPrefix(n, "wifi"):
		return "wifi"
	case strings.HasPrefix(n, "eth") || strings.HasPrefix(n, "en") || linkType == "ether":
		return "ethernet"
	case strings.HasPrefix(n, "wwan") || strings.HasPrefix(n, "cdc"):
		return "gsm"
	case strings.HasPrefix(n, "bnep") || strings.HasPrefix(n, "bt"):
		return "bluetooth"
	default:
		return "other"
	}
}

func netplanWiredSet(ctx context.Context, a Args) error {
	iface := a.Iface
	if iface == "" {
		return fmt.Errorf("iface is required for netplan")
	}
	method := strings.ToLower(a.Method)
	key := fmt.Sprintf("network.ethernets.%s", iface)
	switch method {
	case "", "dhcp", "auto":
		if _, err := mustPriv(ctx, "netplan", "set", key+".dhcp4=true"); err != nil {
			return err
		}
		_, _ = mustPriv(ctx, "netplan", "set", key+".addresses=null")
		_, _ = mustPriv(ctx, "netplan", "set", key+".routes=null")
		_, _ = mustPriv(ctx, "netplan", "set", key+".nameservers=null")
	case "static", "manual":
		if a.Address == "" {
			return fmt.Errorf("address is required for static")
		}
		prefix := a.Prefix
		if prefix == 0 {
			prefix = 24
		}
		addr := fmt.Sprintf("%s/%d", a.Address, prefix)
		if _, err := mustPriv(ctx, "netplan", "set", key+".dhcp4=false"); err != nil {
			return err
		}
		if _, err := mustPriv(ctx, "netplan", "set", fmt.Sprintf("%s.addresses=[%s]", key, addr)); err != nil {
			return err
		}
		if a.Gateway != "" {
			_, _ = mustPriv(ctx, "netplan", "set",
				fmt.Sprintf("%s.routes=[{to:default,via:%s}]", key, a.Gateway))
		}
		if len(a.DNS) > 0 {
			dns := strings.Join(a.DNS, ",")
			_, _ = mustPriv(ctx, "netplan", "set", fmt.Sprintf("%s.nameservers.addresses=[%s]", key, dns))
		}
	default:
		return fmt.Errorf("unknown method %q", a.Method)
	}
	_, err := mustPriv(ctx, "netplan", "apply")
	return err
}
