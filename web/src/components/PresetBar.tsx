import React, { useState } from 'react';
import { Database, Smartphone, CreditCard, Skull, Scissors, Sparkles, Check } from 'lucide-react';
import { Preset, Proxy } from '../types';

interface PresetBarProps {
  presets: Preset[];
  proxies: Proxy[];
  onApplyPreset: (proxyName: string, presetId: string) => Promise<void>;
}

export const PresetBar: React.FC<PresetBarProps> = ({ presets, proxies, onApplyPreset }) => {
  const [selectedProxy, setSelectedProxy] = useState<string>(proxies[0]?.name || '');
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [appliedId, setAppliedId] = useState<string | null>(null);

  // Sync default selection if proxies change
  React.useEffect(() => {
    if (!selectedProxy && proxies.length > 0) {
      setSelectedProxy(proxies[0].name);
    }
  }, [proxies, selectedProxy]);

  const handleApply = async (presetId: string) => {
    if (!selectedProxy) return;
    try {
      setApplyingId(presetId);
      await onApplyPreset(selectedProxy, presetId);
      setAppliedId(presetId);
      setTimeout(() => setAppliedId(null), 2000);
    } finally {
      setApplyingId(null);
    }
  };

  const getPresetIcon = (id: string) => {
    switch (id) {
      case 'flaky_db':
        return <Database className="h-4 w-4 text-amber-400" />;
      case 'cellular_3g':
        return <Smartphone className="h-4 w-4 text-cyan-400" />;
      case 'payment_outage':
        return <CreditCard className="h-4 w-4 text-rose-400" />;
      case 'zombie_worker':
        return <Skull className="h-4 w-4 text-purple-400" />;
      case 'packet_shredder':
        return <Scissors className="h-4 w-4 text-red-500" />;
      default:
        return <Sparkles className="h-4 w-4 text-indigo-400" />;
    }
  };

  if (presets.length === 0) return null;

  return (
    <div className="bg-[#111726] border border-[#232f4c] rounded-2xl p-4 shadow-xl">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 mb-3">
        <div className="flex items-center gap-2">
          <Sparkles className="h-4 w-4 text-rose-400" />
          <h2 className="text-sm font-semibold text-white tracking-wide">1-Click Chaos Scenarios</h2>
          <span className="text-[11px] font-mono text-slate-400">Pre-configured resilience profiles</span>
        </div>

        {/* Target Proxy Selector */}
        <div className="flex items-center gap-2">
          <label htmlFor="target-proxy" className="text-xs text-slate-400 font-mono">Target Proxy:</label>
          <select
            id="target-proxy"
            value={selectedProxy}
            onChange={(e) => setSelectedProxy(e.target.value)}
            disabled={proxies.length === 0}
            className="bg-[#182138] border border-[#232f4c] rounded-lg px-2.5 py-1 text-xs text-white font-mono focus:outline-none focus:border-indigo-500 disabled:opacity-50"
          >
            {proxies.length === 0 && <option value="">No active proxies</option>}
            {proxies.map((p) => (
              <option key={p.name} value={p.name}>
                {p.name} ({p.listen} → {p.upstream})
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Preset Cards Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-2.5">
        {presets.map((preset) => {
          const isApplying = applyingId === preset.id;
          const isApplied = appliedId === preset.id;

          return (
            <button
              key={preset.id}
              onClick={() => handleApply(preset.id)}
              disabled={!selectedProxy || isApplying}
              className="group relative flex flex-col items-start p-3 rounded-xl bg-[#182138]/60 hover:bg-[#182138] border border-[#232f4c] hover:border-slate-500/50 text-left transition duration-200 active:scale-[0.98] disabled:opacity-50"
            >
              <div className="flex items-center justify-between w-full mb-1.5">
                <div className="p-1.5 rounded-lg bg-[#111726] border border-[#232f4c]">
                  {getPresetIcon(preset.id)}
                </div>
                {isApplied ? (
                  <span className="flex items-center gap-1 text-[10px] font-mono font-medium text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded border border-emerald-500/20">
                    <Check className="h-3 w-3" />
                    Applied
                  </span>
                ) : (
                  <span className="text-[10px] font-mono text-slate-400 group-hover:text-slate-200">
                    {preset.toxics.length} {preset.toxics.length === 1 ? 'toxic' : 'toxics'}
                  </span>
                )}
              </div>
              <h3 className="text-xs font-semibold text-white group-hover:text-rose-300 transition-colors">
                {preset.name}
              </h3>
              <p className="text-[11px] text-slate-400 line-clamp-2 mt-0.5 leading-snug font-sans">
                {preset.description}
              </p>
            </button>
          );
        })}
      </div>
    </div>
  );
};
