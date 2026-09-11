import { useEffect, useState } from 'react'
import { Outlet, Link, useLocation } from 'react-router-dom'
import { Activity, Server, Network, BarChart3, Settings, LogOut } from 'lucide-react'
import { api } from './api'
import type { AuthSession } from './types'
import Login from './pages/Login'
import RequiredPasswordChange from './pages/RequiredPasswordChange'

export default function App() {
  const location = useLocation()
  const [session, setSession] = useState<AuthSession | null>(null)
  const [authError, setAuthError] = useState('')
  const nav = [
    { path: '/', label: 'Overview', icon: BarChart3 },
    { path: '/agents', label: 'Agents', icon: Server },
    { path: '/links', label: 'Links', icon: Network },
    { path: '/settings', label: 'Settings', icon: Settings },
  ]

  useEffect(() => {
    api.getAuthSession()
      .then(setSession)
      .catch(err => setAuthError(err instanceof Error ? err.message : 'Failed to load authentication state'))
  }, [])

  async function logout() {
    await api.logout()
    setSession(current => current ? { ...current, authenticated: false } : current)
  }

  if (authError) return <div className="p-6 text-red-600">{authError}</div>
  if (!session) return <div className="p-6">Loading...</div>
  if (session.auth_enabled && !session.authenticated) return <Login onLogin={setSession} />
  if (session.auth_enabled && session.must_change_password) return <RequiredPasswordChange session={session} onChanged={setSession} />

  return (
    <div className="flex h-screen bg-gray-50">
      <nav className="w-64 bg-white border-r border-gray-200 flex flex-col">
        <div className="p-4 border-b border-gray-200">
          <h1 className="text-xl font-bold text-gray-800 flex items-center gap-2">
            <Activity className="w-6 h-6 text-blue-600" />
            NetProb
          </h1>
        </div>
        <div className="flex-1 p-2">
          {nav.map(({ path, label, icon: Icon }) => (
            <Link
              key={path}
              to={path}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                location.pathname === path
                  ? 'bg-blue-50 text-blue-700'
                  : 'text-gray-600 hover:bg-gray-100'
              }`}
            >
              <Icon className="w-4 h-4" />
              {label}
            </Link>
          ))}
        </div>
        {session.auth_enabled && session.authenticated && (
          <div className="p-2 border-t border-gray-200">
            <div className="px-3 py-2 text-xs text-gray-500 truncate">{session.email}</div>
            <button onClick={logout} className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100">
              <LogOut className="w-4 h-4" />
              Sign out
            </button>
          </div>
        )}
      </nav>
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  )
}
