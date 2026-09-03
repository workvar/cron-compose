export type SelectOption = { value: string; label: string };

export type MappedPort = {
  proto: string;
  address: string;
  port: number;
  pid: number;
  process: string;
  ref: string;
  name: string;
  protected: boolean;
  connector_id: string;
  server_id: string;
  server_name: string;
  kind: string;
  label?: string;
};

export function shouldShowServerPromo(serverCount: number): boolean {
  return serverCount < 1;
}

export function filterSelectOptions(options: SelectOption[], query: string): SelectOption[] {
  const q = query.trim().toLowerCase();
  if (!q) return options;
  return options.filter((o) =>
    o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q),
  );
}

export function secretNeedsScopeId(scope: string): boolean {
  return scope === "server" || scope === "job";
}

export function filterMappedPorts(rows: MappedPort[], query: string): MappedPort[] {
  const q = query.trim().toLowerCase();
  if (!q) return rows;
  return rows.filter((r) => {
    const hay = [
      r.label, r.process, r.name, r.ref, r.address, String(r.port),
      r.server_name, r.kind, r.proto,
    ].join(" ").toLowerCase();
    return hay.includes(q);
  });
}
