// Day × hour heatmap of run counts.
export type HeatCell = { day: number; hour: number; value: number };

const DAYS = ["S", "M", "T", "W", "T", "F", "S"];

export function Heatmap({ cells }: { cells: HeatCell[] }) {
  const max = Math.max(1, ...cells.map((c) => c.value));
  const lookup = new Map(cells.map((c) => [`${c.day}-${c.hour}`, c.value]));

  return (
    <div className="heatmap" role="img" aria-label="Run activity heatmap">
      <div className="heatmap-hours">
        {[0, 6, 12, 18].map((h) => (
          <span key={h} style={{ gridColumn: h + 2 }}>{h}</span>
        ))}
      </div>
      {DAYS.map((label, day) => (
        <div className="heatmap-row" key={day}>
          <span className="heatmap-day">{label}</span>
          {Array.from({ length: 24 }, (_, hour) => {
            const v = lookup.get(`${day}-${hour}`) ?? 0;
            const intensity = v / max;
            return (
              <span
                key={hour}
                className="heatmap-cell"
                title={`${label} ${hour}:00 · ${v}`}
                style={{ opacity: v === 0 ? 0.18 : 0.35 + intensity * 0.65 }}
              />
            );
          })}
        </div>
      ))}
    </div>
  );
}
