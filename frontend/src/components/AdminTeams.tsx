import { useState, useEffect } from 'react'
import { adminFetch } from './AdminDashboard'

interface Player { id: string; name: string; email: string }
interface Team { id: string; player1: Player; player2: Player; locked: boolean }
interface Props { onBracketGenerated: () => void }

interface EditState {
  team: Team
  name1: string
  email1: string
  name2: string
  email2: string
}

export default function AdminTeams({ onBracketGenerated }: Props) {
  const [teams, setTeams] = useState<Team[]>([])
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState(false)
  const [swapState, setSwapState] = useState<{ playerID: string; playerName: string } | null>(null)
  const [editState, setEditState] = useState<EditState | null>(null)
  const [saving, setSaving] = useState(false)

  const fetchTeams = () =>
    adminFetch('/api/admin/teams')
      .then(r => r.json())
      .then(d => { setTeams(d.teams || []); setLoading(false) })

  useEffect(() => { fetchTeams() }, [])

  // ---- Swap logic ----
  const handlePlayerClick = async (clickedPlayer: Player) => {
    if (editState) return
    if (!swapState) {
      setSwapState({ playerID: clickedPlayer.id, playerName: clickedPlayer.name })
      return
    }
    if (swapState.playerID === clickedPlayer.id) { setSwapState(null); return }
    const res = await adminFetch('/api/admin/teams/swap', {
      method: 'PUT',
      body: JSON.stringify({ player1_id: swapState.playerID, player2_id: clickedPlayer.id }),
    })
    setSwapState(null)
    if (res.ok) fetchTeams()
    else alert('Swap failed')
  }

  // ---- Edit logic ----
  const openEdit = (team: Team) => {
    setSwapState(null)
    setEditState({
      team,
      name1: team.player1.name,
      email1: team.player1.email,
      name2: team.player2.name,
      email2: team.player2.email,
    })
  }

  const saveEdit = async () => {
    if (!editState) return
    if (!editState.name1.trim() || !editState.name2.trim()) {
      alert('Both player names are required')
      return
    }
    setSaving(true)
    try {
      const [r1, r2] = await Promise.all([
        adminFetch(`/api/admin/players/${editState.team.player1.id}`, {
          method: 'PUT',
          body: JSON.stringify({ name: editState.name1.trim(), email: editState.email1.trim() }),
        }),
        adminFetch(`/api/admin/players/${editState.team.player2.id}`, {
          method: 'PUT',
          body: JSON.stringify({ name: editState.name2.trim(), email: editState.email2.trim() }),
        }),
      ])
      if (r1.ok && r2.ok) {
        setEditState(null)
        fetchTeams()
      } else {
        alert('Save failed — check the names and try again')
      }
    } finally {
      setSaving(false)
    }
  }

  // ---- Bracket generation ----
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
      {/* Edit modal */}
      {editState && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6">
            <h3 className="text-lg font-bold text-gray-900 mb-1">Edit Team Members</h3>
            <p className="text-sm text-gray-500 mb-5">
              Replace a player who can't make it. The live bracket updates instantly after saving.
            </p>

            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Player 1</label>
            <input
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm mb-1 focus:outline-none focus:ring-2 focus:ring-indigo-400"
              placeholder="Full name"
              value={editState.name1}
              onChange={e => setEditState(s => s ? { ...s, name1: e.target.value } : s)}
            />
            <input
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm mb-4 text-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-300"
              placeholder="Email (optional)"
              value={editState.email1}
              onChange={e => setEditState(s => s ? { ...s, email1: e.target.value } : s)}
            />

            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Player 2</label>
            <input
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm mb-1 focus:outline-none focus:ring-2 focus:ring-indigo-400"
              placeholder="Full name"
              value={editState.name2}
              onChange={e => setEditState(s => s ? { ...s, name2: e.target.value } : s)}
            />
            <input
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm mb-6 text-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-300"
              placeholder="Email (optional)"
              value={editState.email2}
              onChange={e => setEditState(s => s ? { ...s, email2: e.target.value } : s)}
            />

            <div className="flex gap-3">
              <button
                onClick={() => setEditState(null)}
                disabled={saving}
                className="flex-1 px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={saveEdit}
                disabled={saving}
                className="flex-1 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-300 text-white rounded-lg text-sm font-medium"
              >
                {saving ? 'Saving…' : 'Save Changes'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Header */}
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
          🔄 Swapping <strong>{swapState.playerName}</strong> — click another player to swap. Click same player to cancel.
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
              <div className="flex justify-between items-center mb-3">
                <p className="text-xs font-bold text-gray-400 uppercase">Team {i + 1}</p>
                <button
                  onClick={() => openEdit(team)}
                  className="text-xs px-2 py-1 rounded bg-gray-100 hover:bg-indigo-50 hover:text-indigo-600 text-gray-500 transition-colors"
                >
                  ✏️ Edit
                </button>
              </div>
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
                  <span
                    className="text-xs block truncate"
                    style={{ color: swapState?.playerID === p.id ? 'rgba(255,255,255,0.7)' : '#9ca3af' }}
                  >
                    {p.email}
                  </span>
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
      <p className="text-xs text-gray-400 mt-4">
        ✏️ <strong>Edit</strong> to replace a player &nbsp;·&nbsp; Click a player name to select for swapping between teams
      </p>
    </div>
  )
}
