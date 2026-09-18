// Vercel-style .env paste parsing: strip comments/blanks, KEY=value rows.

export type ParsedEnvVar = { key: string; value: string };

export function parseDotEnv(raw: string): ParsedEnvVar[] {
  const out: ParsedEnvVar[] = [];
  const seen = new Map<string, number>();
  for (const lineRaw of raw.split("\n")) {
    let line = lineRaw.trim();
    if (!line || line.startsWith("#")) continue;
    if (line.startsWith("export ")) line = line.slice(7).trim();
    const i = line.indexOf("=");
    if (i <= 0) continue;
    let key = line.slice(0, i).trim().replace(/^['"]|['"]$/g, "");
    if (!key || /\s/.test(key)) continue;
    let value = line.slice(i + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    const idx = seen.get(key);
    if (idx !== undefined) {
      out[idx] = { key, value };
      continue;
    }
    seen.set(key, out.length);
    out.push({ key, value });
  }
  return out;
}
