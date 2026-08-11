// PM2 process definitions for CronCompose.
//
// Usage:
//   make control-plane && cd web && npm ci && npm run build && cd ..
//   pm2 start ecosystem.config.js --env production
//   pm2 save && pm2 startup
//
// Secrets are read from the shell/.env, never hardcoded here:
//   SESSION_SECRET, SECRETS_MASTER_KEY, SEED_ADMIN_EMAIL, SEED_ADMIN_PASSWORD,
//   DATABASE_URL, PUBLIC_BASE_URL, TLS_HOSTS

const path = require('path');

const root = __dirname;
const logDir = process.env.CC_LOG_DIR || path.join(root, 'logs');

/** Env shared by both processes. */
const common = {
  APP_ENV: 'production',
  LOG_LEVEL: process.env.LOG_LEVEL || 'info',
};

/** Control plane: REST API, /app reverse proxy, agent mTLS gRPC. */
const controlPlane = {
  name: 'croncompose-control-plane',
  cwd: path.join(root, 'control-plane'),
  script: path.join(root, 'control-plane', 'bin', 'control-plane'),
  exec_mode: 'fork',
  instances: 1,
  autorestart: true,
  max_restarts: 10,
  restart_delay: 2000,
  kill_timeout: 10000,
  max_memory_restart: '512M',
  out_file: path.join(logDir, 'control-plane.out.log'),
  error_file: path.join(logDir, 'control-plane.err.log'),
  merge_logs: true,
  time: true,
  env: {
    ...common,
    HTTP_ADDR: process.env.HTTP_ADDR || ':8080',
    GRPC_ADDR: process.env.GRPC_ADDR || ':9090',
    TLS_DIR: process.env.TLS_DIR || path.join(root, 'control-plane', 'tls'),
    TLS_HOSTS: process.env.TLS_HOSTS || 'localhost,127.0.0.1',
    WEB_UPSTREAM: process.env.WEB_UPSTREAM || 'http://127.0.0.1:3000',
    DATABASE_URL:
      process.env.DATABASE_URL ||
      'postgres://croncompose:croncompose@localhost:5432/croncompose?sslmode=disable',
    ENROLL_TOKEN_TTL: process.env.ENROLL_TOKEN_TTL || '30m',
    // Set PUBLIC_BASE_URL and PUBLIC_HTTP_URL / PUBLIC_GRPC_ADDR are derived.
    PUBLIC_BASE_URL: process.env.PUBLIC_BASE_URL || 'http://localhost:8080',
  },
};

/** Next.js UI. Internal only; reached through the control plane's /app proxy. */
const web = {
  name: 'croncompose-web',
  cwd: path.join(root, 'web'),
  script: 'npm',
  args: 'run start',
  exec_mode: 'fork',
  instances: 1,
  autorestart: true,
  max_restarts: 10,
  restart_delay: 2000,
  kill_timeout: 5000,
  max_memory_restart: '768M',
  out_file: path.join(logDir, 'web.out.log'),
  error_file: path.join(logDir, 'web.err.log'),
  merge_logs: true,
  time: true,
  env: {
    ...common,
    NODE_ENV: 'production',
    HOSTNAME: process.env.WEB_HOST || '127.0.0.1',
    PORT: process.env.WEB_PORT || 3000,
    API_BASE: process.env.API_BASE || 'http://127.0.0.1:8080/api/v1',
  },
};

module.exports = { apps: [controlPlane, web] };
