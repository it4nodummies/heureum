"use client";

// StackedAreaChart per la CFD: date sull'asse X, categorie impilate.
export function StackedAreaChart({
  dates,
  categories,
  data,
  height = 240,
}: {
  dates: string[];
  categories: string[];
  data: Record<string, number[]>;
  height?: number;
}) {
  const w = 640;
  const pad = 32;
  const colors: Record<string, string> = { todo: "#8993a4", inprogress: "#0052cc", done: "#00875a" };
  const n = Math.max(1, dates.length - 1);
  const x = (i: number) => pad + (i * (w - 2 * pad)) / n;
  // cumulativo per punto
  const cum = dates.map((_, i) => categories.reduce((a, c) => a + (data[c]?.[i] ?? 0), 0));
  const max = Math.max(1, ...cum);
  const y = (v: number) => height - pad - (v / max) * (height - 2 * pad);
  let below = dates.map(() => 0);
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" role="img" data-testid="cfd-chart">
      {categories.map((c) => {
        const top = dates.map((_, i) => below[i] + (data[c]?.[i] ?? 0));
        const area =
          dates.map((_, i) => `${x(i)},${y(top[i])}`).join(" ") +
          " " +
          dates.map((_, i) => `${x(dates.length - 1 - i)},${y(below[dates.length - 1 - i])}`).join(" ");
        below = top;
        return <polygon key={c} points={area} fill={colors[c] ?? "#c1c7d0"} fillOpacity={0.85} />;
      })}
      {categories.map((c, ci) => (
        <g key={c}>
          <rect x={pad + ci * 110} y={8} width={10} height={10} fill={colors[c] ?? "#c1c7d0"} />
          <text x={pad + ci * 110 + 14} y={17} fontSize={11} fill="#6b7280">{c}</text>
        </g>
      ))}
    </svg>
  );
}
