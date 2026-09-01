// Package dbmigrate applies SQL migration files to Postgres, recording each applied
// file in schema_migrations so re-runs are safe.
package dbmigrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Apply runs pending *.sql files from dir in lexical order. Returns how many files
// were newly applied.
func Apply(ctx context.Context, pool *pgxpool.Pool, dir string) (int, error) {
	files, err := sqlFiles(dir)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("no .sql files found in %s", dir)
	}

	if err := ensureTable(ctx, pool); err != nil {
		return 0, err
	}
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return 0, err
	}

	pending := 0
	for _, f := range files {
		version := filepath.Base(f)
		if applied[version] {
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			return pending, fmt.Errorf("read %s: %w", version, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return pending, fmt.Errorf("apply %s: %w", version, err)
		}
		if _, err := pool.Exec(ctx,
			"insert into schema_migrations (version, applied_at) values ($1, now())", version); err != nil {
			return pending, fmt.Errorf("record %s: %w", version, err)
		}
		pending++
	}
	return pending, nil
}

func ensureTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `create table if not exists schema_migrations (
		version    text primary key,
		applied_at timestamptz not null default now()
	)`)
	return err
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "select version from schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func sqlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}
