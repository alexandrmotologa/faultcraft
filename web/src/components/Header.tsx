import React from 'react';
import { Activity, Plus, RotateCcw, Zap, Radio } from 'lucide-react';

interface HeaderProps {
  connected: boolean;
  proxyCount: number;
  totalConns: number;
  totalFaults: number;
  onReset: () => void;
  onNewProxy: () => void;
  isResetting: boolean;
}

export const Header: React.FC<HeaderProps> = ({
  connected,
  proxyCount,
  totalConns,
  totalFaults,
  onReset,
  onNewProxy,
  isResetting,
}) => {
  return (
    <header className="border-b border-[#232f4c] bg-[#111726]/90 backdrop-blur sticky top-0 z-40">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3.5 flex flex-col md:flex-row md:items-center justify-between gap-4">
        {/* Brand / Logo */}
        <div className="flex items-center space-x-3">
          <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-rose-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-rose-500/20 ring-1 ring-white/20">
            <Zap className="h-5 w-5 text-white stroke-[2.5]" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-lg tracking-tight text-white">faultcraft</span>
              <span className="text-[10px] uppercase font-mono px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-400 border border-rose-500/20 font-semibold tracking-wider">
                Chaos Matrix
              </span>
            </div>
            <p className="text-xs text-slate-400 font-mono">Network Fault Injection Engine</p>
          </div>
        </div>

        {/* Global Telemetry Metrics */}
        <div className="flex items-center gap-3 text-xs font-mono">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#182138] border border-[#232f4c]">
            <Radio className={`h-3.5 w-3.5 ${connected ? 'text-emerald-400 animate-pulse' : 'text-slate-500'}`} />
            <span className="text-slate-300">{connected ? 'LIVE SSE' : 'CONNECTING'}</span>
          </div>

          <div className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#182138] border border-[#232f4c]">
            <span className="text-slate-400">Proxies:</span>
            <span className="text-white font-semibold">{proxyCount}</span>
          </div>

          <div className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#182138] border border-[#232f4c]">
            <span className="text-slate-400">Sockets:</span>
            <span className="text-emerald-400 font-semibold">{totalConns}</span>
          </div>

          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#182138] border border-[#232f4c]">
            <Activity className="h-3.5 w-3.5 text-rose-400" />
            <span className="text-slate-400">Faults:</span>
            <span className="text-rose-400 font-semibold">{totalFaults}</span>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-2.5">
          <button
            onClick={onReset}
            disabled={isResetting}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/30 text-xs font-medium transition active:scale-95 disabled:opacity-50"
            title="Reset all toxics and enable all proxies"
          >
            <RotateCcw className={`h-3.5 w-3.5 ${isResetting ? 'animate-spin' : ''}`} />
            <span>Reset All</span>
          </button>

          <button
            onClick={onNewProxy}
            className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium shadow-md shadow-indigo-600/30 transition active:scale-95"
          >
            <Plus className="h-3.5 w-3.5 stroke-[2.5]" />
            <span>New Proxy</span>
          </button>
        </div>
      </div>
    </header>
  );
};
