import { useEffect, useState } from 'react'
import { Plus, Trash2, Copy, CheckCircle, XCircle, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react'
import { api } from '../api'
import type { Agent } from '../types'

export default function Agents() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [showRegister, setShowRegister] = useState(false)
  const [registerLoading, setRegisterLoading] = useState(false)
  const [newAgent, setNewAgent] = useState<{ id: string; token: string } | null>(null)
  const [copiedToken, setCopiedToken] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  async function loadAgents(targetPage = page, targetPageSize = pageSize) {
    setLoading(true)
    try {
      const data = await api.listAgentsPage(targetPage, targetPageSize)
      if (data.total > 0 && targetPage > data.total_pages) {
        setPage(data.total_pages)
        return
      }
      setAgents(data.agents)
      setTotal(data.total)
      setTotalPages(data.total_pages)
    } catch (err) {
      console.error('Failed to load agents:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    let active = true
    setLoading(true)
    api.listAgentsPage(page, pageSize)
      .then(data => {
        if (!active) return
        if (data.total > 0 && page > data.total_pages) {
          setPage(data.total_pages)
          return
        }
        setAgents(data.agents)
        setTotal(data.total)
        setTotalPages(data.total_pages)
      })
      .catch(err => console.error('Failed to load agents:', err))
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [page, pageSize])

  async function handleRegister() {
    setRegisterLoading(true)
    try {
      const result = await api.registerAgent({})
      setNewAgent(result)
      setShowRegister(false)
      setCopiedToken(false)
      if (page === 1) {
        loadAgents(1, pageSize)
      } else {
        setPage(1)
      }
    } catch (err) {
      console.error('Failed to register agent:', err)
    } finally {
      setRegisterLoading(false)
    }
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this agent?')) return
    try {
      await api.deleteAgent(id)
      if (agents.length === 1 && page > 1) {
        setPage(page - 1)
      } else {
        loadAgents(page, pageSize)
      }
    } catch (err) {
      console.error('Failed to delete agent:', err)
    }
  }

  function copyToken() {
    if (newAgent) {
      navigator.clipboard.writeText(newAgent.token)
      setCopiedToken(true)
      setTimeout(() => setCopiedToken(false), 2000)
    }
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold text-gray-800">Agents</h1>
        <button
          onClick={() => setShowRegister(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
          Create Enrollment Token
        </button>
      </div>

      {newAgent && (
        <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
          <div className="flex items-center gap-2 mb-2">
            <CheckCircle className="w-5 h-5 text-green-600" />
            <span className="font-medium text-green-800">Enrollment token created</span>
          </div>
          <p className="text-sm text-green-700 mb-2">
            ID: <strong>{newAgent.id}</strong>
          </p>
          <p className="text-sm text-green-700 mb-2">
            Enrollment token: <strong className="font-mono text-xs">{newAgent.token}</strong>
          </p>
          <p className="text-xs text-green-600">Save this reusable token—it won't be shown again.</p>
          <button
            onClick={copyToken}
            className="mt-2 px-3 py-1 bg-green-100 text-green-700 rounded text-xs flex items-center gap-1"
          >
            {copiedToken ? <CheckCircle className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
            {copiedToken ? 'Copied!' : 'Copy token'}
          </button>
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <RefreshCw className="w-8 h-8 text-gray-400 animate-spin" />
        </div>
      ) : agents.length === 0 ? (
        <p className="text-gray-500">No agents registered.</p>
      ) : (
        <div className="bg-white rounded-lg shadow overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Status</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">ID</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Hostname</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Version</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Addresses</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Location</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Provider</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Capabilities</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Last Seen</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-gray-700">Actions</th>
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
                  <td className="px-4 py-2 font-mono text-xs">{agent.id}</td>
                  <td className="px-4 py-2">{agent.hostname}</td>
                  <td className="px-4 py-2">{agent.version}</td>
                  <td className="px-4 py-2">
                    <div className="font-mono text-sm font-medium">{agent.primary_address || '—'}</div>
                    {agent.addresses.length > 1 && (
                      <details className="text-xs text-gray-500">
                        <summary className="cursor-pointer">{agent.addresses.length} detected addresses</summary>
                        <div className="mt-1 max-w-xs break-all">{agent.addresses.join(', ')}</div>
                      </details>
                    )}
                  </td>
                  <td className="px-4 py-2">
                    {[agent.location.city, agent.location.region, agent.location.country_code].filter(Boolean).join(', ') || '—'}
                    {agent.location.public_ip && <div className="font-mono text-xs text-gray-500">{agent.location.public_ip}</div>}
                  </td>
                  <td className="px-4 py-2">
                    <div>{agent.location.as_name || agent.location.isp || '—'}</div>
                    {agent.location.as_name && agent.location.isp && agent.location.as_name !== agent.location.isp && (
                      <div className="text-xs text-gray-500">{agent.location.isp}</div>
                    )}
                  </td>
                  <td className="px-4 py-2">{agent.capabilities.join(', ')}</td>
                  <td className="px-4 py-2">
                    {agent.last_seen ? new Date(agent.last_seen).toLocaleString() : 'Never'}
                  </td>
                  <td className="px-4 py-2">
                    <button
                      onClick={() => handleDelete(agent.id)}
                      className="text-red-600 hover:bg-red-50 p-1 rounded"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
            </table>
          </div>
          <div className="flex flex-col gap-3 border-t border-gray-100 px-4 py-3 text-sm text-gray-600 sm:flex-row sm:items-center sm:justify-between">
            <div>
              Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total} agents
            </div>
            <div className="flex items-center gap-3">
              <label className="flex items-center gap-2">
                Rows
                <select
                  value={pageSize}
                  onChange={event => {
                    setPageSize(Number(event.target.value))
                    setPage(1)
                  }}
                  className="rounded border border-gray-300 bg-white px-2 py-1"
                >
                  {[10, 20, 50, 100].map(size => (
                    <option key={size} value={size}>{size}</option>
                  ))}
                </select>
              </label>
              <span>Page {page} of {Math.max(totalPages, 1)}</span>
              <button
                type="button"
                onClick={() => setPage(current => current - 1)}
                disabled={page <= 1}
                aria-label="Previous page"
                className="rounded border border-gray-300 p-1.5 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <button
                type="button"
                onClick={() => setPage(current => current + 1)}
                disabled={totalPages === 0 || page >= totalPages}
                aria-label="Next page"
                className="rounded border border-gray-300 p-1.5 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Register Modal */}
      {showRegister && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
          <div className="bg-white rounded-lg p-6 w-96">
            <h2 className="text-xl font-bold mb-4">Create Enrollment Token</h2>
            <p className="text-sm text-gray-600">
              Registration creates a reusable enrollment token. You can use it on multiple machines; each hostname appears as a separate agent. Network addresses, public location, version, and capabilities are detected automatically.
            </p>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowRegister(false)}
                className="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={handleRegister}
                disabled={registerLoading}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {registerLoading ? 'Creating...' : 'Create Token'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
