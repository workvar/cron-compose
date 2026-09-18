// Bubble chart — size encodes a third metric.
export type Bubble = { x: number; y: number; r: number; label: string; tone?: string };

export function BubbleChart({ bubbles }: { bubbles: Bubble[] }) {
  const w = 320;
  const h = 160;
  const pad = 28;
  const maxX = Math.max(1, ...bubbles.map((b) => b.x));
  const maxY = Math.max(1, ...bubbles.map((b) => b.y));
  const maxR = Math.max(1, ...bubbles.map((b) => b.r));

  return (
    <svg className="chart-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Bubble chart">
      {bubbles.map((b, i) => {
        const cx = pad + (b.x / maxX) * (w - pad * 2);
        const cy = h - pad - (b.y / maxY) * (h - pad * 2);
        const r = 8 + (b.r / maxR) * 22;
        return (
          <g key={i}>
            <circle cx={cx} cy={cy} r={r} className="bubble" style={{ fill: b.tone || "var(--green-mint)" }} />
            <text x={cx} y={cy + 3} textAnchor="middle" className="bubble-label">{b.label.slice(0, 8)}</text>
          </g>
        );
      })}
    </svg>
  );
}
