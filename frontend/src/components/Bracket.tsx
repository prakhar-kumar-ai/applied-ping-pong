import { useState, useEffect } from 'react'

interface Player {
  id: string
  name: string
  email: string
}

interface Team {
  id: string
  player1: Player
  player2: Player
}

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

interface BracketData {
  tournament: { id: string; status: string } | null
  teams: Team[]
  matches: Match[]
}

function teamName(team: Team | null) {
  if (!team) return 'TBD'
  return `${team.player1.name} & ${team.player2.name}`
}

function RRStandings({ teams, matches }: { teams: Team[]; matches: Match[] }) {
  const rrMatches = matches.filter(m => m.round === 'rr' && m.status === 'complete')

  const stats: Record<string, { wins: number; losses: number; draws: number; pts: number; gf: number; ga: number }> = {}
  teams.forEach(t => { stats[t.id] = { wins: 0, losses: 0, draws: 0, pts: 0, gf: 0, ga: 0 } })

  rrMatches.forEach(m => {
    if (!m.team1 || !m.team2) return
    const s1 = m.team1_score ?? 0
    const s2 = m.team2_score ?? 0
    if (m.winner_team_id === m.team1.id) {
      stats[m.team1.id].wins++; stats[m.team1.id].pts += 2
      stats[m.team2.id].losses++
    } else if (m.winner_team_id === m.team2.id) {
      stats[m.team2.id].wins++; stats[m.team2.id].pts += 2
      stats[m.team1.id].losses++
    } else {
      stats[m.team1.id].draws++; stats[m.team1.id].pts += 1
      stats[m.team2.id].draws++; stats[m.team2.id].pts += 1
    }
    stats[m.team1.id].gf += s1; stats[m.team1.id].ga += s2
    stats[m.team2.id].gf += s2; stats[m.team2.id].ga += s1
  })

  const sorted = [...teams].sort((a, b) => {
    const diff = stats[b.id].pts - stats[a.id].pts
    if (diff !== 0) return diff
    return (stats[b.id].gf - stats[b.id].ga) - (stats[a.id].gf - stats[a.id].ga)
  })

  return (
    <div className="mb-8">
      <h2 className="text-lg font-bold text-gray-800 mb-3">Round Robin Standings</h2>
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="bg-gray-100">
              <th className="text-left p-2 border">#</th>
              <th className="text-left p-2 border">Team</th>
              <th className="p-2 border">W</th>
              <th className="p-2 border">D</th>
              <th className="p-2 border">L</th>
              <th className="p-2 border">Pts</th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((t, i) => (
              <tr key={t.id} className={i < 4 ? 'bg-green-50' : ''}>
                <td className="p-2 border text-gray-500">{i + 1}</td>
                <td className="p-2 border font-medium">{teamName(t)}</td>
                <td className="p-2 border text-center">{stats[t.id].wins}</td>
                <td className="p-2 border text-center">{stats[t.id].draws}</td>
                <td className="p-2 border text-center">{stats[t.id].losses}</td>
                <td className="p-2 border text-center font-bold">{stats[t.id].pts}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {teams.length >= 4 && <p className="text-xs text-green-700 mt-1">🟢 Top 4 advance to semifinals</p>}
    </div>
  )
}

function KnockoutBracket({ matches }: { matches: Match[] }) {
  const sfs = matches.filter(m => m.round === 'sf').sort((a, b) => a.match_order - b.match_order)
  const finals = matches.filter(m => m.round === 'final')
  const final = finals[0]

  const MatchBox = ({ match, label }: { match?: Match; label: string }) => (
    <div className="bg-white border-2 border-gray-200 rounded-lg p-3 w-56">
      <p className="text-xs font-bold text-gray-400 uppercase mb-2">{label}</p>
      <div className={`flex justify-between items-center py-1 px-2 rounded mb-1 ${match?.winner_team_id === match?.team1?.id ? 'bg-green-100' : ''}`}>
        <span className="text-sm font-medium truncate">{match?.team1 ? teamName(match.team1) : 'TBD'}</span>
        <span className="text-sm font-bold ml-2">{match?.team1_score ?? '-'}</span>
      </div>
      <div className={`flex justify-between items-center py-1 px-2 rounded ${match?.winner_team_id === match?.team2?.id ? 'bg-green-100' : ''}`}>
        <span className="text-sm font-medium truncate">{match?.team2 ? teamName(match.team2) : 'TBD'}</span>
        <span className="text-sm font-bold ml-2">{match?.team2_score ?? '-'}</span>
      </div>
    </div>
  )

  return (
    <div className="mb-8">
      <h2 className="text-lg font-bold text-gray-800 mb-4">Knockout Stage</h2>
      <div className="flex items-center gap-6 overflow-x-auto pb-4">
        <div className="flex flex-col gap-6">
          <MatchBox match={sfs[0]} label="Semifinal 1" />
          <MatchBox match={sfs[1]} label="Semifinal 2" />
        </div>
        <div className="text-gray-400 text-2xl">→</div>
        <div>
          <MatchBox match={final} label="🏆 Final" />
          {final?.status === 'complete' && final.winner_team_id && (
            <div className="mt-2 text-center text-green-700 font-bold text-sm">
              🥇 Champion: {teamName(final.winner_team_id === final.team1?.id ? final.team1 : final.team2)}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default function Bracket() {
  const [data, setData] = useState<BracketData | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchBracket = () => {
    fetch('/api/tournament/bracket')
      .then(r => r.json())
      .then(d => { setData(d); setLoading(false) })
      .catch(() => setLoading(false))
  }

  useEffect(() => {
    fetchBracket()
    const interval = setInterval(fetchBracket, 10000)
    return () => clearInterval(interval)
  }, [])

  const hasKnockout = data?.matches?.some(m => m.round === 'sf' || m.round === 'final')

  return (
    <div className="min-h-screen bg-gray-50 p-6">
      <div className="max-w-3xl mx-auto">
        <div className="flex items-center gap-3 mb-6">
          <span className="text-4xl">🏓</span>
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Applied Ping Pong Tournament</h1>
            <p className="text-sm text-gray-500">Live results • updates every 10 seconds</p>
          </div>
        </div>

        {loading && <p className="text-gray-400">Loading bracket...</p>}

        {!loading && !data?.tournament && (
          <div className="bg-white rounded-xl p-8 text-center text-gray-400 shadow">
            <div className="text-4xl mb-3">⏳</div>
            <p className="font-medium">Tournament hasn't started yet.</p>
            <p className="text-sm mt-1">Check back soon!</p>
          </div>
        )}

        {data?.tournament && (
          <>
            {data.teams.length > 0 && <RRStandings teams={data.teams} matches={data.matches} />}
            {hasKnockout && <KnockoutBracket matches={data.matches} />}
          </>
        )}
      </div>
    </div>
  )
}
