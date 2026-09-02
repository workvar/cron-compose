import assert from "node:assert/strict";
import type { UpdateStatus } from "./types.ts";
import { summarizeUpdates } from "./update-helpers.ts";

const empty: UpdateStatus = { items: [] };
{
  const s = summarizeUpdates(null);
  assert.equal(s.headline, "Update status is unavailable.");
  assert.match(s.checkError ?? "", /control plane/);
}

{
  const s = summarizeUpdates(empty);
  assert.equal(s.headline, "No GitHub release found yet.");
  assert.equal(s.pending.length, 0);
}

{
  const s = summarizeUpdates({
    latest_version: "v0.0.2",
    repo: "workvar/cron-compose",
    items: [],
  });
  assert.equal(s.headline, "Latest release is v0.0.2.");
  assert.equal(s.latest, "v0.0.2");
  assert.equal(s.repo, "workvar/cron-compose");
}

{
  const s = summarizeUpdates({
    latest_version: "v0.0.2",
    items: [
      { server_id: "a", server_name: "alpha", status: "online", current_version: "v0.0.1", update_available: true, can_update: true },
      { server_id: "b", server_name: "bravo", status: "online", current_version: "v0.0.2", update_available: false, can_update: false },
    ],
  });
  assert.equal(s.pending.length, 1);
  assert.equal(s.updatable.length, 1);
  assert.equal(s.current.length, 1);
  assert.match(s.headline, /1 server/);
}

{
  const s = summarizeUpdates({
    latest_version: "v0.0.2",
    items: [
      { server_id: "a", server_name: "alpha", status: "online", current_version: "v0.0.2", update_available: false, can_update: false },
    ],
  });
  assert.equal(s.headline, "All agents are on v0.0.2.");
}

{
  const s = summarizeUpdates({
    latest_version: "v0.0.2",
    items: [
      { server_id: "a", server_name: "alpha", status: "pending", update_available: false, can_update: false },
    ],
  });
  assert.equal(s.headline, "Latest release is v0.0.2. Agents have not reported a version yet.");
}

{
  const s = summarizeUpdates({
    check_error: "github api: HTTP 404",
    items: [],
  });
  assert.equal(s.headline, "Could not check GitHub for a newer release.");
  assert.equal(s.checkError, "github api: HTTP 404");
}

console.log("ok");
