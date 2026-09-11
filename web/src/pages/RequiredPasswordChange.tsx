import { useState, type FormEvent } from 'react'
import { api } from '../api'
import type { AuthSession } from '../types'

export default function RequiredPasswordChange({ session, onChanged }: { session: AuthSession; onChanged: (session: AuthSession) => void }) {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (newPassword !== confirmation) {
      setError('New passwords do not match')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      onChanged(await api.updateAdminAccount({
        email: session.email || 'admin@netprob.local',
        current_password: currentPassword,
        new_password: newPassword,
      }))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to change password')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
      <form onSubmit={submit} className="w-full max-w-md bg-white border border-gray-200 rounded-xl shadow-sm p-8">
        <h1 className="text-2xl font-bold text-gray-800 mb-2">Change temporary password</h1>
        <p className="text-gray-500 mb-6">You must replace the default password before using NetProb.</p>
        <label className="block text-sm font-medium text-gray-700 mb-1">Current password</label>
        <input type="password" required autoComplete="current-password" value={currentPassword} onChange={event => setCurrentPassword(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg mb-4" />
        <label className="block text-sm font-medium text-gray-700 mb-1">New password</label>
        <input type="password" required minLength={8} autoComplete="new-password" value={newPassword} onChange={event => setNewPassword(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg mb-4" />
        <label className="block text-sm font-medium text-gray-700 mb-1">Confirm new password</label>
        <input type="password" required minLength={8} autoComplete="new-password" value={confirmation} onChange={event => setConfirmation(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />
        {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
        <button disabled={submitting} className="mt-6 w-full px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50">
          {submitting ? 'Saving...' : 'Change password'}
        </button>
      </form>
    </div>
  )
}
