"use client";

const POINTS = [
  { x: 48, y: 148, label: "SSH" },
  { x: 110, y: 118, label: "Scripts" },
  { x: 172, y: 88, label: "Cron" },
  { x: 234, y: 58, label: "Agents" },
  { x: 296, y: 36, label: "Fleet" },
];

export function EfficiencyCurve() {
  return (
    <div className="curve-panel in">
      <div className="curve-meta">
        <strong>Operational leverage</strong>
        <span className="subtle" style={{ fontSize: 12 }}>from ad-hoc ops → composed fleet</span>
      </div>
      <svg className="curve-svg" viewBox="0 0 360 180" role="img" aria-label="Operational leverage curve">
        <text x="12" y="24" className="curve-label">HIGH</text>
        <text x="12" y="168" className="curve-label">LOW</text>
        <line x1="40" y1="20" x2="40" y2="160" stroke="var(--border)" strokeWidth="1" />
        <line x1="40" y1="160" x2="340" y2="160" stroke="var(--border)" strokeWidth="1" />
        <path
          className="curve-path"
          d="M48 148 C 100 140, 120 120, 172 88 S 250 48, 296 36"
        />
        {POINTS.map((p) => (
          <g key={p.label}>
            <circle className="curve-dot" cx={p.x} cy={p.y} r="5" />
            <text x={p.x} y={p.y - 12} textAnchor="middle" className="curve-label">{p.label}</text>
          </g>
        ))}
      </svg>
    </div>
  );
}
