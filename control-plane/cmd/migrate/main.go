package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/dbmigrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "migrations", "directory containing *.sql migration files")
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres connection string (defaults to $DATABASE_URL)")
	flag.Parse()

	if *dbURL == "" {
		return fmt.Errorf("no database URL: set $DATABASE_URL or pass -db")
	}

	files, err := sqlFiles(*dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .sql files found in %s", *dir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := connect(ctx, *dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	pending, err := dbmigrate.Apply(ctx, pool, *dir)
	if err != nil {
		return err
	}

	if pending == 0 {
		fmt.Println("database is up to date")
	} else {
		fmt.Printf("applied %d migration(s)\n", pending)
	}
	return nil
}

// connect dials the database, retrying briefly so the tool tolerates being run the
// instant a freshly started Postgres is accepting connections.
func connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("connect: %w", lastErr)
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
	return files, nil
}
