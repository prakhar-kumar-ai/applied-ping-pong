import { useState, useEffect } from 'react'
import { adminFetch } from './AdminDashboard'

interface Player { id: string; name: string; email: string }
interface Team { id: string; player1: Player; player2: Player; locked: boolean }
interface Props { onBracketGenerated: () => void }

export default function AdminTeams({ onBracketGenerated }: Props) {
  const [teams, setTeams] = useState<Team[]>([])
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState(false)
  const [swapState, setSwapState] = useState<{ playerID: string; playerName: string } | null>(null)

  const fetchTeams = () =>
    adminFetch('/api/admin/teams')
      .then(r => r.json())
      .then(d => { setTeams(d.teams || []); setLoading(false) })

  useEffect(() => { fetchTeams() }, [])

  const handlePlayerClick = async (clickedPlayer: Player) => {
    if (!swapState) {
      setSwapState({ playerID: clickedPlayer.id, playerName: clickedPlayer.name })
      return
    }
    if (swapState.playerID === clickedPlayer.id) {
      setSwapState(null)
      return
    }
    const res = await adminFetch('/api/admin/teams/swap', {
      method: 'PUT',
      body: JSON.stringify({ player1_id: swapState.playerID, player2_id: clickedPlayer.id }),
    })
    if (res.ok) {
      setSwapState(null)
      fetchTeams()
    } else {
      alert('Swap failed')
      setSwapState(null)
    }
  }

  const generateBracket = async () => {
    if (!confirm(`Lock ${teams.length} teams and generate the bracket?`)) return
    setGenerating(true)
    const res = await adminFetch('/api/admin/bracket/generate', { method: 'POST' })
    const data = await res.json()
    setGenerating(false)
    if (res.ok) {
      alert(`Bracket generated! ${data.rr_matches} round-robin matches created.`)
      onBracketGenerated()
    } else {
      alert('Error: ' + (data.error || 'unknown'))
    }
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <div>
          <h2 className="text-xl font-bold text-gray-900">Teams</h2>
          <p className="text-sm text-gray-500">{teams.length} team{teams.length !== 1 ? 's' : ''}</p>
        </div>
        <button
          onClick={generateBracket}
          disabled={generating || teams.length < 2}
          className="px-4 py-2 bg-green-600 hover:bg-green-700 disabled:bg-green-200 text-white text-sm font-medium rounded-lg"
        >
          {generating ? 'Generating...' : '🏆 Lock Teams & Generate Bracket'}
        </button>
      </div>

      {swapState && (
        <div className="bg-indigo-50 border border-indigo-200 rounded-lg p-3 mb-4 text-sm text-indigo-800">
          🔄 Swapping <strong>{swapState.playerName}</strong> — click another player to swap with them. Click the same player to cancel.
        </div>
      )}

      {loading ? (
        <p className="text-gray-400">Loading...</p>
      ) : teams.length === 0 ? (
        <div className="bg-white rounded-xl p-8 text-center text-gray-400 shadow">
          No teams yet. Go to Registrations and randomize first.
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {teams.map((team, i) => (
            <div key={team.id} className="bg-white rounded-xl shadow p-4 border border-gray-100">
              <p className="text-xs font-bold text-gray-400 uppercase mb-3">Team {i + 1}</p>
              {[team.player1, team.player2].map(p => (
                <button
                  key={p.id}
                  onClick={() => handlePlayerClick(p)}
                  className={`w-full text-left px-3 py-2 rounded-lg mb-1 text-sm transition-colors ${
                    swapState?.playerID === p.id
                      ? 'bg-indigo-600 text-white'
                      : swapState
                      ? 'bg-yellow-50 border border-yellow-300 hover:bg-yellow-100'
                      : 'bg-gray-50 hover:bg-gray-100'
                  }`}
                >
                  <span className="font-medium">{p.name}</span>
                  <span className="text-xs block truncate" style={{ color: swapState?.playerID === p.id ? 'rgba(255,255,255,0.7)' : '#9ca3af' }}>{p.email}</span>
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
      <p className="text-xs text-gray-400 mt-4">💡 Click a player to select them, then click another player to swap their team positions.</p>
    </div>
  )
}
