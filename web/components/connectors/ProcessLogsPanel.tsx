"use client";

import { downloadLogsPdf } from "@/lib/logs-pdf";

type Props = {
  title: string;
  logs: string;
  busy?: boolean;
  onClose: () => void;
  onRefresh?: () => void;
};

export function ProcessLogsPanel({ title, logs, busy, onClose, onRefresh }: Props) {
  function downloadPdf() {
    downloadLogsPdf({
      title,
      body: logs,
      filename: `${title}-logs`,
    });
  }

  function downloadTxt() {
    const blob = new Blob([logs || ""], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${title.replace(/[^\w.-]+/g, "_").slice(0, 80)}-logs.txt`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="process-logs-panel">
      <div className="cluster" style={{ justifyContent: "space-between", marginBottom: 8, gap: 8 }}>
        <div className="card-title" style={{ margin: 0 }}>
          Logs · {title}
        </div>
        <div className="cluster" style={{ gap: 6 }}>
          {onRefresh && (
            <button type="button" className="button secondary sm" disabled={busy} onClick={onRefresh}>
              Refresh
            </button>
          )}
          <button
            type="button"
            className="button secondary sm"
            disabled={busy || !logs}
            onClick={downloadTxt}
            title="Download as text"
          >
            .txt
          </button>
          <button
            type="button"
            className="button secondary sm"
            disabled={busy || !logs}
            onClick={downloadPdf}
            title="Download as PDF"
          >
            Download PDF
          </button>
          <button type="button" className="button secondary sm" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
      <pre className="process-logs-pre">{busy ? "Loading…" : logs || "(no output)"}</pre>
    </div>
  );
}
