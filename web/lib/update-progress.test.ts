import assert from "node:assert/strict";
import {
  pipelineFor,
  stepState,
  type UpdatePhase,
} from "./update-progress.ts";

{
  const steps = pipelineFor(true);
  const labels = steps.map((s) => s.label);
  assert.ok(labels.includes("Downloading images and files"));
  assert.ok(labels.includes("Stopping the server"));
  assert.ok(labels.includes("Restarting services"));
}

{
  const steps = pipelineFor(false);
  const labels = steps.map((s) => s.label);
  assert.ok(labels.includes("Cloning the release"));
  assert.ok(labels.includes("Restarting the agent"));
  assert.equal(labels.includes("Downloading images and files"), false);
}

{
  const steps = pipelineFor(true);
  const states = steps.map((s) => stepState(s.id, "building"));
  const current = steps.find((s) => stepState(s.id, "building") === "current");
  assert.ok(current, "building should highlight a current step");
  const doneCount = states.filter((s) => s === "done").length;
  assert.ok(doneCount >= 1, "earlier steps should be marked done");
  const pendingCount = states.filter((s) => s === "pending").length;
  assert.ok(pendingCount >= 1, "later steps should stay pending");
}

{
  assert.equal(stepState("restarting", "done" as UpdatePhase), "done");
  assert.equal(stepState("verifying", "offered"), "pending");
}
