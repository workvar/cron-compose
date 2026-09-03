// Package config loads runtime config from environment variables. Keep this small;
// production deployments can layer a file-based loader on top of these defaults.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config is the resolved set of options the control plane runs with.
type Config struct {
	HTTPAddr        string   // Fiber REST listener, e.g. :8080
	GRPCAddr        string   // gRPC agent listener, e.g. :9090
	WebUpstream     string   // internal Next.js UI address to reverse-proxy /app to; empty disables
	DatabaseURL     string   // Postgres connection string
	ProjectRoot     string   // repo root for dev bootstrap (docker-compose, migrations)
	MigrationsDir   string   // SQL migrations directory; empty means <ProjectRoot>/migrations
	AutoBootstrapDB bool     // in dev, prepare Postgres + migrate when db is down at boot
	BootstrapMode   string   // local | docker | none
	Env             string   // dev | prod
	LogLevel        string   // debug | info | warn | error
	EnrollTokenTTL  string   // e.g. "30m"
	TLSDir          string   // where ca.crt / ca.key / server.crt / server.key live
	TLSHosts        []string // SANs the server cert should cover

	SessionSecret     string // HMAC secret for session cookies
	SeedAdminEmail    string // optional: created/updated on every boot
	SeedAdminPassword string

	// PublicBaseURL is the single source of truth for how the control plane is reached
	// from outside (e.g. https://cron.example.com or http://raspberrypi.local:8080).
	// When set, it derives PublicHTTPURL, PublicGRPCAddr, the OIDC redirect, and adds
	// its host to the TLS SANs. Set it once to change every advertised URL.
	PublicBaseURL string

	// PublicHTTPURL is the externally reachable URL for the REST API (used in the
	// install command shown after creating a server). PublicGRPCAddr is the same for
	// the mTLS gRPC endpoint. InstallScriptURL is the agent installer URL. These are
	// derived from PublicBaseURL when it is set, unless explicitly overridden.
	PublicHTTPURL    string
	PublicGRPCAddr   string
	InstallScriptURL string

	SecretsMasterKey string // 32 bytes hex

	// Retention windows, in days. Zero disables pruning for that table, which is the
	// default: deleting a user's history is not something to start doing unasked.
	// Logs and runs age differently, so they have separate windows.
	RetentionRunDays       int
	RetentionRunLogDays    int
	RetentionAuditDays     int
	RetentionOperationDays int

	// RunLogMaxBytes caps how much output one run may store. Zero uses the default.
	RunLogMaxBytes int

	// MetricsToken, when set, is required as a bearer token on /metrics. Empty leaves
	// the endpoint open, which is fine on a private network and not fine on the
	// internet.
	MetricsToken string

	// Agent self-update. Inert unless AgentUpdateVersion, AgentUpdateURL and
	// AgentUpdateSHA256 are all set: the control plane will not ask an agent to
	// replace its own binary without a pinned checksum to verify it against.
	//
	// AgentUpdateURL is a template; {version}, {os} and {arch} are substituted.
	// AgentUpdateSHA256 is either a bare hex digest (one platform) or a JSON object
	// mapping "os/arch" to a digest.
	AgentUpdateVersion string
	AgentUpdateURL     string
	AgentUpdateSHA256  string
	AgentUpdateRestart bool

	// GitHubReleaseRepo is checked for new agent tags when no manual AGENT_UPDATE_*
	// policy is configured. Empty disables GitHub polling.
	GitHubReleaseRepo      string
	AgentUpdatePollMinutes int

	// OIDC SSO. Empty issuer URL disables the SSO path.
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCDefaultRole  string
}

// Load reads config from the environment with sensible dev defaults.
func Load() (Config, error) {
	loadDotEnvFromCWD()
	c := Config{
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		GRPCAddr:        env("GRPC_ADDR", ":9090"),
		WebUpstream:     env("WEB_UPSTREAM", ""),
		DatabaseURL:     env("DATABASE_URL", "postgres://croncompose:croncompose@localhost:5432/croncompose?sslmode=disable"),
		ProjectRoot:     env("CRONCOMPOSE_ROOT", ""),
		MigrationsDir:   env("MIGRATIONS_DIR", ""),
		AutoBootstrapDB: envBool("AUTO_BOOTSTRAP_DB", env("APP_ENV", "dev") == "dev"),
		BootstrapMode:   env("DB_BOOTSTRAP", "local"),
		Env:             env("APP_ENV", "dev"),
		LogLevel:        env("LOG_LEVEL", "info"),
		EnrollTokenTTL:  env("ENROLL_TOKEN_TTL", "30m"),
		TLSDir:          env("TLS_DIR", "./tls"),
		TLSHosts:        splitCSV(env("TLS_HOSTS", "localhost,127.0.0.1")),

		SessionSecret:     env("SESSION_SECRET", "dev-only-do-not-use-in-prod"),
		SeedAdminEmail:    env("SEED_ADMIN_EMAIL", ""),
		SeedAdminPassword: env("SEED_ADMIN_PASSWORD", ""),

		PublicBaseURL:    env("PUBLIC_BASE_URL", ""),
		PublicHTTPURL:    env("PUBLIC_HTTP_URL", "http://localhost:8080/api/v1"),
		PublicGRPCAddr:   env("PUBLIC_GRPC_ADDR", "localhost:9090"),
		InstallScriptURL: env("INSTALL_SCRIPT_URL", "https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh"),

		// 32-byte hex. Default is a clearly-marked dev key so local dev works; prod
		// MUST set this to a real value generated with `openssl rand -hex 32`.
		RetentionRunDays:       envInt("RETENTION_RUN_DAYS", 0),
		RetentionRunLogDays:    envInt("RETENTION_RUN_LOG_DAYS", 0),
		RetentionAuditDays:     envInt("RETENTION_AUDIT_DAYS", 0),
		RetentionOperationDays: envInt("RETENTION_OPERATION_DAYS", 0),
		RunLogMaxBytes:         envInt("RUN_LOG_MAX_BYTES", 0),
		MetricsToken:           env("METRICS_TOKEN", ""),

		AgentUpdateVersion: env("AGENT_UPDATE_VERSION", ""),
		AgentUpdateURL:     env("AGENT_UPDATE_URL", ""),
		AgentUpdateSHA256:  env("AGENT_UPDATE_SHA256", ""),
		AgentUpdateRestart: env("AGENT_UPDATE_RESTART", "1") != "0",

		GitHubReleaseRepo:      env("GITHUB_RELEASE_REPO", "workvar/cron-compose"),
		AgentUpdatePollMinutes: envInt("AGENT_UPDATE_POLL_MINUTES", 1440),

		SecretsMasterKey: env("SECRETS_MASTER_KEY", "0000000000000000000000000000000000000000000000000000000000000000"),

		OIDCIssuerURL:    env("OIDC_ISSUER_URL", ""),
		OIDCClientID:     env("OIDC_CLIENT_ID", ""),
		OIDCClientSecret: env("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:  env("OIDC_REDIRECT_URL", ""),
		OIDCDefaultRole:  env("OIDC_DEFAULT_ROLE", "viewer"),
	}
	if err := c.applyPublicBaseURL(); err != nil {
		return c, err
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.SessionSecret) < 16 {
		return c, fmt.Errorf("SESSION_SECRET must be at least 16 chars")
	}
	return c, nil
}

// applyPublicBaseURL makes PublicBaseURL the single point of change: when set, it
// derives the REST URL, the gRPC dial address, the OIDC redirect, and the TLS SAN.
// Any value explicitly provided via its own env var still wins, so advanced setups
// (e.g. gRPC behind a different host/port) remain possible.
func (c *Config) applyPublicBaseURL() error {
	if c.PublicBaseURL == "" {
		return nil
	}
	base := strings.TrimRight(c.PublicBaseURL, "/")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid PUBLIC_BASE_URL %q: %v", c.PublicBaseURL, err)
	}
	host := u.Hostname()

	if os.Getenv("PUBLIC_HTTP_URL") == "" {
		c.PublicHTTPURL = base + "/api/v1"
	}
	if os.Getenv("PUBLIC_GRPC_ADDR") == "" {
		c.PublicGRPCAddr = net.JoinHostPort(host, portOf(c.GRPCAddr, "9090"))
	}
	if c.OIDCIssuerURL != "" && os.Getenv("OIDC_REDIRECT_URL") == "" {
		c.OIDCRedirectURL = base + "/api/v1/auth/oidc/callback"
	}
	c.TLSHosts = ensureHost(c.TLSHosts, host)
	return nil
}

// portOf returns the port from a listen address like ":9090" or "0.0.0.0:9090",
// falling back to def.
func portOf(addr, def string) string {
	if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
		return port
	}
	return def
}

// ensureHost appends host to hosts if not already present.
func ensureHost(hosts []string, host string) []string {
	if host == "" {
		return hosts
	}
	for _, h := range hosts {
		if h == host {
			return hosts
		}
	}
	return append(hosts, host)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// envInt reads an integer env var, falling back to the default on anything
// unparseable. A typo should not silently mean "zero", which for a retention window
// would mean "never prune" and for a byte cap would mean "use the default" anyway.
func envBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "":
		return def
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	return def
}

func envInt(key string, def int) int {
	v := unquoteEnv(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func env(key, def string) string {
	if v := unquoteEnv(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
