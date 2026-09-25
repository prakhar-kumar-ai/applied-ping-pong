import { useState, useEffect } from 'react'
import { adminFetch } from './AdminDashboard'

interface Player { id: string; name: string }
interface Team { id: string; player1: Player; player2: Player }
interface Match {
  id: string
  team1: Team | null
  team2: Team | null
  team1_score: number | null
  team2_score: number | null
  winner_team_id: string | null
  status: string
  round: string
  match_order: number
  round_number: number
}

function teamName(t: Team | null) {
  if (!t) return 'TBD'
  return `${t.player1.name} & ${t.player2.name}`
}

export default function AdminBracket() {
  const [matches, setMatches] = useState<Match[]>([])
  const [loading, setLoading] = useState(true)
  const [scoreModal, setScoreModal] = useState<Match | null>(null)
  const [score1, setScore1] = useState('')
  const [score2, setScore2] = useState('')
  const [saving, setSaving] = useState(false)

  const fetchBracket = () =>
    fetch('/api/tournament/bracket')
      .then(r => r.json())
      .then(d => { setMatches(d.matches || []); setLoading(false) })

  useEffect(() => { fetchBracket() }, [])

  const openScoreModal = (m: Match) => {
    setScoreModal(m)
    setScore1(m.team1_score?.toString() ?? '')
    setScore2(m.team2_score?.toString() ?? '')
  }

  const submitScore = async () => {
    if (!scoreModal) return
    setSaving(true)
    const res = await adminFetch(`/api/admin/matches/${scoreModal.id}/score`, {
      method: 'POST',
      body: JSON.stringify({ team1_score: parseInt(score1), team2_score: parseInt(score2) }),
    })
    setSaving(false)
    if (res.ok) {
      setScoreModal(null)
      fetchBracket()
    } else {
      alert('Failed to save score')
    }
  }

  const rrMatches = matches.filter(m => m.round === 'rr')
  const rounds = [...new Set(rrMatches.map(m => m.round_number))].sort((a, b) => a - b)
  const sfMatches = matches.filter(m => m.round === 'sf').sort((a, b) => a.match_order - b.match_order)
  const finalMatch = matches.find(m => m.round === 'final')

  const MatchRow = ({ m }: { m: Match }) => (
    <div className={`flex items-center justify-between p-3 rounded-lg mb-2 border ${m.status === 'complete' ? 'bg-green-50 border-green-200' : 'bg-white border-gray-200'}`}>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">
          {teamName(m.team1)} <span className="text-gray-400">vs</span> {teamName(m.team2)}
        </p>
        {m.status === 'complete' && (
          <p className="text-xs text-green-700 font-bold mt-0.5">{m.team1_score} — {m.team2_score}</p>
        )}
      </div>
      <button
        onClick={() => openScoreModal(m)}
        disabled={!m.team1 || !m.team2}
        className="ml-3 px-3 py-1 text-xs font-medium bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-100 disabled:text-gray-400 text-white rounded-lg flex-shrink-0"
      >
        {m.status === 'complete' ? 'Edit' : 'Enter Score'}
      </button>
    </div>
  )

  return (
    <div>
      <h2 className="text-xl font-bold text-gray-900 mb-4">Bracket & Scores</h2>

      {loading && <p className="text-gray-400">Loading...</p>}

      {!loading && matches.length === 0 && (
        <div className="bg-white rounded-xl p-8 text-center text-gray-400 shadow">
          No bracket yet. Go to the Teams tab and generate the bracket first.
        </div>
      )}

      {rounds.length > 0 && (
        <div className="mb-6">
          <h3 className="font-semibold text-gray-700 mb-3">Round Robin</h3>
          {rounds.map(r => (
            <div key={r} className="mb-4">
              <p className="text-xs font-bold text-gray-400 uppercase mb-2">Round {r + 1}</p>
              {rrMatches.filter(m => m.round_number === r).map(m => <MatchRow key={m.id} m={m} />)}
            </div>
          ))}
        </div>
      )}

      {sfMatches.length > 0 && (
        <div className="mb-6">
          <h3 className="font-semibold text-gray-700 mb-3">Semifinals</h3>
          {sfMatches.map(m => <MatchRow key={m.id} m={m} />)}
        </div>
      )}

      {finalMatch && (
        <div className="mb-6">
          <h3 className="font-semibold text-gray-700 mb-3">🏆 Final</h3>
          <MatchRow m={finalMatch} />
        </div>
      )}

      {/* Score entry modal */}
      {scoreModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-xl p-6 w-full max-w-sm">
            <h3 className="font-bold text-gray-900 mb-1">Enter Score</h3>
            <p className="text-sm text-gray-500 mb-4">{teamName(scoreModal.team1)} vs {teamName(scoreModal.team2)}</p>
            <div className="space-y-3 mb-4">
              <div>
                <label className="text-xs font-medium text-gray-600 mb-1 block">{teamName(scoreModal.team1)}</label>
                <input
                  type="number"
                  min="0"
                  value={score1}
                  onChange={e => setScore1(e.target.value)}
                  className="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label className="text-xs font-medium text-gray-600 mb-1 block">{teamName(scoreModal.team2)}</label>
                <input
                  type="number"
                  min="0"
                  value={score2}
                  onChange={e => setScore2(e.target.value)}
                  className="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => setScoreModal(null)}
                className="flex-1 py-2 border rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={submitScore}
                disabled={saving || score1 === '' || score2 === ''}
                className="flex-1 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-200 text-white rounded-lg text-sm font-medium"
              >
                {saving ? 'Saving...' : 'Save Score'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
