import assert from "node:assert/strict";
import { parseDotEnv } from "./parse-dotenv.ts";

{
  const got = parseDotEnv(`
# comment
NODE_ENV=production

API_KEY="secret value"
export PORT=3000
`);
  assert.deepEqual(got, [
    { key: "NODE_ENV", value: "production" },
    { key: "API_KEY", value: "secret value" },
    { key: "PORT", value: "3000" },
  ]);
}

{
  const got = parseDotEnv("A=1\nA=2\n");
  assert.deepEqual(got, [{ key: "A", value: "2" }]);
}

console.log("parse-dotenv: ok");
