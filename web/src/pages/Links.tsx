import { useCallback, useEffect, useMemo, useState } from 'react'
import { CheckCircle, ChevronLeft, ChevronRight, Plus, RefreshCw, Search, Trash2, XCircle } from 'lucide-react'
import { Link as RouterLink, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import type { Agent, LinkStatus, LinkWithAgents } from '../types'

const statusStyles: Record<LinkStatus, string> = {
  healthy: 'bg-green-100 text-green-800',
  degraded: 'bg-amber-100 text-amber-800',
  down: 'bg-red-100 text-red-800',
  inactive: 'bg-gray-100 text-gray-600',
}

function positiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

function LinkSearch({ value, onCommit }: { value: string; onCommit: (value: string) => void }) {
  const [draft, setDraft] = useState(value)
  useEffect(() => {
    if (draft === value) return
    const timer = window.setTimeout(() => onCommit(draft.trim()), 350)
    return () => window.clearTimeout(timer)
  }, [draft, value, onCommit])
  return (
    <label className="relative xl:col-span-2">
      <Search className="absolute left-3 top-2.5 h-4 w-4 text-gray-400" />
      <input value={draft} onChange={event => setDraft(event.target.value)} placeholder="Search link, hostname, or target IP" className="w-full rounded border border-gray-300 py-2 pl-9 pr-3 text-sm" />
    </label>
  )
}

export default function Links() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = positiveInteger(searchParams.get('page'), 1)
  const pageSize = Math.min(100, positiveInteger(searchParams.get('page_size'), 20))
  const search = searchParams.get('search') || ''
  const status = (searchParams.get('status') || '') as LinkStatus | ''
  const agentId = searchParams.get('agent_id') || ''
  const sourceAgentId = searchParams.get('source_agent_id') || ''
  const destinationAgentId = searchParams.get('destination_agent_id') || ''
  const selectedAgentId = agentId || sourceAgentId || destinationAgentId
  const agentRole = sourceAgentId ? 'source' : destinationAgentId ? 'destination' : 'any'
  const sort = (searchParams.get('sort') || 'status') as 'status' | 'name' | 'latency' | 'loss' | 'updated'
  const order = (searchParams.get('order') || (sort === 'name' ? 'asc' : 'desc')) as 'asc' | 'desc'

  const [links, setLinks] = useState<LinkWithAgents[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [showCreate, setShowCreate] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [deletingLink, setDeletingLink] = useState('')

  const [filterAgentSearch, setFilterAgentSearch] = useState('')
  const [filterAgents, setFilterAgents] = useState<Agent[]>([])
  const [createAgentSearch, setCreateAgentSearch] = useState('')
  const [createAgents, setCreateAgents] = useState<Agent[]>([])

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [sourceAgent, setSourceAgent] = useState('')
  const [destAgent, setDestAgent] = useState('')
  const [targetAddress, setTargetAddress] = useState('')
  const [pingInterval, setPingInterval] = useState(5)
  const [mtrInterval, setMtrInterval] = useState(120)
  const [pingEnabled, setPingEnabled] = useState(true)
  const [mtrEnabled, setMtrEnabled] = useState(true)

  const updateQuery = useCallback((updates: Record<string, string | number | undefined>, resetPage = true) => {
    const next = new URLSearchParams(searchParams)
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') next.delete(key)
      else next.set(key, String(value))
    })
    if (resetPage && !Object.prototype.hasOwnProperty.call(updates, 'page')) next.delete('page')
    setSearchParams(next)
  }, [searchParams, setSearchParams])

  const updateSearch = useCallback((value: string) => updateQuery({ search: value }), [updateQuery])

  useEffect(() => {
    let active = true
    async function loadLinks(initial = false) {
      if (initial) setLoading(true)
      try {
        const data = await api.listLinksPage({ page, pageSize, search, status, agentId, sourceAgentId, destinationAgentId, sort, order })
        if (!active) return
        if (data.total > 0 && page > data.total_pages) {
          updateQuery({ page: data.total_pages }, false)
          return
        }
        setLinks(data.links)
        setTotal(data.total)
        setTotalPages(data.total_pages)
        setError('')
      } catch (err) {
        if (active) setError(err instanceof Error ? err.message : 'Failed to load links')
      } finally {
        if (active && initial) setLoading(false)
      }
    }
    loadLinks(true)
    const refresh = window.setInterval(() => loadLinks(), 30_000)
    return () => {
      active = false
      window.clearInterval(refresh)
    }
  }, [page, pageSize, search, status, agentId, sourceAgentId, destinationAgentId, sort, order, updateQuery])

  useEffect(() => {
    let active = true
    const timer = window.setTimeout(() => {
      api.listAgentsPage(1, 100, filterAgentSearch)
        .then(data => { if (active) setFilterAgents(data.agents) })
        .catch(err => console.error('Failed to find filter agents:', err))
    }, 250)
    return () => { active = false; window.clearTimeout(timer) }
  }, [filterAgentSearch])

  useEffect(() => {
    if (!showCreate) return
    let active = true
    const timer = window.setTimeout(() => {
      api.listAgentsPage(1, 100, createAgentSearch, 'online')
        .then(data => {
          if (!active) return
          setCreateAgents(current => {
            const selected = current.filter(agent => agent.id === sourceAgent || agent.id === destAgent)
            return Array.from(new Map([...selected, ...data.agents].map(agent => [agent.id, agent])).values())
          })
        })
        .catch(err => console.error('Failed to find online agents:', err))
    }, 250)
    return () => { active = false; window.clearTimeout(timer) }
  }, [showCreate, createAgentSearch, sourceAgent, destAgent])

  const selectedFilterAgent = useMemo(() => filterAgents.find(agent => agent.id === selectedAgentId), [filterAgents, selectedAgentId])

  async function reloadCurrentPage() {
    const data = await api.listLinksPage({ page, pageSize, search, status, agentId, sourceAgentId, destinationAgentId, sort, order })
    setLinks(data.links)
    setTotal(data.total)
    setTotalPages(data.total_pages)
  }

  async function handleCreate() {
    setCreateLoading(true)
    try {
      const link = await api.createLink({ name, description })
      const source = createAgents.find(agent => agent.id === sourceAgent)
      await api.createDirection(link.id, {
        source_agent_id: sourceAgent, destination_agent_id: destAgent, target_address: targetAddress,
        ping_interval_seconds: pingInterval, mtr_interval_seconds: mtrInterval,
        ping_enabled: pingEnabled, mtr_enabled: mtrEnabled,
      })
      await api.createDirection(link.id, {
        source_agent_id: destAgent, destination_agent_id: sourceAgent,
        target_address: source?.primary_address || source?.addresses[0] || '',
        ping_interval_seconds: pingInterval, mtr_interval_seconds: mtrInterval,
        ping_enabled: pingEnabled, mtr_enabled: mtrEnabled,
      })
      setShowCreate(false)
      setName('')
      setDescription('')
      setSourceAgent('')
      setDestAgent('')
      setTargetAddress('')
      setCreateAgentSearch('')
      if (page === 1) await reloadCurrentPage()
      else updateQuery({ page: 1 }, false)
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
      if (links.length === 1 && page > 1) updateQuery({ page: page - 1 }, false)
      else await reloadCurrentPage()
    } catch (err) {
      console.error('Failed to delete link:', err)
    } finally {
      setDeletingLink('')
    }
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div><h1 className="text-2xl font-bold text-gray-800">Links</h1><p className="mt-1 text-xs text-gray-400">Auto-refreshes every 30 seconds</p></div>
        <button onClick={() => setShowCreate(true)} className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-white hover:bg-blue-700">
          <Plus className="h-4 w-4" />Create Link
        </button>
      </div>

      <div className="mb-5 grid gap-3 rounded-lg bg-white p-4 shadow-sm md:grid-cols-2 xl:grid-cols-4">
        <LinkSearch key={search} value={search} onCommit={updateSearch} />
        <select value={status} onChange={event => updateQuery({ status: event.target.value })} className="rounded border border-gray-300 bg-white px-3 py-2 text-sm">
          <option value="">All statuses</option><option value="healthy">Healthy</option><option value="degraded">Degraded</option><option value="down">Down</option><option value="inactive">Inactive</option>
        </select>
        <select value={`${sort}:${order}`} onChange={event => { const [nextSort, nextOrder] = event.target.value.split(':'); updateQuery({ sort: nextSort, order: nextOrder }) }} className="rounded border border-gray-300 bg-white px-3 py-2 text-sm">
          <option value="status:desc">Problems first</option><option value="name:asc">Name A–Z</option><option value="name:desc">Name Z–A</option><option value="latency:desc">Highest latency</option><option value="loss:desc">Highest loss</option><option value="updated:desc">Recently updated</option>
        </select>
        <input value={filterAgentSearch} onChange={event => setFilterAgentSearch(event.target.value)} placeholder="Find agent for filter" className="rounded border border-gray-300 px-3 py-2 text-sm" />
        <select value={agentRole} onChange={event => {
          const role = event.target.value
          updateQuery({
            agent_id: role === 'any' ? selectedAgentId : undefined,
            source_agent_id: role === 'source' ? selectedAgentId : undefined,
            destination_agent_id: role === 'destination' ? selectedAgentId : undefined,
          })
        }} className="rounded border border-gray-300 bg-white px-3 py-2 text-sm">
          <option value="any">Agent in either direction</option><option value="source">Source agent only</option><option value="destination">Destination agent only</option>
        </select>
        <select value={selectedAgentId} onChange={event => updateQuery({
          agent_id: agentRole === 'any' ? event.target.value : undefined,
          source_agent_id: agentRole === 'source' ? event.target.value : undefined,
          destination_agent_id: agentRole === 'destination' ? event.target.value : undefined,
        })} className="rounded border border-gray-300 bg-white px-3 py-2 text-sm">
          <option value="">Any source/destination agent</option>
          {selectedAgentId && !selectedFilterAgent && <option value={selectedAgentId}>{selectedAgentId}</option>}
          {filterAgents.map(agent => <option key={agent.id} value={agent.id}>{agent.hostname} · {agent.primary_address || agent.id}</option>)}
        </select>
        {(search || status || selectedAgentId) && <button onClick={() => { setFilterAgentSearch(''); setSearchParams(new URLSearchParams()) }} className="rounded border border-gray-300 px-3 py-2 text-sm text-gray-600 hover:bg-gray-50">Clear filters</button>}
      </div>

      {error && <div className="mb-4 rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">{error}</div>}
      {loading ? (
        <div className="flex justify-center py-12"><RefreshCw className="h-8 w-8 animate-spin text-gray-400" /></div>
      ) : links.length === 0 ? (
        <p className="rounded-lg bg-white p-6 text-gray-500 shadow-sm">No links match the current filters.</p>
      ) : (
        <div className="space-y-4">
          {links.map(link => (
            <div key={link.id} className="rounded-lg bg-white p-4 shadow transition-shadow hover:shadow-md">
              <div className="flex items-start justify-between">
                <div><RouterLink to={`/links/${link.id}`} className="text-lg font-semibold text-gray-800 hover:text-blue-600">{link.name}</RouterLink>{link.description && <p className="text-sm text-gray-500">{link.description}</p>}</div>
                <div className="flex items-center gap-2">
                  {link.status && <span className={`rounded-full px-2 py-1 text-xs capitalize ${statusStyles[link.status]}`}>{link.status}</span>}
                  <button onClick={() => handleDelete(link.id, link.name)} disabled={deletingLink === link.id} className="p-1 text-gray-400 hover:text-red-600 disabled:opacity-50" title="Delete link"><Trash2 className="h-4 w-4" /></button>
                </div>
              </div>
              {link.directions.length > 0 && (
                <RouterLink to={`/links/${link.id}`} className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
                  {link.directions.map(direction => (
                    <div key={direction.id} className="rounded-lg border border-gray-100 p-3">
                      <div className="mb-1 flex items-center justify-between"><span className="text-sm font-medium">{direction.source_agent?.hostname || direction.source_agent_id} → {direction.dest_agent?.hostname || direction.destination_agent_id}</span>{direction.online ? <CheckCircle className="h-4 w-4 text-green-400" /> : <XCircle className="h-4 w-4 text-gray-400" />}</div>
                      {direction.latest_ping ? <div className="text-xs text-gray-600">Latency: {direction.latest_ping.avg_rtt_ms?.toFixed(1) ?? 'N/A'} ms | Loss: {direction.latest_ping.packet_loss_percent?.toFixed(1) ?? 'N/A'}%</div> : <div className="text-xs text-gray-400">No ping sample yet</div>}
                    </div>
                  ))}
                </RouterLink>
              )}
            </div>
          ))}
        </div>
      )}

      <div className="mt-5 flex flex-col gap-3 rounded-lg bg-white px-4 py-3 text-sm text-gray-600 shadow-sm sm:flex-row sm:items-center sm:justify-between">
        <div>{total === 0 ? 'Showing 0 links' : `Showing ${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, total)} of ${total} links`}</div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2">Rows<select value={pageSize} onChange={event => updateQuery({ page_size: event.target.value })} className="rounded border border-gray-300 bg-white px-2 py-1">{[10, 20, 50, 100].map(size => <option key={size} value={size}>{size}</option>)}</select></label>
          <span>Page {page} of {Math.max(totalPages, 1)}</span>
          <button onClick={() => updateQuery({ page: page - 1 }, false)} disabled={page <= 1} aria-label="Previous page" className="rounded border border-gray-300 p-1.5 hover:bg-gray-50 disabled:opacity-40"><ChevronLeft className="h-4 w-4" /></button>
          <button onClick={() => updateQuery({ page: page + 1 }, false)} disabled={totalPages === 0 || page >= totalPages} aria-label="Next page" className="rounded border border-gray-300 p-1.5 hover:bg-gray-50 disabled:opacity-40"><ChevronRight className="h-4 w-4" /></button>
        </div>
      </div>

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-lg bg-white p-6">
            <h2 className="mb-4 text-xl font-bold">Create Link</h2>
            <div className="space-y-4">
              <label className="block text-sm font-medium text-gray-700">Link Name<input value={name} onChange={event => setName(event.target.value)} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" placeholder="e.g. Jakarta ↔ Singapore" /></label>
              <label className="block text-sm font-medium text-gray-700">Description<input value={description} onChange={event => setDescription(event.target.value)} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" placeholder="Optional" /></label>
              <label className="block text-sm font-medium text-gray-700">Search online agents<input value={createAgentSearch} onChange={event => setCreateAgentSearch(event.target.value)} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" placeholder="Hostname or IP; up to 100 matches" /></label>
              <label className="block text-sm font-medium text-gray-700">Source Agent<select value={sourceAgent} onChange={event => setSourceAgent(event.target.value)} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2"><option value="">Select source agent</option>{createAgents.map(agent => <option key={agent.id} value={agent.id}>{agent.hostname} · {agent.primary_address || agent.id}</option>)}</select></label>
              <label className="block text-sm font-medium text-gray-700">Destination Agent<select value={destAgent} onChange={event => { setDestAgent(event.target.value); const agent = createAgents.find(candidate => candidate.id === event.target.value); setTargetAddress(agent?.primary_address || agent?.addresses[0] || '') }} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2"><option value="">Select destination agent</option>{createAgents.filter(agent => agent.id !== sourceAgent).map(agent => <option key={agent.id} value={agent.id}>{agent.hostname} · {agent.primary_address || agent.id}</option>)}</select></label>
              <label className="block text-sm font-medium text-gray-700">Target Address (Source → Dest)<input value={targetAddress} onChange={event => setTargetAddress(event.target.value)} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" /></label>
              <div className="grid grid-cols-2 gap-3"><label className="text-sm font-medium text-gray-700">Ping interval (sec)<input type="number" min="1" value={pingInterval} onChange={event => setPingInterval(Number(event.target.value))} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" /></label><label className="text-sm font-medium text-gray-700">MTR interval (sec)<input type="number" min="1" value={mtrInterval} onChange={event => setMtrInterval(Number(event.target.value))} className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2" /></label></div>
              <div className="flex gap-6 text-sm text-gray-700"><label className="flex items-center gap-2"><input type="checkbox" checked={pingEnabled} onChange={event => setPingEnabled(event.target.checked)} />Enable ping</label><label className="flex items-center gap-2"><input type="checkbox" checked={mtrEnabled} onChange={event => setMtrEnabled(event.target.checked)} />Enable MTR</label></div>
            </div>
            <div className="mt-6 flex justify-end gap-3"><button onClick={() => setShowCreate(false)} className="rounded-lg px-4 py-2 text-gray-600 hover:bg-gray-100">Cancel</button><button onClick={handleCreate} disabled={createLoading || !name || !sourceAgent || !destAgent || !targetAddress || pingInterval <= 0 || mtrInterval <= 0} className="rounded-lg bg-blue-600 px-4 py-2 text-white hover:bg-blue-700 disabled:opacity-50">{createLoading ? 'Creating...' : 'Create'}</button></div>
          </div>
        </div>
      )}
    </div>
  )
}
