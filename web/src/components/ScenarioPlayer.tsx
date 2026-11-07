import React, { useState, useEffect } from 'react';
import { Play, Square, GitCommit, CheckCircle2, AlertTriangle } from 'lucide-react';
import { Proxy } from '../types';

export interface SequenceStep {
  name: string;
  description: string;
  duration_sec: number;
}

export interface ChaosSequence {
  id: string;
  name: string;
  description: string;
  steps: SequenceStep[];
}

export interface SequenceStatus {
  sequence_id: string;
  proxy_name: string;
  current_step: number;
  total_steps: number;
  step_name: string;
  remaining_sec: number;
  running: boolean;
}

interface ScenarioPlayerProps {
  proxies: Proxy[];
  status: SequenceStatus;
  onRefresh: () => void;
}

export const ScenarioPlayer: React.FC<ScenarioPlayerProps> = ({ proxies, status, onRefresh }) => {
  const [sequences, setSequences] = useState<ChaosSequence[]>([]);
  const [selectedSeq, setSelectedSeq] = useState<string>('db_failover_drill');
  const [selectedProxy, setSelectedProxy] = useState<string>(proxies[0]?.name || '');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetch('/api/v1/sequences')
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setSequences(data);
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (!selectedProxy && proxies.length > 0) {
      setSelectedProxy(proxies[0].name);
    }
  }, [proxies, selectedProxy]);

  const activeSeq = sequences.find((s) => s.id === (status.running ? status.sequence_id : selectedSeq));

  const handleStart = async () => {
    if (!selectedProxy || !selectedSeq) return;
    setLoading(true);
    try {
      await fetch('/api/v1/sequences/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ proxy: selectedProxy, sequence_id: selectedSeq }),
      });
      onRefresh();
    } finally {
      setLoading(false);
    }
  };

  const handleStop = async () => {
    setLoading(true);
    try {
      await fetch('/api/v1/sequences/stop', { method: 'POST' });
      onRefresh();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-[#111726] border border-[#232f4c] rounded-2xl p-4 shadow-xl">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
        <div className="flex items-center gap-2.5">
          <div className="p-2 rounded-xl bg-purple-500/10 text-purple-400 border border-purple-500/20">
            <GitCommit className="h-4 w-4" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-white tracking-tight font-mono">Automated Incident Timeline Player</h2>
              {status.running && (
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-400 border border-rose-500/30 font-semibold animate-pulse flex items-center gap-1">
                  <span className="h-1.5 w-1.5 rounded-full bg-rose-400 animate-ping" />
                  DRILL IN PROGRESS
                </span>
              )}
            </div>
            <p className="text-xs text-slate-400 font-mono">Execute phased, timed multi-stage network incident simulations</p>
          </div>
        </div>

        {/* Controls */}
        <div className="flex items-center gap-2 flex-wrap">
          <select
            value={selectedSeq}
            onChange={(e) => setSelectedSeq(e.target.value)}
            disabled={status.running}
            className="bg-[#182138] border border-[#232f4c] rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:outline-none focus:border-indigo-500 disabled:opacity-50"
          >
            {sequences.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.steps.length} phases)
              </option>
            ))}
          </select>

          <select
            value={selectedProxy}
            onChange={(e) => setSelectedProxy(e.target.value)}
            disabled={status.running || proxies.length === 0}
            className="bg-[#182138] border border-[#232f4c] rounded-xl px-2.5 py-1.5 text-xs text-white font-mono focus:outline-none focus:border-indigo-500 disabled:opacity-50"
          >
            {proxies.map((p) => (
              <option key={p.name} value={p.name}>
                {p.name}
              </option>
            ))}
          </select>

          {status.running ? (
            <button
              onClick={handleStop}
              disabled={loading}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-mono font-medium shadow-md shadow-rose-600/30 transition active:scale-95"
            >
              <Square className="h-3.5 w-3.5 fill-current" />
              <span>Abort Drill</span>
            </button>
          ) : (
            <button
              onClick={handleStart}
              disabled={loading || proxies.length === 0}
              className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-purple-600 hover:bg-purple-500 text-white text-xs font-mono font-medium shadow-md shadow-purple-600/30 transition active:scale-95 disabled:opacity-50"
            >
              <Play className="h-3.5 w-3.5 fill-current" />
              <span>Start Incident Drill</span>
            </button>
          )}
        </div>
      </div>

      {/* Timeline Steps Bar */}
      {activeSeq && (
        <div className="space-y-3">
          <div className="p-3 rounded-xl bg-[#090d16] border border-[#232f4c] flex flex-col md:flex-row md:items-center justify-between gap-3">
            <div className="flex-1">
              <span className="text-[11px] text-slate-400 font-sans">{activeSeq.description}</span>
            </div>
            {status.running && (
              <div className="flex items-center gap-2 font-mono text-xs">
                <span className="text-slate-400">Step {status.current_step}/{status.total_steps}:</span>
                <span className="text-rose-400 font-semibold">{status.step_name}</span>
                <span className="px-2 py-0.5 rounded bg-rose-500/20 text-rose-300 font-bold border border-rose-500/30">
                  {status.remaining_sec}s left
                </span>
              </div>
            )}
          </div>

          {/* Stepper Progression */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-2.5">
            {activeSeq.steps.map((step, idx) => {
              const stepNumber = idx + 1;
              const isActive = status.running && status.current_step === stepNumber;
              const isCompleted = status.running && status.current_step > stepNumber;

              return (
                <div
                  key={step.name}
                  className={`p-3 rounded-xl border transition ${
                    isActive
                      ? 'bg-rose-500/10 border-rose-500/50 ring-1 ring-rose-500/30'
                      : isCompleted
                      ? 'bg-[#182138]/40 border-emerald-500/30 text-slate-400'
                      : 'bg-[#182138]/20 border-[#232f4c] text-slate-500'
                  }`}
                >
                  <div className="flex items-center justify-between mb-1.5">
                    <span className={`text-[10px] font-mono font-bold px-1.5 py-0.5 rounded ${
                      isActive
                        ? 'bg-rose-500 text-white'
                        : isCompleted
                        ? 'bg-emerald-500/20 text-emerald-400'
                        : 'bg-[#182138] text-slate-400'
                    }`}>
                      PHASE {stepNumber}
                    </span>
                    <span className="text-[10px] font-mono text-slate-400">
                      {step.duration_sec}s
                    </span>
                  </div>
                  <h4 className={`text-xs font-semibold ${isActive ? 'text-white' : 'text-slate-300'}`}>
                    {step.name}
                  </h4>
                  <p className="text-[10px] text-slate-400 mt-0.5 line-clamp-2">
                    {step.description}
                  </p>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};
