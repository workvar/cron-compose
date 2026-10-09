/**
 * Build a simple multi-page PDF (Courier text) from log output and trigger a download.
 * No external deps — plain PDF 1.4 with Type1 Courier.
 */

function pdfEscape(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/\(/g, "\\(").replace(/\)/g, "\\)");
}

function wrapLine(line: string, maxChars: number): string[] {
  if (line.length <= maxChars) return [line];
  const out: string[] = [];
  for (let i = 0; i < line.length; i += maxChars) {
    out.push(line.slice(i, i + maxChars));
  }
  return out;
}

function buildPageContent(lines: string[], title: string, pageNum: number, pageCount: number): string {
  const parts: string[] = ["BT", "/F1 9 Tf", "50 762 Td"];
  parts.push(`(${pdfEscape(`${title} — page ${pageNum}/${pageCount}`)}) Tj`);
  parts.push("0 -16 Td", "/F1 8 Tf");
  for (let i = 0; i < lines.length; i++) {
    if (i > 0) parts.push("0 -11 Td");
    parts.push(`(${pdfEscape(lines[i])}) Tj`);
  }
  parts.push("ET");
  return parts.join("\n");
}

function byteLength(s: string): number {
  return new TextEncoder().encode(s).length;
}

export function downloadLogsPdf(opts: {
  title: string;
  body: string;
  filename?: string;
}): void {
  const maxChars = 96;
  const linesPerPage = 62;
  const raw = (opts.body || "(empty)").replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  const wrapped: string[] = [];
  for (const line of raw.split("\n")) {
    wrapped.push(...wrapLine(line.length ? line : " ", maxChars));
  }
  if (wrapped.length === 0) wrapped.push("(empty)");

  const pages: string[][] = [];
  for (let i = 0; i < wrapped.length; i += linesPerPage) {
    pages.push(wrapped.slice(i, i + linesPerPage));
  }
  const pageCount = pages.length;
  const title = opts.title || "Process logs";
  const fontObj = 3;

  type Block = { pageObj: number; contentObj: number; stream: string };
  const blocks: Block[] = [];
  let next = 4;
  for (let i = 0; i < pages.length; i++) {
    const contentObj = next++;
    const pageObj = next++;
    blocks.push({
      pageObj,
      contentObj,
      stream: buildPageContent(pages[i], title, i + 1, pageCount),
    });
  }

  const objBodies: Record<number, string> = {
    1: "<< /Type /Catalog /Pages 2 0 R >>",
    2: `<< /Type /Pages /Kids [${blocks.map((b) => `${b.pageObj} 0 R`).join(" ")}] /Count ${pageCount} >>`,
    3: "<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>",
  };
  for (const b of blocks) {
    const len = byteLength(b.stream);
    objBodies[b.contentObj] = `<< /Length ${len} >>\nstream\n${b.stream}\nendstream`;
    objBodies[b.pageObj] =
      `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents ${b.contentObj} 0 R /Resources << /Font << /F1 ${fontObj} 0 R >> >> >>`;
  }

  const maxObj = next - 1;
  let pdf = "%PDF-1.4\n";
  const offsets: number[] = [0];
  for (let i = 1; i <= maxObj; i++) {
    offsets[i] = byteLength(pdf);
    pdf += `${i} 0 obj\n${objBodies[i]}\nendobj\n`;
  }
  const xrefPos = byteLength(pdf);
  pdf += `xref\n0 ${maxObj + 1}\n`;
  pdf += "0000000000 65535 f \n";
  for (let i = 1; i <= maxObj; i++) {
    pdf += `${String(offsets[i]).padStart(10, "0")} 00000 n \n`;
  }
  pdf += `trailer\n<< /Size ${maxObj + 1} /Root 1 0 R >>\nstartxref\n${xrefPos}\n%%EOF\n`;

  const blob = new Blob([pdf], { type: "application/pdf" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  const safe = (opts.filename || title).replace(/[^\w.-]+/g, "_").slice(0, 80);
  a.href = url;
  a.download = safe.endsWith(".pdf") ? safe : `${safe}.pdf`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
