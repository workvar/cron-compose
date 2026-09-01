package connectors

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// ErrSnapshotNotFound is returned when a rollback names a snapshot that is gone.
var ErrSnapshotNotFound = errors.New("snapshot not found")

const opCols = `
  id, connector_id, server_id, request_id, op, action, ref, dry_run,
  status, message, steps, actor_user_id, created_at, finished_at
`

// StartOperation records a command before it is sent, so an operation that never comes
// back still leaves a row. The caller passes the request id it will put on the wire.
func (s *Store) StartOperation(ctx context.Context, op Operation) (string, error) {
	id := ids.New()
	_, err := s.pool.Exec(ctx, `
		insert into connector_operations
		  (id, connector_id, server_id, request_id, op, action, ref, dry_run, status, actor_user_id)
		values ($1,$2,$3,$4,$5,$6,$7,$8,'pending',nullif($9,''))
	`, id, op.ConnectorID, op.ServerID, op.RequestID, op.Op, op.Action, op.Ref, op.DryRun,
		derefStr(op.ActorUserID))
	if err != nil {
		return "", err
	}
	return id, nil
}

// FinishOperation closes the row out with whatever the agent said, or with the reason
// we never heard back. Called on every exit path, including timeouts.
func (s *Store) FinishOperation(ctx context.Context, id, status, message string, steps []Step) error {
	if steps == nil {
		steps = []Step{}
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		update connector_operations
		   set status = $2, message = $3, steps = $4, finished_at = now()
		 where id = $1
	`, id, status, message, raw)
	return err
}

// ListOperations returns recent operations for one connector, newest first.
func (s *Store) ListOperations(ctx context.Context, connectorID string, limit int) ([]Operation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		select `+opCols+`
		  from connector_operations
		 where connector_id = $1
		 order by created_at desc
		 limit $2
	`, connectorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Operation{}
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

func scanOperation(row pgx.Row) (Operation, error) {
	var o Operation
	var steps []byte
	if err := row.Scan(&o.ID, &o.ConnectorID, &o.ServerID, &o.RequestID, &o.Op, &o.Action,
		&o.Ref, &o.DryRun, &o.Status, &o.Message, &steps, &o.ActorUserID,
		&o.CreatedAt, &o.FinishedAt); err != nil {
		return o, err
	}
	o.Steps = []Step{}
	if len(steps) > 0 {
		_ = json.Unmarshal(steps, &o.Steps)
	}
	return o, nil
}

// SaveSnapshot stores the pre-apply bytes of a config file. Called before the command
// goes out, so a crash between the snapshot and the apply leaves a usable backup
// rather than none.
func (s *Store) SaveSnapshot(ctx context.Context, snap Snapshot, content []byte) (string, error) {
	id := ids.New()
	_, err := s.pool.Exec(ctx, `
		insert into connector_snapshots
		  (id, connector_id, ref, checksum, content, size_bytes, reason, operation_id, actor_user_id)
		values ($1,$2,$3,$4,$5,$6,$7,nullif($8,''),nullif($9,''))
	`, id, snap.ConnectorID, snap.Ref, snap.Checksum, content, int64(len(content)),
		snap.Reason, derefStr(snap.OperationID), derefStr(snap.ActorUserID))
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListSnapshots returns the backup history for a connector, newest first, WITHOUT the
// file bytes. Pass an empty ref for every file.
func (s *Store) ListSnapshots(ctx context.Context, connectorID, ref string, limit int) ([]Snapshot, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		select id, connector_id, ref, checksum, size_bytes, reason,
		       operation_id, actor_user_id, created_at
		  from connector_snapshots
		 where connector_id = $1 and ($2 = '' or ref = $2)
		 order by created_at desc
		 limit $3
	`, connectorID, ref, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Snapshot{}
	for rows.Next() {
		var sn Snapshot
		if err := rows.Scan(&sn.ID, &sn.ConnectorID, &sn.Ref, &sn.Checksum, &sn.SizeBytes,
			&sn.Reason, &sn.OperationID, &sn.ActorUserID, &sn.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// GetSnapshot returns one snapshot including its bytes. Used by rollback.
func (s *Store) GetSnapshot(ctx context.Context, id string) (Snapshot, []byte, error) {
	var sn Snapshot
	var content []byte
	err := s.pool.QueryRow(ctx, `
		select id, connector_id, ref, checksum, size_bytes, reason,
		       operation_id, actor_user_id, created_at, content
		  from connector_snapshots
		 where id = $1
	`, id).Scan(&sn.ID, &sn.ConnectorID, &sn.Ref, &sn.Checksum, &sn.SizeBytes, &sn.Reason,
		&sn.OperationID, &sn.ActorUserID, &sn.CreatedAt, &content)
	if errors.Is(err, pgx.ErrNoRows) {
		return sn, nil, ErrSnapshotNotFound
	}
	return sn, content, err
}

// PruneSnapshots keeps the newest n snapshots per (connector, ref) and deletes the
// rest. Config files are small but an operator iterating on one over a week should not
// grow the table without bound.
func (s *Store) PruneSnapshots(ctx context.Context, connectorID, ref string, keep int) error {
	if keep <= 0 {
		keep = 20
	}
	_, err := s.pool.Exec(ctx, `
		delete from connector_snapshots
		 where id in (
		   select id from connector_snapshots
		    where connector_id = $1 and ref = $2
		    order by created_at desc
		    offset $3
		 )
	`, connectorID, ref, keep)
	return err
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
