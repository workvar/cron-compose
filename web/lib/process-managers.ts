import type { SelectOption } from "./ui-helpers";

/** Options for the process-manager searchable dropdown on a project block. */
export const PROCESS_MANAGER_OPTIONS: SelectOption[] = [
  { value: "none", label: "None — attach later" },
  { value: "pm2", label: "PM2" },
  { value: "systemd", label: "systemd (user unit)" },
  { value: "docker", label: "Docker Compose" },
];
