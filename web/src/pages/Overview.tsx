import { useEffect, useState } from 'react'
import { Link as RouterLink } from 'react-router-dom'
import { Network, Users, CheckCircle, XCircle } from 'lucide-react'
import { api } from '../api'
import type { Agent, LinkWithAgents } from '../types'

export default function Overview() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [links, setLinks] = useState<LinkWithAgents[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function loadData() {
      try {
        const [agentsData, linksData] = await Promise.all([api.listAgents(), api.listLinks()])
        setAgents(agentsData)
        setLinks(linksData)
      } catch (err) {
        console.error('Failed to load data:', err)
      } finally {
        setLoading(false)
      }
    }
    loadData()
  }, [])

  if (loading) return <div className="p-6">Loading...</div>

  const onlineAgents = agents.filter(a => a.online).length
  const offlineAgents = agents.length - onlineAgents
  const totalLinks = links.length

  const allDirections = links.flatMap(l => l.directions)
  const onlineDirs = allDirections.filter(d => d.online).length

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold text-gray-800 mb-6">NetProb Overview</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <div className="bg-white rounded-lg shadow p-4 flex items-center gap-3">
          <Users className="w-8 h-8 text-blue-500" />
          <div>
            <div className="text-2xl font-bold">{agents.length}</div>
            <div className="text-sm text-gray-500">Total Agents</div>
          </div>
        </div>
        <div className="bg-white rounded-lg shadow p-4 flex items-center gap-3">
          <CheckCircle className="w-8 h-8 text-green-500" />
          <div>
            <div className="text-2xl font-bold text-green-600">{onlineAgents}</div>
            <div className="text-sm text-gray-500">Online Agents</div>
          </div>
        </div>
        <div className="bg-white rounded-lg shadow p-4 flex items-center gap-3">
          <XCircle className="w-8 h-8 text-red-500" />
          <div>
            <div className="text-2xl font-bold text-red-600">{offlineAgents}</div>
            <div className="text-sm text-gray-500">Offline Agents</div>
          </div>
        </div>
        <div className="bg-white rounded-lg shadow p-4 flex items-center gap-3">
          <Network className="w-8 h-8 text-purple-500" />
          <div>
            <div className="text-2xl font-bold">{totalLinks}</div>
            <div className="text-sm text-gray-500">Links</div>
          </div>
        </div>
      </div>

      <h2 className="text-xl font-semibold text-gray-800 mb-4">Recent Links</h2>
      {links.length === 0 ? (
        <p className="text-gray-500">No links configured.</p>
      ) : (
        <div className="space-y-3">
          {links.map(link => {
            const online = onlineDirs > 0
            const firstDir = link.directions[0]
            const latestPing = firstDir?.latest_ping
            return (
              <RouterLink key={link.id} to={`/links/${link.id}`} className="block">
                <div className="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow">
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold text-gray-800">{link.name}</h3>
                      {link.description && <p className="text-sm text-gray-500">{link.description}</p>}
                      <div className="mt-2 flex gap-4 text-sm text-gray-600">
                        {link.directions.map(dir => (
                          <span key={dir.id}>
                            {dir.source_agent?.hostname || dir.source_agent_id} → {dir.dest_agent?.hostname || dir.destination_agent_id}
                          </span>
                        ))}
                      </div>
                    </div>
                    <div className={`w-3 h-3 rounded-full ${online ? 'bg-green-500' : 'bg-red-500'}`} />
                  </div>
                  {latestPing && (
                    <div className="mt-2 text-sm text-gray-600">
                      Latest latency: {latestPing.avg_rtt_ms?.toFixed(1) ?? 'N/A'} ms
                      {' | '}
                      Packet loss: {latestPing.packet_loss_percent?.toFixed(1)}%
                    </div>
                  )}
                </div>
              </RouterLink>
            )
          })}
        </div>
      )}

      <h2 className="text-xl font-semibold text-gray-800 mb-4 mt-8">All Agents</h2>
      {agents.length === 0 ? (
        <p className="text-gray-500">No agents registered.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full bg-white rounded-lg shadow overflow-hidden">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Status</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Hostname</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Version</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Addresses</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Region</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {agents.map(agent => (
                <tr key={agent.id} className="border-t border-gray-100">
                  <td className="px-4 py-2">
                    {agent.online ? (
                      <CheckCircle className="w-5 h-5 text-green-500" />
                    ) : (
                      <XCircle className="w-5 h-5 text-gray-400" />
                    )}
                  </td>
                  <td className="px-4 py-2 font-medium">{agent.hostname}</td>
                  <td className="px-4 py-2">{agent.version}</td>
                  <td className="px-4 py-2 font-mono text-sm">{agent.primary_address || '—'}</td>
                  <td className="px-4 py-2">{agent.location.region || '—'}</td>
                  <td className="px-4 py-2">{agent.last_seen ? new Date(agent.last_seen).toLocaleString() : 'Never'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
