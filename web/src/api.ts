import type { Agent, AgentPage, AuthSession, Link, PingResult, MTRRun, RetentionSettings, RetentionSettingsUpdate, SecuritySettings } from './types'

const API_BASE = import.meta.env.DEV ? '' : ''

export async function fetchAPI<T>(path: string, options: RequestInit = {}): Promise<T> {
  const resp = await fetch(`${API_BASE}${path}`, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(`${resp.status}: ${text}`)
  }
  if (resp.status === 204) {
    return undefined as T
  }
  return resp.json() as Promise<T>
}

export const api = {
  // Authentication
  getAuthSession: (): Promise<AuthSession> => fetchAPI('/api/auth/session'),
  login: (email: string, password: string): Promise<AuthSession> =>
    fetchAPI('/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  logout: (): Promise<void> => fetchAPI('/api/auth/logout', { method: 'POST' }),
  updateAdminAccount: (data: { email: string; current_password: string; new_password: string }): Promise<AuthSession> =>
    fetchAPI('/api/auth/account', { method: 'PUT', body: JSON.stringify(data) }),

  // Agents
  registerAgent: (data: Partial<Agent>): Promise<{ id: string; token: string }> =>
    fetchAPI('/api/agents/register', { method: 'POST', body: JSON.stringify(data) }),
  listAgents: (): Promise<Agent[]> => fetchAPI('/api/agents'),
  listAgentsPage: (page: number, pageSize: number): Promise<AgentPage> =>
    fetchAPI(`/api/agents?page=${page}&page_size=${pageSize}`),
  getAgent: (id: string): Promise<Agent> => fetchAPI(`/api/agents/${id}`),
  deleteAgent: (id: string): Promise<void> => fetchAPI(`/api/agents/${id}`, { method: 'DELETE' }),

  // Links
  createLink: (data: { name: string; description?: string }): Promise<Link> =>
    fetchAPI('/api/links', { method: 'POST', body: JSON.stringify(data) }),
  listLinks: (): Promise<any[]> => fetchAPI('/api/links'),
  getLink: (id: string): Promise<Link> => fetchAPI(`/api/links/${id}`),
  updateLink: (id: string, data: any): Promise<void> =>
    fetchAPI(`/api/links/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteLink: (id: string): Promise<void> =>
    fetchAPI(`/api/links/${id}`, { method: 'DELETE' }),

  // Directions
  createDirection: (linkId: string, data: any): Promise<any> =>
    fetchAPI(`/api/links/${linkId}/directions`, { method: 'POST', body: JSON.stringify(data) }),
  updateDirection: (linkId: string, directionId: string, data: any): Promise<void> =>
    fetchAPI(`/api/links/${linkId}/directions/${directionId}`, { method: 'PUT', body: JSON.stringify(data) }),
  listDirections: (linkId: string): Promise<any[]> =>
    fetchAPI(`/api/links/${linkId}/directions`),

  // Ping results
  getPingResults: (linkId: string, directionId: string, limit?: number): Promise<PingResult[]> =>
    fetchAPI(`/api/links/${linkId}/directions/${directionId}/ping${limit ? `?limit=${limit}` : ''}`),

  // MTR runs
  getMTRRuns: (linkId: string, directionId: string, limit?: number): Promise<MTRRun[]> =>
    fetchAPI(`/api/links/${linkId}/directions/${directionId}/mtr${limit ? `?limit=${limit}` : ''}`),
  getMTRRun: (id: string): Promise<MTRRun> => fetchAPI(`/api/mtr-runs/${id}`),

  // Settings
  getRetentionSettings: (): Promise<RetentionSettings> => fetchAPI('/api/settings/retention'),
  updateRetentionSettings: (data: Pick<RetentionSettings, 'ping_retention_days' | 'mtr_retention_days'>): Promise<RetentionSettingsUpdate> =>
    fetchAPI('/api/settings/retention', { method: 'PUT', body: JSON.stringify(data) }),
  updateSecuritySettings: (authEnabled: boolean): Promise<SecuritySettings> =>
    fetchAPI('/api/settings/security', { method: 'PUT', body: JSON.stringify({ auth_enabled: authEnabled }) }),

  health: (): Promise<{ status: string }> => fetchAPI('/health'),
}
