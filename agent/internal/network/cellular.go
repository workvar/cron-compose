package network

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func cellStatus(ctx context.Context, ifaces []Interface) CellularStatus {
	st := CellularStatus{Modems: []Modem{}}
	if !has("mmcli") {
		return st
	}
	st.Available = true
	out, err := run(ctx, "mmcli", "-L")
	if err != nil {
		return st
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "/Modem/") {
			continue
		}
		// /org/freedesktop/ModemManager1/Modem/0 [vendor]
		path := ""
		for _, f := range strings.Fields(line) {
			if strings.Contains(f, "/Modem/") {
				path = strings.TrimSuffix(f, ",")
				break
			}
		}
		if path == "" {
			continue
		}
		id := path[strings.LastIndex(path, "/")+1:]
		m := Modem{ID: id, Path: path}
		detail, err := run(ctx, "mmcli", "-m", id)
		if err == nil {
			m.State = mmField(detail, "state")
			m.Operator = mmField(detail, "operator name")
			if sig := mmField(detail, "signal quality"); sig != "" {
				// e.g. "80% (recent)"
				sig = strings.TrimSpace(strings.Split(sig, "%")[0])
				m.Signal, _ = strconv.Atoi(sig)
			}
			m.APN = mmField(detail, "access tech") // best-effort; real APN below
		}
		// Bearer / connection info from NM gsm devices.
		for _, iface := range ifaces {
			if iface.Type == "gsm" || iface.Type == "cdma" {
				if iface.State == "connected" || strings.Contains(iface.State, "connected") {
					m.Connected = true
					m.IPv4 = iface.IPv4
					m.Connection = iface.Connection
				}
			}
		}
		if apn := nmGSMAPN(ctx, m.Connection); apn != "" {
			m.APN = apn
		}
		st.Modems = append(st.Modems, m)
	}
	return st
}

func mmField(blob, key string) string {
	for _, line := range strings.Split(blob, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), strings.ToLower(key)) {
			_, v, ok := strings.Cut(line, ":")
			if ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func nmGSMAPN(ctx context.Context, conn string) string {
	if conn == "" {
		return ""
	}
	out, err := run(ctx, "nmcli", "-g", "gsm.apn", "connection", "show", conn)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func cellConnect(ctx context.Context, a Args) error {
	conn := a.Connection
	if conn == "" {
		conn = "cc-cellular"
	}
	if !nmConnExists(ctx, conn) {
		args := []string{"connection", "add", "type", "gsm", "con-name", conn, "ifname", "*"}
		if a.APN != "" {
			args = append(args, "gsm.apn", a.APN)
		}
		if _, err := mustPriv(ctx, "nmcli", args...); err != nil {
			return err
		}
	} else if a.APN != "" {
		_, _ = mustPriv(ctx, "nmcli", "connection", "modify", conn, "gsm.apn", a.APN)
	}
	if a.PIN != "" && a.ModemID != "" {
		_, _ = mustPriv(ctx, "mmcli", "-m", a.ModemID, "--sim-pin="+a.PIN)
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "up", conn)
	return err
}

func cellDisconnect(ctx context.Context, a Args) error {
	conn := a.Connection
	if conn == "" {
		conn = "cc-cellular"
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "down", conn)
	return err
}

func cellSetAPN(ctx context.Context, a Args) error {
	if a.APN == "" {
		return fmt.Errorf("apn is required")
	}
	conn := a.Connection
	if conn == "" {
		conn = "cc-cellular"
	}
	if !nmConnExists(ctx, conn) {
		_, err := mustPriv(ctx, "nmcli", "connection", "add", "type", "gsm", "con-name", conn, "ifname", "*", "gsm.apn", a.APN)
		return err
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "modify", conn, "gsm.apn", a.APN)
	return err
}
