"use client";

interface Bar {
  label: string;
  value: number;
  color?: string;
}

export function BarChart({ bars, height = 220 }: { bars: Bar[]; height?: number }) {
  const w = 640;
  const pad = 32;
  const max = Math.max(1, ...bars.map((b) => b.value));
  const bw = bars.length ? (w - 2 * pad) / bars.length : 0;
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" role="img" data-testid="bar-chart">
      <line x1={pad} y1={height - pad} x2={w - pad} y2={height - pad} stroke="#d1d5db" />
      {bars.map((b, i) => {
        const h = (b.value / max) * (height - 2 * pad);
        return (
          <g key={b.label}>
            <rect x={pad + i * bw + 6} y={height - pad - h} width={bw - 12} height={h} fill={b.color ?? "#0052cc"} />
            <text x={pad + i * bw + bw / 2} y={height - pad + 14} fontSize={10} textAnchor="middle" fill="#6b7280">{b.label}</text>
            <text x={pad + i * bw + bw / 2} y={height - pad - h - 4} fontSize={10} textAnchor="middle" fill="#1a1f36">{b.value}</text>
          </g>
        );
      })}
    </svg>
  );
}
