export type StreamDirection = 'upstream' | 'downstream';

export interface Toxic {
  name: string;
  type: string;
  stream: StreamDirection;
  toxicity: number;
  attributes: Record<string, any>;
  duration_ms?: number;
  expires_at?: string;
}

export interface Stats {
  active_connections: number;
  total_connections: number;
  bytes_upstream: number;
  bytes_downstream: number;
  error_count: number;
  fault_count: number;
  p50_latency_ms: number;
  p90_latency_ms: number;
  p99_latency_ms: number;
}

export interface TargetHealth {
  status: 'healthy' | 'degraded' | 'down' | 'unknown';
  rtt_ms: number;
  error?: string;
  last_ping: number;
}

export interface Proxy {
  name: string;
  listen: string;
  upstream: string;
  enabled: boolean;
  toxics: Toxic[];
  stats?: Stats;
  target_health?: TargetHealth;
}

export interface PresetDef {
  name: string;
  type: string;
  stream: StreamDirection;
  toxicity: number;
  attributes: Record<string, any>;
}

export interface Preset {
  id: string;
  name: string;
  description: string;
  category: string;
  toxics: PresetDef[];
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

export interface TelemetryPayload {
  timestamp: number;
  proxies: {
    name: string;
    listen: string;
    upstream: string;
    enabled: boolean;
    stats: Stats;
    target_health?: TargetHealth;
    toxics: Toxic[];
  }[];
  recent_frames?: {
    id: string;
    timestamp: string;
    proxy_name: string;
    direction: 'upstream' | 'downstream';
    protocol: string;
    summary: string;
    bytes: number;
    fault_injected?: string;
    latency_ms: number;
  }[];
  sequence_status?: SequenceStatus;
}
