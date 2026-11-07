import React, { useState } from 'react';
import { X, Network } from 'lucide-react';

interface NewProxyModalProps {
  onClose: () => void;
  onSave: (data: { name: string; listen: string; upstream: string }) => Promise<void>;
}

export const NewProxyModal: React.FC<NewProxyModalProps> = ({ onClose, onSave }) => {
  const [name, setName] = useState('');
  const [listen, setListen] = useState('127.0.0.1:15432');
  const [upstream, setUpstream] = useState('127.0.0.1:5432');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const applyTemplate = (tName: string, tListen: string, tUpstream: string) => {
    setName(tName);
    setListen(tListen);
    setUpstream(tUpstream);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !listen.trim() || !upstream.trim()) {
      setError('All fields are required');
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      await onSave({
        name: name.trim(),
        listen: listen.trim(),
        upstream: upstream.trim(),
      });
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed creating proxy');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="w-full max-w-md rounded-2xl bg-[#111726] border border-[#232f4c] shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-5 border-b border-[#232f4c]">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
              <Network className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white font-mono">Create TCP/HTTP Proxy</h2>
              <p className="text-xs text-slate-400 font-mono">Route client traffic through chaos engine</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-[#182138] transition"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Quick Template Buttons */}
        <div className="px-5 pt-4 flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] font-mono text-slate-400 mr-1">Templates:</span>
          <button
            type="button"
            onClick={() => applyTemplate('postgres_chaos', '127.0.0.1:15432', '127.0.0.1:5432')}
            className="text-[11px] font-mono px-2 py-0.5 rounded bg-[#182138] hover:bg-[#232f4c] text-slate-300 border border-[#232f4c] transition"
          >
            PostgreSQL
          </button>
          <button
            type="button"
            onClick={() => applyTemplate('redis_chaos', '127.0.0.1:16379', '127.0.0.1:6379')}
            className="text-[11px] font-mono px-2 py-0.5 rounded bg-[#182138] hover:bg-[#232f4c] text-slate-300 border border-[#232f4c] transition"
          >
            Redis
          </button>
          <button
            type="button"
            onClick={() => applyTemplate('http_api_chaos', '127.0.0.1:18080', '127.0.0.1:8080')}
            className="text-[11px] font-mono px-2 py-0.5 rounded bg-[#182138] hover:bg-[#232f4c] text-slate-300 border border-[#232f4c] transition"
          >
            REST API
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit} className="p-4 sm:p-5 space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-400 text-xs font-mono">
              {error}
            </div>
          )}

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5">Proxy Identifier Name</label>
            <input
              type="text"
              required
              placeholder="e.g. postgres_resilience"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5">
              Listen Address <span className="text-slate-500">(Where apps connect)</span>
            </label>
            <input
              type="text"
              required
              placeholder="127.0.0.1:15432"
              value={listen}
              onChange={(e) => setListen(e.target.value)}
              className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5">
              Upstream Target Address <span className="text-slate-500">(Real service)</span>
            </label>
            <input
              type="text"
              required
              placeholder="127.0.0.1:5432"
              value={upstream}
              onChange={(e) => setUpstream(e.target.value)}
              className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div className="flex items-center justify-end gap-2.5 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-3.5 py-2 rounded-xl bg-[#182138] hover:bg-[#232f4c] text-slate-300 text-xs font-mono transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-mono font-semibold shadow-lg shadow-indigo-600/30 transition active:scale-95 disabled:opacity-50"
            >
              {isSubmitting ? 'Creating...' : 'Create Proxy'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
