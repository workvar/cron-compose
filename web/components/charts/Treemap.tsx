// Nested-rectangle treemap (slice-and-dice by weight).
export type TreeNode = { label: string; value: number; color?: string };

export function Treemap({ nodes }: { nodes: TreeNode[] }) {
  const total = Math.max(1, nodes.reduce((s, n) => s + n.value, 0));
  const sorted = [...nodes].sort((a, b) => b.value - a.value);

  return (
    <div className="treemap" role="img" aria-label="Treemap">
      {sorted.map((n) => {
        const pct = (n.value / total) * 100;
        return (
          <div
            key={n.label}
            className="treemap-cell"
            style={{
              flexGrow: Math.max(n.value, 0.5),
              background: n.color || "var(--green-soft)",
              minWidth: `${Math.max(pct, 8)}%`,
            }}
            title={`${n.label}: ${n.value}`}
          >
            <span className="treemap-label">{n.label}</span>
            <span className="treemap-value">{n.value}</span>
          </div>
        );
      })}
    </div>
  );
}
