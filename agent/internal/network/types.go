package network

import (
	"encoding/json"
	"strings"
)

// Backend names reported in status.
const (
	BackendNetworkManager = "networkmanager"
	BackendNetplan        = "netplan"
	BackendUnavailable    = "unavailable"
)

// Status is the full snapshot returned by op=status.
type Status struct {
	Backend      string          `json:"backend"`
	DualWifi     DualWifiInfo    `json:"dual_wifi"`
	Interfaces   []Interface     `json:"interfaces"`
	Wifi         WifiStatus      `json:"wifi"`
	Bluetooth    BluetoothStatus `json:"bluetooth"`
	Cellular     CellularStatus  `json:"cellular"`
	DefaultRoute string          `json:"default_route,omitempty"`
	ControlPlane string          `json:"control_plane_iface,omitempty"`
	Capabilities []string        `json:"capabilities"`
	Error        string          `json:"error,omitempty"`
}

type DualWifiInfo struct {
	Supported bool   `json:"supported"`
	Detail    string `json:"detail,omitempty"`
}

type Interface struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"` // ethernet | wifi | bluetooth | gsm | loopback | other
	State      string   `json:"state"`
	Connection string   `json:"connection,omitempty"`
	IPv4       []string `json:"ipv4,omitempty"`
	IPv6       []string `json:"ipv6,omitempty"`
	Gateway    string   `json:"gateway,omitempty"`
	DNS        []string `json:"dns,omitempty"`
	Method     string   `json:"method,omitempty"` // auto | manual
	MAC        string   `json:"mac,omitempty"`
	Primary    bool     `json:"primary,omitempty"`
}

type WifiStatus struct {
	Enabled bool          `json:"enabled"`
	Active  *WifiNetwork  `json:"active,omitempty"`
	Saved   []WifiNetwork `json:"saved"`
	Device  string        `json:"device,omitempty"`
}

type WifiNetwork struct {
	SSID       string `json:"ssid"`
	Connection string `json:"connection,omitempty"`
	Signal     int    `json:"signal,omitempty"`
	Security   string `json:"security,omitempty"`
	Active     bool   `json:"active,omitempty"`
	Device     string `json:"device,omitempty"`
	BSSID      string `json:"bssid,omitempty"`
}

type BluetoothStatus struct {
	Available bool              `json:"available"`
	Powered   bool              `json:"powered"`
	Devices   []BluetoothDevice `json:"devices"`
	PAN       []PANLink         `json:"pan"`
}

type BluetoothDevice struct {
	Address   string `json:"address"`
	Name      string `json:"name,omitempty"`
	Paired    bool   `json:"paired"`
	Trusted   bool   `json:"trusted"`
	Connected bool   `json:"connected"`
}

type PANLink struct {
	Address    string   `json:"address"`
	Name       string   `json:"name,omitempty"`
	Connection string   `json:"connection,omitempty"`
	IPv4       []string `json:"ipv4,omitempty"`
	Active     bool     `json:"active"`
}

type CellularStatus struct {
	Available bool    `json:"available"`
	Modems    []Modem `json:"modems"`
}

type Modem struct {
	ID         string   `json:"id"`
	Path       string   `json:"path,omitempty"`
	State      string   `json:"state,omitempty"`
	Operator   string   `json:"operator,omitempty"`
	Signal     int      `json:"signal,omitempty"`
	APN        string   `json:"apn,omitempty"`
	Connected  bool     `json:"connected"`
	IPv4       []string `json:"ipv4,omitempty"`
	Connection string   `json:"connection,omitempty"`
}

// Args is the common JSON body for NetworkRequest.args_json.
type Args struct {
	Iface      string   `json:"iface,omitempty"`
	Connection string   `json:"connection,omitempty"`
	Method     string   `json:"method,omitempty"` // dhcp | static | auto | manual
	Address    string   `json:"address,omitempty"`
	Prefix     int      `json:"prefix,omitempty"`
	Gateway    string   `json:"gateway,omitempty"`
	DNS        []string `json:"dns,omitempty"`
	SSID       string   `json:"ssid,omitempty"`
	PSK        string   `json:"psk,omitempty"`
	Alongside  bool     `json:"alongside,omitempty"`
	AddressBT  string   `json:"address_bt,omitempty"`
	ModemID    string   `json:"modem_id,omitempty"`
	APN        string   `json:"apn,omitempty"`
	PIN        string   `json:"pin,omitempty"`
	PINMethod  string   `json:"pin_method,omitempty"`
	PANType    string   `json:"pan_type,omitempty"` // panu | nap
}

func parseArgs(raw string) (Args, error) {
	var a Args
	if strings.TrimSpace(raw) == "" {
		return a, nil
	}
	err := json.Unmarshal([]byte(raw), &a)
	return a, err
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"encode failed"}`
	}
	return string(b)
}
