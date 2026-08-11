// Package runtime is the agent's top-level loop: it owns the gRPC stream lifecycle,
// the scheduler, the persistent outbox, and the executor. It dials with backoff so the
// agent keeps running through control-plane outages.
package runtime

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/croncompose/croncompose/agent/internal/config"
	"github.com/croncompose/croncompose/agent/internal/connectors"
	"github.com/croncompose/croncompose/agent/internal/identity"
	"github.com/croncompose/croncompose/agent/internal/outbox"
	"github.com/croncompose/croncompose/agent/internal/scheduler"
	"github.com/croncompose/croncompose/agent/internal/store"
	"github.com/croncompose/croncompose/agent/internal/terminal"
	"github.com/croncompose/croncompose/agent/internal/transport"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Runtime ties together transport, scheduler, executor, store, and outbox.
type Runtime struct {
	cfg    config.Config
	log    *slog.Logger
	store  *store.Store
	ident  identity.Identity
	tlsCfg *tls.Config
	outbox *outbox.Outbox

	wake chan struct{}

	// direct carries EPHEMERAL messages (terminal output) the drain loop sends on the
	// stream without persisting to the outbox. The drain loop stays the sole sender.
	direct chan *agentv1.AgentMessage

	conns     *connectors.Registry
	terminals *terminal.Manager

	sched *scheduler.Scheduler

	jobsMu     sync.RWMutex
	jobs       map[string]store.JobDef
	runsMu     sync.Mutex
	runIndex   map[string]activeRun // runID -> active run
	jobBusy    map[string]int       // jobID -> count of in-progress runs
}

// activeRun is the cancel handle for an in-progress run.
type activeRun struct {
	jobID  string
	cancel context.CancelFunc
}

// New constructs a Runtime. Caller is responsible for the tls.Config and the outbox.
func New(cfg config.Config, log *slog.Logger, st *store.Store, ident identity.Identity, tlsCfg *tls.Config, ob *outbox.Outbox) *Runtime {
	r := &Runtime{
		cfg:        cfg,
		log:        log,
		store:      st,
		ident:      ident,
		tlsCfg:     tlsCfg,
		outbox:     ob,
		wake:       make(chan struct{}, 1),
		direct:     make(chan *agentv1.AgentMessage, 256),
		conns:      connectors.NewRegistry(log),
		jobs:     map[string]store.JobDef{},
		runIndex: map[string]activeRun{},
		jobBusy:  map[string]int{},
	}
	r.sched = scheduler.New(r.onSchedulerFire)
	r.initTerminals()
	return r
}

// Run dials with backoff and keeps the stream alive until ctx is canceled. Scheduled
// jobs continue to fire while disconnected; their events accumulate in the outbox and
// flush on reconnect.
func (r *Runtime) Run(ctx context.Context) error {
	// Seed scheduler from the on-disk job cache so jobs fire even before first SyncJobs.
	if cached, err := r.store.LoadJobs(); err == nil {
		r.applyCachedJobs(cached)
	}
	r.sched.Start()
	defer r.sched.Stop()

	addr := r.cfg.ControlPlaneAddr
	if r.ident.ControlPlaneGRPCAddr != "" {
		addr = r.ident.ControlPlaneGRPCAddr
	}

	backoff := newBackoff()
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := r.connectAndServe(ctx, addr); err != nil {
			r.log.Warn("connect cycle ended", "err", err, "pending_outbox", r.outbox.Len())
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff.next()):
		}
	}
}

// connectAndServe runs one connection cycle: dial, open stream, send Hello, drain
// outbox, run recv loop. Returns when the stream errors.
func (r *Runtime) connectAndServe(ctx context.Context, addr string) error {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := transport.Dial(connCtx, addr, r.tlsCfg)
	if err != nil {
		return err
	}
	defer client.Close()

	stream, err := client.OpenStream(connCtx)
	if err != nil {
		return err
	}
	r.log.Info("stream open")

	// Terminal sessions are tied to the connection: when this cycle ends, kill any live
	// shells so none linger across a reconnect.
	defer r.terminals.CloseAll()

	// Enqueue Hello and a periodic heartbeat. The drain loop is the sole sender.
	r.queue(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_Hello{Hello: &agentv1.Hello{
			AgentVersion: r.cfg.AgentVersion,
			Os:           "linux",
			Capabilities: []string{"terminal"},
		}},
	})

	go r.heartbeatLoop(connCtx)
	go r.connectorLoop(connCtx)
	go r.drainLoop(connCtx, stream)

	// Recv loop on this goroutine. When it returns, the cycle is over.
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		r.handleServerMessage(connCtx, msg)
	}
}

// queue appends to the persistent outbox and signals the drain.
func (r *Runtime) queue(msg *agentv1.AgentMessage) {
	if err := r.outbox.Enqueue(msg); err != nil {
		r.log.Warn("outbox enqueue failed", "err", err)
		return
	}
	r.signalWake()
}

func (r *Runtime) signalWake() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// drainLoop is the sole sender on the gRPC stream. It pulls from the outbox in order
// and exits when ctx is done OR when the stream errors.
func (r *Runtime) drainLoop(ctx context.Context, stream agentv1.AgentService_AgentStreamClient) {
	// Drain anything already on disk first.
	if err := r.outbox.Drain(func(m *agentv1.AgentMessage) error { return stream.Send(m) }); err != nil {
		r.log.Warn("initial drain ended", "err", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-r.direct:
			// Ephemeral terminal output: send immediately, never persisted.
			if err := stream.Send(m); err != nil {
				r.log.Warn("direct send ended", "err", err)
				return
			}
			continue
		case <-r.wake:
		case <-time.After(2 * time.Second):
			// Safety tick in case a wake signal was dropped.
		}
		if err := r.outbox.Drain(func(m *agentv1.AgentMessage) error { return stream.Send(m) }); err != nil {
			r.log.Warn("drain ended", "err", err)
			return
		}
	}
}

func (r *Runtime) heartbeatLoop(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.queue(&agentv1.AgentMessage{
				Body: &agentv1.AgentMessage_Heartbeat{Heartbeat: &agentv1.Heartbeat{
					Ts: timestamppb.Now(),
				}},
			})
		}
	}
}

// --- backoff ---

type backoff struct {
	cur time.Duration
}

func newBackoff() *backoff { return &backoff{cur: time.Second} }

func (b *backoff) next() time.Duration {
	d := b.cur
	b.cur *= 2
	if b.cur > 30*time.Second {
		b.cur = 30 * time.Second
	}
	return d
}
