package agentgw

import (
	"context"
	"errors"
	"io"
	"math"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/croncompose/croncompose/control-plane/internal/metrics"
	"github.com/croncompose/croncompose/control-plane/internal/pki"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// AgentStream is the long-lived bidi stream every connected agent holds open. It
// authenticates via the peer's client certificate (mTLS), registers a Conn for the
// server, and then runs two goroutines: one pumping outbound ServerMessages, one
// consuming inbound AgentMessages.
func (s *service) AgentStream(stream agentv1.AgentService_AgentStreamServer) error {
	ctx := stream.Context()

	serverID, err := s.authenticate(ctx)
	if err != nil {
		return err
	}

	conn := s.registry.Add(serverID)
	defer s.registry.Remove(conn)

	metrics.AgentsConnected.Inc()
	defer metrics.AgentsConnected.Dec()

	s.log.Info("agent connected", "server_id", serverID)
	defer s.log.Info("agent disconnected", "server_id", serverID)

	// Outbound: drain the per-conn channel onto the stream.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-conn.done:
				return
			case msg := <-conn.out:
				if err := stream.Send(msg); err != nil {
					s.log.Warn("stream send failed", "server_id", serverID, "err", err)
					return
				}
			}
		}
	}()

	// Push an initial SyncJobs full snapshot so the agent immediately has its job set.
	if err := s.sendFullSync(ctx, conn, serverID); err != nil {
		s.log.Warn("initial sync failed", "server_id", serverID, "err", err)
	}

	// Inbound: dispatch every AgentMessage.
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.handleAgentMessage(ctx, serverID, msg); err != nil {
			s.log.Warn("handle agent msg", "server_id", serverID, "err", err)
		}
	}
}

// authenticate resolves the peer's client cert fingerprint to a server row.
func (s *service) authenticate(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "no peer")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "no tls")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return "", status.Error(codes.Unauthenticated, "no client cert")
	}
	fp := pki.FingerprintDER(tlsInfo.State.PeerCertificates[0].Raw)

	var serverID string
	err := s.pool.QueryRow(ctx, `select id from servers where cert_fingerprint = $1`, fp).Scan(&serverID)
	if errors.Is(err, context.Canceled) || err != nil {
		return "", status.Error(codes.Unauthenticated, "unknown cert")
	}
	return serverID, nil
}

// handleAgentMessage routes one inbound message.
func (s *service) handleAgentMessage(ctx context.Context, serverID string, msg *agentv1.AgentMessage) error {
	switch body := msg.GetBody().(type) {
	case *agentv1.AgentMessage_Hello:
		return s.onHello(ctx, serverID, body.Hello)
	case *agentv1.AgentMessage_Heartbeat:
		return s.onHeartbeat(ctx, serverID, body.Heartbeat)
	case *agentv1.AgentMessage_ConfigAck:
		// no-op for the MVP; in Phase 2 we track applied cursor per server.
		return nil
	case *agentv1.AgentMessage_RunStarted:
		return s.onRunStarted(ctx, serverID, body.RunStarted)
	case *agentv1.AgentMessage_LogChunk:
		return s.onLogChunk(ctx, serverID, body.LogChunk)
	case *agentv1.AgentMessage_RunFinished:
		return s.onRunFinished(ctx, serverID, body.RunFinished)
	case *agentv1.AgentMessage_ConnectorEvent:
		return s.onConnectorEvent(ctx, serverID, body.ConnectorEvent)
	case *agentv1.AgentMessage_ConnectorResult:
		s.pending.Resolve(body.ConnectorResult)
		return nil
	case *agentv1.AgentMessage_TerminalOutput:
		s.terminals.Deliver(body.TerminalOutput)
		return nil
	}
	return nil
}

func (s *service) onHello(ctx context.Context, serverID string, h *agentv1.Hello) error {
	_, err := s.pool.Exec(ctx, `
		update servers set agent_version = $1, os = $2, arch = $3, status = 'online', last_seen_at = now()
		where id = $4
	`, h.GetAgentVersion(), h.GetOs(), h.GetArch(), serverID)
	if err != nil {
		return err
	}

	// Hello is the only moment we reliably know this agent's version, os and arch,
	// so it is where the update offer belongs. Best effort: an agent that cannot be
	// reached right now will say hello again on its next connect.
	if up := s.update.For(h.GetAgentVersion(), h.GetOs(), h.GetArch()); up != nil {
		s.log.Info("offering agent update", "server_id", serverID,
			"from", h.GetAgentVersion(), "to", up.GetTargetVersion())
		if sendErr := s.registry.Send(serverID, &agentv1.ServerMessage{
			Body: &agentv1.ServerMessage_UpdateAgent{UpdateAgent: up},
		}); sendErr != nil {
			s.log.Warn("agent update offer not delivered", "server_id", serverID, "err", sendErr)
		}
	}
	return nil
}

func (s *service) onHeartbeat(ctx context.Context, serverID string, _ *agentv1.Heartbeat) error {
	_, err := s.pool.Exec(ctx, `update servers set last_seen_at = now(), status = 'online' where id = $1`, serverID)
	return err
}

func (s *service) onRunStarted(ctx context.Context, serverID string, r *agentv1.RunStarted) error {
	_, err := s.pool.Exec(ctx, `
		insert into runs (id, job_id, job_version_id, server_id, trigger, status, started_at)
		values ($1, $2, $3, $4, $5, 'running', $6)
		on conflict (id) do update set status = excluded.status, started_at = excluded.started_at
	`, r.GetRunId(), r.GetJobId(), r.GetJobVersionId(), serverID, r.GetTrigger(), r.GetStartedAt().AsTime())
	return err
}

func (s *service) onLogChunk(ctx context.Context, _ string, c *agentv1.LogChunk) error {
	// Live subscribers see everything, cap or no cap: somebody watching a run should
	// not have the stream cut off under them. The cap governs what is stored.
	s.broker.Publish(c.GetRunId(), c)
	metrics.RunLogBytes.Add(float64(len(c.GetData())))

	store, notice := s.logCap.admit(c.GetRunId(), len(c.GetData()))
	if notice {
		s.log.Info("run log truncated", "run_id", c.GetRunId(), "limit_bytes", s.logCap.max)
		// A sentinel seq, not the current chunk's: if stderr is the stream that
		// overflowed, reusing its seq would collide with a line already stored and the
		// ON CONFLICT would silently drop the notice, which is the one line that must
		// not go missing. MaxInt32 also sorts it last, where a reader expects it.
		_, err := s.pool.Exec(ctx, `
			insert into run_logs (run_id, stream, seq, chunk)
			values ($1, 'stderr', $2, $3)
			on conflict (run_id, stream, seq) do nothing
		`, c.GetRunId(), int32(math.MaxInt32),
			"croncompose: output limit reached; the rest of this run's log was not stored")
		return err
	}
	if !store {
		return nil
	}

	_, err := s.pool.Exec(ctx, `
		insert into run_logs (run_id, stream, seq, chunk)
		values ($1, $2, $3, $4)
		on conflict (run_id, stream, seq) do nothing
	`, c.GetRunId(), c.GetStream(), c.GetSeq(), string(c.GetData()))
	return err
}

func (s *service) onRunFinished(ctx context.Context, serverID string, r *agentv1.RunFinished) error {
	_, err := s.pool.Exec(ctx, `
		update runs
		set status = $1, exit_code = $2, finished_at = $3, duration_ms = $4, error = nullif($5, '')
		where id = $6
	`, r.GetStatus(), r.GetExitCode(), r.GetFinishedAt().AsTime(), r.GetDurationMs(), r.GetError(), r.GetRunId())
	s.logCap.done(r.GetRunId())
	if err == nil {
		metrics.RunsTotal.WithLabelValues(r.GetStatus()).Inc()
		metrics.RunDuration.WithLabelValues(r.GetStatus()).Observe(float64(r.GetDurationMs()) / 1000)
		s.broker.PublishFinished(r.GetRunId(), r)
		// Fire failure notification if hook installed and status is non-success.
		if s.onFailed != nil && r.GetStatus() != "succeeded" {
			var jobID string
			_ = s.pool.QueryRow(ctx, `select job_id from runs where id = $1`, r.GetRunId()).Scan(&jobID)
			go s.onFailed.FireRunFailed(serverID, jobID, r.GetRunId(), r.GetStatus(), r.GetExitCode(), r.GetDurationMs(), r.GetError())
		}
	}
	return err
}

// sendFullSync loads every enabled job for the server and pushes one SyncJobs.
func (s *service) sendFullSync(ctx context.Context, conn *Conn, serverID string) error {
	sync, err := BuildFullSync(ctx, s.pool, s.resolver, s.log, serverID)
	if err != nil {
		return err
	}
	return conn.Send(&agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_SyncJobs{SyncJobs: sync},
	})
}
