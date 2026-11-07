import { Proxy, Toxic, Preset } from '../types';

const BASE_URL = '';

export async function fetchProxies(): Promise<Record<string, Proxy>> {
  const res = await fetch(`${BASE_URL}/proxies`);
  if (!res.ok) throw new Error(`Failed to fetch proxies: ${res.statusText}`);
  return res.json();
}

export async function createProxy(data: { name: string; listen: string; upstream: string; enabled?: boolean }): Promise<Proxy> {
  const res = await fetch(`${BASE_URL}/proxies`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ title: res.statusText }));
    throw new Error(err.title || err.message || 'Failed creating proxy');
  }
  return res.json();
}

export async function updateProxy(name: string, data: { enabled?: boolean; listen?: string; upstream?: string }): Promise<Proxy> {
  const res = await fetch(`${BASE_URL}/proxies/${encodeURIComponent(name)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error(`Failed updating proxy: ${res.statusText}`);
  return res.json();
}

export async function deleteProxy(name: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/proxies/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error(`Failed deleting proxy: ${res.statusText}`);
}

export async function addToxic(proxyName: string, toxic: {
  name: string;
  type: string;
  stream: string;
  toxicity: number;
  attributes: Record<string, any>;
}): Promise<Toxic> {
  const res = await fetch(`${BASE_URL}/proxies/${encodeURIComponent(proxyName)}/toxics`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(toxic),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ title: res.statusText }));
    throw new Error(err.title || err.message || 'Failed adding toxic');
  }
  return res.json();
}

export async function deleteToxic(proxyName: string, toxicName: string): Promise<void> {
  const res = await fetch(
    `${BASE_URL}/proxies/${encodeURIComponent(proxyName)}/toxics/${encodeURIComponent(toxicName)}`,
    { method: 'DELETE' }
  );
  if (!res.ok) throw new Error(`Failed deleting toxic: ${res.statusText}`);
}

export async function resetAll(): Promise<void> {
  const res = await fetch(`${BASE_URL}/reset`, { method: 'POST' });
  if (!res.ok) throw new Error(`Reset failed: ${res.statusText}`);
}

export async function fetchPresets(): Promise<Preset[]> {
  const res = await fetch(`${BASE_URL}/api/v1/presets`);
  if (!res.ok) throw new Error(`Failed fetching presets: ${res.statusText}`);
  return res.json();
}

export async function applyPreset(proxyName: string, presetId: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/api/v1/presets/apply`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ proxy: proxyName, preset: presetId }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ title: res.statusText }));
    throw new Error(err.title || err.message || 'Failed applying preset');
  }
}
