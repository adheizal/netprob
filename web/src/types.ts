export interface Agent {
  id: string
  hostname: string
  version: string
  addresses: string[]
  primary_address: string
  capabilities: string[]
  location: AgentLocation
  online: boolean
  last_seen: string | null
  created_at: string
  updated_at: string
}

export interface AgentLocation {
  public_ip: string
  country_code: string
  region: string
  city: string
  timezone: string
  as_name: string
  isp: string
}

export interface Link {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface Direction {
  id: string
  link_id: string
  source_agent_id: string
  destination_agent_id: string
  target_address: string
  ping_interval_seconds: number
  mtr_interval_seconds: number
  ping_enabled: boolean
  mtr_enabled: boolean
  created_at: string
  updated_at: string
}

export interface PingResult {
  id: number
  direction_id: string
  source_agent_id: string
  destination_agent_id: string
  timestamp: string
  min_rtt_ms?: number
  avg_rtt_ms?: number
  max_rtt_ms?: number
  jitter_ms?: number
  packet_loss_percent?: number
  packets_sent: number
  packets_received: number
}

export interface MTRHop {
  hop: number
  host?: string
  ip?: string
  loss_percent: number
  sent: number
  last_ms?: number
  avg_ms?: number
  best_ms?: number
  worst_ms?: number
}

export interface MTRRun {
  id: string
  direction_id: string
  source_agent_id: string
  destination_agent_id: string
  timestamp: string
  status: 'success' | 'error'
  error?: string
  hops: MTRHop[]
}

export interface LinkWithAgents {
  id: string
  name: string
  description: string
  directions: DirectionSummary[]
  created_at: string
  updated_at: string
}

export interface DirectionSummary {
  id: string
  source_agent_id: string
  destination_agent_id: string
  source_agent?: AgentSummary
  dest_agent?: AgentSummary
  target_address: string
  ping_interval_seconds: number
  mtr_interval_seconds: number
  ping_enabled: boolean
  mtr_enabled: boolean
  online: boolean
  latest_ping?: PingResult
}

export interface AgentSummary {
  id: string
  hostname: string
  version: string
  addresses: string[]
  primary_address: string
  online: boolean
  last_seen?: string
}

export interface RetentionSettings {
  ping_retention_days: number
  mtr_retention_days: number
  updated_at: string
}

export interface RetentionSettingsUpdate extends RetentionSettings {
  cleanup: {
    ping_results_deleted: number
    mtr_runs_deleted: number
  }
}

export interface AuthSession {
  auth_enabled: boolean
  authenticated: boolean
  email?: string
  must_change_password: boolean
}

export interface SecuritySettings {
  auth_enabled: boolean
  updated_at: string
}
