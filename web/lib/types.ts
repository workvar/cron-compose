// Shared types mirroring the control-plane API responses. Keep these intentionally
// small; expand as new endpoints are wired in.

export type Me = { id: string; email: string; name: string; role: string };

export type Passkey = {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string | null;
};

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
  agent_root_enabled: boolean;
  agent_euid_root: boolean;
  agent_root_changed_at?: string | null;
  agent_root_changed_by?: string | null;
  agent_service_user?: string;
};

// One OS account the web terminal could switch to, from GET
// /servers/:id/terminal/users. `available` is false when the account exists but the
// agent on that server isn't running as root, so it can't actually become that user
// yet. Unavailable accounts are hidden until root mode is active
// (agent_root_enabled && agent_euid_root).
export type SystemUser = {
  username: string;
  uid: number;
  home: string;
  shell: string;
  available: boolean;
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
  /** Live self-update stage from the agent, when an update is in flight. */
  update_phase?: string;
  update_detail?: string;
  update_percent?: number;
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
  /** Which event families this fires for. Empty means job runs only. */
  events?: ("job_run" | "deploy")[];
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

export type GitConnection = {
  id: string;
  provider: "github" | "gitlab" | string;
  purpose: string;
  login: string;
  email: string;
  provider_user_id: string;
  created_at: string;
};

// Admin-configured GitHub/GitLab OAuth app credentials (Settings > Git OAuth). The
// client secret itself is never sent to the browser, only whether one is set.
export type OAuthSettings = {
  provider: "github" | "gitlab";
  client_id: string;
  has_secret: boolean;
  redirect_url: string;
  base_url?: string;
  configured: boolean;
  updated_at: string;
};

export type GitRepo = {
  id: string;
  full_name: string;
  description?: string;
  default_branch: string;
  clone_url: string;
  private: boolean;
};

export type DeployDetection = {
  language: string;
  install_script: string;
  has_pm2_ecosystem: boolean;
  supports_port: boolean;
  workspaces: string[];
  root_directory: string;
};

export type DeployInspect = DeployDetection & {
  clone_url: string;
  default_branch: string;
  clone_path: string;
  process_manager: string;
};

export type DeployApp = {
  name: string;
  root: string;
  language?: string;
  install?: string;
  port?: number;
  process_manager?: string;
};

export type DeployHealthState = "unknown" | "healthy" | "degraded" | "rolled_back";

export type DeployProject = {
  id: string;
  name: string;
  provider: string;
  repo_full_name: string;
  repo_id?: string;
  clone_url: string;
  default_branch: string;
  server_id: string;
  language: string;
  install_script: string;
  root_directory: string;
  clone_path: string;
  port: number;
  process_manager: string;
  env: Record<string, string>;
  apps: DeployApp[];
  write_spec: boolean;
  /** Redeploy the last successful commit automatically after a failed run. */
  auto_rollback: boolean;
  /**
   * Opt-in health probe. With an empty path a deploy is successful as soon as the
   * install script exits 0; with one set, the app has to answer on
   * 127.0.0.1:<health_port || port><health_path> before the run counts.
   */
  health_path: string;
  health_port: number;
  health_timeout_seconds: number;
  /** Whole-run budget handed to the agent. 0 means the agent's default. */
  deploy_timeout_seconds: number;
  /** Where the project stands now, as opposed to what its last run did. */
  health_state: DeployHealthState;
  created_by?: string;
  created_at: string;
  updated_at: string;
  deploy_token?: string;
};

export type DeployRun = {
  id: string;
  project_id: string;
  server_id: string;
  trigger: string;
  status: "pending" | "running" | "succeeded" | "failed" | "canceled" | "agent_offline";
  branch: string;
  /** Filled in once the agent reports the commit it checked out. */
  commit_sha?: string;
  exit_code?: number;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
};

export type DeploySettings = {
  language_paths: Record<string, string>;
  updated_at: string;
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
