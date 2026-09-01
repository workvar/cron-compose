'use strict';

const { parseEnvText } = require('./pm2-env');
const assert = require('assert');

const got = parseEnvText([
  '# comment',
  'PORT="3007"',
  'CC_WEB_PORT="3007"',
  'PUBLIC_BASE_URL="https://admin.workvar.com"',
  "SINGLE='ok'",
  'PLAIN=3000',
  'PASSWORD="p#ass$word"',
  '',
].join('\n'));

assert.strictEqual(got.PORT, '3007', 'quoted PORT must drop the quotes');
assert.strictEqual(got.CC_WEB_PORT, '3007');
assert.strictEqual(got.PUBLIC_BASE_URL, 'https://admin.workvar.com');
assert.strictEqual(got.SINGLE, 'ok');
assert.strictEqual(got.PLAIN, '3000');
assert.strictEqual(got.PASSWORD, 'p#ass$word');
assert.ok(!('' in got));

console.log('ok');
