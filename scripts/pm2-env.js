// Minimal .env reader for ecosystem.config.js. Mirrors the control plane's
// dotenv loader: the first '=' splits key/value, surrounding quotes are stripped,
// '#' lines and blanks are skipped. No dependency on dotenv so `pm2 start` works
// on a bare checkout.
//
// The installer writes PORT="3007" (quoted). Next.js does parseInt(PORT) and
// falls back to 3000 if the quotes are left in, so unquoting is load-bearing.

const fs = require('fs');

function unquoteEnv(v) {
  v = v.trim();
  const n = v.length;
  if (n >= 2 && v[0] === '"' && v[n - 1] === '"') {
    return v.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, '\\');
  }
  if (n >= 2 && v[0] === "'" && v[n - 1] === "'") {
    return v.slice(1, -1);
  }
  const i = v.indexOf(' #');
  if (i >= 0) return v.slice(0, i).trim();
  return v;
}

/** @returns {Record<string,string>} parsed pairs. */
function parseEnvText(content) {
  const out = {};
  for (const line of content.split('\n')) {
    const trimmed = line.replace(/\r$/, '').trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const i = trimmed.indexOf('=');
    if (i <= 0) continue;
    const k = trimmed.slice(0, i).trim();
    if (!k) continue;
    out[k] = unquoteEnv(trimmed.slice(i + 1));
  }
  return out;
}

/** @returns {Record<string,string>} parsed pairs, or {} if the file is absent. */
function loadEnvFile(file) {
  if (!fs.existsSync(file)) return {};
  return parseEnvText(fs.readFileSync(file, 'utf8'));
}

/** Process env wins over .env, so `FOO=x pm2 restart` still overrides. */
function mergeEnv(file) {
  return { ...loadEnvFile(file), ...process.env };
}

module.exports = { unquoteEnv, parseEnvText, loadEnvFile, mergeEnv };
