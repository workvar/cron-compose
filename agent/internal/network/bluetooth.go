package network

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PinWaiter correlates interactive PIN replies with in-flight pair requests.
type PinWaiter struct {
	mu   sync.Mutex
	wait map[string]chan string // requestID -> pin
}

func NewPinWaiter() *PinWaiter {
	return &PinWaiter{wait: map[string]chan string{}}
}

func (p *PinWaiter) Arm(requestID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wait[requestID] = make(chan string, 1)
}

func (p *PinWaiter) Deliver(requestID, pin string) bool {
	p.mu.Lock()
	ch := p.wait[requestID]
	p.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- pin:
		return true
	default:
		return false
	}
}

func (p *PinWaiter) Wait(ctx context.Context, requestID string) (string, error) {
	p.mu.Lock()
	ch := p.wait[requestID]
	p.mu.Unlock()
	if ch == nil {
		return "", fmt.Errorf("no pin waiter for %s", requestID)
	}
	defer func() {
		p.mu.Lock()
		delete(p.wait, requestID)
		p.mu.Unlock()
	}()
	select {
	case pin := <-ch:
		return pin, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func btStatus(ctx context.Context) BluetoothStatus {
	st := BluetoothStatus{Devices: []BluetoothDevice{}, PAN: []PANLink{}}
	if !has("bluetoothctl") {
		return st
	}
	st.Available = true
	out, err := run(ctx, "bluetoothctl", "show")
	if err == nil {
		st.Powered = strings.Contains(out, "Powered: yes")
	}
	st.Devices = btDevices(ctx)
	return st
}

func btDevices(ctx context.Context) []BluetoothDevice {
	out, err := run(ctx, "bluetoothctl", "devices")
	if err != nil {
		return nil
	}
	var list []BluetoothDevice
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Device ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		addr := fields[1]
		name := strings.TrimSpace(strings.TrimPrefix(line, "Device "+addr))
		d := BluetoothDevice{Address: addr, Name: name}
		info, err := run(ctx, "bluetoothctl", "info", addr)
		if err == nil {
			d.Paired = strings.Contains(info, "Paired: yes")
			d.Trusted = strings.Contains(info, "Trusted: yes")
			d.Connected = strings.Contains(info, "Connected: yes")
		}
		list = append(list, d)
	}
	return list
}

func btScan(ctx context.Context) ([]BluetoothDevice, error) {
	if !has("bluetoothctl") {
		return nil, fmt.Errorf("bluetoothctl not available")
	}
	_, _ = mustPriv(ctx, "bluetoothctl", "power", "on")
	cmd := exec.CommandContext(ctx, "bluetoothctl")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	_, _ = io.WriteString(stdin, "scan on\n")
	time.Sleep(8 * time.Second)
	_, _ = io.WriteString(stdin, "scan off\n")
	_, _ = io.WriteString(stdin, "quit\n")
	_ = stdin.Close()
	_ = cmd.Wait()
	return btDevices(ctx), nil
}

// EventEmitter is called for live NetworkEvent payloads (PIN challenges, logs).
type EventEmitter func(kind, pinDevice, pinMethod, pinPrompt string, data []byte)

func btPair(ctx context.Context, requestID string, a Args, pins *PinWaiter, emit EventEmitter) error {
	addr := a.AddressBT
	if addr == "" {
		return fmt.Errorf("address_bt is required")
	}
	if !has("bluetoothctl") {
		return fmt.Errorf("bluetoothctl not available")
	}
	_, _ = mustPriv(ctx, "bluetoothctl", "power", "on")

	pins.Arm(requestID)
	cmd := exec.CommandContext(ctx, "bluetoothctl")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for sc.Scan() {
			line := sc.Text()
			if emit != nil {
				emit("log", "", "", "", []byte(line+"\n"))
			}
			lower := strings.ToLower(line)
			if strings.Contains(lower, "enter pin") || strings.Contains(lower, "pin code") ||
				strings.Contains(lower, "passkey") || strings.Contains(lower, "confirm passkey") {
				method := "pin"
				if strings.Contains(lower, "passkey") {
					method = "passkey"
				}
				if strings.Contains(lower, "confirm") {
					method = "confirm"
				}
				if emit != nil {
					emit("pin_required", addr, method, line, nil)
				}
				waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				pin, err := pins.Wait(waitCtx, requestID)
				cancel()
				if err != nil {
					done <- err
					return
				}
				if method == "confirm" {
					_, _ = io.WriteString(stdin, "yes\n")
				} else {
					_, _ = io.WriteString(stdin, pin+"\n")
				}
			}
		}
		done <- sc.Err()
	}()

	_, _ = io.WriteString(stdin, "agent on\n")
	_, _ = io.WriteString(stdin, "default-agent\n")
	_, _ = io.WriteString(stdin, "pair "+addr+"\n")
	time.Sleep(30 * time.Second)
	_, _ = io.WriteString(stdin, "trust "+addr+"\n")
	_, _ = io.WriteString(stdin, "quit\n")
	_ = stdin.Close()
	waitErr := cmd.Wait()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	default:
	}
	return waitErr
}

func btConnect(ctx context.Context, a Args) error {
	addr := a.AddressBT
	if addr == "" {
		return fmt.Errorf("address_bt is required")
	}
	_, err := mustPriv(ctx, "bluetoothctl", "connect", addr)
	return err
}

func btDisconnect(ctx context.Context, a Args) error {
	addr := a.AddressBT
	if addr == "" {
		return fmt.Errorf("address_bt is required")
	}
	_, err := mustPriv(ctx, "bluetoothctl", "disconnect", addr)
	return err
}

func btForget(ctx context.Context, a Args) error {
	addr := a.AddressBT
	if addr == "" {
		return fmt.Errorf("address_bt is required")
	}
	_, _ = mustPriv(ctx, "bluetoothctl", "disconnect", addr)
	_, err := mustPriv(ctx, "bluetoothctl", "remove", addr)
	return err
}

func btPANConnect(ctx context.Context, a Args) error {
	addr := a.AddressBT
	if addr == "" {
		return fmt.Errorf("address_bt is required")
	}
	panType := a.PANType
	if panType == "" {
		panType = "panu"
	}
	name := "cc-bt-pan-" + strings.ReplaceAll(addr, ":", "")
	if !nmConnExists(ctx, name) {
		args := []string{
			"connection", "add", "type", "bluetooth",
			"con-name", name,
			"bluetooth.type", panType,
			"bluetooth.bdaddr", addr,
		}
		if _, err := mustPriv(ctx, "nmcli", args...); err != nil {
			return err
		}
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "up", name)
	return err
}

func btPANDisconnect(ctx context.Context, a Args) error {
	addr := a.AddressBT
	name := a.Connection
	if name == "" && addr != "" {
		name = "cc-bt-pan-" + strings.ReplaceAll(addr, ":", "")
	}
	if name == "" {
		return fmt.Errorf("connection or address_bt is required")
	}
	_, err := mustPriv(ctx, "nmcli", "connection", "down", name)
	return err
}
