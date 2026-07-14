"use client";

interface Slice {
  label: string;
  count: number;
}

const PALETTE = ["#0052cc", "#00875a", "#de350b", "#ff991f", "#6554c0", "#00b8d9", "#8993a4"];

export function PieChart({ slices }: { slices: Slice[] }) {
  const total = slices.reduce((a, s) => a + s.count, 0) || 1;
  const cx = 90, cy = 90, r = 80;
  let acc = 0;
  const arcs = slices.map((s, i) => {
    const start = (acc / total) * 2 * Math.PI;
    acc += s.count;
    const end = (acc / total) * 2 * Math.PI;
    const large = end - start > Math.PI ? 1 : 0;
    const x1 = cx + r * Math.sin(start), y1 = cy - r * Math.cos(start);
    const x2 = cx + r * Math.sin(end), y2 = cy - r * Math.cos(end);
    return { d: `M${cx},${cy} L${x1},${y1} A${r},${r} 0 ${large} 1 ${x2},${y2} Z`, color: PALETTE[i % PALETTE.length], s };
  });
  return (
    <div className="flex items-center gap-6" data-testid="pie-chart">
      <svg viewBox="0 0 180 180" className="h-44 w-44">
        {arcs.map((a) => <path key={a.s.label} d={a.d} fill={a.color} />)}
      </svg>
      <ul className="text-sm">
        {arcs.map((a) => (
          <li key={a.s.label} className="flex items-center gap-2">
            <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: a.color }} />
            <span className="text-[#1a1f36]">{a.s.label}</span>
            <span className="text-slate-400">{a.s.count}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
