package network

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Manager owns PIN waiters shared across network requests on this agent.
type Manager struct {
	Pins *PinWaiter
}

func NewManager() *Manager {
	return &Manager{Pins: NewPinWaiter()}
}

// Result is the agent-side outcome of a NetworkRequest.
type Result struct {
	Status     string
	Error      string
	ResultJSON string
}

// Handle runs one network op. emit may be nil for unary ops.
func (m *Manager) Handle(ctx context.Context, requestID, op, argsJSON string, emit EventEmitter) Result {
	a, err := parseArgs(argsJSON)
	if err != nil {
		return fail(fmt.Sprintf("invalid args_json: %v", err))
	}
	op = strings.ToLower(strings.TrimSpace(op))
	backend := DetectBackend(ctx)

	switch op {
	case "status":
		return m.status(ctx, backend)
	case "wired_get":
		return m.status(ctx, backend)
	case "wired_set":
		return m.wiredSet(ctx, backend, a)
	case "wifi_scan":
		return m.wifiScan(ctx, backend)
	case "wifi_list_saved":
		st, err := m.statusPayload(ctx, backend)
		if err != nil {
			return fail(err.Error())
		}
		return ok(mustJSON(st.Wifi.Saved))
	case "wifi_save":
		return m.wifiMut(ctx, backend, func() error { return nmWifiSave(ctx, a) })
	case "wifi_update_psk":
		return m.wifiMut(ctx, backend, func() error { return nmWifiUpdatePSK(ctx, a) })
	case "wifi_connect":
		return m.wifiMut(ctx, backend, func() error { return nmWifiConnect(ctx, a) })
	case "wifi_disconnect":
		return m.wifiMut(ctx, backend, func() error { return nmWifiDisconnect(ctx, a) })
	case "wifi_forget":
		return m.wifiMut(ctx, backend, func() error { return nmWifiForget(ctx, a) })
	case "bt_scan":
		list, err := btScan(ctx)
		if err != nil {
			return mapErr(err)
		}
		return ok(mustJSON(list))
	case "bt_devices":
		return ok(mustJSON(btDevices(ctx)))
	case "bt_pair":
		err := btPair(ctx, requestID, a, m.Pins, emit)
		if err != nil {
			return mapErr(err)
		}
		return ok(mustJSON(btDevices(ctx)))
	case "bt_pin_reply":
		if !m.Pins.Deliver(requestID, a.PIN) {
			return fail("no pending pin challenge for this request_id")
		}
		return ok(`{"accepted":true}`)
	case "bt_connect":
		return mapErr2(btConnect(ctx, a))
	case "bt_disconnect":
		return mapErr2(btDisconnect(ctx, a))
	case "bt_forget":
		return mapErr2(btForget(ctx, a))
	case "bt_pan_connect":
		return mapErr2(btPANConnect(ctx, a))
	case "bt_pan_disconnect":
		return mapErr2(btPANDisconnect(ctx, a))
	case "cell_modems":
		st, err := m.statusPayload(ctx, backend)
		if err != nil {
			return fail(err.Error())
		}
		return ok(mustJSON(st.Cellular))
	case "cell_connect":
		return mapErr2(cellConnect(ctx, a))
	case "cell_disconnect":
		return mapErr2(cellDisconnect(ctx, a))
	case "cell_set_apn":
		return mapErr2(cellSetAPN(ctx, a))
	default:
		return fail("unknown op " + op)
	}
}

func (m *Manager) status(ctx context.Context, backend string) Result {
	st, err := m.statusPayload(ctx, backend)
	if err != nil {
		return Result{Status: "failed", Error: err.Error(), ResultJSON: mustJSON(st)}
	}
	return ok(mustJSON(st))
}

func (m *Manager) statusPayload(ctx context.Context, backend string) (Status, error) {
	switch backend {
	case BackendNetworkManager:
		return nmStatus(ctx)
	case BackendNetplan:
		return netplanStatus(ctx)
	default:
		st := Status{Backend: BackendUnavailable, Error: "no NetworkManager or netplan on this host"}
		return st, fmt.Errorf("%s", st.Error)
	}
}

func (m *Manager) wiredSet(ctx context.Context, backend string, a Args) Result {
	var err error
	switch backend {
	case BackendNetworkManager:
		err = nmWiredSet(ctx, a)
	case BackendNetplan:
		err = netplanWiredSet(ctx, a)
	default:
		err = fmt.Errorf("network backend unavailable")
	}
	return mapErr2(err)
}

func (m *Manager) wifiScan(ctx context.Context, backend string) Result {
	if backend != BackendNetworkManager {
		return fail("wifi scan requires NetworkManager")
	}
	list, err := nmWifiScan(ctx)
	if err != nil {
		return mapErr(err)
	}
	return ok(mustJSON(list))
}

func (m *Manager) wifiMut(ctx context.Context, backend string, fn func() error) Result {
	if backend != BackendNetworkManager {
		return fail("wifi management requires NetworkManager")
	}
	return mapErr2(fn())
}

func ok(json string) Result {
	return Result{Status: "succeeded", ResultJSON: json}
}

func fail(msg string) Result {
	return Result{Status: "failed", Error: msg}
}

func mapErr(err error) Result {
	if err == nil {
		return ok("{}")
	}
	return mapErr2(err)
}

func mapErr2(err error) Result {
	if err == nil {
		return ok(`{"ok":true}`)
	}
	if errors.Is(err, errNoPrivilege) {
		return fail(err.Error() + "; grant passwordless sudo for nmcli/bluetoothctl/mmcli/netplan/iw/ip in /etc/sudoers.d/croncompose-agent")
	}
	return fail(err.Error())
}
