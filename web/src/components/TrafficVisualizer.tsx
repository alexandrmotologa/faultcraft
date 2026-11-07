import React, { useEffect, useState } from 'react';
import { Activity, ArrowDownRight, ArrowUpRight } from 'lucide-react';
import { Proxy } from '../types';

interface TrafficVisualizerProps {
  proxies: Proxy[];
}

interface DataPoint {
  time: number;
  upBytes: number;
  downBytes: number;
  connections: number;
}

export const TrafficVisualizer: React.FC<TrafficVisualizerProps> = ({ proxies }) => {
  const [history, setHistory] = useState<DataPoint[]>([]);

  useEffect(() => {
    let totalUp = 0;
    let totalDown = 0;
    let totalConns = 0;

    proxies.forEach((p) => {
      if (p.stats) {
        totalUp += p.stats.bytes_upstream;
        totalDown += p.stats.bytes_downstream;
        totalConns += p.stats.active_connections;
      }
    });

    setHistory((prev) => {
      const now = Date.now();
      const updated = [...prev, { time: now, upBytes: totalUp, downBytes: totalDown, connections: totalConns }];
      if (updated.length > 20) {
        return updated.slice(updated.length - 20);
      }
      return updated;
    });
  }, [proxies]);

  // Compute rate diffs
  const currentUp = history.length > 1 ? Math.max(0, history[history.length - 1].upBytes - history[history.length - 2].upBytes) : 0;
  const currentDown = history.length > 1 ? Math.max(0, history[history.length - 1].downBytes - history[history.length - 2].downBytes) : 0;

  const formatRate = (bytesPerHalfSec: number): string => {
    const bytesPerSec = bytesPerHalfSec * 2;
    if (bytesPerSec < 1024) return `${bytesPerSec} B/s`;
    if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`;
    return `${(bytesPerSec / (1024 * 1024)).toFixed(2)} MB/s`;
  };

  // Sparkline coordinates
  const width = 280;
  const height = 48;
  const maxVal = Math.max(1024, ...history.map((h, i) => {
    if (i === 0) return 0;
    return (h.upBytes - history[i - 1].upBytes) + (h.downBytes - history[i - 1].downBytes);
  }));

  const points = history.map((h, idx) => {
    const x = (idx / Math.max(1, history.length - 1)) * width;
    const diff = idx > 0 ? (h.upBytes - history[idx - 1].upBytes) + (h.downBytes - history[idx - 1].downBytes) : 0;
    const y = height - Math.min(height, (diff / maxVal) * (height - 6) + 3);
    return `${x},${y}`;
  }).join(' ');

  return (
    <div className="bg-[#111726] border border-[#232f4c] rounded-2xl p-4 shadow-xl flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div className="flex items-center gap-3">
        <div className="p-2.5 rounded-xl bg-[#182138] border border-[#232f4c] text-indigo-400">
          <Activity className="h-5 w-5" />
        </div>
        <div>
          <h2 className="text-sm font-semibold text-white tracking-tight font-mono">Live Network Bandwidth</h2>
          <div className="flex items-center gap-3 mt-1 text-xs font-mono">
            <span className="flex items-center gap-1 text-cyan-400">
              <ArrowUpRight className="h-3.5 w-3.5" />
              ↑ {formatRate(currentUp)}
            </span>
            <span className="flex items-center gap-1 text-emerald-400">
              <ArrowDownRight className="h-3.5 w-3.5" />
              ↓ {formatRate(currentDown)}
            </span>
          </div>
        </div>
      </div>

      {/* SVG Sparkline */}
      <div className="flex items-center gap-3 self-end sm:self-center">
        <div className="relative w-[180px] sm:w-[240px] h-[48px] bg-[#090d16] rounded-xl p-1.5 border border-[#232f4c] overflow-hidden flex items-center">
          <svg className="w-full h-full overflow-visible" viewBox={`0 0 ${width} ${height}`}>
            <defs>
              <linearGradient id="streamGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stopColor="#6366f1" stopOpacity="0.8" />
                <stop offset="100%" stopColor="#ec4899" stopOpacity="0.2" />
              </linearGradient>
            </defs>
            {points && (
              <polyline
                fill="none"
                stroke="#818cf8"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                points={points}
              />
            )}
          </svg>
        </div>
      </div>
    </div>
  );
};
