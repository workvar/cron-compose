package agentgw

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Deploy log rows used to be inserted one at a time on the agent's receive
// loop. Every chunk waited for a Postgres round trip and commit before the next
// message was read, so a chatty build backed up the whole stream (and with it
// the phase lines the run board waits for). The broker still publishes each
// chunk immediately; only storage is batched here, off the receive loop.
const (
	deployLogFlushEvery = 50 * time.Millisecond
	deployLogBatchRows  = 500
	deployLogQueue      = 8192
)

type deployLogRow struct {
	runID string
	seq   int32
	chunk string
}

// deployLogWriter stores deploy log rows in batches. A nil writer stores each
// row inline, which keeps tests that build a bare service working.
type deployLogWriter struct {
	pool  *pgxpool.Pool
	log   *slog.Logger
	rows  chan deployLogRow
	flush chan chan struct{}
}

func newDeployLogWriter(pool *pgxpool.Pool, log *slog.Logger) *deployLogWriter {
	w := &deployLogWriter{
		pool:  pool,
		log:   log,
		rows:  make(chan deployLogRow, deployLogQueue),
		flush: make(chan chan struct{}),
	}
	if pool != nil {
		go w.loop()
	}
	return w
}

// Add queues one row. It blocks only when the queue is full, which is the
// backpressure we want if Postgres falls far behind.
func (w *deployLogWriter) Add(ctx context.Context, pool *pgxpool.Pool, row deployLogRow) {
	row.chunk = cleanLogText(row.chunk)
	if w == nil || w.pool == nil {
		insertDeployLogs(ctx, pool, nil, []deployLogRow{row})
		return
	}
	w.rows <- row
}

// Flush returns once every row queued before the call is stored. The finished
// handler calls it so a run's status never lands ahead of its last log lines.
func (w *deployLogWriter) Flush(ctx context.Context) {
	if w == nil || w.pool == nil {
		return
	}
	done := make(chan struct{})
	select {
	case w.flush <- done:
	case <-ctx.Done():
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (w *deployLogWriter) loop() {
	ticker := time.NewTicker(deployLogFlushEvery)
	defer ticker.Stop()
	var pending []deployLogRow
	write := func() {
		if len(pending) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		insertDeployLogs(ctx, w.pool, w.log, pending)
		cancel()
		pending = pending[:0]
	}
	for {
		select {
		case row := <-w.rows:
			pending = append(pending, row)
			if len(pending) >= deployLogBatchRows {
				write()
			}
		case <-ticker.C:
			write()
		case done := <-w.flush:
			// Take everything queued before the flush request, then write.
			for drained := false; !drained; {
				select {
				case row := <-w.rows:
					pending = append(pending, row)
				default:
					drained = true
				}
			}
			write()
			close(done)
		}
	}
}

const insertDeployLogsSQL = `
	insert into deploy_run_logs (run_id, stream, seq, chunk)
	select r, 'stdout', s, c from unnest($1::text[], $2::int[], $3::text[]) as t(r, s, c)
	on conflict (run_id, stream, seq) do nothing
`

// insertDeployLogs writes rows in one statement. If that fails, rows are retried
// one by one so a single bad chunk cannot drop its neighbours.
func insertDeployLogs(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, rows []deployLogRow) {
	if pool == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, len(rows))
	seqs := make([]int32, len(rows))
	chunks := make([]string, len(rows))
	for i, r := range rows {
		ids[i], seqs[i], chunks[i] = r.runID, r.seq, r.chunk
	}
	_, err := pool.Exec(ctx, insertDeployLogsSQL, ids, seqs, chunks)
	if err == nil {
		return
	}
	if log != nil {
		log.Warn("deploy log batch insert failed; retrying per row", "rows", len(rows), "err", err)
	}
	for _, r := range rows {
		_, _ = pool.Exec(ctx, insertDeployLogsSQL, []string{r.runID}, []int32{r.seq}, []string{r.chunk})
	}
}

// cleanLogText makes a chunk storable as Postgres text: no NUL bytes and valid
// UTF-8. Either one would otherwise fail the insert.
func cleanLogText(s string) string {
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return strings.ToValidUTF8(s, "�")
}
