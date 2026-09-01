// Package agentenroll serves the REST endpoint agents call to swap a one-time
// enrollment token for a signed client certificate. We use REST rather than gRPC for
// this because the call happens once per agent lifetime and avoids the chicken-and-egg
// of "the agent needs a cert to talk gRPC, but it gets the cert via gRPC".
package agentenroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/pki"
)

// Request is the JSON body POSTed by the agent.
type Request struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agent_version"`
	CSRPEM       string `json:"csr_pem"`        // raw PEM or base64-wrapped PEM
	CSRBase64    string `json:"csr_pem_base64"` // optional, convenience for clients
}

// Response is the JSON body returned on success.
type Response struct {
	ServerID             string `json:"server_id"`
	ClientCertPEM        string `json:"client_cert_pem"`
	ServerCAPEM          string `json:"server_ca_pem"`
	ControlPlaneGRPCAddr string `json:"control_plane_grpc_addr"`
}

type handler struct {
	log      *slog.Logger
	pool     *pgxpool.Pool
	bundle   *pki.Bundle
	grpcAddr string
}

func Register(r fiber.Router, log *slog.Logger, pool *pgxpool.Pool, bundle *pki.Bundle, grpcAddr string) {
	h := &handler{log: log, pool: pool, bundle: bundle, grpcAddr: grpcAddr}
	r.Post("/agents/enroll", h.enroll)
}

func (h *handler) enroll(c fiber.Ctx) error {
	var req Request
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, "bad_request", err)
	}
	if req.Token == "" {
		return badRequest(c, "missing_token", errors.New("token is required"))
	}
	csrPEM, err := decodeCSR(req)
	if err != nil {
		return badRequest(c, "bad_csr", err)
	}

	serverID, err := h.consumeToken(c.Context(), req.Token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "invalid_token", "message": err.Error()},
		})
	}

	certPEM, fingerprint, err := h.bundle.SignAgentCSR(csrPEM, serverID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "sign_failed", "message": err.Error()},
		})
	}

	if _, err := h.pool.Exec(c.Context(), `
		update servers
		set cert_fingerprint = $1, os = $2, arch = $3, agent_version = $4, status = 'online', last_seen_at = now()
		where id = $5
	`, fingerprint, req.OS, req.Arch, req.AgentVersion, serverID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "bind_failed", "message": err.Error()},
		})
	}

	h.log.Info("agent enrolled", "server_id", serverID, "hostname", req.Hostname)

	return c.JSON(Response{
		ServerID:             serverID,
		ClientCertPEM:        string(certPEM),
		ServerCAPEM:          string(h.bundle.CACertPEM),
		ControlPlaneGRPCAddr: h.grpcAddr,
	})
}

// consumeToken validates the one-time token and marks it used in a single transaction.
func (h *handler) consumeToken(ctx context.Context, plain string) (string, error) {
	hash := sha256Hex(plain)

	var serverID string
	var expiresAt time.Time
	var usedAt *time.Time
	err := h.pool.QueryRow(ctx, `
		select server_id, expires_at, used_at from enrollment_tokens where token_hash = $1
	`, hash).Scan(&serverID, &expiresAt, &usedAt)
	if err != nil {
		return "", errors.New("token not recognised")
	}
	if usedAt != nil {
		return "", errors.New("token already used")
	}
	if time.Now().After(expiresAt) {
		return "", errors.New("token expired")
	}
	if _, err := h.pool.Exec(ctx, `update enrollment_tokens set used_at = now() where token_hash = $1`, hash); err != nil {
		return "", err
	}
	return serverID, nil
}

func decodeCSR(r Request) ([]byte, error) {
	if r.CSRPEM != "" {
		return []byte(r.CSRPEM), nil
	}
	if r.CSRBase64 != "" {
		raw, err := base64.StdEncoding.DecodeString(r.CSRBase64)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	return nil, errors.New("csr_pem or csr_pem_base64 is required")
}

func badRequest(c fiber.Ctx, code string, err error) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": err.Error()},
	})
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
