/** Human-readable message from a failed fetch; never dumps HTML error pages. */
export async function apiErrorMessage(res: Response, fallback: string): Promise<string> {
  const text = await res.text().catch(() => "");
  const trimmed = text.trim();
  if (!trimmed) return `${fallback} (HTTP ${res.status})`;
  if (/^<!DOCTYPE|^<html/i.test(trimmed)) {
    return `${fallback} (HTTP ${res.status})`;
  }
  try {
    const parsed = JSON.parse(trimmed) as { error?: { message?: string } };
    if (parsed?.error?.message) return parsed.error.message;
  } catch {
    /* not JSON */
  }
  if (trimmed.length > 180) return `${fallback} (HTTP ${res.status})`;
  return trimmed;
}
