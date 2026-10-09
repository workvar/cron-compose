package network

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func nmStatus(ctx context.Context) (Status, error) {
	st := Status{
		Backend:      BackendNetworkManager,
		Interfaces:   []Interface{},
		Wifi:         WifiStatus{Saved: []WifiNetwork{}},
		Bluetooth:    BluetoothStatus{Devices: []BluetoothDevice{}, PAN: []PANLink{}},
		Cellular:     CellularStatus{Modems: []Modem{}},
		Capabilities: ProbeCapabilities(ctx),
		DualWifi:     probeDualWifi(ctx),
	}

	devs, err := nmDeviceList(ctx)
	if err != nil {
		st.Error = err.Error()
		return st, err
	}
	st.Interfaces = devs
	st.DefaultRoute, st.ControlPlane = nmDefaultRoute(ctx, devs)

	wifiDev := ""
	for _, d := range devs {
		if d.Type == "wifi" {
			wifiDev = d.Name
			break
		}
	}
	st.Wifi.Device = wifiDev
	st.Wifi.Enabled = wifiDev != ""
	st.Wifi.Saved = nmWifiSaved(ctx)
	if active := nmWifiActive(ctx); active != nil {
		st.Wifi.Active = active
	}

	st.Bluetooth = btStatus(ctx)
	st.Bluetooth.PAN = nmPANLinks(ctx, devs)
	st.Cellular = cellStatus(ctx, devs)
	return st, nil
}

func nmDeviceList(ctx context.Context) ([]Interface, error) {
	out, err := mustPriv(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status")
	if err != nil {
		// Fall back to unprivileged read.
		out, err = run(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status")
		if err != nil {
			return nil, err
		}
	}
	var ifaces []Interface
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := splitNM(line, 4)
		if len(parts) < 3 {
			continue
		}
		name, typ, state := parts[0], mapDeviceType(parts[1]), parts[2]
		conn := ""
		if len(parts) > 3 && parts[3] != "--" {
			conn = parts[3]
		}
		if name == "lo" || typ == "loopback" {
			continue
		}
		iface := Interface{Name: name, Type: typ, State: state, Connection: conn}
		fillIP(ctx, &iface)
		ifaces = append(ifaces, iface)
	}
	return ifaces, nil
}

func fillIP(ctx context.Context, iface *Interface) {
	out, err := run(ctx, "nmcli", "-t", "-f", "IP4.ADDRESS,IP4.GATEWAY,IP4.DNS,IP6.ADDRESS,GENERAL.HWADDR,IP4.METHOD", "device", "show", iface.Name)
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch k {
		case "IP4.ADDRESS":
			iface.IPv4 = append(iface.IPv4, v)
		case "IP6.ADDRESS":
			iface.IPv6 = append(iface.IPv6, v)
		case "IP4.GATEWAY":
			if v != "" && v != "--" {
				iface.Gateway = v
			}
		case "IP4.DNS":
			if v != "" && v != "--" {
				iface.DNS = append(iface.DNS, v)
			}
		case "GENERAL.HWADDR":
			iface.MAC = v
		case "IP4.METHOD":
			iface.Method = v
		}
	}
}

func nmDefaultRoute(ctx context.Context, ifaces []Interface) (gateway, primaryIface string) {
	out, err := run(ctx, "ip", "-4", "route", "show", "default")
	if err != nil {
		return "", ""
	}
	// default via 192.168.1.1 dev eth0 …
	fields := strings.Fields(out)
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			gateway = fields[i+1]
		}
		if f == "dev" && i+1 < len(fields) {
			primaryIface = fields[i+1]
		}
	}
	for i := range ifaces {
		if ifaces[i].Name == primaryIface {
			ifaces[i].Primary = true
		}
	}
	return gateway, primaryIface
}

func nmWifiSaved(ctx context.Context) []WifiNetwork {
	out, err := run(ctx, "nmcli", "-t", "-f", "NAME,TYPE,DEVICE", "connection", "show")
	if err != nil {
		return nil
	}
	var saved []WifiNetwork
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 3)
		if len(parts) < 2 || parts[1] != "802-11-wireless" {
			continue
		}
		name := parts[0]
		ssid := nmConnSSID(ctx, name)
		if ssid == "" {
			ssid = name
		}
		wn := WifiNetwork{SSID: ssid, Connection: name}
		if len(parts) > 2 && parts[2] != "" && parts[2] != "--" {
			wn.Active = true
			wn.Device = parts[2]
		}
		saved = append(saved, wn)
	}
	return saved
}

func nmConnSSID(ctx context.Context, conn string) string {
	out, err := run(ctx, "nmcli", "-g", "802-11-wireless.ssid", "connection", "show", conn)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func nmWifiActive(ctx context.Context) *WifiNetwork {
	out, err := run(ctx, "nmcli", "-t", "-f", "ACTIVE,SSID,SIGNAL,SECURITY,DEVICE,BSSID", "device", "wifi", "list")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 6)
		if len(parts) < 2 || parts[0] != "yes" {
			continue
		}
		sig, _ := strconv.Atoi(parts[2])
		wn := &WifiNetwork{
			Active:   true,
			SSID:     parts[1],
			Signal:   sig,
			Security: parts[3],
		}
		if len(parts) > 4 {
			wn.Device = parts[4]
		}
		if len(parts) > 5 {
			wn.BSSID = parts[5]
		}
		wn.Connection = nmActiveWifiConn(ctx)
		return wn
	}
	return nil
}

func nmActiveWifiConn(ctx context.Context) string {
	out, err := run(ctx, "nmcli", "-t", "-f", "NAME,TYPE,DEVICE", "connection", "show", "--active")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 3)
		if len(parts) >= 2 && parts[1] == "802-11-wireless" {
			return parts[0]
		}
	}
	return ""
}

func nmWifiScan(ctx context.Context) ([]WifiNetwork, error) {
	_, _ = mustPriv(ctx, "nmcli", "device", "wifi", "rescan")
	out, err := mustPriv(ctx, "nmcli", "-t", "-f", "SSID,SIGNAL,SECURITY,BSSID,IN-USE", "device", "wifi", "list")
	if err != nil {
		out, err = run(ctx, "nmcli", "-t", "-f", "SSID,SIGNAL,SECURITY,BSSID,IN-USE", "device", "wifi", "list")
		if err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	var list []WifiNetwork
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 5)
		if len(parts) < 3 {
			continue
		}
		ssid := parts[0]
		if ssid == "" || seen[ssid] {
			continue
		}
		seen[ssid] = true
		sig, _ := strconv.Atoi(parts[1])
		wn := WifiNetwork{SSID: ssid, Signal: sig, Security: parts[2]}
		if len(parts) > 3 {
			wn.BSSID = parts[3]
		}
		if len(parts) > 4 && parts[4] == "*" {
			wn.Active = true
		}
		list = append(list, wn)
	}
	return list, nil
}

func nmWifiSave(ctx context.Context, a Args) error {
	if a.SSID == "" {
		return fmt.Errorf("ssid is required")
	}
	name := a.Connection
	if name == "" {
		name = a.SSID
	}
	// Update existing profile in place (no activate).
	if nmConnExists(ctx, name) {
		if a.PSK != "" {
			_, err := mustPriv(ctx, "nmcli", "connection", "modify", name, "wifi-sec.psk", a.PSK)
			return err
		}
		return nil
	}
	args := []string{
		"connection", "add", "type", "wifi", "con-name", name,
		"ifname", "*", "ssid", a.SSID,
	}
	if a.PSK != "" {
		args = append(args, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", a.PSK)
	}
	args = append(args, "autoconnect", "no")
	_, err := mustPriv(ctx, "nmcli", args...)
	return err
}

func nmWifiUpdatePSK(ctx context.Context, a Args) error {
	name := a.Connection
	if name == "" {
		name = a.SSID
	}
	if name == "" {
		return fmt.Errorf("connection or ssid is required")
	}
	if a.PSK == "" {
		return fmt.Errorf("psk is required")
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "modify", name, "wifi-sec.psk", a.PSK)
	return err
}

func nmWifiConnect(ctx context.Context, a Args) error {
	name := a.Connection
	if name == "" {
		name = a.SSID
	}
	if name == "" {
		return fmt.Errorf("connection or ssid is required")
	}
	if a.Alongside {
		return wifiConnectAlongside(ctx, a)
	}
	if !nmConnExists(ctx, name) {
		if a.SSID == "" {
			return fmt.Errorf("unknown connection %q", name)
		}
		if err := nmWifiSave(ctx, a); err != nil {
			return err
		}
		name = a.SSID
		if a.Connection != "" {
			name = a.Connection
		}
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "up", name)
	return err
}

func nmWifiDisconnect(ctx context.Context, a Args) error {
	if a.Connection != "" {
		_, err := mustPriv(ctx, "nmcli", "connection", "down", a.Connection)
		return err
	}
	dev := a.Iface
	if dev == "" {
		dev = nmWifiDevice(ctx)
	}
	if dev == "" {
		return fmt.Errorf("no wifi device")
	}
	_, err := mustPriv(ctx, "nmcli", "device", "disconnect", dev)
	return err
}

func nmWifiForget(ctx context.Context, a Args) error {
	name := a.Connection
	if name == "" {
		name = a.SSID
	}
	if name == "" {
		return fmt.Errorf("connection or ssid is required")
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "delete", name)
	return err
}

func nmWifiDevice(ctx context.Context) string {
	out, err := run(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE", "device", "status")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 2)
		if len(parts) == 2 && parts[1] == "wifi" {
			return parts[0]
		}
	}
	return ""
}

func nmConnExists(ctx context.Context, name string) bool {
	_, err := run(ctx, "nmcli", "-g", "connection.id", "connection", "show", name)
	return err == nil
}

func nmWiredSet(ctx context.Context, a Args) error {
	conn := a.Connection
	iface := a.Iface
	if conn == "" && iface != "" {
		conn = nmConnForIface(ctx, iface)
	}
	if conn == "" {
		return fmt.Errorf("connection or iface is required")
	}
	method := strings.ToLower(a.Method)
	switch method {
	case "", "dhcp", "auto":
		_, err := mustPriv(ctx, "nmcli", "connection", "modify", conn,
			"ipv4.method", "auto",
			"ipv4.addresses", "",
			"ipv4.gateway", "",
			"ipv4.dns", "",
		)
		if err != nil {
			return err
		}
	case "static", "manual":
		if a.Address == "" {
			return fmt.Errorf("address is required for static")
		}
		prefix := a.Prefix
		if prefix == 0 {
			prefix = 24
		}
		addr := fmt.Sprintf("%s/%d", a.Address, prefix)
		args := []string{"connection", "modify", conn, "ipv4.method", "manual", "ipv4.addresses", addr}
		if a.Gateway != "" {
			args = append(args, "ipv4.gateway", a.Gateway)
		}
		if len(a.DNS) > 0 {
			args = append(args, "ipv4.dns", strings.Join(a.DNS, ","))
		}
		if _, err := mustPriv(ctx, "nmcli", args...); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown method %q", a.Method)
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "up", conn)
	return err
}

func nmConnForIface(ctx context.Context, iface string) string {
	out, err := run(ctx, "nmcli", "-t", "-f", "DEVICE,CONNECTION", "device", "status")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 2)
		if len(parts) == 2 && parts[0] == iface && parts[1] != "--" {
			return parts[1]
		}
	}
	return ""
}

func nmPANLinks(ctx context.Context, ifaces []Interface) []PANLink {
	out, err := run(ctx, "nmcli", "-t", "-f", "NAME,TYPE,DEVICE", "connection", "show")
	if err != nil {
		return nil
	}
	var links []PANLink
	for _, line := range strings.Split(out, "\n") {
		parts := splitNM(line, 3)
		if len(parts) < 2 || parts[1] != "bluetooth" {
			continue
		}
		link := PANLink{Connection: parts[0], Active: len(parts) > 2 && parts[2] != "" && parts[2] != "--"}
		addr, _ := run(ctx, "nmcli", "-g", "bluetooth.bdaddr", "connection", "show", parts[0])
		link.Address = strings.TrimSpace(addr)
		name, _ := run(ctx, "nmcli", "-g", "connection.id", "connection", "show", parts[0])
		link.Name = strings.TrimSpace(name)
		if link.Active && len(parts) > 2 {
			for _, iface := range ifaces {
				if iface.Name == parts[2] {
					link.IPv4 = iface.IPv4
				}
			}
		}
		links = append(links, link)
	}
	return links
}

func mapDeviceType(t string) string {
	switch t {
	case "ethernet", "wifi", "bluetooth", "gsm", "cdma", "loopback", "bridge", "bond", "vlan", "tun", "tap":
		return t
	default:
		return "other"
	}
}
