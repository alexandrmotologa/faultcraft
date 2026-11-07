import React from 'react';
import { ArrowRight, Plus, Terminal, Trash2, ShieldAlert, Cpu, Activity, Clock, Database, Timer } from 'lucide-react';
import { Proxy, Toxic } from '../types';

interface ProxyCardProps {
  proxy: Proxy;
  onToggleEnabled: (proxy: Proxy) => void;
  onAddToxic: (proxy: Proxy) => void;
  onDeleteToxic: (proxy: Proxy, toxicName: string) => void;
  onDeleteProxy: (proxy: Proxy) => void;
  onExportCli: (proxy: Proxy) => void;
}

export const ProxyCard: React.FC<ProxyCardProps> = ({
  proxy,
  onToggleEnabled,
  onAddToxic,
  onDeleteToxic,
  onDeleteProxy,
  onExportCli,
}) => {
  const stats = proxy.stats || {
    active_connections: 0,
    total_connections: 0,
    bytes_upstream: 0,
    bytes_downstream: 0,
    error_count: 0,
    fault_count: 0,
    p50_latency_ms: 0,
    p90_latency_ms: 0,
    p99_latency_ms: 0,
  };

  const formatBytes = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  };

  const formatAttributes = (t: Toxic): string => {
    switch (t.type) {
      case 'latency':
        return `${t.attributes.latency || 0}ms ± ${t.attributes.jitter || 0}ms`;
      case 'bandwidth':
        return `${t.attributes.rate || 0} KB/s`;
      case 'slicer':
        return `${t.attributes.average_size || 0}B chunks`;
      case 'timeout':
        return `${t.attributes.timeout || 0}ms cutoff`;
      case 'reset_peer':
        return t.attributes.timeout ? `RST in ${t.attributes.timeout}ms` : 'Immediate RST';
      case 'limit_data':
        return `${formatBytes(t.attributes.bytes || 0)} limit`;
      case 'http_error':
        return `HTTP ${t.attributes.status || 503}`;
      case 'hang':
        return 'Frozen socket';
      case 'circuit_breaker_probe':
        return `Trip on ${t.attributes.trip_after_errors || 5} errs, ${t.attributes.cooldown_ms || 3000}ms cool`;
      case 'corrupt':
        return 'Bit-flip corruption';
      default:
        return JSON.stringify(t.attributes);
    }
  };

  return (
    <div className={`rounded-2xl border transition duration-200 bg-[#111726] shadow-xl ${
      proxy.enabled ? 'border-[#232f4c]' : 'border-slate-800 opacity-70'
    }`}>
      {/* Top Banner */}
      <div className="p-4 sm:p-5 border-b border-[#232f4c] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2.5">
            <h3 className="font-bold text-white text-base tracking-tight font-mono">{proxy.name}</h3>
            <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-mono font-medium ${
              proxy.enabled
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                : 'bg-slate-700/30 text-slate-400 border border-slate-700'
            }`}>
              <span className={`h-1.5 w-1.5 rounded-full ${proxy.enabled ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'}`} />
              {proxy.enabled ? 'ONLINE' : 'DISABLED'}
            </span>
            {proxy.toxics.length > 0 && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono bg-rose-500/10 text-rose-400 border border-rose-500/30 font-semibold">
                <ShieldAlert className="h-3 w-3" />
                {proxy.toxics.length} {proxy.toxics.length === 1 ? 'FAULT' : 'FAULTS'} ACTIVE
              </span>
            )}
          </div>

          {/* Network Route & Upstream Target Health */}
          <div className="flex items-center gap-2 mt-1.5 text-xs font-mono text-slate-400 flex-wrap">
            <span className="text-slate-300 bg-[#182138] px-2 py-0.5 rounded border border-[#232f4c]">
              {proxy.listen}
            </span>
            <ArrowRight className="h-3.5 w-3.5 text-indigo-400" />
            <span className="text-slate-300 bg-[#182138] px-2 py-0.5 rounded border border-[#232f4c]">
              {proxy.upstream}
            </span>
            {proxy.target_health && proxy.target_health.status !== 'unknown' && (
              <span
                className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono font-medium ${
                  proxy.target_health.status === 'healthy'
                    ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                    : proxy.target_health.status === 'degraded'
                    ? 'bg-amber-500/10 text-amber-400 border border-amber-500/30'
                    : 'bg-rose-500/10 text-rose-400 border border-rose-500/30'
                }`}
                title={proxy.target_health.error || `Target response time: ${proxy.target_health.rtt_ms.toFixed(1)}ms`}
              >
                <span className={`h-1.5 w-1.5 rounded-full ${
                  proxy.target_health.status === 'healthy' ? 'bg-emerald-400' :
                  proxy.target_health.status === 'degraded' ? 'bg-amber-400' : 'bg-rose-400'
                }`} />
                {proxy.target_health.status === 'healthy' && `TARGET UP (${proxy.target_health.rtt_ms.toFixed(1)}ms)`}
                {proxy.target_health.status === 'degraded' && `SLOW (${proxy.target_health.rtt_ms.toFixed(1)}ms)`}
                {proxy.target_health.status === 'down' && 'TARGET UNREACHABLE'}
              </span>
            )}
          </div>
        </div>

        {/* Action Controls */}
        <div className="flex items-center gap-2 self-end sm:self-center">
          {/* Toggle Switch */}
          <button
            onClick={() => onToggleEnabled(proxy)}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none ${
              proxy.enabled ? 'bg-indigo-600' : 'bg-slate-700'
            }`}
            title={proxy.enabled ? 'Disable proxy' : 'Enable proxy'}
          >
            <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              proxy.enabled ? 'translate-x-6' : 'translate-x-1'
            }`} />
          </button>

          <button
            onClick={() => onExportCli(proxy)}
            className="p-1.5 rounded-lg bg-[#182138] hover:bg-[#232f4c] text-slate-300 hover:text-white border border-[#232f4c] transition"
            title="Export curl / CLI command"
          >
            <Terminal className="h-4 w-4" />
          </button>

          <button
            onClick={() => onDeleteProxy(proxy)}
            className="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 border border-rose-500/20 transition"
            title="Delete proxy"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Telemetry Gauge Strip */}
      <div className="grid grid-cols-2 sm:grid-cols-4 divide-x divide-y sm:divide-y-0 divide-[#232f4c] bg-[#182138]/40 border-b border-[#232f4c]">
        <div className="p-3">
          <div className="flex items-center gap-1.5 text-slate-400 text-[11px] font-mono">
            <Cpu className="h-3.5 w-3.5 text-indigo-400" />
            <span>Active Sockets</span>
          </div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-base font-bold text-white font-mono">{stats.active_connections}</span>
            <span className="text-[10px] text-slate-500 font-mono">({stats.total_connections} total)</span>
          </div>
        </div>

        <div className="p-3">
          <div className="flex items-center gap-1.5 text-slate-400 text-[11px] font-mono">
            <Database className="h-3.5 w-3.5 text-cyan-400" />
            <span>Throughput</span>
          </div>
          <div className="mt-1 flex items-baseline gap-2 font-mono">
            <span className="text-xs text-slate-300">↑ {formatBytes(stats.bytes_upstream)}</span>
            <span className="text-xs text-slate-300">↓ {formatBytes(stats.bytes_downstream)}</span>
          </div>
        </div>

        <div className="p-3">
          <div className="flex items-center gap-1.5 text-slate-400 text-[11px] font-mono">
            <Clock className="h-3.5 w-3.5 text-amber-400" />
            <span>P99 Latency</span>
          </div>
          <div className="mt-1 flex items-baseline gap-1 font-mono">
            <span className={`text-base font-bold ${stats.p99_latency_ms > 200 ? 'text-amber-400' : 'text-white'}`}>
              {stats.p99_latency_ms.toFixed(1)}
            </span>
            <span className="text-[10px] text-slate-500">ms</span>
          </div>
        </div>

        <div className="p-3">
          <div className="flex items-center gap-1.5 text-slate-400 text-[11px] font-mono">
            <Activity className="h-3.5 w-3.5 text-rose-400" />
            <span>Injected Faults</span>
          </div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-base font-bold text-rose-400 font-mono">{stats.fault_count}</span>
            {stats.error_count > 0 && (
              <span className="text-[10px] text-red-400 font-mono">({stats.error_count} errors)</span>
            )}
          </div>
        </div>
      </div>

      {/* Active Toxics Pipeline */}
      <div className="p-4 sm:p-5">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-300 font-mono">
              Fault Pipeline
            </span>
            <span className="text-xs font-mono text-slate-500">({proxy.toxics.length} configured)</span>
          </div>

          <button
            onClick={() => onAddToxic(proxy)}
            className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 text-xs font-mono font-medium transition active:scale-95"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>Add Toxic</span>
          </button>
        </div>

        {proxy.toxics.length === 0 ? (
          <div className="p-4 rounded-xl border border-dashed border-[#232f4c] text-center">
            <p className="text-xs text-slate-400 font-sans">
              No faults injected. All network traffic is forwarding with zero interference.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5">
            {proxy.toxics.map((toxic) => (
              <div
                key={toxic.name}
                className="group relative p-3 rounded-xl bg-[#182138] border border-[#232f4c] hover:border-slate-500/50 flex flex-col justify-between transition"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs font-bold text-white tracking-wide truncate">
                      {toxic.name}
                    </span>
                    <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded border uppercase ${
                      toxic.stream === 'downstream'
                        ? 'bg-blue-500/10 text-blue-400 border-blue-500/30'
                        : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                    }`}>
                      {toxic.stream}
                    </span>
                  </div>

                  <div className="mt-1 flex items-center gap-1.5 text-xs text-rose-300 font-mono font-semibold">
                    <span>{toxic.type}</span>
                    <span className="text-slate-500 font-normal">→</span>
                    <span className="text-slate-300 font-normal">{formatAttributes(toxic)}</span>
                  </div>

                  {toxic.toxicity < 1.0 && (
                    <div className="mt-1 text-[10px] font-mono text-slate-400">
                      Rate: {(toxic.toxicity * 100).toFixed(0)}% probability
                    </div>
                  )}

                  {(toxic.duration_ms || toxic.expires_at) && (
                    <div className="mt-1 flex items-center gap-1 text-[10px] font-mono text-cyan-400">
                      <Timer className="h-3 w-3 animate-spin text-cyan-400" />
                      <span>Auto-Heal Active</span>
                    </div>
                  )}
                </div>

                <div className="mt-2 pt-2 border-t border-[#232f4c]/60 flex items-center justify-end">
                  <button
                    onClick={() => onDeleteToxic(proxy, toxic.name)}
                    className="text-slate-400 hover:text-rose-400 text-xs font-mono flex items-center gap-1 transition"
                  >
                    <Trash2 className="h-3 w-3" />
                    <span>Remove</span>
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
