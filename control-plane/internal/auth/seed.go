package auth

import (
	"context"
	"fmt"
	"log/slog"
)

// SeedAdmin upserts the configured admin user if both env values are set. Safe to call
// every boot; password is updated to match env each time so operators can rotate by
// editing the env and restarting.
func SeedAdmin(ctx context.Context, log *slog.Logger, s *Store, email, password string) error {
	if email == "" || password == "" {
		log.Warn("admin seed skipped (SEED_ADMIN_EMAIL / SEED_ADMIN_PASSWORD not set)")
		return fmt.Errorf("SEED_ADMIN_EMAIL / SEED_ADMIN_PASSWORD not set")
	}
	hash, err := Hash(password)
	if err != nil {
		log.Error("admin seed hash failed", "err", err)
		return err
	}
	if _, err := s.Upsert(ctx, email, "Admin", "owner", hash); err != nil {
		log.Error("admin seed failed", "err", err)
		return err
	}
	log.Info("admin seeded", "email", email)
	return nil
}
