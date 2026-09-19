import assert from "node:assert/strict";
import { apiErrorMessage } from "./api-error.ts";

{
  const res = new Response("<!DOCTYPE html><html><title>502</title></html>", { status: 502 });
  assert.equal(await apiErrorMessage(res, "Could not list folders"), "Could not list folders (HTTP 502)");
}
{
  const res = new Response(JSON.stringify({ error: { message: "git api 404: Not Found" } }), { status: 502 });
  assert.equal(await apiErrorMessage(res, "Could not list folders"), "git api 404: Not Found");
}
{
  const res = new Response("", { status: 500 });
  assert.equal(await apiErrorMessage(res, "Could not list folders"), "Could not list folders (HTTP 500)");
}

console.log("api-error.test.ts: ok");
