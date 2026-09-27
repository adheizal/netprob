import type { Agent, AgentPage, AuthSession, Link, LinkPage, LinkStatus, MTRRun, OverviewData, PingResult, RetentionSettings, RetentionSettingsUpdate, SecuritySettings } from './types'

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
  listAgentsPage: (page: number, pageSize: number, search = '', status = ''): Promise<AgentPage> => {
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
    if (search) params.set('search', search)
    if (status) params.set('status', status)
    return fetchAPI(`/api/agents?${params}`)
  },
  getAgent: (id: string): Promise<Agent> => fetchAPI(`/api/agents/${id}`),
  deleteAgent: (id: string): Promise<void> => fetchAPI(`/api/agents/${id}`, { method: 'DELETE' }),

  // Links
  createLink: (data: { name: string; description?: string }): Promise<Link> =>
    fetchAPI('/api/links', { method: 'POST', body: JSON.stringify(data) }),
  listLinks: (): Promise<any[]> => fetchAPI('/api/links'),
  listLinksPage: (options: {
    page: number
    pageSize: number
    search?: string
    status?: LinkStatus | ''
    agentId?: string
    sourceAgentId?: string
    destinationAgentId?: string
    sort?: 'status' | 'name' | 'latency' | 'loss' | 'updated'
    order?: 'asc' | 'desc'
  }): Promise<LinkPage> => {
    const params = new URLSearchParams({ page: String(options.page), page_size: String(options.pageSize) })
    if (options.search) params.set('search', options.search)
    if (options.status) params.set('status', options.status)
    if (options.agentId) params.set('agent_id', options.agentId)
    if (options.sourceAgentId) params.set('source_agent_id', options.sourceAgentId)
    if (options.destinationAgentId) params.set('destination_agent_id', options.destinationAgentId)
    if (options.sort) params.set('sort', options.sort)
    if (options.order) params.set('order', options.order)
    return fetchAPI(`/api/links?${params}`)
  },
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

  // Dashboard summary
  getOverview: (problemLimit = 10): Promise<OverviewData> =>
    fetchAPI(`/api/overview?problem_limit=${problemLimit}`),

  // Settings
  getRetentionSettings: (): Promise<RetentionSettings> => fetchAPI('/api/settings/retention'),
  updateRetentionSettings: (data: Pick<RetentionSettings, 'ping_retention_days' | 'mtr_retention_days'>): Promise<RetentionSettingsUpdate> =>
    fetchAPI('/api/settings/retention', { method: 'PUT', body: JSON.stringify(data) }),
  updateSecuritySettings: (authEnabled: boolean): Promise<SecuritySettings> =>
    fetchAPI('/api/settings/security', { method: 'PUT', body: JSON.stringify({ auth_enabled: authEnabled }) }),

  health: (): Promise<{ status: string }> => fetchAPI('/health'),
}
