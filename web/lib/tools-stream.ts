import type { ToolStatus } from "@/lib/types";

export type ToolsStreamLog = {
  request_id?: string;
  kind?: string;
  chunk?: string;
  percent?: number;
  seq?: number;
};

export type ToolsStreamDone = {
  op?: string;
  run_as?: string;
  tool?: string;
  status?: string;
  exit_code?: number;
  error?: string;
  log?: string;
  tools?: ToolStatus[];
};

export type ToolsStreamHandlers = {
  onLog: (ev: ToolsStreamLog) => void;
  onDone: (ev: ToolsStreamDone) => void;
};

/** Parse an SSE body from a fetch Response (POST install/uninstall). */
export async function consumeToolsSSE(res: Response, handlers: ToolsStreamHandlers): Promise<void> {
  if (!res.body) {
    // Fallback for proxies that buffer the whole response as text.
    const text = await res.text();
    parseSSEBuffer(text, handlers);
    return;
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    buf = flushSSE(buf, handlers);
  }
  if (buf.trim()) flushSSE(buf + "\n\n", handlers);
}

function flushSSE(buf: string, handlers: ToolsStreamHandlers): string {
  let rest = buf;
  for (;;) {
    const sep = rest.indexOf("\n\n");
    if (sep < 0) return rest;
    const block = rest.slice(0, sep);
    rest = rest.slice(sep + 2);
    dispatchBlock(block, handlers);
  }
}

function parseSSEBuffer(text: string, handlers: ToolsStreamHandlers) {
  flushSSE(text.endsWith("\n\n") ? text : text + "\n\n", handlers);
}

function dispatchBlock(block: string, handlers: ToolsStreamHandlers) {
  let event = "message";
  const dataLines: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith(":") || line.trim() === "") continue;
    if (line.startsWith("event:")) {
      event = line.slice(6).trim();
      continue;
    }
    if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).trimStart());
    }
  }
  if (dataLines.length === 0) return;
  const data = dataLines.join("\n");
  try {
    const parsed = JSON.parse(data) as ToolsStreamLog & ToolsStreamDone;
    if (event === "log") handlers.onLog(parsed);
    else if (event === "done") handlers.onDone(parsed);
  } catch {
    /* ignore malformed */
  }
}
