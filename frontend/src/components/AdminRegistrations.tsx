import { useState, useEffect } from 'react'
import { adminFetch } from './AdminDashboard'

interface Player {
  id: string
  name: string
  email: string
  registered_at: string
}

interface Props {
  onTeamsCreated: () => void
}

export default function AdminRegistrations({ onTeamsCreated }: Props) {
  const [players, setPlayers] = useState<Player[]>([])
  const [registrationOpen, setRegistrationOpen] = useState(true)
  const [loading, setLoading] = useState(true)
  const [randomizing, setRandomizing] = useState(false)
  const [toggling, setToggling] = useState(false)

  const fetchData = async () => {
    const [playersRes, statusRes] = await Promise.all([
      adminFetch('/api/admin/players').then(r => r.json()),
      fetch('/api/tournament/status').then(r => r.json()),
    ])
    setPlayers(playersRes.players || [])
    setRegistrationOpen(statusRes.registration_open)
    setLoading(false)
  }

  useEffect(() => { fetchData() }, [])

  const toggleRegistration = async () => {
    setToggling(true)
    const endpoint = registrationOpen ? '/api/admin/registration/close' : '/api/admin/registration/open'
    await adminFetch(endpoint, { method: 'POST' })
    await fetchData()
    setToggling(false)
  }

  const randomizeTeams = async () => {
    if (!confirm(`Randomize ${players.length} players into teams of 2?`)) return
    setRandomizing(true)
    const res = await adminFetch('/api/admin/teams/randomize', { method: 'POST' })
    const data = await res.json()
    setRandomizing(false)
    if (res.ok) {
      alert(`Created ${data.teams_created} teams!${data.odd_player_id ? ' ⚠️ One player has no partner — add another player or adjust teams manually.' : ''}`)
      onTeamsCreated()
    } else {
      alert('Error: ' + (data.error || 'unknown'))
    }
  }

  const oddCount = players.length % 2 !== 0

  return (
    <div>
      <div className="flex flex-wrap justify-between items-start gap-3 mb-4">
        <div>
          <h2 className="text-xl font-bold text-gray-900">Registrations</h2>
          <p className="text-sm text-gray-500">{players.length} player{players.length !== 1 ? 's' : ''} signed up</p>
        </div>
        <div className="flex gap-3 flex-wrap">
          <button
            onClick={toggleRegistration}
            disabled={toggling}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              registrationOpen
                ? 'bg-red-100 text-red-700 hover:bg-red-200'
                : 'bg-green-100 text-green-700 hover:bg-green-200'
            }`}
          >
            {toggling ? '...' : registrationOpen ? '🔒 Close Registration' : '🔓 Open Registration'}
          </button>
          <button
            onClick={randomizeTeams}
            disabled={randomizing || registrationOpen || players.length < 2}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-200 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {randomizing ? 'Randomizing...' : '🎲 Randomize into Teams'}
          </button>
        </div>
      </div>

      {registrationOpen && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 mb-4 text-sm text-yellow-800">
          ⚠️ Registration is still <strong>open</strong>. Close it before randomizing teams.
        </div>
      )}

      {oddCount && !registrationOpen && (
        <div className="bg-orange-50 border border-orange-200 rounded-lg p-3 mb-4 text-sm text-orange-800">
          ⚠️ <strong>Odd number of players ({players.length})</strong> — one player will be left without a partner. Ask someone else to sign up or remove a player before randomizing.
        </div>
      )}

      {loading ? (
        <p className="text-gray-400">Loading...</p>
      ) : players.length === 0 ? (
        <div className="bg-white rounded-xl p-8 text-center text-gray-400 shadow">
          No registrations yet. Share the registration link!
        </div>
      ) : (
        <div className="bg-white rounded-xl shadow overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="text-left p-3 font-medium text-gray-600">#</th>
                <th className="text-left p-3 font-medium text-gray-600">Name</th>
                <th className="text-left p-3 font-medium text-gray-600">Email</th>
                <th className="text-left p-3 font-medium text-gray-600">Registered</th>
              </tr>
            </thead>
            <tbody>
              {players.map((p, i) => (
                <tr key={p.id} className="border-t border-gray-100 hover:bg-gray-50">
                  <td className="p-3 text-gray-400">{i + 1}</td>
                  <td className="p-3 font-medium">{p.name}</td>
                  <td className="p-3 text-gray-500">{p.email}</td>
                  <td className="p-3 text-gray-400">{new Date(p.registered_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
