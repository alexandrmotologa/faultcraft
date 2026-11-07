import React, { useState } from 'react';
import { Eye, Trash2, ArrowUpRight, ArrowDownRight, Layers, ChevronDown, ChevronUp } from 'lucide-react';

export interface FrameRecord {
  id: string;
  timestamp: string;
  proxy_name: string;
  direction: 'upstream' | 'downstream';
  protocol: string;
  summary: string;
  bytes: number;
  fault_injected?: string;
  latency_ms: number;
}

interface TrafficInspectorProps {
  frames: FrameRecord[];
  onClear: () => void;
}

export const TrafficInspector: React.FC<TrafficInspectorProps> = ({ frames, onClear }) => {
  const [filterProto, setFilterProto] = useState<string>('ALL');
  const [isOpen, setIsOpen] = useState(true);

  const protocols = ['ALL', 'HTTP/1.1', 'Redis RESP', 'PostgreSQL', 'TCP'];

  const filtered = frames.filter((f) => {
    if (filterProto === 'ALL') return true;
    return f.protocol === filterProto;
  });

  const getProtocolBadge = (proto: string) => {
    switch (proto) {
      case 'HTTP/1.1':
        return 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30';
      case 'Redis RESP':
        return 'bg-rose-500/10 text-rose-400 border-rose-500/30';
      case 'PostgreSQL':
        return 'bg-blue-500/10 text-blue-400 border-blue-500/30';
      default:
        return 'bg-slate-700/30 text-slate-400 border-slate-700';
    }
  };

  const formatTime = (ts: string) => {
    try {
      const d = new Date(ts);
      return d.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) +
        '.' + d.getMilliseconds().toString().padStart(3, '0');
    } catch {
      return ts;
    }
  };

  return (
    <div className="bg-[#111726] border border-[#232f4c] rounded-2xl shadow-xl overflow-hidden">
      {/* Header Bar */}
      <div className="p-4 border-b border-[#232f4c] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="p-2 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
            <Eye className="h-4 w-4" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-white tracking-tight font-mono">L7 Protocol Dissector & Live Inspector</h2>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-[#182138] text-slate-400 border border-[#232f4c]">
                {frames.length} frames captured
              </span>
            </div>
            <p className="text-xs text-slate-400 font-mono">Real-time deep packet inspection for HTTP, Redis, and PostgreSQL</p>
          </div>
        </div>

        <div className="flex items-center gap-2 self-end sm:self-center">
          {/* Protocol Filters */}
          <div className="flex items-center gap-1 bg-[#182138] p-1 rounded-xl border border-[#232f4c]">
            {protocols.map((p) => (
              <button
                key={p}
                onClick={() => setFilterProto(p)}
                className={`px-2 py-1 rounded-lg text-[10px] font-mono transition ${
                  filterProto === p
                    ? 'bg-indigo-600 text-white font-semibold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {p}
              </button>
            ))}
          </div>

          <button
            onClick={onClear}
            className="p-1.5 rounded-xl bg-[#182138] hover:bg-[#232f4c] text-slate-400 hover:text-rose-400 border border-[#232f4c] transition"
            title="Clear frame log"
          >
            <Trash2 className="h-4 w-4" />
          </button>

          <button
            onClick={() => setIsOpen(!isOpen)}
            className="p-1.5 rounded-xl bg-[#182138] hover:bg-[#232f4c] text-slate-400 hover:text-white border border-[#232f4c] transition"
          >
            {isOpen ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          </button>
        </div>
      </div>

      {/* Frame Stream Table */}
      {isOpen && (
        <div className="max-h-72 overflow-y-auto divide-y divide-[#232f4c]/60 bg-[#090d16]">
          {filtered.length === 0 ? (
            <div className="py-8 text-center text-xs font-mono text-slate-500">
              <Layers className="h-6 w-6 mx-auto mb-2 opacity-40 text-slate-400" />
              Waiting for network packets... Connect PostgreSQL, Redis, or HTTP clients to see live frames.
            </div>
          ) : (
            filtered.slice().reverse().map((f) => (
              <div
                key={f.id}
                className="px-4 py-2.5 flex flex-col sm:flex-row sm:items-center justify-between gap-2 hover:bg-[#111726]/80 transition font-mono text-xs"
              >
                <div className="flex items-center gap-2.5 flex-1 min-w-0">
                  <span className="text-[11px] text-slate-500 shrink-0">
                    {formatTime(f.timestamp)}
                  </span>

                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#182138] text-slate-300 border border-[#232f4c] shrink-0 font-medium">
                    {f.proxy_name}
                  </span>

                  <span className={`inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded border uppercase shrink-0 font-semibold ${getProtocolBadge(f.protocol)}`}>
                    {f.direction === 'upstream' ? (
                      <ArrowUpRight className="h-3 w-3 text-cyan-400" />
                    ) : (
                      <ArrowDownRight className="h-3 w-3 text-emerald-400" />
                    )}
                    {f.protocol}
                  </span>

                  <span className="text-slate-200 truncate font-mono text-[11px]">
                    {f.summary}
                  </span>
                </div>

                <div className="flex items-center gap-3 shrink-0 self-end sm:self-center text-[11px]">
                  <span className="text-slate-400">{f.bytes} B</span>
                  {f.latency_ms > 0 && (
                    <span className={`px-1.5 py-0.5 rounded font-medium ${
                      f.latency_ms > 100
                        ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                        : 'text-slate-400'
                    }`}>
                      +{f.latency_ms.toFixed(1)}ms
                    </span>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
};
