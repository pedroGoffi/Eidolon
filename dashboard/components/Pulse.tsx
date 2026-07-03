"use client";

/**
 * Pulse é o elemento de assinatura do painel: uma linha de batimento
 * (como um monitor de sinais vitais) que traduz o volume de requests
 * por segundo em uma forma visual imediata — o "fantasma" do Eidolon
 * observando o tráfego passar em tempo real.
 */
export function Pulse({ values, height = 64 }: { values: number[]; height?: number }) {
  const width = 480;
  const max = Math.max(1, ...values);
  const points = values
    .map((v, i) => {
      const x = (i / Math.max(1, values.length - 1)) * width;
      const y = height - (v / max) * (height - 8) - 4;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="h-16 w-full"
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id="pulse-fade" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="#4FD1C5" stopOpacity="0" />
          <stop offset="70%" stopColor="#4FD1C5" stopOpacity="1" />
          <stop offset="100%" stopColor="#4FD1C5" stopOpacity="1" />
        </linearGradient>
      </defs>
      <polyline
        points={points}
        fill="none"
        stroke="url(#pulse-fade)"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
