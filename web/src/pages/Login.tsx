import { useState, type FormEvent } from 'react'
import { Activity } from 'lucide-react'
import { api } from '../api'
import type { AuthSession } from '../types'

export default function Login({ onLogin }: { onLogin: (session: AuthSession) => void }) {
  const [email, setEmail] = useState('admin@netprob.local')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      onLogin(await api.login(email, password))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
      <form onSubmit={submit} className="w-full max-w-md bg-white border border-gray-200 rounded-xl shadow-sm p-8">
        <div className="flex items-center gap-3 mb-2">
          <Activity className="w-8 h-8 text-blue-600" />
          <h1 className="text-2xl font-bold text-gray-800">NetProb</h1>
        </div>
        <p className="text-gray-500 mb-6">Sign in to manage agents and network monitoring.</p>

        <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
        <input type="email" required autoComplete="username" value={email} onChange={event => setEmail(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg mb-4" />

        <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
        <input type="password" required autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg" />

        {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
        <button disabled={submitting} className="mt-6 w-full px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50">
          {submitting ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
