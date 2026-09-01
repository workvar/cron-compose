// Package retention prunes old history so a long-running control plane does not grow
// without bound.
//
// Two windows, because the two things age differently. Run logs are large and are read
// within hours; the runs themselves are small and are what "has this job been failing
// all month?" is answered from. Keeping logs for two weeks and runs for three months is
// a far better default than one window for both.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/metrics"
)

// Config is the retention policy. A zero or negative window disables pruning for that
// table, which is the right default for anyone who has not thought about it yet.
type Config struct {
	RunDays       int
	RunLogDays    int
	AuditDays     int
	OperationDays int
	// Interval between sweeps. Zero uses the default.
	Interval time.Duration
}

// DefaultInterval is how often the pruner sweeps. Hourly is frequent enough that the
// table never carries more than an hour of expired rows, and rare enough that the
// deletes never compete with real traffic for long.
const DefaultInterval = time.Hour

// Pruner deletes expired rows on a schedule.
type Pruner struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	cfg  Config
}

// New builds a Pruner.
func New(pool *pgxpool.Pool, log *slog.Logger, cfg Config) *Pruner {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	return &Pruner{pool: pool, log: log, cfg: cfg}
}

// Enabled reports whether any window is set. A disabled pruner is not started at all,
// so its goroutine does not exist rather than existing and doing nothing.
func (p *Pruner) Enabled() bool {
	return p.cfg.RunDays > 0 || p.cfg.RunLogDays > 0 || p.cfg.AuditDays > 0 || p.cfg.OperationDays > 0
}

// Start runs the pruner until ctx is cancelled. The first sweep is delayed rather than
// immediate: startup is the worst moment to add database load, and an hour of extra
// history has never hurt anyone.
func (p *Pruner) Start(ctx context.Context) {
	p.log.Info("retention pruner started",
		"run_days", p.cfg.RunDays, "run_log_days", p.cfg.RunLogDays,
		"audit_days", p.cfg.AuditDays, "operation_days", p.cfg.OperationDays,
		"interval", p.cfg.Interval)

	t := time.NewTicker(p.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.sweep(ctx)
		}
	}
}

// sweep runs one pass. Each delete is independent: one failing table must not stop the
// others from being pruned.
func (p *Pruner) sweep(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Logs first. They are the bulk of the data, and deleting them before the runs
	// means the run cascade has less to do.
	p.prune(ctx, "run_logs", p.cfg.RunLogDays, `
		delete from run_logs
		 where ctid in (
		   select l.ctid from run_logs l
		     join runs r on r.id = l.run_id
		    where r.created_at < now() - make_interval(days => $1)
		    limit $2
		 )`)

	p.prune(ctx, "runs", p.cfg.RunDays, `
		delete from runs
		 where id in (
		   select id from runs
		    where created_at < now() - make_interval(days => $1)
		    limit $2
		 )`)

	p.prune(ctx, "audit_log", p.cfg.AuditDays, `
		delete from audit_log
		 where id in (
		   select id from audit_log
		    where ts < now() - make_interval(days => $1)
		    limit $2
		 )`)

	p.prune(ctx, "connector_operations", p.cfg.OperationDays, `
		delete from connector_operations
		 where id in (
		   select id from connector_operations
		    where created_at < now() - make_interval(days => $1)
		    limit $2
		 )`)
}

// batchSize bounds one DELETE. Deleting a month of logs in a single statement would
// hold locks and bloat the WAL; looping in batches keeps each statement short enough
// that normal traffic never notices.
const batchSize = 5000

// prune deletes in batches until a pass removes nothing, or the context expires.
func (p *Pruner) prune(ctx context.Context, table string, days int, sql string) {
	if days <= 0 {
		return
	}
	total := int64(0)
	for {
		if ctx.Err() != nil {
			break
		}
		tag, err := p.pool.Exec(ctx, sql, days, batchSize)
		if err != nil {
			p.log.Warn("retention prune failed", "table", table, "err", err)
			return
		}
		n := tag.RowsAffected()
		total += n
		metrics.RetentionDeletedTotal.WithLabelValues(table).Add(float64(n))
		if n < batchSize {
			break
		}
	}
	if total > 0 {
		p.log.Info("retention pruned", "table", table, "rows", total, "older_than_days", days)
	}
}
