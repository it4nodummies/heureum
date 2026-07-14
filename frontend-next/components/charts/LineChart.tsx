"use client";

interface Series {
  name: string;
  color: string;
  values: number[];
}

// LineChart: grafico a linee dependency-free (SVG). labels sull'asse X, una o più serie.
export function LineChart({ labels, series, height = 220 }: { labels: string[]; series: Series[]; height?: number }) {
  const w = 640;
  const pad = 32;
  const max = Math.max(1, ...series.flatMap((s) => s.values));
  const n = Math.max(1, labels.length - 1);
  const x = (i: number) => pad + (i * (w - 2 * pad)) / n;
  const y = (v: number) => height - pad - (v / max) * (height - 2 * pad);
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" role="img" data-testid="line-chart">
      <line x1={pad} y1={height - pad} x2={w - pad} y2={height - pad} stroke="#d1d5db" />
      <line x1={pad} y1={pad} x2={pad} y2={height - pad} stroke="#d1d5db" />
      {series.map((s) => (
        <polyline
          key={s.name}
          fill="none"
          stroke={s.color}
          strokeWidth={2}
          points={s.values.map((v, i) => `${x(i)},${y(v)}`).join(" ")}
        />
      ))}
      {series.map((s, si) => (
        <g key={s.name}>
          <rect x={pad + si * 120} y={8} width={10} height={10} fill={s.color} />
          <text x={pad + si * 120 + 14} y={17} fontSize={11} fill="#6b7280">{s.name}</text>
        </g>
      ))}
    </svg>
  );
}
