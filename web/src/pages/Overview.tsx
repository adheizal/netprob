import { useEffect, useState } from 'react'
import { Link as RouterLink } from 'react-router-dom'
import { AlertTriangle, CheckCircle, Network, Users, XCircle } from 'lucide-react'
import { api } from '../api'
import type { LinkStatus, OverviewData } from '../types'

const statusStyles: Record<LinkStatus, string> = {
  healthy: 'bg-green-100 text-green-800',
  degraded: 'bg-amber-100 text-amber-800',
  down: 'bg-red-100 text-red-800',
  inactive: 'bg-gray-100 text-gray-600',
}

export default function Overview() {
  const [data, setData] = useState<OverviewData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    async function loadData(initial = false) {
      if (initial) setLoading(true)
      try {
        const response = await api.getOverview(10)
        if (!active) return
        setData(response)
        setError('')
      } catch (err) {
        if (active) setError(err instanceof Error ? err.message : 'Failed to load overview')
      } finally {
        if (active && initial) setLoading(false)
      }
    }
    loadData(true)
    const refresh = window.setInterval(() => loadData(), 15_000)
    return () => {
      active = false
      window.clearInterval(refresh)
    }
  }, [])

  if (loading) return <div className="p-6">Loading...</div>
  if (!data) return <div className="p-6 text-red-600">{error || 'Overview is unavailable.'}</div>

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-800">NetProb Overview</h1>
        <span className="text-xs text-gray-400">Auto-refreshes every 15 seconds</span>
      </div>

      {error && <div className="mb-4 rounded border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">Refresh failed: {error}</div>}

      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <RouterLink to="/agents" className="flex items-center gap-3 rounded-lg bg-white p-4 shadow hover:shadow-md">
          <Users className="h-8 w-8 text-blue-500" />
          <div><div className="text-2xl font-bold">{data.agents.total}</div><div className="text-sm text-gray-500">Total Agents</div></div>
        </RouterLink>
        <RouterLink to="/agents" className="flex items-center gap-3 rounded-lg bg-white p-4 shadow hover:shadow-md">
          <CheckCircle className="h-8 w-8 text-green-500" />
          <div><div className="text-2xl font-bold text-green-600">{data.agents.online}</div><div className="text-sm text-gray-500">Online Agents</div></div>
        </RouterLink>
        <RouterLink to="/agents" className="flex items-center gap-3 rounded-lg bg-white p-4 shadow hover:shadow-md">
          <XCircle className="h-8 w-8 text-red-500" />
          <div><div className="text-2xl font-bold text-red-600">{data.agents.offline}</div><div className="text-sm text-gray-500">Offline Agents</div></div>
        </RouterLink>
        <RouterLink to="/links" className="flex items-center gap-3 rounded-lg bg-white p-4 shadow hover:shadow-md">
          <Network className="h-8 w-8 text-purple-500" />
          <div><div className="text-2xl font-bold">{data.links.total}</div><div className="text-sm text-gray-500">Total Links</div></div>
        </RouterLink>
      </div>

      <div className="mb-8 grid grid-cols-2 gap-3 md:grid-cols-4">
        {(['healthy', 'degraded', 'down', 'inactive'] as LinkStatus[]).map(status => (
          <RouterLink key={status} to={`/links?status=${status}`} className="rounded-lg bg-white p-3 shadow-sm hover:shadow">
            <div className="text-xs font-medium uppercase tracking-wide text-gray-500">{status}</div>
            <div className="mt-1 text-xl font-semibold">{data.links[status]}</div>
          </RouterLink>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-8 xl:grid-cols-2">
        <section>
          <div className="mb-3 flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-xl font-semibold text-gray-800"><AlertTriangle className="h-5 w-5 text-amber-500" />Problem Links</h2>
            <RouterLink to="/links?sort=status" className="text-sm text-blue-600 hover:underline">View all links</RouterLink>
          </div>
          {data.problem_links.length === 0 ? (
            <div className="rounded-lg bg-white p-5 text-sm text-green-700 shadow">No link problems detected.</div>
          ) : (
            <div className="space-y-3">
              {data.problem_links.map(link => (
                <RouterLink key={link.id} to={`/links/${link.id}`} className="block rounded-lg bg-white p-4 shadow hover:shadow-md">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-semibold text-gray-800">{link.name}</div>
                      <div className="mt-1 text-xs text-gray-500">
                        {link.directions.map(direction => `${direction.source_agent?.hostname || direction.source_agent_id} → ${direction.dest_agent?.hostname || direction.destination_agent_id}`).join(' · ')}
                      </div>
                    </div>
                    {link.status && <span className={`rounded-full px-2 py-1 text-xs capitalize ${statusStyles[link.status]}`}>{link.status}</span>}
                  </div>
                </RouterLink>
              ))}
            </div>
          )}
        </section>

        <section>
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-xl font-semibold text-gray-800">Recently Offline Agents</h2>
            <RouterLink to="/agents" className="text-sm text-blue-600 hover:underline">View all agents</RouterLink>
          </div>
          {data.offline_agents.length === 0 ? (
            <div className="rounded-lg bg-white p-5 text-sm text-green-700 shadow">All agents are online.</div>
          ) : (
            <div className="overflow-hidden rounded-lg bg-white shadow">
              {data.offline_agents.map(agent => (
                <div key={agent.id} className="flex items-center justify-between border-b border-gray-100 px-4 py-3 last:border-0">
                  <div><div className="font-medium text-gray-800">{agent.hostname}</div><div className="text-xs text-gray-500">{agent.primary_address || 'No primary address'}</div></div>
                  <div className="text-right text-xs text-gray-500">{agent.last_seen ? new Date(agent.last_seen).toLocaleString() : 'Never seen'}</div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
