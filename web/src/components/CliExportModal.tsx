import React, { useState } from 'react';
import { X, Copy, Check, Terminal, Code2 } from 'lucide-react';
import { Proxy } from '../types';

interface CliExportModalProps {
  proxy: Proxy;
  onClose: () => void;
}

export const CliExportModal: React.FC<CliExportModalProps> = ({ proxy, onClose }) => {
  const [tab, setTab] = useState<'curl' | 'gate' | 'python' | 'go'>('curl');
  const [copied, setCopied] = useState(false);

  const getCurlSnippet = (): string => {
    let script = `# 1. Create Proxy\ncurl -X POST http://localhost:8474/proxies \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "name": "${proxy.name}",\n    "listen": "${proxy.listen}",\n    "upstream": "${proxy.upstream}",\n    "enabled": true\n  }'\n\n`;

    if (proxy.toxics.length > 0) {
      script += `# 2. Inject Active Faults\n`;
      proxy.toxics.forEach((t) => {
        script += `curl -X POST http://localhost:8474/proxies/${proxy.name}/toxics \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "name": "${t.name}",\n    "type": "${t.type}",\n    "stream": "${t.stream}",\n    "toxicity": ${t.toxicity},\n    "attributes": ${JSON.stringify(t.attributes)}\n  }'\n\n`;
      });
    }

    return script.trim();
  };

  const getGateSnippet = (): string => {
    return `# Run your integration test suite under FaultCraft CI/CD quality gate
faultcraft gate \\
  --proxy "${proxy.name}=${proxy.listen}->${proxy.upstream}" \\
  --preset flaky_db \\
  --max-p99-latency 500ms \\
  --max-error-rate 0.05 \\
  --report-format junit \\
  --output test-results/resilience.xml \\
  -- npm test`;
  };

  const getPythonSnippet = (): string => {
    return `from toxiproxy import Toxiproxy

client = Toxiproxy(host="localhost", port=8474)
proxy = client.create(name="${proxy.name}", listen="${proxy.listen}", upstream="${proxy.upstream}")

${proxy.toxics.map(t => `# Add toxic: ${t.name}
proxy.add_toxic(
    name="${t.name}",
    type="${t.type}",
    stream="${t.stream}",
    toxicity=${t.toxicity},
    attributes=${JSON.stringify(t.attributes, null, 4)}
)`).join('\n\n')}

# Run your test assertions here...

# Reset afterwards
client.reset()`;
  };

  const getGoSnippet = (): string => {
    return `package main

import (
    "testing"
    toxiproxy "github.com/Shopify/toxiproxy/v2/client"
)

func TestResilience(t *testing.T) {
    client := toxiproxy.NewClient("localhost:8474")

    proxy, err := client.CreateProxy("${proxy.name}", "${proxy.listen}", "${proxy.upstream}")
    if err != nil {
        t.Fatalf("failed to create proxy: %v", err)
    }
    defer proxy.Delete()

${proxy.toxics.map(t => `    _, _ = proxy.AddToxic("${t.name}", "${t.type}", "${t.stream}", ${t.toxicity}, toxiproxy.Attributes${fmtGoAttrs(t.attributes)})`).join('\n')}

    // Run client requests against ${proxy.listen} and assert resilience
}`;
  };

  const fmtGoAttrs = (attrs: Record<string, any>): string => {
    return JSON.stringify(attrs).replace(/"/g, '`');
  };

  const getActiveCode = (): string => {
    switch (tab) {
      case 'curl':
        return getCurlSnippet();
      case 'gate':
        return getGateSnippet();
      case 'python':
        return getPythonSnippet();
      case 'go':
        return getGoSnippet();
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(getActiveCode());
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="w-full max-w-2xl rounded-2xl bg-[#111726] border border-[#232f4c] shadow-2xl overflow-hidden flex flex-col max-h-[85vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-5 border-b border-[#232f4c]">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
              <Terminal className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white font-mono">Export CLI & Integration Code</h2>
              <p className="text-xs text-slate-400 font-mono">Reproduce this exact chaos state</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-[#182138] transition"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Tab Selector */}
        <div className="px-5 pt-3 flex items-center gap-2 border-b border-[#232f4c]">
          {(['curl', 'gate', 'python', 'go'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`pb-2.5 px-2 text-xs font-mono font-medium transition border-b-2 uppercase ${
                tab === t
                  ? 'border-indigo-500 text-white'
                  : 'border-transparent text-slate-400 hover:text-slate-200'
              }`}
            >
              {t === 'gate' ? 'CI/CD Gate' : t}
            </button>
          ))}
        </div>

        {/* Code Content */}
        <div className="p-4 sm:p-5 overflow-y-auto flex-1 bg-[#090d16]">
          <div className="relative">
            <button
              onClick={handleCopy}
              className="absolute top-2 right-2 flex items-center gap-1 px-2.5 py-1 rounded-lg bg-[#182138] hover:bg-[#232f4c] border border-[#232f4c] text-xs font-mono text-slate-300 transition"
            >
              {copied ? (
                <>
                  <Check className="h-3.5 w-3.5 text-emerald-400" />
                  <span className="text-emerald-400">Copied!</span>
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5" />
                  <span>Copy</span>
                </>
              )}
            </button>
            <pre className="p-4 rounded-xl bg-[#111726] border border-[#232f4c] text-xs font-mono text-slate-200 overflow-x-auto selection:bg-indigo-500/30">
              <code>{getActiveCode()}</code>
            </pre>
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-[#232f4c] flex items-center justify-between bg-[#111726]">
          <span className="text-[11px] font-mono text-slate-400 flex items-center gap-1.5">
            <Code2 className="h-3.5 w-3.5 text-indigo-400" />
            Compatible with official Toxiproxy v2 client SDKs
          </span>
          <button
            onClick={onClose}
            className="px-4 py-1.5 rounded-xl bg-[#182138] hover:bg-[#232f4c] text-slate-200 text-xs font-mono transition"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
