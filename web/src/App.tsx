import React, { useEffect, useState, useCallback } from 'react';
import { Header } from './components/Header';
import { PresetBar } from './components/PresetBar';
import { ScenarioPlayer } from './components/ScenarioPlayer';
import { TrafficVisualizer } from './components/TrafficVisualizer';
import { TrafficInspector, FrameRecord } from './components/TrafficInspector';
import { ProxyCard } from './components/ProxyCard';
import { ToxicModal } from './components/ToxicModal';
import { NewProxyModal } from './components/NewProxyModal';
import { CliExportModal } from './components/CliExportModal';
import {
  fetchProxies,
  fetchPresets,
  createProxy,
  updateProxy,
  deleteProxy,
  addToxic,
  deleteToxic,
  resetAll,
  applyPreset,
} from './services/api';
import { Proxy, Preset, StreamDirection, TelemetryPayload } from './types';
import { Network, Search } from 'lucide-react';

export const App: React.FC = () => {
  const [proxies, setProxies] = useState<Record<string, Proxy>>({});
  const [presets, setPresets] = useState<Preset[]>([]);
  const [frames, setFrames] = useState<FrameRecord[]>([]);
  const [seqStatus, setSeqStatus] = useState<any>({ running: false });
  const [connected, setConnected] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [isResetting, setIsResetting] = useState(false);

  // Modals state
  const [toxicProxy, setToxicProxy] = useState<Proxy | null>(null);
  const [cliProxy, setCliProxy] = useState<Proxy | null>(null);
  const [isNewProxyOpen, setIsNewProxyOpen] = useState(false);

  // Notification toast
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  };

  // Initial Data Load
  const loadInitialData = useCallback(async () => {
    try {
      const [proxyData, presetData] = await Promise.all([fetchProxies(), fetchPresets()]);
      setProxies(proxyData);
      setPresets(presetData);
    } catch (err: any) {
      console.error('Initialization error:', err);
    }
  }, []);

  useEffect(() => {
    loadInitialData();
  }, [loadInitialData]);

  // Server-Sent Events (SSE) Telemetry Stream
  useEffect(() => {
    let es: EventSource | null = null;
    let fallbackInterval: NodeJS.Timeout | null = null;

    const connectSSE = () => {
      es = new EventSource('/api/v1/events');

      es.onopen = () => {
        setConnected(true);
        if (fallbackInterval) {
          clearInterval(fallbackInterval);
          fallbackInterval = null;
        }
      };

      es.onmessage = (event) => {
        try {
          const payload: TelemetryPayload = JSON.parse(event.data);
          if (payload) {
            if (payload.sequence_status) {
              setSeqStatus(payload.sequence_status);
            }
            if (payload.recent_frames) {
              setFrames(payload.recent_frames as any);
            }
            if (payload.proxies) {
              setProxies((prev) => {
                const updated = { ...prev };
                payload.proxies.forEach((p) => {
                  updated[p.name] = {
                    ...p,
                    stats: p.stats,
                    target_health: p.target_health,
                    toxics: p.toxics || [],
                  };
                });
                return updated;
              });
            }
          }
        } catch (e) {
          console.error('SSE JSON parse error:', e);
        }
      };

      es.onerror = () => {
        setConnected(false);
        if (es) {
          es.close();
          es = null;
        }
        // Start polling fallback every 2 seconds
        if (!fallbackInterval) {
          fallbackInterval = setInterval(async () => {
            try {
              const data = await fetchProxies();
              setProxies(data);
              // Retry SSE
              connectSSE();
            } catch (e) {
              // Ignore
            }
          }, 2000);
        }
      };
    };

    connectSSE();

    return () => {
      if (es) es.close();
      if (fallbackInterval) clearInterval(fallbackInterval);
    };
  }, []);

  // Handlers
  const handleToggleEnabled = async (proxy: Proxy) => {
    try {
      const updated = await updateProxy(proxy.name, { enabled: !proxy.enabled });
      setProxies((prev) => ({ ...prev, [proxy.name]: updated }));
      showToast(`Proxy ${proxy.name} ${updated.enabled ? 'enabled' : 'disabled'}`);
    } catch (err: any) {
      showToast(err.message, 'error');
    }
  };

  const handleDeleteProxy = async (proxy: Proxy) => {
    if (!window.confirm(`Are you sure you want to delete proxy "${proxy.name}"?`)) return;
    try {
      await deleteProxy(proxy.name);
      setProxies((prev) => {
        const next = { ...prev };
        delete next[proxy.name];
        return next;
      });
      showToast(`Proxy ${proxy.name} removed`);
    } catch (err: any) {
      showToast(err.message, 'error');
    }
  };

  const handleAddToxic = async (toxicData: {
    name: string;
    type: string;
    stream: StreamDirection;
    toxicity: number;
    attributes: Record<string, any>;
  }) => {
    if (!toxicProxy) return;
    try {
      await addToxic(toxicProxy.name, toxicData);
      const refreshed = await fetchProxies();
      setProxies(refreshed);
      showToast(`Fault toxic "${toxicData.name}" injected on ${toxicProxy.name}`);
    } catch (err: any) {
      showToast(err.message, 'error');
      throw err;
    }
  };

  const handleDeleteToxic = async (proxy: Proxy, toxicName: string) => {
    try {
      await deleteToxic(proxy.name, toxicName);
      const refreshed = await fetchProxies();
      setProxies(refreshed);
      showToast(`Removed toxic "${toxicName}" from ${proxy.name}`);
    } catch (err: any) {
      showToast(err.message, 'error');
    }
  };

  const handleResetAll = async () => {
    if (!window.confirm('Clear all toxics across all proxies and re-enable them?')) return;
    try {
      setIsResetting(true);
      await resetAll();
      const refreshed = await fetchProxies();
      setProxies(refreshed);
      showToast('All toxics reset and proxies restored.');
    } catch (err: any) {
      showToast(err.message, 'error');
    } finally {
      setIsResetting(false);
    }
  };

  const handleApplyPreset = async (proxyName: string, presetId: string) => {
    try {
      await applyPreset(proxyName, presetId);
      const refreshed = await fetchProxies();
      setProxies(refreshed);
      showToast(`Preset "${presetId}" applied to ${proxyName}`);
    } catch (err: any) {
      showToast(err.message, 'error');
      throw err;
    }
  };

  const handleCreateProxy = async (data: { name: string; listen: string; upstream: string }) => {
    try {
      const created = await createProxy(data);
      setProxies((prev) => ({ ...prev, [created.name]: created }));
      showToast(`Proxy "${created.name}" created and listening on ${created.listen}`);
    } catch (err: any) {
      showToast(err.message, 'error');
      throw err;
    }
  };

  const proxyList = Object.values(proxies);
  const filteredProxies = proxyList.filter(
    (p) =>
      p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      p.listen.toLowerCase().includes(searchQuery.toLowerCase()) ||
      p.upstream.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const totalSockets = proxyList.reduce((acc, p) => acc + (p.stats?.active_connections || 0), 0);
  const totalFaults = proxyList.reduce((acc, p) => acc + (p.stats?.fault_count || 0), 0);

  return (
    <div className="min-h-screen bg-[#090d16] text-slate-100 flex flex-col font-sans">
      {/* Toast Notification */}
      {toast && (
        <div className="fixed bottom-6 right-6 z-50 animate-in slide-in-from-bottom-5 duration-200">
          <div className={`px-4 py-2.5 rounded-xl border shadow-2xl text-xs font-mono font-medium ${
            toast.type === 'success'
              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
              : 'bg-rose-500/10 text-rose-400 border-rose-500/30'
          }`}>
            {toast.message}
          </div>
        </div>
      )}

      {/* Main Header */}
      <Header
        connected={connected}
        proxyCount={proxyList.length}
        totalConns={totalSockets}
        totalFaults={totalFaults}
        onReset={handleResetAll}
        onNewProxy={() => setIsNewProxyOpen(true)}
        onExport={async () => {
          try {
            const res = await fetch('/api/v1/config/export');
            const blob = await res.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `faultcraft-config-${new Date().toISOString().slice(0, 10)}.json`;
            a.click();
            window.URL.revokeObjectURL(url);
            showToast('Configuration exported');
          } catch (err: any) {
            showToast(err.message, 'error');
          }
        }}
        onImport={async (file: File) => {
          try {
            const text = await file.text();
            const res = await fetch('/api/v1/config/import', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: text,
            });
            if (!res.ok) throw new Error('Failed importing configuration file');
            const summary = await res.json();
            await loadInitialData();
            showToast(`Restored ${summary.imported_proxies || 0} proxies and ${summary.imported_toxics || 0} toxics`);
          } catch (err: any) {
            showToast(err.message, 'error');
          }
        }}
        isResetting={isResetting}
      />

      {/* Content Body */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6 flex-1 w-full">
        {/* Preset Bar */}
        <PresetBar
          presets={presets}
          proxies={proxyList}
          onApplyPreset={handleApplyPreset}
        />

        {/* Incident Timeline Scenario Player */}
        <ScenarioPlayer
          proxies={proxyList}
          status={seqStatus}
          onRefresh={loadInitialData}
        />

        {/* Live Traffic Visualizer */}
        {proxyList.length > 0 && <TrafficVisualizer proxies={proxyList} />}

        {/* Live L7 Traffic Frame Inspector */}
        <TrafficInspector
          frames={frames}
          onClear={async () => {
            try {
              await fetch('/api/v1/inspector', { method: 'DELETE' });
              setFrames([]);
            } catch (e) {
              // Ignore
            }
          }}
        />

        {/* Filter / Search Bar */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-2">
          <div className="flex items-center gap-2">
            <h2 className="text-base font-bold text-white tracking-tight font-mono">Registered Proxies</h2>
            <span className="text-xs font-mono px-2 py-0.5 rounded-full bg-[#182138] text-slate-400 border border-[#232f4c]">
              {filteredProxies.length}
            </span>
          </div>

          <div className="relative w-full sm:w-64">
            <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-500" />
            <input
              type="text"
              placeholder="Search proxies or ports..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-[#111726] border border-[#232f4c] rounded-xl pl-9 pr-3 py-1.5 text-xs font-mono text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500"
            />
          </div>
        </div>

        {/* Proxy Grid */}
        {filteredProxies.length === 0 ? (
          <div className="py-16 text-center rounded-2xl bg-[#111726]/60 border border-dashed border-[#232f4c]">
            <div className="h-12 w-12 mx-auto rounded-2xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 mb-3">
              <Network className="h-6 w-6" />
            </div>
            <h3 className="text-sm font-semibold text-white font-mono">No Proxies Configured</h3>
            <p className="text-xs text-slate-400 max-w-sm mx-auto mt-1 mb-4 font-sans">
              Create your first proxy to intercept traffic between your application and PostgreSQL, Redis, or external APIs.
            </p>
            <button
              onClick={() => setIsNewProxyOpen(true)}
              className="px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-mono font-medium shadow-lg shadow-indigo-600/30 transition active:scale-95"
            >
              Add New Proxy
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-5">
            {filteredProxies.map((proxy) => (
              <ProxyCard
                key={proxy.name}
                proxy={proxy}
                onToggleEnabled={handleToggleEnabled}
                onAddToxic={(p) => setToxicProxy(p)}
                onDeleteToxic={handleDeleteToxic}
                onDeleteProxy={handleDeleteProxy}
                onExportCli={(p) => setCliProxy(p)}
              />
            ))}
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-[#232f4c] py-4 bg-[#111726]/50 text-center text-xs font-mono text-slate-500">
        faultcraft v0.1.0 • Toxiproxy v2 Compatible Chaos Proxy Engine
      </footer>

      {/* Modals */}
      {toxicProxy && (
        <ToxicModal
          proxy={toxicProxy}
          onClose={() => setToxicProxy(null)}
          onSave={handleAddToxic}
        />
      )}

      {isNewProxyOpen && (
        <NewProxyModal
          onClose={() => setIsNewProxyOpen(false)}
          onSave={handleCreateProxy}
        />
      )}

      {cliProxy && (
        <CliExportModal
          proxy={cliProxy}
          onClose={() => setCliProxy(null)}
        />
      )}
    </div>
  );
};
