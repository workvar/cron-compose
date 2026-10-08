package agentgw

import (
	"errors"
	"sync"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// ErrAgentOffline is returned when the API tries to send to an agent that has no
// active stream. Callers can fall back to "the agent will sync on next connect".
var ErrAgentOffline = errors.New("agent offline")

// Conn is an active AgentStream for one server. Send is safe to call from any goroutine.
type Conn struct {
	serverID string
	out      chan *agentv1.ServerMessage
	done     chan struct{}
}

// Send queues a message to the agent. Returns ErrAgentOffline if the stream is closed.
func (c *Conn) Send(msg *agentv1.ServerMessage) error {
	select {
	case <-c.done:
		return ErrAgentOffline
	case c.out <- msg:
		return nil
	}
}

// Registry maps server_id -> Conn for currently connected agents. The API layer uses
// it to push commands (RunNow, CancelRun, SyncJobs) without knowing about gRPC.
type Registry struct {
	mu    sync.RWMutex
	conns map[string]*Conn
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{conns: map[string]*Conn{}}
}

// Add inserts a new Conn for serverID, replacing any prior one. The previous Conn (if
// any) is closed so its goroutines exit.
func (r *Registry) Add(serverID string) *Conn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.conns[serverID]; ok {
		close(prev.done)
	}
	c := &Conn{
		serverID: serverID,
		out:      make(chan *agentv1.ServerMessage, 32),
		done:     make(chan struct{}),
	}
	r.conns[serverID] = c
	return c
}

// Remove drops a Conn only if it is still the registered one for serverID.
// Returns true when this Conn was the live stream (so the server is now offline).
// A superseded reconnect leaves the newer Conn in place and returns false.
func (r *Registry) Remove(c *Conn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.conns[c.serverID]; ok && cur == c {
		delete(r.conns, c.serverID)
		close(c.done)
		return true
	}
	return false
}

// Send pushes a message to the named server, or returns ErrAgentOffline.
func (r *Registry) Send(serverID string, msg *agentv1.ServerMessage) error {
	r.mu.RLock()
	c, ok := r.conns[serverID]
	r.mu.RUnlock()
	if !ok {
		return ErrAgentOffline
	}
	return c.Send(msg)
}
