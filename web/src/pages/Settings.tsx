import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../api'
import type { AuthSession } from '../types'

export default function Settings() {
  const [pingDays, setPingDays] = useState(0)
  const [mtrDays, setMtrDays] = useState(0)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [session, setSession] = useState<AuthSession | null>(null)
  const [authEnabled, setAuthEnabled] = useState(true)
  const [savingSecurity, setSavingSecurity] = useState(false)
  const [email, setEmail] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')
  const [savingAccount, setSavingAccount] = useState(false)
  const [accountMessage, setAccountMessage] = useState('')
  const [accountError, setAccountError] = useState('')

  useEffect(() => {
    let active = true
    Promise.all([api.getRetentionSettings(), api.getAuthSession()])
      .then(([settings, authSession]) => {
        if (!active) return
        setPingDays(settings.ping_retention_days)
        setMtrDays(settings.mtr_retention_days)
        setSession(authSession)
        setAuthEnabled(authSession.auth_enabled)
        setEmail(authSession.email || '')
      })
      .catch(err => {
        if (active) setError(err instanceof Error ? err.message : 'Failed to load retention settings')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => { active = false }
  }, [])

  async function saveSecurity() {
    const warning = authEnabled
      ? 'Enable authentication? Users will need to sign in to access NetProb.'
      : 'Disable authentication? The dashboard, network data, and management API will become public.'
    if (!window.confirm(warning)) return
    setSavingSecurity(true)
    setError('')
    try {
      await api.updateSecuritySettings(authEnabled)
      window.location.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save security settings')
      setSavingSecurity(false)
    }
  }

  async function saveAccount(event: FormEvent) {
    event.preventDefault()
    setAccountError('')
    setAccountMessage('')
    if (newPassword !== passwordConfirmation) {
      setAccountError('New passwords do not match')
      return
    }
    setSavingAccount(true)
    try {
      const updated = await api.updateAdminAccount({
        email,
        current_password: currentPassword,
        new_password: newPassword,
      })
      setSession(updated)
      setCurrentPassword('')
      setNewPassword('')
      setPasswordConfirmation('')
      setAccountMessage('Admin email and password updated.')
    } catch (err) {
      setAccountError(err instanceof Error ? err.message : 'Failed to update admin account')
    } finally {
      setSavingAccount(false)
    }
  }

  async function saveRetention() {
    if (pingDays < 0 || mtrDays < 0) return
    if (!window.confirm('Save retention settings? Data older than the new limits will be deleted permanently.')) return

    setSaving(true)
    setError('')
    setMessage('')
    try {
      const result = await api.updateRetentionSettings({
        ping_retention_days: pingDays,
        mtr_retention_days: mtrDays,
      })
      setMessage(`Saved. Cleanup removed ${result.cleanup.ping_results_deleted} ping samples and ${result.cleanup.mtr_runs_deleted} MTR runs.`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save retention settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <div className="p-6">Loading...</div>

  return (
    <div className="p-6 max-w-3xl">
      <h1 className="text-2xl font-bold text-gray-800 mb-2">Settings</h1>
      <p className="text-gray-500 mb-6">Manage how long probe history is stored in the controller database.</p>

      <div className="bg-white border border-gray-200 rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold text-gray-800 mb-1">Authentication</h2>
        <p className="text-sm text-gray-500 mb-5">
          Require an admin login for the dashboard and management API. Agent WebSocket connections always use their enrollment token.
        </p>
        <label className="flex items-center gap-3 text-sm font-medium text-gray-700">
          <input type="checkbox" checked={authEnabled} onChange={event => setAuthEnabled(event.target.checked)} />
          Enable authentication
        </label>
        <div className="mt-5 flex justify-end">
          <button onClick={saveSecurity} disabled={savingSecurity || authEnabled === session?.auth_enabled} className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50">
            {savingSecurity ? 'Saving...' : 'Save authentication'}
          </button>
        </div>
      </div>

      {session?.authenticated && (
        <form onSubmit={saveAccount} className="bg-white border border-gray-200 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-800 mb-1">Admin account</h2>
          <p className="text-sm text-gray-500 mb-5">Change the administrator email and password. Existing sessions will be revoked.</p>
          <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
          <input type="email" required value={email} onChange={event => setEmail(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg mb-4" />
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Current password</label>
              <input type="password" required autoComplete="current-password" value={currentPassword} onChange={event => setCurrentPassword(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">New password</label>
              <input type="password" required minLength={8} autoComplete="new-password" value={newPassword} onChange={event => setNewPassword(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Confirm password</label>
              <input type="password" required minLength={8} autoComplete="new-password" value={passwordConfirmation} onChange={event => setPasswordConfirmation(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />
            </div>
          </div>
          {accountError && <p className="mt-4 text-sm text-red-600">{accountError}</p>}
          {accountMessage && <p className="mt-4 text-sm text-green-700">{accountMessage}</p>}
          <div className="mt-5 flex justify-end">
            <button disabled={savingAccount} className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50">
              {savingAccount ? 'Saving...' : 'Update account'}
            </button>
          </div>
        </form>
      )}

      <div className="bg-white border border-gray-200 rounded-lg p-6">
        <h2 className="text-lg font-semibold text-gray-800 mb-1">History retention</h2>
        <p className="text-sm text-gray-500 mb-5">
          Enter a number of days, or use 0 to keep that history forever. Cleanup runs immediately after saving and then every hour.
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Ping metrics (days)</label>
            <input
              type="number"
              min="0"
              step="1"
              value={pingDays}
              onChange={event => setPingDays(Number(event.target.value))}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">MTR history (days)</label>
            <input
              type="number"
              min="0"
              step="1"
              value={mtrDays}
              onChange={event => setMtrDays(Number(event.target.value))}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg"
            />
          </div>
        </div>

        {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
        {message && <p className="mt-4 text-sm text-green-700">{message}</p>}

        <div className="mt-6 flex justify-end">
          <button
            onClick={saveRetention}
            disabled={saving || pingDays < 0 || mtrDays < 0 || !Number.isInteger(pingDays) || !Number.isInteger(mtrDays)}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save retention'}
          </button>
        </div>
      </div>
    </div>
  )
}
