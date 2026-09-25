import { useState } from 'react'
import AdminRegistrations from './AdminRegistrations'
import AdminTeams from './AdminTeams'
import AdminBracket from './AdminBracket'

const TOKEN_KEY = 'pp_admin_token'

export function getAdminToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function adminFetch(url: string, options: RequestInit = {}) {
  return fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Token': getAdminToken(),
      ...(options.headers || {}),
    },
  })
}

type AdminTab = 'registrations' | 'teams' | 'bracket'

export default function AdminDashboard() {
  const [token, setToken] = useState(localStorage.getItem(TOKEN_KEY) || '')
  const [passphrase, setPassphrase] = useState('')
  const [authError, setAuthError] = useState('')
  const [authing, setAuthing] = useState(false)
  const [activeTab, setActiveTab] = useState<AdminTab>('registrations')

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setAuthing(true)
    setAuthError('')
    try {
      const res = await fetch('/api/admin/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Admin-Token': passphrase },
      })
      if (res.ok) {
        localStorage.setItem(TOKEN_KEY, passphrase)
        setToken(passphrase)
      } else {
        setAuthError('Wrong passphrase. Try again.')
      }
    } catch {
      setAuthError('Network error.')
    } finally {
      setAuthing(false)
    }
  }

  const handleLogout = () => {
    localStorage.removeItem(TOKEN_KEY)
    setToken('')
  }

  if (!token) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center p-4">
        <div className="bg-white rounded-2xl shadow-lg p-8 w-full max-w-sm">
          <div className="text-center mb-6">
            <div className="text-4xl mb-2">🔐</div>
            <h1 className="text-xl font-bold text-gray-900">Admin Login</h1>
            <p className="text-sm text-gray-500 mt-1">Ping Pong Tournament</p>
          </div>
          <form onSubmit={handleLogin} className="space-y-4">
            <input
              type="password"
              value={passphrase}
              onChange={e => setPassphrase(e.target.value)}
              placeholder="Admin passphrase"
              required
              className="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
            {authError && <p className="text-red-500 text-sm">{authError}</p>}
            <button
              type="submit"
              disabled={authing}
              className="w-full bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-300 text-white font-semibold py-2 rounded-lg transition-colors"
            >
              {authing ? 'Checking...' : 'Login'}
            </button>
          </form>
        </div>
      </div>
    )
  }

  const tabs: { id: AdminTab; label: string }[] = [
    { id: 'registrations', label: '📋 Registrations' },
    { id: 'teams', label: '👥 Teams' },
    { id: 'bracket', label: '🏆 Bracket & Scores' },
  ]

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-gray-900 text-white px-6 py-4 flex justify-between items-center">
        <div className="flex items-center gap-3">
          <span className="text-2xl">🏓</span>
          <h1 className="text-lg font-bold">Ping Pong Admin</h1>
        </div>
        <div className="flex items-center gap-4">
          <a href="/bracket" target="_blank" className="text-sm text-gray-300 hover:text-white underline">
            Public bracket ↗
          </a>
          <button onClick={handleLogout} className="text-sm text-gray-400 hover:text-white">
            Logout
          </button>
        </div>
      </header>

      <div className="border-b border-gray-200 bg-white px-6">
        <nav className="flex gap-1">
          {tabs.map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.id
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      <main className="p-6 max-w-5xl mx-auto">
        {activeTab === 'registrations' && <AdminRegistrations onTeamsCreated={() => setActiveTab('teams')} />}
        {activeTab === 'teams' && <AdminTeams onBracketGenerated={() => setActiveTab('bracket')} />}
        {activeTab === 'bracket' && <AdminBracket />}
      </main>
    </div>
  )
}
