import React, { useState } from 'react';
import { X, ShieldAlert } from 'lucide-react';
import { Proxy, StreamDirection } from '../types';

interface ToxicModalProps {
  proxy: Proxy;
  onClose: () => void;
  onSave: (toxic: {
    name: string;
    type: string;
    stream: StreamDirection;
    toxicity: number;
    attributes: Record<string, any>;
  }) => Promise<void>;
}

export const ToxicModal: React.FC<ToxicModalProps> = ({ proxy, onClose, onSave }) => {
  const [type, setType] = useState('latency');
  const [name, setName] = useState('');
  const [stream, setStream] = useState<StreamDirection>('downstream');
  const [toxicity, setToxicity] = useState(1.0);
  const [durationSec, setDurationSec] = useState(0); // 0 = persistent
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Type specific attributes
  const [latency, setLatency] = useState(250);
  const [jitter, setJitter] = useState(50);
  const [bandwidthRate, setBandwidthRate] = useState(256);
  const [slicerSize, setSlicerSize] = useState(128);
  const [slicerDelay, setSlicerDelay] = useState(1000);
  const [timeoutMs, setTimeoutMs] = useState(1000);
  const [limitBytes, setLimitBytes] = useState(4096);
  const [httpStatus, setHttpStatus] = useState(503);
  const [httpBody, setHttpBody] = useState('');
  const [corruptIndex, setCorruptIndex] = useState(-1);
  const [tripErrors, setTripErrors] = useState(5);
  const [cooldownMs, setCooldownMs] = useState(3000);
  const [probeSuccesses, setProbeSuccesses] = useState(2);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    const toxicName = name.trim() || `${type}_${stream}_${Date.now() % 10000}`;
    const attributes: Record<string, any> = {};

    switch (type) {
      case 'latency':
        attributes.latency = Number(latency);
        attributes.jitter = Number(jitter);
        break;
      case 'bandwidth':
        attributes.rate = Number(bandwidthRate);
        break;
      case 'slicer':
        attributes.average_size = Number(slicerSize);
        attributes.delay = Number(slicerDelay);
        break;
      case 'timeout':
        attributes.timeout = Number(timeoutMs);
        break;
      case 'reset_peer':
        attributes.timeout = Number(timeoutMs);
        break;
      case 'limit_data':
        attributes.bytes = Number(limitBytes);
        break;
      case 'http_error':
        attributes.status = Number(httpStatus);
        if (httpBody.trim()) attributes.body = httpBody.trim();
        break;
      case 'circuit_breaker_probe':
        attributes.trip_after_errors = Number(tripErrors);
        attributes.cooldown_ms = Number(cooldownMs);
        attributes.probe_successes = Number(probeSuccesses);
        attributes.status_code = Number(httpStatus);
        break;
      case 'corrupt':
        attributes.byte_index = Number(corruptIndex);
        break;
      case 'hang':
        break;
    }

    if (durationSec > 0) {
      attributes.duration = Number(durationSec) * 1000;
    }

    try {
      await onSave({
        name: toxicName,
        type,
        stream,
        toxicity,
        attributes,
      });
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed to add toxic');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="w-full max-w-lg rounded-2xl bg-[#111726] border border-[#232f4c] shadow-2xl overflow-hidden">
        {/* Modal Header */}
        <div className="flex items-center justify-between p-4 sm:p-5 border-b border-[#232f4c]">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-rose-500/10 text-rose-400 border border-rose-500/20">
              <ShieldAlert className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white font-mono">Inject Fault Toxic</h2>
              <p className="text-xs text-slate-400 font-mono">Target: {proxy.name}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-[#182138] transition"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Modal Body */}
        <form onSubmit={handleSubmit} className="p-4 sm:p-5 space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-400 text-xs font-mono">
              {error}
            </div>
          )}

          {/* Toxic Type Selector */}
          <div>
            <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5">Toxic Type</label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value)}
              className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            >
              <option value="latency">Latency Jitter (Gaussian normal distribution)</option>
              <option value="bandwidth">Bandwidth Throttler (Token bucket rate)</option>
              <option value="slicer">Packet Slicer (TCP buffer chunking)</option>
              <option value="http_error">HTTP Status Code Rewrite (500/502/503/504)</option>
              <option value="hang">Zombie Socket Hang (Freeze without FIN)</option>
              <option value="reset_peer">TCP Reset (RST abort)</option>
              <option value="limit_data">Data Limit (Quota cutoff)</option>
              <option value="circuit_breaker_probe">Circuit Breaker Probe (Trip & Half-Open Tester)</option>
              <option value="timeout">Read/Write Timeout</option>
              <option value="corrupt">Bit-Flip Payload Corruption</option>
            </select>
          </div>

          {/* Name & Direction */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5">
                Toxic Name <span className="text-slate-500">(Optional)</span>
              </label>
              <input
                type="text"
                placeholder={`${type}_${stream}`}
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
              />
            </div>

            <div>
              <label className="block text-xs font-mono font-medium text-slate-300 mb-1.5">Stream Direction</label>
              <select
                value={stream}
                onChange={(e) => setStream(e.target.value as StreamDirection)}
                className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
              >
                <option value="downstream">Downstream (Target → Client)</option>
                <option value="upstream">Upstream (Client → Target)</option>
              </select>
            </div>
          </div>

          {/* Probability / Toxicity Slider */}
          <div>
            <div className="flex justify-between items-center text-xs font-mono mb-1.5">
              <span className="text-slate-300">Toxicity Probability</span>
              <span className="text-rose-400 font-bold">{(toxicity * 100).toFixed(0)}%</span>
            </div>
            <input
              type="range"
              min="0.01"
              max="1.0"
              step="0.01"
              value={toxicity}
              onChange={(e) => setToxicity(parseFloat(e.target.value))}
              className="w-full accent-rose-500 bg-[#182138] h-1.5 rounded-lg appearance-none cursor-pointer"
            />
          </div>

          {/* Auto-Heal Duration Timer */}
          <div>
            <div className="flex justify-between items-center text-xs font-mono mb-1.5">
              <span className="text-slate-300">Auto-Heal Expiration</span>
              <span className="text-cyan-400 font-semibold">
                {durationSec > 0 ? `${durationSec} seconds` : 'Persistent (Never)'}
              </span>
            </div>
            <input
              type="number"
              min="0"
              step="5"
              placeholder="0 (Persistent)"
              value={durationSec}
              onChange={(e) => setDurationSec(Math.max(0, parseInt(e.target.value) || 0))}
              className="w-full bg-[#182138] border border-[#232f4c] rounded-xl px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
            />
            <p className="text-[11px] text-slate-500 font-sans mt-1">
              Automatically remove this fault and restore the socket after elapsed seconds.
            </p>
          </div>

          {/* Dynamic Parameter Fields */}
          <div className="p-3.5 rounded-xl bg-[#182138]/50 border border-[#232f4c] space-y-3">
            {type === 'latency' && (
              <div className="grid grid-cols-2 gap-3 font-mono text-xs">
                <div>
                  <label className="block text-slate-400 mb-1">Latency Mean (ms)</label>
                  <input
                    type="number"
                    min="0"
                    value={latency}
                    onChange={(e) => setLatency(Number(e.target.value))}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  />
                </div>
                <div>
                  <label className="block text-slate-400 mb-1">Jitter (ms)</label>
                  <input
                    type="number"
                    min="0"
                    value={jitter}
                    onChange={(e) => setJitter(Number(e.target.value))}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  />
                </div>
              </div>
            )}

            {type === 'bandwidth' && (
              <div className="font-mono text-xs">
                <label className="block text-slate-400 mb-1">Rate Limit (KB/s)</label>
                <input
                  type="number"
                  min="1"
                  value={bandwidthRate}
                  onChange={(e) => setBandwidthRate(Number(e.target.value))}
                  className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
            )}

            {type === 'slicer' && (
              <div className="grid grid-cols-2 gap-3 font-mono text-xs">
                <div>
                  <label className="block text-slate-400 mb-1">Chunk Size (Bytes)</label>
                  <input
                    type="number"
                    min="1"
                    value={slicerSize}
                    onChange={(e) => setSlicerSize(Number(e.target.value))}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  />
                </div>
                <div>
                  <label className="block text-slate-400 mb-1">Inter-slice Delay (µs)</label>
                  <input
                    type="number"
                    min="0"
                    value={slicerDelay}
                    onChange={(e) => setSlicerDelay(Number(e.target.value))}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  />
                </div>
              </div>
            )}

            {type === 'http_error' && (
              <div className="space-y-3 font-mono text-xs">
                <div>
                  <label className="block text-slate-400 mb-1">HTTP Status Code</label>
                  <select
                    value={httpStatus}
                    onChange={(e) => setHttpStatus(Number(e.target.value))}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  >
                    <option value={500}>500 Internal Server Error</option>
                    <option value={502}>502 Bad Gateway</option>
                    <option value={503}>503 Service Unavailable</option>
                    <option value={504}>504 Gateway Timeout</option>
                    <option value={429}>429 Too Many Requests</option>
                  </select>
                </div>
                <div>
                  <label className="block text-slate-400 mb-1">Custom Response Body (Optional JSON/Text)</label>
                  <input
                    type="text"
                    placeholder='{"error":"service_unavailable"}'
                    value={httpBody}
                    onChange={(e) => setHttpBody(e.target.value)}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  />
                </div>
              </div>
            )}

            {(type === 'timeout' || type === 'reset_peer') && (
              <div className="font-mono text-xs">
                <label className="block text-slate-400 mb-1">Delay Before Cutoff (ms)</label>
                <input
                  type="number"
                  min="0"
                  value={timeoutMs}
                  onChange={(e) => setTimeoutMs(Number(e.target.value))}
                  className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
            )}

            {type === 'limit_data' && (
              <div className="font-mono text-xs">
                <label className="block text-slate-400 mb-1">Byte Quota Before Terminating Socket</label>
                <input
                  type="number"
                  min="1"
                  value={limitBytes}
                  onChange={(e) => setLimitBytes(Number(e.target.value))}
                  className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
            )}

            {type === 'circuit_breaker_probe' && (
              <div className="space-y-3 font-mono text-xs">
                <div className="grid grid-cols-3 gap-2">
                  <div>
                    <label className="block text-slate-400 mb-1">Trip After Errors</label>
                    <input
                      type="number"
                      min="1"
                      value={tripErrors}
                      onChange={(e) => setTripErrors(Number(e.target.value))}
                      className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-400 mb-1">Cooldown (ms)</label>
                    <input
                      type="number"
                      min="100"
                      value={cooldownMs}
                      onChange={(e) => setCooldownMs(Number(e.target.value))}
                      className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                    />
                  </div>
                  <div>
                    <label className="block text-slate-400 mb-1">Probe Successes</label>
                    <input
                      type="number"
                      min="1"
                      value={probeSuccesses}
                      onChange={(e) => setProbeSuccesses(Number(e.target.value))}
                      className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-slate-400 mb-1">Failure Status Code</label>
                  <select
                    value={httpStatus}
                    onChange={(e) => setHttpStatus(Number(e.target.value))}
                    className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                  >
                    <option value={503}>503 Service Unavailable</option>
                    <option value={502}>502 Bad Gateway</option>
                    <option value={504}>504 Gateway Timeout</option>
                    <option value={500}>500 Internal Server Error</option>
                  </select>
                </div>
              </div>
            )}

            {type === 'hang' && (
              <p className="text-xs text-amber-300 font-sans">
                The connection will accept TCP handshakes but never forward data packets, testing client read deadlines.
              </p>
            )}

            {type === 'corrupt' && (
              <div className="font-mono text-xs">
                <label className="block text-slate-400 mb-1">Byte Index (-1 for random)</label>
                <input
                  type="number"
                  value={corruptIndex}
                  onChange={(e) => setCorruptIndex(Number(e.target.value))}
                  className="w-full bg-[#111726] border border-[#232f4c] rounded-lg px-2.5 py-1.5 text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
            )}
          </div>

          {/* Modal Actions */}
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
              className="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-mono font-semibold shadow-lg shadow-rose-600/30 transition active:scale-95 disabled:opacity-50"
            >
              {isSubmitting ? 'Injecting...' : 'Inject Fault'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
