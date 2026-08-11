// Minimal .env reader for ecosystem.config.js. Mirrors croncompose-ctl.sh: the first
// '=' splits key/value, values are kept verbatim, '#' lines and blanks are skipped.
// No dependency on dotenv so `pm2 start` works on a bare checkout.

const fs = require('fs');

/** @returns {Record<string,string>} parsed pairs, or {} if the file is absent. */
function loadEnvFile(file) {
  if (!fs.existsSync(file)) return {};
  const out = {};
  for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const i = trimmed.indexOf('=');
    if (i <= 0) continue;
    out[trimmed.slice(0, i)] = trimmed.slice(i + 1);
  }
  return out;
}

/** Process env wins over .env, so `FOO=x pm2 restart` still overrides. */
function mergeEnv(file) {
  return { ...loadEnvFile(file), ...process.env };
}

module.exports = { loadEnvFile, mergeEnv };
