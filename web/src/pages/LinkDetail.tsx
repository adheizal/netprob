import { useCallback, useEffect, useState } from 'react'
import { useParams, Link as RouterLink } from 'react-router-dom'
import { ChevronDown, ChevronLeft, ChevronUp, Settings } from 'lucide-react'
import { api } from '../api'
import type { LinkWithAgents, PingResult, MTRRun, DirectionSummary } from '../types'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Line } from 'react-chartjs-2'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

export default function LinkDetail() {
  const { linkId } = useParams<{ linkId: string }>()
  const [link, setLink] = useState<LinkWithAgents | null>(null)
  const [loading, setLoading] = useState(true)
  const [pingData, setPingData] = useState<Record<string, PingResult[]>>({})
  const [mtrData, setMtrData] = useState<Record<string, MTRRun[]>>({})
  const [expandedMTRID, setExpandedMTRID] = useState<string | null>(null)
  const [editingDirection, setEditingDirection] = useState<DirectionSummary | null>(null)
  const [editPingInterval, setEditPingInterval] = useState(5)
  const [editMtrInterval, setEditMtrInterval] = useState(120)
  const [editPingEnabled, setEditPingEnabled] = useState(true)
  const [editMtrEnabled, setEditMtrEnabled] = useState(true)
  const [savingSettings, setSavingSettings] = useState(false)
  const [settingsError, setSettingsError] = useState('')

  const loadData = useCallback(async () => {
      if (!linkId) return
      try {
        const links = await api.listLinks()
        const found = links.find(l => l.id === linkId)
        if (found) {
          setLink(found)
          // Load ping and MTR data for each direction
          const pingResults: Record<string, PingResult[]> = {}
          const mtrResults: Record<string, MTRRun[]> = {}
          for (const dir of found.directions) {
            try {
              pingResults[dir.id] = await api.getPingResults(linkId!, dir.id, 100)
              mtrResults[dir.id] = await api.getMTRRuns(linkId!, dir.id, 10)
            } catch (err) {
              console.error(`Failed to load data for direction ${dir.id}:`, err)
            }
          }
          setPingData(pingResults)
          setMtrData(mtrResults)
        }
      } catch (err) {
        console.error('Failed to load link:', err)
      } finally {
        setLoading(false)
      }
  }, [linkId])

  useEffect(() => {
    const initialLoad = window.setTimeout(() => void loadData(), 0)
    const interval = window.setInterval(() => void loadData(), 10_000)
    return () => {
      window.clearTimeout(initialLoad)
      window.clearInterval(interval)
    }
  }, [loadData])

  if (loading) return <div className="p-6">Loading...</div>
  if (!link) return <div className="p-6">Link not found</div>

  const formatRTT = (ms?: number) => ms !== undefined ? `${ms.toFixed(1)} ms` : 'N/A'
  const formatLoss = (pct?: number) => pct !== undefined ? `${pct.toFixed(1)}%` : 'N/A'

  function formatAgent(agent?: DirectionSummary['source_agent'], fallbackID?: string) {
    if (!agent) return fallbackID || 'Unknown'
    const hostname = agent.hostname || fallbackID || 'Unknown'
    return agent.primary_address ? `${hostname} (${agent.primary_address})` : hostname
  }

  function openDirectionSettings(direction: DirectionSummary) {
    setEditingDirection(direction)
    setEditPingInterval(direction.ping_interval_seconds)
    setEditMtrInterval(direction.mtr_interval_seconds)
    setEditPingEnabled(direction.ping_enabled)
    setEditMtrEnabled(direction.mtr_enabled)
    setSettingsError('')
  }

  async function saveDirectionSettings() {
    if (!linkId || !editingDirection || editPingInterval <= 0 || editMtrInterval <= 0) return
    setSavingSettings(true)
    setSettingsError('')
    try {
      await api.updateDirection(linkId, editingDirection.id, {
        ping_interval_seconds: editPingInterval,
        mtr_interval_seconds: editMtrInterval,
        ping_enabled: editPingEnabled,
        mtr_enabled: editMtrEnabled,
      })
      setLink(current => current ? {
        ...current,
        directions: current.directions.map(direction => direction.id === editingDirection.id ? {
          ...direction,
          ping_interval_seconds: editPingInterval,
          mtr_interval_seconds: editMtrInterval,
          ping_enabled: editPingEnabled,
          mtr_enabled: editMtrEnabled,
        } : direction),
      } : current)
      setEditingDirection(null)
    } catch (err) {
      setSettingsError(err instanceof Error ? err.message : 'Failed to update probe settings')
    } finally {
      setSavingSettings(false)
    }
  }

  function getPingChartData(directionId: string) {
    const results = pingData[directionId] || []
    const labels = results.map(r => new Date(r.timestamp).toLocaleTimeString())
    const avgData = results.map(r => r.avg_rtt_ms || 0)
    const lossData = results.map(r => r.packet_loss_percent || 0)
    return {
      labels: labels.reverse(),
      datasets: [
        {
          label: 'Avg RTT (ms)',
          data: avgData.reverse(),
          borderColor: 'rgb(59, 130, 246)',
          backgroundColor: 'rgba(59, 130, 246, 0.1)',
          fill: true,
          yAxisID: 'y',
        },
        {
          label: 'Packet Loss (%)',
          data: lossData.reverse(),
          borderColor: 'rgb(239, 68, 68)',
          backgroundColor: 'rgba(239, 68, 68, 0.1)',
          fill: true,
          yAxisID: 'y1',
        },
      ],
    }
  }

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    scales: {
      y: {
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        title: { display: true, text: 'RTT (ms)' },
      },
      y1: {
        type: 'linear' as const,
        display: true,
        position: 'right' as const,
        title: { display: true, text: 'Loss (%)' },
        grid: { drawOnChartArea: false },
      },
    },
  }

  return (
    <div className="p-6">
      <RouterLink to="/links" className="flex items-center gap-1 text-gray-600 hover:text-gray-800 mb-4">
        <ChevronLeft className="w-4 h-4" />
        Back to Links
      </RouterLink>

      <h1 className="text-2xl font-bold text-gray-800 mb-2">{link.name}</h1>
      {link.description && <p className="text-gray-500 mb-6">{link.description}</p>}
      <p className="text-xs text-gray-400 mb-6">Metrics and MTR history refresh automatically every 10 seconds.</p>

      {/* Direction summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
        {link.directions.map(dir => {
          const results = pingData[dir.id] || []
          const latest = results[0]
          const mtrRuns = mtrData[dir.id] || []
          const latestMTR = mtrRuns[0]
          return (
            <div key={dir.id} className="border-2 border-gray-200 rounded-lg p-4">
              <div className="flex justify-between items-center mb-3">
                <h3 className="font-semibold">
                  {dir.source_agent?.hostname || dir.source_agent_id} → {dir.dest_agent?.hostname || dir.destination_agent_id}
                </h3>
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => openDirectionSettings(dir)}
                    className="text-gray-500 hover:text-blue-600"
                    title="Edit probe settings"
                  >
                    <Settings className="w-4 h-4" />
                  </button>
                  <div className={`w-3 h-3 rounded-full ${dir.online ? 'bg-green-500' : 'bg-red-400'}`} />
                </div>
              </div>

              <div className="mb-3 text-xs text-gray-500">
                Ping: {dir.ping_enabled ? `every ${dir.ping_interval_seconds}s` : 'off'}
                {' | '}
                MTR: {dir.mtr_enabled ? `every ${dir.mtr_interval_seconds}s` : 'off'}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <span className="text-xs text-gray-500">Latency</span>
                  <div className="text-lg font-bold">{formatRTT(latest?.avg_rtt_ms)}</div>
                </div>
                <div>
                  <span className="text-xs text-gray-500">Packet Loss</span>
                  <div className="text-lg font-bold">{formatLoss(latest?.packet_loss_percent)}</div>
                </div>
                <div>
                  <span className="text-xs text-gray-500">Last MTR</span>
                  <div className="text-lg font-bold">
                    {latestMTR ? new Date(latestMTR.timestamp).toLocaleTimeString() : 'Never'}
                  </div>
                </div>
                <div>
                  <span className="text-xs text-gray-500">Packets</span>
                  <div className="text-lg font-bold">
                    {latest ? `${latest.packets_received}/${latest.packets_sent}` : 'N/A'}
                  </div>
                </div>
              </div>

              {latest && (
                <div className="mt-3 text-xs text-gray-500">
                  RTT range: {formatRTT(latest.min_rtt_ms)} - {formatRTT(latest.max_rtt_ms)}
                  {latest.jitter_ms !== undefined && ` | Jitter: ${latest.jitter_ms.toFixed(1)} ms`}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {editingDirection && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-10">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-1">Probe Settings</h2>
            <p className="text-sm text-gray-500 mb-4">
              {editingDirection.source_agent?.hostname || editingDirection.source_agent_id} → {editingDirection.dest_agent?.hostname || editingDirection.destination_agent_id}
            </p>
            <div className="grid grid-cols-2 gap-3 mb-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Ping interval (sec)</label>
                <input type="number" min="1" value={editPingInterval} onChange={(e) => setEditPingInterval(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">MTR interval (sec)</label>
                <input type="number" min="1" value={editMtrInterval} onChange={(e) => setEditMtrInterval(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />
              </div>
            </div>
            <div className="flex gap-6 text-sm text-gray-700 mb-4">
              <label className="flex items-center gap-2">
                <input type="checkbox" checked={editPingEnabled} onChange={(e) => setEditPingEnabled(e.target.checked)} />
                Enable ping
              </label>
              <label className="flex items-center gap-2">
                <input type="checkbox" checked={editMtrEnabled} onChange={(e) => setEditMtrEnabled(e.target.checked)} />
                Enable MTR
              </label>
            </div>
            {settingsError && <p className="text-sm text-red-600 mb-4">{settingsError}</p>}
            <div className="flex justify-end gap-3">
              <button onClick={() => setEditingDirection(null)} className="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg">Cancel</button>
              <button
                onClick={saveDirectionSettings}
                disabled={savingSettings || editPingInterval <= 0 || editMtrInterval <= 0}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {savingSettings ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Charts */}
      {link.directions.map(dir => {
        const results = pingData[dir.id] || []
        const dirLabel = `${dir.source_agent?.hostname || dir.source_agent_id} → ${dir.dest_agent?.hostname || dir.destination_agent_id}`
        return (
          <div key={dir.id} className="mb-12">
            <h2 className="text-xl font-semibold text-gray-800 mb-4">{dirLabel}</h2>
            {results.length > 0 ? (
              <div className="bg-white rounded-lg shadow p-4" style={{ height: '300px' }}>
                <Line data={getPingChartData(dir.id)} options={chartOptions} />
              </div>
            ) : (
              <p className="text-gray-500">No ping data available.</p>
            )}
          </div>
        )
      })}

      {/* MTR History */}
      <div className="mb-12">
        <h2 className="text-xl font-semibold text-gray-800 mb-4">MTR History</h2>
        {link.directions.map(dir => {
          const mtrRuns = mtrData[dir.id] || []
          const dirLabel = `${dir.source_agent?.hostname || dir.source_agent_id} → ${dir.dest_agent?.hostname || dir.destination_agent_id}`
          return (
            <div key={dir.id} className="mb-8">
              <h3 className="font-medium text-gray-700 mb-2">{dirLabel}</h3>
              {mtrRuns.length === 0 ? (
                <p className="text-sm text-gray-500">No MTR runs.</p>
              ) : (
                <div className="space-y-2">
                  {mtrRuns.map(run => {
                    const expanded = expandedMTRID === run.id
                    return (
                      <div key={run.id} className={`border rounded-lg ${run.status === 'error' ? 'border-red-200 bg-red-50' : 'border-gray-200'}`}>
                      <div className="flex justify-between items-center p-3">
                        <div>
                          <div className="text-sm text-gray-600">
                            {new Date(run.timestamp).toLocaleString()}
                          </div>
                          {run.status === 'error' && (
                            <div className="mt-1 text-sm text-red-700">MTR failed: {run.error || 'Unknown error'}</div>
                          )}
                        </div>
                        {run.status !== 'error' && (
                          <button
                            onClick={() => setExpandedMTRID(current => current === run.id ? null : run.id)}
                            className="flex items-center gap-1 text-sm text-blue-600 hover:text-blue-800"
                            aria-expanded={expanded}
                            aria-controls={`mtr-details-${run.id}`}
                          >
                            {expanded ? 'Hide details' : 'View details'}
                            {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                          </button>
                        )}
                      </div>

                      {expanded && (
                        <div id={`mtr-details-${run.id}`} className="border-t border-gray-200 p-3 sm:p-4">
                          <div className="mb-4 text-sm text-gray-600">
                            Source: {formatAgent(dir.source_agent, run.source_agent_id)}
                            <br />
                            Destination: {formatAgent(dir.dest_agent, run.destination_agent_id)}
                            <br />
                            Timestamp: {new Date(run.timestamp).toLocaleString()}
                          </div>

                          <div className="overflow-x-auto">
                            <table className="min-w-full table-auto border-collapse">
                              <thead className="bg-gray-50">
                                <tr>
                                  <th className="px-2 py-1 text-left text-xs font-medium text-gray-700">Hop</th>
                                  <th className="px-2 py-1 text-left text-xs font-medium text-gray-700">Host</th>
                                  <th className="px-2 py-1 text-left text-xs font-medium text-gray-700">IP</th>
                                  <th className="px-2 py-1 text-right text-xs font-medium text-gray-700">Loss %</th>
                                  <th className="px-2 py-1 text-right text-xs font-medium text-gray-700">Sent</th>
                                  <th className="px-2 py-1 text-right text-xs font-medium text-gray-700">Last (ms)</th>
                                  <th className="px-2 py-1 text-right text-xs font-medium text-gray-700">Avg (ms)</th>
                                  <th className="px-2 py-1 text-right text-xs font-medium text-gray-700">Best (ms)</th>
                                  <th className="px-2 py-1 text-right text-xs font-medium text-gray-700">Worst (ms)</th>
                                </tr>
                              </thead>
                              <tbody>
                                {run.hops.map(hop => (
                                  <tr key={hop.hop} className={hop.hop % 2 === 0 ? 'bg-gray-50' : ''}>
                                    <td className="px-2 py-1 text-sm">{hop.hop}</td>
                                    <td className="px-2 py-1 text-sm">{hop.host || ''}</td>
                                    <td className="px-2 py-1 text-sm font-mono">{hop.ip || ''}</td>
                                    <td className="px-2 py-1 text-sm text-right">{hop.loss_percent.toFixed(1)}</td>
                                    <td className="px-2 py-1 text-sm text-right">{hop.sent}</td>
                                    <td className="px-2 py-1 text-sm text-right">{hop.last_ms?.toFixed(1) ?? ''}</td>
                                    <td className="px-2 py-1 text-sm text-right">{hop.avg_ms?.toFixed(1) ?? ''}</td>
                                    <td className="px-2 py-1 text-sm text-right">{hop.best_ms?.toFixed(1) ?? ''}</td>
                                    <td className="px-2 py-1 text-sm text-right">{hop.worst_ms?.toFixed(1) ?? ''}</td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                        </div>
                      )}
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
