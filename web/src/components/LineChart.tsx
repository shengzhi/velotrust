import React, { useState } from 'react';

interface TrafficPoint {
  time: string;
  rx: number; // in MB/s or MB
  tx: number;
}

interface LineChartProps {
  data: TrafficPoint[];
  height?: number;
}

export const LineChart: React.FC<LineChartProps> = ({ data, height = 220 }) => {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  if (!data || data.length === 0) {
    return <div className="h-48 flex items-center justify-center text-xs text-slate-400">暂无流量数据</div>;
  }

  const padding = { top: 20, right: 20, bottom: 30, left: 45 };
  const width = 500; // viewBox width

  const maxVal = Math.max(...data.map((d) => Math.max(d.rx, d.tx)), 100);
  const roundedMax = Math.ceil(maxVal / 100) * 100;

  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  const getX = (index: number) => padding.left + (index / (data.length - 1)) * chartWidth;
  const getY = (val: number) => padding.top + chartHeight - (val / roundedMax) * chartHeight;

  const rxPoints = data.map((d, i) => `${getX(i)},${getY(d.rx)}`).join(' ');
  const txPoints = data.map((d, i) => `${getX(i)},${getY(d.tx)}`).join(' ');

  // Gradient area fill
  const rxAreaPoints = `${getX(0)},${padding.top + chartHeight} ${rxPoints} ${getX(
    data.length - 1
  )},${padding.top + chartHeight}`;
  const txAreaPoints = `${getX(0)},${padding.top + chartHeight} ${txPoints} ${getX(
    data.length - 1
  )},${padding.top + chartHeight}`;

  const yTicks = [0, roundedMax * 0.25, roundedMax * 0.5, roundedMax * 0.75, roundedMax];

  return (
    <div className="w-full flex flex-col">
      {/* Legend & Current Hover Value */}
      <div className="flex items-center justify-between pb-2 text-xs">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-sm bg-blue-500" />
            <span className="text-slate-600 font-medium">接收流量 (RX)</span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-sm bg-emerald-500" />
            <span className="text-slate-600 font-medium">发送流量 (TX)</span>
          </div>
        </div>

        {hoverIndex !== null && (
          <div className="flex items-center gap-3 font-mono text-[11px] bg-slate-100/80 px-2 py-0.5 rounded-md text-slate-700">
            <span className="font-semibold text-slate-900">{data[hoverIndex].time}</span>
            <span className="text-blue-600">RX: {data[hoverIndex].rx} MB</span>
            <span className="text-emerald-600">TX: {data[hoverIndex].tx} MB</span>
          </div>
        )}
      </div>

      <div className="relative w-full">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          className="w-full overflow-visible"
          style={{ maxHeight: height }}
        >
          <defs>
            <linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.25" />
              <stop offset="100%" stopColor="#3b82f6" stopOpacity="0.0" />
            </linearGradient>
            <linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#10b981" stopOpacity="0.2" />
              <stop offset="100%" stopColor="#10b981" stopOpacity="0.0" />
            </linearGradient>
          </defs>

          {/* Grid lines and Y axis ticks */}
          {yTicks.map((tickVal, idx) => {
            const y = getY(tickVal);
            return (
              <g key={idx}>
                <line
                  x1={padding.left}
                  y1={y}
                  x2={width - padding.right}
                  y2={y}
                  stroke="#f1f5f9"
                  strokeWidth="1"
                  strokeDasharray="4 4"
                />
                <text
                  x={padding.left - 8}
                  y={y + 3}
                  textAnchor="end"
                  className="text-[9px] fill-slate-400 font-mono"
                >
                  {tickVal}M
                </text>
              </g>
            );
          })}

          {/* Area fills */}
          <polygon points={rxAreaPoints} fill="url(#rxGrad)" />
          <polygon points={txAreaPoints} fill="url(#txGrad)" />

          {/* Polylines */}
          <polyline
            fill="none"
            stroke="#3b82f6"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            points={rxPoints}
          />
          <polyline
            fill="none"
            stroke="#10b981"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            points={txPoints}
          />

          {/* X axis labels and hover detection bars */}
          {data.map((d, i) => {
            const x = getX(i);
            const isHovered = hoverIndex === i;
            return (
              <g key={i}>
                {/* Vertical hover guide */}
                {isHovered && (
                  <line
                    x1={x}
                    y1={padding.top}
                    x2={x}
                    y2={padding.top + chartHeight}
                    stroke="#94a3b8"
                    strokeWidth="1"
                    strokeDasharray="2 2"
                  />
                )}

                {/* RX dot */}
                <circle
                  cx={x}
                  cy={getY(d.rx)}
                  r={isHovered ? 4.5 : 2.5}
                  fill="#ffffff"
                  stroke="#3b82f6"
                  strokeWidth="2"
                  className="transition-all"
                />

                {/* TX dot */}
                <circle
                  cx={x}
                  cy={getY(d.tx)}
                  r={isHovered ? 4.5 : 2.5}
                  fill="#ffffff"
                  stroke="#10b981"
                  strokeWidth="2"
                  className="transition-all"
                />

                {/* X label */}
                <text
                  x={x}
                  y={height - 8}
                  textAnchor="middle"
                  className={`text-[9px] font-mono transition-colors ${
                    isHovered ? 'fill-slate-900 font-semibold' : 'fill-slate-400'
                  }`}
                >
                  {d.time}
                </text>

                {/* Invisible hover trigger area */}
                <rect
                  x={x - chartWidth / (data.length * 2)}
                  y={padding.top}
                  width={chartWidth / data.length}
                  height={chartHeight}
                  fill="transparent"
                  className="cursor-pointer"
                  onMouseEnter={() => setHoverIndex(i)}
                  onMouseLeave={() => setHoverIndex(null)}
                />
              </g>
            );
          })}
        </svg>
      </div>
    </div>
  );
};
