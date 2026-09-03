// Shared types mirroring the control-plane API responses. Keep these intentionally
// small; expand as new endpoints are wired in.

export type Me = { id: string; email: string; name: string; role: string };

export type Server = {
  id: string;
  name: string;
  description?: string;
  os?: string;
  arch?: string;
  labels: Record<string, string>;
  status: "pending" | "online" | "offline";
  agent_version?: string;
  last_seen_at?: string | null;
  created_at: string;
};

export type ListResponse<T> = {
  items: T[];
  next_cursor?: string;
};

export type UpdateServerStatus = {
  server_id: string;
  server_name: string;
  status: Server["status"];
  current_version?: string;
  update_available: boolean;
  can_update: boolean;
  /** True when this agent runs on the control-plane host (full stack rebuild). */
  stack?: boolean;
};

export type UpdateStatus = {
  repo?: string;
  latest_version?: string;
  release_url?: string;
  published_at?: string;
  check_error?: string;
  items: UpdateServerStatus[];
};

export type CreateServerResponse = {
  server: Server;
  enrollment: { token: string; expires_at: string };
  install_command: string;
};

export type Job = {
  id: string;
  target_kind: "server" | "labels";
  server_id?: string | null;
  target_labels: Record<string, string>;
  name: string;
  description?: string;
  interpreter: string;
  schedule_cron: string;
  timezone: string;
  enabled: boolean;
  timeout_seconds: number;
  concurrency_policy: "skip" | "allow" | "queue";
  catchup_policy: "once" | "all" | "skip";
  max_retries: number;
  working_dir?: string;
  run_as_user?: string;
  cpu_quota_percent: number;
  memory_max_mb: number;
  tasks_max: number;
  io_weight: number;
  current_version_id: string;
  current_version: number;
  script_body: string;
  env: Record<string, string>;
  secret_refs: string[];
  created_at: string;
  updated_at: string;
};

export type RunNowResult = {
  runs: Array<{
    server_id: string;
    run_id: string;
    status: "queued" | "agent_offline";
  }>;
};

export type Run = {
  id: string;
  job_id: string;
  job_version_id: string;
  server_id: string;
  trigger: "schedule" | "manual" | "api";
  status: "pending" | "running" | "succeeded" | "failed" | "timed_out" | "canceled" | "skipped";
  scheduled_for?: string;
  started_at?: string;
  finished_at?: string;
  exit_code?: number;
  duration_ms?: number;
  error?: string;
  created_at: string;
};

export type LogLine = {
  stream: "stdout" | "stderr";
  seq: number;
  chunk: string;
  ts: string;
};

export type Secret = {
  id: string;
  scope: "global" | "server" | "job";
  scope_id?: string;
  name: string;
  created_at: string;
};

export type NotificationTarget = {
  id: string;
  name: string;
  kind: "webhook" | "slack" | "email";
  url?: string;
  enabled: boolean;
  /** Channel-specific settings. Secret values come back as "********". */
  config?: Record<string, string>;
  /** Empty means every server. */
  server_labels?: Record<string, string>;
  /** Empty means every non-success outcome. */
  on_statuses?: string[];
  last_error?: string;
  last_fired_at?: string | null;
  created_at: string;
};

export type Connector = {
  id: string;
  server_id: string;
  server_name?: string;
  kind: string;
  instance: string;
  version?: string;
  status: "running" | "stopped" | "degraded" | "unknown";
  manageable: boolean;
  capabilities: Record<string, boolean>;
  config_paths: string[];
  object_count: number;
  detail?: Record<string, string>;
  last_seen_at?: string | null;
  created_at: string;
};

export type ConnectorResource = {
  id: string;
  connector_id: string;
  type: "config_file" | "object";
  ref: string;
  name: string;
  state?: string;
  checksum?: string;
  size_bytes?: number;
  attributes?: Record<string, string>;
  updated_at: string;
};

export type ConnectorStep = {
  name: string;
  ok: boolean;
  output?: string;
  exit_code?: number;
};

export type ConnectorOperation = {
  id: string;
  connector_id: string;
  server_id: string;
  request_id: string;
  op: string;
  action?: string;
  ref?: string;
  dry_run: boolean;
  status: string;
  message?: string;
  steps: ConnectorStep[];
  actor_user_id?: string | null;
  created_at: string;
  finished_at?: string | null;
};

export type ConnectorSnapshot = {
  id: string;
  connector_id: string;
  ref: string;
  checksum?: string;
  size_bytes: number;
  reason: string;
  operation_id?: string | null;
  actor_user_id?: string | null;
  created_at: string;
};

export type ConnectorCommandResponse = {
  operation_id: string;
  status: string;
  message?: string;
  checksum?: string;
  steps?: ConnectorStep[];
};

export type ConnectorPort = {
  proto: string;
  address: string;
  port: number;
  pid: number;
  process: string;
  ref: string;
  name: string;
  protected: boolean;
  label?: string;
};

export type JobTemplate = {
  id: string;
  name: string;
  description?: string;
  category: string;
  interpreter: string;
  script_body: string;
  schedule_cron: string;
  timezone: string;
  env: Record<string, string>;
  /** Built-ins ship with CronCompose and cannot be edited or deleted. */
  builtin: boolean;
  created_by?: string | null;
  created_at: string;
};
