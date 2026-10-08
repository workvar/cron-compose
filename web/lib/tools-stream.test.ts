import assert from "node:assert/strict";
import { consumeToolsSSE } from "./tools-stream.ts";

const body = [
  ": connected\n\n",
  'event: log\ndata: {"chunk":"line1\\n","percent":12,"seq":1}\n\n',
  'event: log\ndata: {"chunk":"line2\\n","percent":40,"seq":2}\n\n',
  'event: done\ndata: {"status":"succeeded","exit_code":0,"log":"line1\\nline2\\n","tools":[]}\n\n',
].join("");

const stream = new ReadableStream({
  start(controller) {
    controller.enqueue(new TextEncoder().encode(body));
    controller.close();
  },
});
const res = new Response(stream, {
  headers: { "content-type": "text/event-stream" },
});

const logs: string[] = [];
let doneStatus = "";
await consumeToolsSSE(res, {
  onLog: (ev) => logs.push(ev.chunk || ""),
  onDone: (ev) => {
    doneStatus = ev.status || "";
  },
});

assert.equal(logs.join(""), "line1\nline2\n");
assert.equal(doneStatus, "succeeded");
console.log("tools-stream.test.ts: ok");
