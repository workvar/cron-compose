package agentgw

import (
	"context"
	"encoding/json"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// onConnectorEvent upserts the connector + resource cache from an agent discovery push.
//
// The raw SQL lives here, in the gateway, rather than in the connectors REST package on
// purpose: the connectors package imports agentgw for the gateway in later phases, so the
// gateway must not import connectors, or the two would form an import cycle. The cache is
// a denormalized projection, so plain upserts are enough.
func (s *service) onConnectorEvent(ctx context.Context, serverID string, ev *agentv1.ConnectorEvent) error {
	for _, dc := range ev.GetConnectors() {
		if err := s.upsertConnector(ctx, serverID, dc); err != nil {
			s.log.Warn("upsert connector", "server_id", serverID, "kind", dc.GetKind(), "err", err)
		}
	}
	return nil
}

func (s *service) upsertConnector(ctx context.Context, serverID string, dc *agentv1.DiscoveredConnector) error {
	caps := map[string]bool{
		"manages_config":  dc.GetManagesConfig(),
		"manages_objects": dc.GetManagesObjects(),
		"can_validate":    dc.GetCanValidate(),
		"can_reload":      dc.GetCanReload(),
		"can_lifecycle":   dc.GetCanLifecycle(),
		"can_edit":        dc.GetCanEdit(),
	}
	capsJSON, _ := json.Marshal(caps)
	pathsJSON, _ := json.Marshal(nonNilStrings(dc.GetConfigPaths()))
	detailJSON, _ := json.Marshal(nonNilMap(dc.GetDetail()))

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Upsert the connector. RETURNING id yields the stable existing id on conflict, since
	// id is not in the SET list.
	var connID string
	err = tx.QueryRow(ctx, `
		insert into connectors (id, server_id, kind, instance, version, status, manageable,
			capabilities, config_paths, object_count, detail, last_seen_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
		on conflict (server_id, kind, instance) do update set
			version = excluded.version, status = excluded.status, manageable = excluded.manageable,
			capabilities = excluded.capabilities, config_paths = excluded.config_paths,
			object_count = excluded.object_count, detail = excluded.detail, last_seen_at = now()
		returning id
	`,
		ids.New(), serverID, dc.GetKind(), dc.GetInstance(), nullStr(dc.GetVersion()),
		dc.GetStatus(), dc.GetManageable(), capsJSON, pathsJSON, dc.GetObjectCount(), detailJSON,
	).Scan(&connID)
	if err != nil {
		return err
	}

	// Discovery always carries the full current resource set, so replace wholesale to
	// drop anything that disappeared since the last push.
	if _, err := tx.Exec(ctx, `delete from connector_resources where connector_id = $1`, connID); err != nil {
		return err
	}
	for _, r := range dc.GetResources() {
		attrsJSON, _ := json.Marshal(nonNilMap(r.GetAttributes()))
		if _, err := tx.Exec(ctx, `
			insert into connector_resources
				(id, connector_id, type, ref, name, state, checksum, size_bytes, attributes, updated_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
			on conflict (connector_id, type, ref) do update set
				name = excluded.name, state = excluded.state, checksum = excluded.checksum,
				size_bytes = excluded.size_bytes, attributes = excluded.attributes, updated_at = now()
		`,
			ids.New(), connID, r.GetType(), r.GetRef(), r.GetName(), nullStr(r.GetState()),
			nullStr(r.GetChecksum()), r.GetSizeBytes(), attrsJSON,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// nullStr maps "" to a SQL NULL so nullable text columns stay null rather than empty.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nonNilMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return in
}
