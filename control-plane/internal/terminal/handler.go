// Package terminal exposes the web terminal: a WebSocket endpoint that bridges a
// browser to an interactive shell (or one-shot command) running on a managed server.
//
// The browser opens a WebSocket; the control plane mints a session id, relays the open
// over the agent's existing gRPC stream, and pumps bytes in both directions. Terminal
// output is routed back through agentgw.TerminalBus. Access is admin/owner only and every
// session open/close is written to the audit log.
package terminal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

const (
	pingInterval = 30 * time.Second
	writeWait    = 10 * time.Second
	initWait     = 15 * time.Second
)

type handler struct {
	log        *slog.Logger
	gw         *agentgw.Gateway
	audit      audit.Writer
	up         websocket.FastHTTPUpgrader
	allowHosts map[string]bool // extra Origin hostnames allowed (from PUBLIC_HTTP_URL)
}

// newHandler builds the handler and its upgrader. publicURL (PUBLIC_HTTP_URL) seeds the
// Origin allowlist so the public hostname is accepted even when the control plane sees a
// different internal Host behind a proxy.
func newHandler(log *slog.Logger, gw *agentgw.Gateway, writer audit.Writer, publicURL string) *handler {
	h := &handler{log: log, gw: gw, audit: writer, allowHosts: map[string]bool{}}
	if host := hostnameOf(publicURL); host != "" {
		h.allowHosts[host] = true
	}
	h.up = websocket.FastHTTPUpgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     h.checkOrigin,
	}
	return h
}

// checkOrigin guards against cross-site WebSocket hijacking. A browser always sends an
// Origin; we accept it only when it is same-origin (same hostname as the request), an
// explicitly allowed public host, or a loopback address (so `next dev` works locally). An
// empty Origin means a non-browser client, which cannot be driven cross-site, so it is
// allowed; the session cookie + admin/owner role still apply either way.
func (h *handler) checkOrigin(ctx *fasthttp.RequestCtx) bool {
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return false
	}
	oh := u.Hostname()
	if oh == hostnamePart(string(ctx.Host())) || isLoopback(oh) || h.allowHosts[oh] {
		return true
	}
	h.log.Warn("terminal websocket: rejected origin", "origin", origin, "host", string(ctx.Host()))
	return false
}

// clientMsg is the JSON envelope the browser sends in text frames. Raw binary frames are
// treated as stdin.
type clientMsg struct {
	Type    string `json:"type"`    // init | stdin | resize | signal | close
	Mode    string `json:"mode"`    // init: shell | command
	Command string `json:"command"` // init (command mode): the one-shot command line
	Data    string `json:"data"`    // stdin
	Cols    uint32 `json:"cols"`    // init / resize
	Rows    uint32 `json:"rows"`    // init / resize
	Signal  string `json:"signal"`  // signal: INT | TERM | ...
}

// serverEvent is a control frame the server sends in text frames. PTY output is sent as
// binary frames instead.
type serverEvent struct {
	Type    string `json:"type"` // started | exit | error
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ws upgrades the request, then bridges the browser to an agent terminal session.
func (h *handler) ws(c fiber.Ctx) error {
	serverID := c.Params("id")
	actor := auth.CurrentUserID(c)
	return h.up.Upgrade(c.RequestCtx(), func(conn *websocket.Conn) {
		h.serve(conn, serverID, actor)
	})
}

func (h *handler) serve(conn *websocket.Conn, serverID, actor string) {
	defer conn.Close()

	// The first frame must be an "init" describing the session.
	init, ok := h.readInit(conn)
	if !ok {
		writeEvent(conn, serverEvent{Type: "error", Message: "expected init frame"})
		return
	}
	mode := init.Mode
	if mode != "command" {
		mode = "shell"
	}

	sessionID := ids.New()
	sub := h.gw.Terminals().Open(sessionID)
	defer h.gw.Terminals().Close(sessionID)

	if err := h.toAgent(serverID, &agentv1.TerminalInput{
		SessionId: sessionID, Op: "open", Kind: mode,
		Cols: init.Cols, Rows: init.Rows, Command: init.Command,
	}); err != nil {
		writeEvent(conn, serverEvent{Type: "error", Message: "agent offline"})
		return
	}

	bg := context.Background()
	h.audit.Write(bg, actor, "terminal.open", "server", serverID,
		map[string]any{"session_id": sessionID, "mode": mode})
	defer h.audit.Write(bg, actor, "terminal.close", "server", serverID,
		map[string]any{"session_id": sessionID})

	// agent -> browser: the pump is the SOLE writer on the conn.
	go h.pumpOut(conn, sub)

	// browser -> agent: this goroutine only reads.
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		in := decode(sessionID, mt, data)
		if in == nil {
			continue
		}
		if err := h.toAgent(serverID, in); err != nil {
			break
		}
		if in.GetOp() == "close" {
			break
		}
	}

	// Browser went away: tell the agent to kill the PTY.
	_ = h.toAgent(serverID, &agentv1.TerminalInput{SessionId: sessionID, Op: "close"})
}

// readInit reads and validates the first frame.
func (h *handler) readInit(conn *websocket.Conn) (clientMsg, bool) {
	_ = conn.SetReadDeadline(time.Now().Add(initWait))
	mt, data, err := conn.ReadMessage()
	if err != nil || mt != websocket.TextMessage {
		return clientMsg{}, false
	}
	_ = conn.SetReadDeadline(time.Time{}) // clear: interactive sessions are long-lived

	var m clientMsg
	if json.Unmarshal(data, &m) != nil || m.Type != "init" {
		return clientMsg{}, false
	}
	return m, true
}

func (h *handler) pumpOut(conn *websocket.Conn, sub *agentgw.TerminalSub) {
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-sub.Done():
			return
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		case out := <-sub.Out():
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			switch out.GetKind() {
			case "data":
				if conn.WriteMessage(websocket.BinaryMessage, out.GetData()) != nil {
					return
				}
			case "started":
				writeEvent(conn, serverEvent{Type: "started"})
			case "exit":
				writeEvent(conn, serverEvent{Type: "exit", Code: int(out.GetExitCode()), Message: out.GetMessage()})
				_ = conn.Close()
				return
			case "error":
				writeEvent(conn, serverEvent{Type: "error", Message: out.GetMessage()})
				_ = conn.Close()
				return
			}
		}
	}
}

func (h *handler) toAgent(serverID string, in *agentv1.TerminalInput) error {
	return h.gw.Registry().Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_TerminalInput{TerminalInput: in},
	})
}

// decode turns one browser frame into a TerminalInput. Binary frames are raw stdin; text
// frames are JSON envelopes. Returns nil for frames that carry nothing actionable.
func decode(sessionID string, mt int, data []byte) *agentv1.TerminalInput {
	if mt == websocket.BinaryMessage {
		return &agentv1.TerminalInput{SessionId: sessionID, Op: "data", Data: data}
	}
	var m clientMsg
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	switch m.Type {
	case "stdin":
		return &agentv1.TerminalInput{SessionId: sessionID, Op: "data", Data: []byte(m.Data)}
	case "resize":
		return &agentv1.TerminalInput{SessionId: sessionID, Op: "resize", Cols: m.Cols, Rows: m.Rows}
	case "signal":
		return &agentv1.TerminalInput{SessionId: sessionID, Op: "signal", Signal: m.Signal}
	case "close":
		return &agentv1.TerminalInput{SessionId: sessionID, Op: "close"}
	default:
		return nil
	}
}

func writeEvent(conn *websocket.Conn, ev serverEvent) {
	b, _ := json.Marshal(ev)
	_ = conn.WriteMessage(websocket.TextMessage, b)
}

// hostnameOf returns the hostname component of a URL (no scheme, no port), or "".
func hostnameOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// hostnamePart strips an optional :port from a host[:port] value.
func hostnamePart(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
