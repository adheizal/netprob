import { useEffect, useState } from 'react'
import { Plus, CheckCircle, XCircle, Trash2 } from 'lucide-react'
import { Link as RouterLink } from 'react-router-dom'
import { api } from '../api'
import type { LinkWithAgents } from '../types'

export default function Links() {
  const [links, setLinks] = useState<LinkWithAgents[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [deletingLink, setDeletingLink] = useState('')

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')

  // Agent selection state
  const [agents, setAgents] = useState<any[]>([])
  const [sourceAgent, setSourceAgent] = useState('')
  const [destAgent, setDestAgent] = useState('')
  const [targetAddress, setTargetAddress] = useState('')
  const [pingInterval, setPingInterval] = useState(5)
  const [mtrInterval, setMtrInterval] = useState(120)
  const [pingEnabled, setPingEnabled] = useState(true)
  const [mtrEnabled, setMtrEnabled] = useState(true)

  async function loadData() {
    setLoading(true)
    try {
      const [linksData, agentsData] = await Promise.all([
        api.listLinks(),
        api.listAgents()
      ])
      setLinks(linksData)
      setAgents(agentsData)
    } catch (err) {
      console.error('Failed to load:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    let active = true
    Promise.all([api.listLinks(), api.listAgents()])
      .then(([linksData, agentsData]) => {
        if (!active) return
        setLinks(linksData)
        setAgents(agentsData)
      })
      .catch(err => console.error('Failed to load:', err))
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [])

  async function handleCreate() {
    setCreateLoading(true)
    try {
      const link = await api.createLink({ name, description })
      // Create both directions
      if (agents.length >= 2) {
        // Direction 1: source -> dest
        await api.createDirection(link.id, {
          source_agent_id: sourceAgent,
          destination_agent_id: destAgent,
          target_address: targetAddress,
          ping_interval_seconds: pingInterval,
          mtr_interval_seconds: mtrInterval,
          ping_enabled: pingEnabled,
          mtr_enabled: mtrEnabled,
        })
        // Direction 2: dest -> source
        const sourceAgentData = agents.find(a => a.id === sourceAgent)
        await api.createDirection(link.id, {
          source_agent_id: destAgent,
          destination_agent_id: sourceAgent,
          target_address: sourceAgentData?.primary_address || sourceAgentData?.addresses[0] || '',
          ping_interval_seconds: pingInterval,
          mtr_interval_seconds: mtrInterval,
          ping_enabled: pingEnabled,
          mtr_enabled: mtrEnabled,
        })
      }
      setShowCreate(false)
      setName('')
      setDescription('')
      setSourceAgent('')
      setDestAgent('')
      setTargetAddress('')
      loadData()
    } catch (err) {
      console.error('Failed to create link:', err)
    } finally {
      setCreateLoading(false)
    }
  }

  async function handleDelete(linkId: string, linkName: string) {
    if (!window.confirm(`Delete link "${linkName}" and all of its probe history?`)) return
    setDeletingLink(linkId)
    try {
      await api.deleteLink(linkId)
      setLinks(current => current.filter(link => link.id !== linkId))
    } catch (err) {
      console.error('Failed to delete link:', err)
    } finally {
      setDeletingLink('')
    }
  }


  const onlineAgents = agents.filter(a => a.online)

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold text-gray-800">Links</h1>
        <button
          onClick={() => { setShowCreate(true); loadData() }}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
          Create Link
        </button>
      </div>

      {loading ? (
        <p className="text-gray-500">Loading...</p>
      ) : links.length === 0 ? (
        <p className="text-gray-500">No links configured.</p>
      ) : (
        <div className="space-y-4">
          {links.map(link => (
              <div key={link.id} className="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow">
                <div className="flex justify-between items-start">
                  <div>
                    <RouterLink to={`/links/${link.id}`} className="text-lg font-semibold text-gray-800 hover:text-blue-600">
                      {link.name}
                    </RouterLink>
                    {link.description && <p className="text-sm text-gray-500">{link.description}</p>}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`px-2 py-1 text-xs rounded-full ${
                      link.directions.some(d => d.online) ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-500'
                    }`}>
                      {link.directions.some(d => d.online) ? 'Active' : 'Inactive'}
                    </span>
                    <button
                      onClick={() => handleDelete(link.id, link.name)}
                      disabled={deletingLink === link.id}
                      className="p-1 text-gray-400 hover:text-red-600 disabled:opacity-50"
                      title="Delete link"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {link.directions.length > 0 && (
                  <RouterLink to={`/links/${link.id}`} className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
                    {link.directions.map(dir => {
                      const isOnline = dir.online
                      const ping = dir.latest_ping
                      return (
                        <div key={dir.id} className="border border-gray-100 rounded-lg p-3">
                          <div className="flex justify-between items-center mb-1">
                            <span className="text-sm font-medium">
                              {dir.source_agent?.hostname || dir.source_agent_id} → {dir.dest_agent?.hostname || dir.destination_agent_id}
                            </span>
                            {isOnline ? (
                              <CheckCircle className="w-4 h-4 text-green-400" />
                            ) : (
                              <XCircle className="w-4 h-4 text-gray-400" />
                            )}
                          </div>
                          {ping && (
                            <div className="text-xs text-gray-600">
                              <span>Latency: {ping.avg_rtt_ms?.toFixed(1)} ms</span>
                              {' | '}
                              <span>Loss: {ping.packet_loss_percent?.toFixed(1)}%</span>
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </RouterLink>
                )}
              </div>
          ))}
        </div>
      )}

      {/* Create Link Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
          <div className="bg-white rounded-lg p-6 w-96">
            <h2 className="text-xl font-bold mb-4">Create Link</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Link Name</label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                  placeholder="e.g. Jakarta ↔ Singapore"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                  placeholder="Optional"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Source Agent</label>
                <select
                  value={sourceAgent}
                  onChange={(e) => {
                    setSourceAgent(e.target.value)
                  }}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                >
                  <option value="">Select source agent</option>
                  {onlineAgents.map(a => (
                    <option key={a.id} value={a.id}>{a.hostname}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Destination Agent</label>
                <select
                  value={destAgent}
                  onChange={(e) => {
                    setDestAgent(e.target.value)
                    const agent = agents.find(a => a.id === e.target.value)
                    setTargetAddress(agent?.primary_address || agent?.addresses[0] || '')
                  }}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                >
                  <option value="">Select destination agent</option>
                  {onlineAgents.filter(a => a.id !== sourceAgent).map(a => (
                    <option key={a.id} value={a.id}>{a.hostname}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Target Address (Source → Dest)</label>
                <input
                  type="text"
                  value={targetAddress}
                  onChange={(e) => setTargetAddress(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Ping interval (sec)</label>
                  <input
                    type="number"
                    min="1"
                    value={pingInterval}
                    onChange={(e) => setPingInterval(Number(e.target.value))}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">MTR interval (sec)</label>
                  <input
                    type="number"
                    min="1"
                    value={mtrInterval}
                    onChange={(e) => setMtrInterval(Number(e.target.value))}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                  />
                </div>
              </div>
              <div className="flex gap-6 text-sm text-gray-700">
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked={pingEnabled} onChange={(e) => setPingEnabled(e.target.checked)} />
                  Enable ping
                </label>
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked={mtrEnabled} onChange={(e) => setMtrEnabled(e.target.checked)} />
                  Enable MTR
                </label>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowCreate(false)}
                className="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={createLoading || !name || !sourceAgent || !destAgent || pingInterval <= 0 || mtrInterval <= 0}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {createLoading ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
