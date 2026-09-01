// PM2 process definitions for CronCompose. A drop-in replacement for the generated
// croncompose-ctl.sh: same processes, same .env, plus restarts, log rotation and
// boot persistence.
//
//   ./install/install.sh --non-interactive   # builds binaries + web, writes .env
//   ./croncompose-ctl.sh stop                # free the ports first
//   make pm2-start && make pm2-save
//
// All config comes from the repo-root .env written by the installer. Shell env wins
// over .env, so `LOG_LEVEL=debug pm2 restart ecosystem.config.js --update-env` works.

const path = require('path');
const fs = require('fs');
const { mergeEnv } = require('./scripts/pm2-env');

const root = __dirname;
const env = mergeEnv(path.join(root, '.env'));

const runtimeDir = env.CC_RUNTIME_DIR || path.join(root, '.run');
const logDir = path.join(runtimeDir, 'logs');
// Next.js does parseInt(PORT) and falls back to 3000 on NaN, so a quoted leftover
// like `"3007"` would silently bind the default and collide with whatever is there.
const webPort = String(parseInt(env.CC_WEB_PORT || env.PORT || '3000', 10) || 3000);
const standalone = path.join(root, 'web', '.next', 'standalone');

/** Options every process shares. */
const base = (name) => ({
  name: `croncompose-${name}`,
  exec_mode: 'fork',
  instances: 1,
  autorestart: true,
  max_restarts: 10,
  restart_delay: 2000,
  out_file: path.join(logDir, `${name}.out.log`),
  error_file: path.join(logDir, `${name}.err.log`),
  merge_logs: true,
  time: true,
});

/** Control plane: REST /api, UI proxy /app, agent mTLS gRPC. The public entry point. */
const controlPlane = {
  ...base('control-plane'),
  cwd: root,
  script: path.join(root, 'control-plane', 'bin', 'control-plane'),
  kill_timeout: 10000,
  max_memory_restart: '512M',
  env,
};

/** Next.js standalone UI. Loopback only; reached via the control plane's /app proxy. */
const web = {
  ...base('web'),
  cwd: standalone, // standalone server.js resolves assets relative to its own dir
  script: 'server.js',
  interpreter: 'node',
  kill_timeout: 5000,
  max_memory_restart: '768M',
  env: { ...env, NODE_ENV: 'production', HOSTNAME: '127.0.0.1', PORT: webPort },
};

/** Optional local agent, only once `agent enroll` has written an identity. */
const agent = {
  ...base('agent'),
  cwd: root,
  script: path.join(root, 'agent', 'bin', 'agent'),
  args: 'run',
  kill_timeout: 15000, // let in-flight job runs finish
  max_memory_restart: '256M',
  env,
};

const apps = [controlPlane];
if (env.CC_ENABLE_WEB !== '0' && fs.existsSync(path.join(standalone, 'server.js'))) {
  apps.push(web);
}
if (env.CC_ENABLE_AGENT === '1' && fs.existsSync(path.join(runtimeDir, 'agent', 'identity.json'))) {
  apps.push(agent);
}

module.exports = { apps };
