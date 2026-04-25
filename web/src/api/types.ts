export type SubscriptionStatus = 'success' | 'failed' | 'never';
export type AdapterStatus = 'supported' | 'unsupported' | 'error';
export type AliveStatus = 'unknown' | 'alive' | 'dead';
export type BindMode = 'all' | 'group' | 'node';
export type SelectionPolicy = 'random' | 'fixed';

export interface SystemHealth {
  status: string;
  time: string;
}

export interface SingBoxStatus {
  enabled: boolean;
  version: string;
  config_version?: string;
  mode: string;
  prefer_native_http_socks: boolean;
  adapter_configured: boolean;
  max_active_engines: number;
  engine_idle_timeout_seconds: number;
  engine_dial_timeout_seconds: number;
  health_check_target?: string;
  enable_udp: boolean;
  quic_enabled: boolean;
  utls_enabled: boolean;
  supported_protocols: string[];
  license: string;
}

export interface Subscription {
  id: number;
  name: string;
  url: string;
  user_agent: string;
  refresh_interval_seconds: number;
  enabled: boolean;
  last_refresh_at?: string;
  next_refresh_at?: string;
  last_status: SubscriptionStatus;
  last_error?: string;
  upload_bytes?: number;
  download_bytes?: number;
  total_bytes?: number;
  expire_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SubscriptionRefreshResult {
  subscription_id: number;
  node_count: number;
  http_status: number;
  sing_box_supported_count: number;
  sing_box_error_count: number;
  unsupported_count: number;
}

export interface SubscriptionRefreshLog {
  id: number;
  subscription_id: number;
  status: string;
  http_status?: number;
  node_count: number;
  sing_box_supported_count: number;
  sing_box_error_count: number;
  unsupported_count: number;
  error?: string;
  started_at: string;
  finished_at?: string;
  created_at: string;
}

export interface ProxyNode {
  id: number;
  subscription_id: number;
  subscription_node_key: string;
  name: string;
  protocol: string;
  server: string;
  port: number;
  raw_uri?: string;
  raw_config_json: string;
  sing_box_outbound_json?: string;
  sing_box_status: AdapterStatus;
  sing_box_error?: string;
  sing_box_version?: string;
  udp_supported: boolean;
  transport_type?: string;
  adapter_status: AdapterStatus;
  enabled: boolean;
  alive_status: AliveStatus;
  last_seen_at?: string;
  last_checked_at?: string;
  latency_ms?: number;
  fail_count: number;
  group_ids?: number[];
  created_at: string;
  updated_at: string;
}

export interface ProxyGroup {
  id: number;
  name: string;
  description: string;
  auto_created: boolean;
  created_at: string;
  updated_at: string;
}

export interface GroupKeyword {
  id: number;
  name: string;
  keywords: string;
  case_sensitive: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CredentialBinding {
  target_type: 'group' | 'node';
  target_id: number;
}

export interface Credential {
  id: number;
  username: string;
  enabled: boolean;
  bind_mode: BindMode;
  selection_policy: SelectionPolicy;
  remark: string;
  bindings?: CredentialBinding[];
  created_at: string;
  updated_at: string;
}

export interface TrafficOverview {
  connections: number;
  success_connections: number;
  failed_connections: number;
  upload_bytes: number;
  download_bytes: number;
}
