package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerPageRoutes wires up the three tournament pages served as standalone HTML.
// These routes intercept BEFORE the embedded React app, so they take priority.
func registerPageRoutes(r *gin.Engine) {
	r.GET("/", serveRegisterPage)
	r.GET("/bracket", serveBracketPage)
	r.GET("/admin", serveAdminPage)
}

func serveRegisterPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(registerPageHTML))
}

func serveBracketPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(bracketPageHTML))
}

func serveAdminPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(adminPageHTML))
}

const registerPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>Applied Ping Pong Tournament</title>
<script src="https://cdn.tailwindcss.com"></script>
</head>
<body style="min-height:100vh;display:flex;align-items:center;justify-content:center;padding:1rem;position:relative;">
<div style="position:fixed;inset:0;z-index:0;background:url('/reg-bg.jpg') center/cover no-repeat;"></div>
<div style="position:fixed;inset:0;z-index:1;background:rgba(0,0,0,0.35);"></div>
<a href="/bracket" style="position:fixed;top:20px;right:20px;z-index:10;background:rgba(0,0,0,0.45);color:white;text-decoration:none;padding:8px 18px;border-radius:99px;font-size:0.8rem;font-weight:600;backdrop-filter:blur(6px);border:1px solid rgba(255,255,255,0.2);letter-spacing:0.02em;">🏆 View Live Bracket</a>
<div class="bg-white/90 backdrop-blur-sm rounded-2xl shadow-2xl p-8 w-full max-w-md" style="position:relative;z-index:2;">
  <div class="text-center mb-6">
    <div class="text-5xl mb-3">🏓</div>
    <h1 class="text-2xl font-bold text-gray-900">Applied Ping Pong Tournament</h1>
    <p class="text-gray-500 mt-1 text-sm">Doubles tournament — register to join!</p>
  </div>
  <div id="content">
    <div class="text-center text-gray-400 py-8">Loading...</div>
  </div>
</div>

<script>
const content = document.getElementById('content');

function showForm() {
  content.innerHTML = ` + "`" + `
    <form id="regForm" class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1">Full Name</label>
        <input id="name" type="text" placeholder="Jane Smith" required
          class="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-green-500"/>
      </div>
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1">Applied Email</label>
        <input id="email" type="email" placeholder="jane@applied.dev" required
          class="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-green-500"/>
      </div>
      <div id="errMsg" class="text-red-500 text-sm hidden"></div>
      <button type="submit" id="submitBtn"
        class="w-full bg-green-600 hover:bg-green-700 text-white font-semibold py-2 px-4 rounded-lg transition-colors">
        Register Me! 🎾
      </button>
    </form>
  ` + "`" + `;
  document.getElementById('regForm').addEventListener('submit', handleSubmit);
}

async function handleSubmit(e) {
  e.preventDefault();
  const name = document.getElementById('name').value.trim();
  const email = document.getElementById('email').value.trim();
  const btn = document.getElementById('submitBtn');
  const err = document.getElementById('errMsg');
  btn.textContent = 'Registering...';
  btn.disabled = true;
  err.classList.add('hidden');
  try {
    const res = await fetch('/api/register', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({name, email})
    });
    const data = await res.json();
    if (res.status === 201) {
      content.innerHTML = '<div class="text-center py-6"><div class="text-4xl mb-3">🎉</div><h2 class="text-xl font-bold text-gray-900">You\'re in!</h2><p class="text-gray-500 mt-2">We\'ll announce teams and match schedules soon. Stay tuned!</p><a href="/bracket" class="inline-block mt-4 px-4 py-2 bg-green-600 text-white text-sm font-semibold rounded-lg hover:bg-green-700">View Live Bracket →</a></div>';
    } else if (data.error === 'already_registered') {
      content.innerHTML = '<div class="text-center py-6"><div class="text-4xl mb-3">✅</div><h2 class="text-xl font-bold text-gray-900">Already registered!</h2><p class="text-gray-500 mt-2">You\'re already signed up. See you on the court!</p></div>';
    } else if (data.error === 'registration_closed') {
      showClosed();
    } else {
      err.textContent = data.error || 'Something went wrong. Please try again.';
      err.classList.remove('hidden');
      btn.textContent = 'Register Me! 🎾';
      btn.disabled = false;
    }
  } catch(ex) {
    err.textContent = 'Network error. Please try again.';
    err.classList.remove('hidden');
    btn.textContent = 'Register Me! 🎾';
    btn.disabled = false;
  }
}

function showClosed() {
  content.innerHTML = '<div class="text-center py-6"><div class="text-4xl mb-3">🔒</div><h2 class="text-xl font-bold text-gray-900">Registration is closed</h2><p class="text-gray-500 mt-2">Sign-ups have ended.</p><a href="/bracket" class="inline-block mt-4 px-4 py-2 bg-green-600 text-white text-sm font-semibold rounded-lg hover:bg-green-700">View Live Bracket →</a></div>';
}

fetch('/api/tournament/status').then(r=>r.json()).then(data => {
  if (!data.registration_open) { showClosed(); } else { showForm(); }
}).catch(() => showForm());
</script>
</body>
</html>`

const bracketPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>Applied Ping Pong Tournament</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    min-height: 100vh;
    background: #111;
    color: #fff;
    position: relative;
  }
  .overlay {
    min-height: 100vh;
    background: linear-gradient(to bottom, rgba(0,0,0,0.75) 0%, rgba(0,0,0,0.85) 100%);
    padding: 0 0 60px 0;
  }
  /* Header */
  .header {
    padding: 24px 32px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid rgba(255,255,255,0.1);
    backdrop-filter: blur(4px);
  }
  .header-left { display: flex; align-items: center; gap: 14px; }
  .header-icon { font-size: 2.4rem; }
  .header h1 { font-size: 1.5rem; font-weight: 800; letter-spacing: -0.02em; }
  .header p { font-size: 0.78rem; color: rgba(255,255,255,0.5); margin-top: 2px; }
  .live-badge {
    display: flex; align-items: center; gap: 7px;
    background: rgba(239,68,68,0.2); border: 1px solid rgba(239,68,68,0.4);
    border-radius: 99px; padding: 5px 12px; font-size: 0.75rem; font-weight: 700;
    color: #fca5a5; letter-spacing: 0.05em;
  }
  .live-dot {
    width: 7px; height: 7px; border-radius: 50%; background: #ef4444;
    animation: pulse 1.5s infinite;
  }
  @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.3} }

  /* Sections */
  .section { max-width: 900px; margin: 40px auto; padding: 0 24px; }
  .section-title {
    font-size: 0.7rem; font-weight: 700; letter-spacing: 0.12em;
    text-transform: uppercase; color: rgba(255,255,255,0.45);
    margin-bottom: 16px; display: flex; align-items: center; gap: 10px;
  }
  .section-title::after {
    content: ''; flex: 1; height: 1px; background: rgba(255,255,255,0.1);
  }

  /* Glass card */
  .glass {
    background: rgba(255,255,255,0.07);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(255,255,255,0.12);
    border-radius: 16px;
    overflow: hidden;
  }

  /* Standings */
  .standings-row {
    display: grid;
    grid-template-columns: 48px 1fr 48px 48px 48px 60px;
    align-items: center;
    padding: 14px 20px;
    border-bottom: 1px solid rgba(255,255,255,0.06);
    transition: background 0.15s;
  }
  .standings-row:last-child { border-bottom: none; }
  .standings-row:hover { background: rgba(255,255,255,0.05); }
  .standings-header {
    font-size: 0.7rem; font-weight: 700; letter-spacing: 0.08em;
    text-transform: uppercase; color: rgba(255,255,255,0.35);
    border-bottom: 1px solid rgba(255,255,255,0.1);
  }
  .rank-badge {
    width: 28px; height: 28px; border-radius: 50%;
    display: flex; align-items: center; justify-content: center;
    font-size: 0.8rem; font-weight: 800;
  }
  .rank-1 { background: rgba(251,191,36,0.25); color: #fbbf24; border: 1px solid rgba(251,191,36,0.4); }
  .rank-2 { background: rgba(148,163,184,0.2); color: #94a3b8; border: 1px solid rgba(148,163,184,0.3); }
  .rank-3 { background: rgba(180,120,76,0.2); color: #cd7f32; border: 1px solid rgba(180,120,76,0.3); }
  .rank-adv { background: rgba(34,197,94,0.15); color: #4ade80; border: 1px solid rgba(34,197,94,0.25); }
  .rank-other { background: rgba(255,255,255,0.05); color: rgba(255,255,255,0.4); }
  .team-name { font-size: 0.95rem; font-weight: 600; }
  .stat { text-align: center; font-size: 0.9rem; color: rgba(255,255,255,0.6); }
  .pts { text-align: center; font-size: 1rem; font-weight: 800; color: #fff; }
  .adv-pill {
    display: inline-block; font-size: 0.6rem; font-weight: 700;
    letter-spacing: 0.06em; text-transform: uppercase;
    background: rgba(34,197,94,0.2); color: #4ade80;
    border: 1px solid rgba(34,197,94,0.3); border-radius: 99px;
    padding: 2px 7px; margin-left: 8px; vertical-align: middle;
  }

  /* Bracket */
  .bracket-wrap {
    display: flex; align-items: center; gap: 0; overflow-x: auto; padding-bottom: 8px;
  }
  .bracket-col { display: flex; flex-direction: column; gap: 20px; }
  .bracket-connectors {
    display: flex; flex-direction: column; justify-content: space-around;
    align-items: center; padding: 0 4px; min-width: 52px; align-self: stretch;
  }
  .match-card {
    background: rgba(255,255,255,0.08);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(255,255,255,0.12);
    border-radius: 14px;
    overflow: hidden;
    min-width: 240px;
    max-width: 280px;
  }
  .match-card-header {
    padding: 8px 14px;
    font-size: 0.65rem; font-weight: 800; letter-spacing: 0.1em;
    text-transform: uppercase; color: rgba(255,255,255,0.4);
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }
  .match-card.final-card { border-color: rgba(251,191,36,0.35); }
  .match-card.final-card .match-card-header { color: #fbbf24; }
  .match-row {
    display: flex; justify-content: space-between; align-items: center;
    padding: 11px 14px; border-bottom: 1px solid rgba(255,255,255,0.06);
    font-size: 0.88rem;
  }
  .match-row:last-child { border-bottom: none; }
  .match-row.winner { background: rgba(34,197,94,0.15); }
  .match-row.final-winner { background: rgba(251,191,36,0.15); }
  .match-team { font-weight: 600; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .match-score { font-weight: 800; font-size: 1rem; margin-left: 12px; min-width: 24px; text-align: right; }
  .match-score.pending { color: rgba(255,255,255,0.25); }
  .tbd { color: rgba(255,255,255,0.3); font-style: italic; }

  /* Champion banner */
  .champ-banner {
    margin-top: 32px;
    background: linear-gradient(135deg, rgba(251,191,36,0.2), rgba(251,191,36,0.08));
    border: 1px solid rgba(251,191,36,0.4);
    border-radius: 16px;
    padding: 28px;
    text-align: center;
  }
  .champ-banner .trophy { font-size: 3rem; margin-bottom: 10px; }
  .champ-banner h2 { font-size: 1rem; font-weight: 700; color: #fbbf24; letter-spacing: 0.05em; text-transform: uppercase; margin-bottom: 6px; }
  .champ-banner p { font-size: 1.5rem; font-weight: 800; }

  /* Connector SVG */
  .connector { stroke: rgba(255,255,255,0.2); stroke-width: 1.5; fill: none; }

  /* Empty state */
  .empty {
    text-align: center; padding: 64px 32px;
    color: rgba(255,255,255,0.35);
  }
  .empty .icon { font-size: 3rem; margin-bottom: 12px; }
  .empty h3 { font-size: 1.1rem; font-weight: 700; color: rgba(255,255,255,0.6); margin-bottom: 6px; }
</style>
</head>
<body>
<!-- Grayscale background image layer -->
<div style="position:fixed;inset:0;z-index:0;background:url('/reg-bg.jpg') center/cover no-repeat;filter:grayscale(100%);"></div>
<div style="position:fixed;inset:0;z-index:1;background:rgba(0,0,0,0.72);"></div>

<div class="overlay" style="position:relative;z-index:2;">

  <!-- Header -->
  <div class="header">
    <div class="header-left">
      <div class="header-icon">🏓</div>
      <div>
        <h1>Applied Ping Pong Tournament</h1>
        <p>Doubles tournament • live results</p>
      </div>
    </div>
    <div class="live-badge"><div class="live-dot"></div>LIVE</div>
  </div>

  <div id="content" style="padding-top:8px;">
    <div class="empty"><div class="icon">⏳</div><h3>Loading...</h3></div>
  </div>

</div>

<script>
function teamName(t) {
  if (!t) return null;
  return t.player1.name + ' & ' + t.player2.name;
}

function rankBadgeClass(i, total) {
  if (i === 0) return 'rank-badge rank-1';
  if (i === 1) return 'rank-badge rank-2';
  if (i === 2) return 'rank-badge rank-3';
  if (i < 4) return 'rank-badge rank-adv';
  return 'rank-badge rank-other';
}

function rankLabel(i) {
  if (i === 0) return '🥇';
  if (i === 1) return '🥈';
  if (i === 2) return '🥉';
  return (i+1).toString();
}

function renderStandings(teams, matches) {
  const rrDone = matches.filter(m => m.round === 'rr' && m.status === 'complete');
  const stats = {};
  teams.forEach(t => { stats[t.id] = {w:0,d:0,l:0,pts:0,gf:0,ga:0}; });
  rrDone.forEach(m => {
    if (!m.team1 || !m.team2) return;
    const s1 = m.team1_score ?? 0, s2 = m.team2_score ?? 0;
    if (m.winner_team_id === m.team1.id) { stats[m.team1.id].w++; stats[m.team1.id].pts+=2; stats[m.team2.id].l++; }
    else if (m.winner_team_id === m.team2.id) { stats[m.team2.id].w++; stats[m.team2.id].pts+=2; stats[m.team1.id].l++; }
    else { stats[m.team1.id].d++; stats[m.team1.id].pts++; stats[m.team2.id].d++; stats[m.team2.id].pts++; }
    stats[m.team1.id].gf+=s1; stats[m.team1.id].ga+=s2;
    stats[m.team2.id].gf+=s2; stats[m.team2.id].ga+=s1;
  });
  const sorted = [...teams].sort((a,b) => {
    const dp = stats[b.id].pts - stats[a.id].pts;
    return dp !== 0 ? dp : (stats[b.id].gf - stats[b.id].ga) - (stats[a.id].gf - stats[a.id].ga);
  });
  const advCount = teams.length >= 4 ? 4 : teams.length;
  const rows = sorted.map((t, i) => ` + "`" + `
    <div class="standings-row">
      <div><span class="${rankBadgeClass(i, sorted.length)}">${rankLabel(i)}</span></div>
      <div class="team-name">${teamName(t) || '—'}${i < advCount ? '<span class="adv-pill">ADV</span>' : ''}</div>
      <div class="stat">${stats[t.id].w}</div>
      <div class="stat">${stats[t.id].d}</div>
      <div class="stat">${stats[t.id].l}</div>
      <div class="pts">${stats[t.id].pts}</div>
    </div>` + "`" + `).join('');

  return ` + "`" + `
    <div class="section">
      <div class="section-title">Group Stage</div>
      <div class="glass">
        <div class="standings-row standings-header">
          <div></div><div>Team</div>
          <div class="stat">W</div><div class="stat">D</div><div class="stat">L</div>
          <div class="pts">Pts</div>
        </div>
        ${rows}
      </div>
    </div>` + "`" + `;
}

function fmtTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleTimeString('en-US', {hour:'numeric',minute:'2-digit',hour12:true});
}

function matchCard(m, label, isFinal) {
  const cardClass = isFinal ? 'match-card final-card' : 'match-card';
  if (!m) return ` + "`" + `
    <div class="${cardClass}">
      <div class="match-card-header">${label}</div>
      <div class="match-row"><span class="match-team tbd">TBD</span><span class="match-score pending">—</span></div>
      <div class="match-row"><span class="match-team tbd">TBD</span><span class="match-score pending">—</span></div>
    </div>` + "`" + `;
  const w1 = m.winner_team_id && m.winner_team_id === m.team1?.id;
  const w2 = m.winner_team_id && m.winner_team_id === m.team2?.id;
  const winClass = isFinal ? 'final-winner' : 'winner';
  const n1 = teamName(m.team1) || 'TBD';
  const n2 = teamName(m.team2) || 'TBD';
  const hasScore = m.team1_score !== null && m.team2_score !== null;
  const timeStr = m.scheduled_time ? fmtTime(m.scheduled_time) : '';
  const tableStr = m.table_num ? ' · Tbl ' + m.table_num : '';
  const headerSuffix = timeStr ? ` + "`" + ` <span style="font-weight:400;opacity:0.55;font-size:0.62rem;">${timeStr}${tableStr}</span>` + "`" + ` : '';
  return ` + "`" + `
    <div class="${cardClass}">
      <div class="match-card-header">${label}${headerSuffix}</div>
      <div class="match-row ${w1 ? winClass : ''}">
        <span class="match-team">${n1}${w1 ? ' <span style="font-size:0.75rem">✓</span>' : ''}</span>
        <span class="match-score ${!hasScore ? 'pending' : ''}">${hasScore ? m.team1_score : '—'}</span>
      </div>
      <div class="match-row ${w2 ? winClass : ''}">
        <span class="match-team">${n2}${w2 ? ' <span style="font-size:0.75rem">✓</span>' : ''}</span>
        <span class="match-score ${!hasScore ? 'pending' : ''}">${hasScore ? m.team2_score : '—'}</span>
      </div>
    </div>` + "`" + `;
}

function renderKnockout(matches, knockoutSize) {
  const qfs = matches.filter(m => m.round === 'qf').sort((a,b) => a.match_order - b.match_order);
  const sfs = matches.filter(m => m.round === 'sf').sort((a,b) => a.match_order - b.match_order);
  const final = matches.find(m => m.round === 'final');

  let champHtml = '';
  if (final?.status === 'complete' && final.winner_team_id) {
    const champTeam = final.winner_team_id === final.team1?.id ? final.team1 : final.team2;
    champHtml = ` + "`" + `
      <div class="champ-banner">
        <div class="trophy">🏆</div>
        <h2>Tournament Champion</h2>
        <p>${teamName(champTeam)}</p>
      </div>` + "`" + `;
  }

  // SF + Final bracket
  const sfFinalHtml = ` + "`" + `
    <div class="bracket-wrap" style="align-items:center;">
      <div class="bracket-col">
        ${matchCard(sfs[0] || null, 'Semifinal 1', false)}
        ${matchCard(sfs[1] || null, 'Semifinal 2', false)}
      </div>
      <div style="display:flex;align-items:center;padding:0 16px;">
        <svg width="48" height="160" style="overflow:visible;">
          <path d="M0,40 H24 V120 H0" class="connector"/>
          <path d="M24,80 H48" class="connector"/>
        </svg>
      </div>
      <div>${matchCard(final || null, '🏆 Final', true)}</div>
    </div>` + "`" + `;

  if (knockoutSize >= 8 && qfs.length > 0) {
    // QF + SF + Final layout
    const qfHtml = ` + "`" + `
      <div class="bracket-wrap" style="align-items:center;margin-bottom:32px;">
        <div class="bracket-col">
          ${matchCard(qfs[0] || null, 'Quarter-final 1', false)}
          ${matchCard(qfs[1] || null, 'Quarter-final 2', false)}
        </div>
        <div style="display:flex;align-items:center;padding:0 12px;">
          <svg width="40" height="160" style="overflow:visible;">
            <path d="M0,40 H20 V120 H0" class="connector"/>
            <path d="M20,80 H40" class="connector"/>
          </svg>
        </div>
        <div class="bracket-col">
          ${matchCard(qfs[2] || null, 'Quarter-final 3', false)}
          ${matchCard(qfs[3] || null, 'Quarter-final 4', false)}
        </div>
      </div>` + "`" + `;
    return ` + "`" + `
      <div class="section">
        <div class="section-title">Knockout Stage</div>
        <div class="section-title" style="margin-bottom:12px;margin-top:0;">Quarter-finals</div>
        ${qfHtml}
        <div class="section-title" style="margin-bottom:12px;">Semi-finals & Final</div>
        ${sfFinalHtml}
        ${champHtml}
      </div>` + "`" + `;
  }

  return ` + "`" + `
    <div class="section">
      <div class="section-title">Knockout Stage</div>
      ${sfFinalHtml}
      ${champHtml}
    </div>` + "`" + `;
}

function renderGroupStage(data) {
  const groups = data.groups || [];
  const matches = data.matches || [];
  const numGroups = data.tournament?.num_groups || 1;
  const groupNames = ['A','B','C','D','E','F','G','H'];

  function groupStats(groupTeams, groupIdx) {
    const stats = {};
    groupTeams.forEach(t => { stats[t.id] = {w:0,d:0,l:0,pts:0,gf:0,ga:0}; });
    matches.filter(m => m.round === 'rr' && m.group_number === groupIdx && m.status === 'complete').forEach(m => {
      if (!m.team1 || !m.team2) return;
      const s1 = m.team1_score ?? 0, s2 = m.team2_score ?? 0;
      if (m.winner_team_id === m.team1.id) { stats[m.team1.id].w++; stats[m.team1.id].pts+=2; stats[m.team2.id].l++; }
      else if (m.winner_team_id === m.team2.id) { stats[m.team2.id].w++; stats[m.team2.id].pts+=2; stats[m.team1.id].l++; }
      else { stats[m.team1.id].d++; stats[m.team1.id].pts++; stats[m.team2.id].d++; stats[m.team2.id].pts++; }
      stats[m.team1.id].gf+=s1; stats[m.team1.id].ga+=s2;
      stats[m.team2.id].gf+=s2; stats[m.team2.id].ga+=s1;
    });
    return stats;
  }

  function renderOneGroup(groupTeams, groupIdx) {
    const stats = groupStats(groupTeams, groupIdx);
    const sorted = [...groupTeams].sort((a,b) => {
      const dp = stats[b.id].pts - stats[a.id].pts;
      return dp !== 0 ? dp : (stats[b.id].gf-stats[b.id].ga) - (stats[a.id].gf-stats[a.id].ga);
    });
    const rows = sorted.map((t,i) => ` + "`" + `
      <div style="display:grid;grid-template-columns:40px 1fr 36px 36px 36px 52px;align-items:center;padding:12px 16px;border-bottom:1px solid rgba(255,255,255,0.06);transition:background 0.15s;" onmouseover="this.style.background='rgba(255,255,255,0.05)'" onmouseout="this.style.background=''">
        <div><span class="${rankBadgeClass(i, sorted.length)}">${rankLabel(i)}</span></div>
        <div style="font-size:0.9rem;font-weight:600;padding-right:8px;">${teamName(t) || '—'}</div>
        <div class="stat">${stats[t.id].w}</div>
        <div class="stat">${stats[t.id].d}</div>
        <div class="stat">${stats[t.id].l}</div>
        <div class="pts">${stats[t.id].pts}</div>
      </div>` + "`" + `).join('');
    return ` + "`" + `
      <div class="glass">
        <div style="padding:10px 16px 6px;font-size:0.7rem;font-weight:800;letter-spacing:0.1em;text-transform:uppercase;color:rgba(255,255,255,0.5);border-bottom:1px solid rgba(255,255,255,0.08);">Group ${groupNames[groupIdx]}</div>
        <div style="display:grid;grid-template-columns:40px 1fr 36px 36px 36px 52px;align-items:center;padding:8px 16px;border-bottom:1px solid rgba(255,255,255,0.1);">
          <div></div>
          <div style="font-size:0.68rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:rgba(255,255,255,0.35);">Team</div>
          <div class="stat" style="font-size:0.68rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:rgba(255,255,255,0.35);">W</div>
          <div class="stat" style="font-size:0.68rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:rgba(255,255,255,0.35);">D</div>
          <div class="stat" style="font-size:0.68rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:rgba(255,255,255,0.35);">L</div>
          <div class="pts" style="font-size:0.68rem;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:rgba(255,255,255,0.35);">Pts</div>
        </div>
        ${rows}
      </div>` + "`" + `;
  }

  const groupCards = groups.map((gt, gi) => renderOneGroup(gt, gi)).join('');
  return ` + "`" + `
    <div class="section">
      <div class="section-title">Group Stage</div>
      <div style="display:grid;grid-template-columns:repeat(2,1fr);gap:16px;">${groupCards}</div>
    </div>` + "`" + `;
}

function playerInitials(name) {
  return (name || '?').split(' ').map(w => w[0]).slice(0,2).join('').toUpperCase();
}

const avatarColors = ['#4f46e5','#0891b2','#059669','#d97706','#7c3aed','#db2777','#2563eb','#0f766e','#b45309','#be123c'];

function renderPlayersPreview(players) {
  const cards = players.map((p, i) => ` + "`" + `
    <div style="background:rgba(255,255,255,0.07);border:1px solid rgba(255,255,255,0.11);border-radius:14px;padding:14px 16px;display:flex;align-items:center;gap:13px;">
      <div style="width:40px;height:40px;border-radius:50%;background:${avatarColors[i % avatarColors.length]};display:flex;align-items:center;justify-content:center;font-weight:800;font-size:0.85rem;flex-shrink:0;letter-spacing:0.02em;">
        ${playerInitials(p.name)}
      </div>
      <span style="font-weight:600;font-size:0.95rem;">${p.name}</span>
    </div>` + "`" + `).join('');
  return ` + "`" + `
    <div class="section">
      <div class="section-title">Who's Playing — ${players.length} player${players.length!==1?'s':''} registered</div>
      <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(210px,1fr));gap:10px;margin-bottom:24px;">${cards}</div>
      <div style="background:rgba(251,191,36,0.08);border:1px solid rgba(251,191,36,0.2);border-radius:14px;padding:18px 22px;display:flex;align-items:center;gap:14px;">
        <span style="font-size:1.8rem;">⏳</span>
        <div>
          <p style="font-weight:700;color:#fbbf24;font-size:0.9rem;margin:0 0 3px;">Teams & bracket coming soon</p>
          <p style="font-size:0.78rem;color:rgba(255,255,255,0.4);margin:0;">Check back once registration closes and teams are announced.</p>
        </div>
      </div>
    </div>` + "`" + `;
}

function renderTeamsPreview(teams) {
  const cards = teams.map((t, i) => {
    const p1init = playerInitials(t.player1?.name || '?');
    const p2init = playerInitials(t.player2?.name || '?');
    const c1 = avatarColors[i*2 % avatarColors.length];
    const c2 = avatarColors[(i*2+1) % avatarColors.length];
    return ` + "`" + `
      <div style="background:rgba(255,255,255,0.07);border:1px solid rgba(255,255,255,0.12);border-radius:16px;padding:18px 20px;">
        <p style="font-size:0.62rem;font-weight:800;letter-spacing:0.12em;text-transform:uppercase;color:rgba(255,255,255,0.35);margin:0 0 14px;">Team ${i+1}</p>
        <div style="display:flex;flex-direction:column;gap:10px;">
          <div style="display:flex;align-items:center;gap:11px;">
            <div style="width:34px;height:34px;border-radius:50%;background:${c1};display:flex;align-items:center;justify-content:center;font-weight:800;font-size:0.75rem;flex-shrink:0;">${p1init}</div>
            <span style="font-weight:600;font-size:0.92rem;">${t.player1?.name || '—'}</span>
          </div>
          <div style="display:flex;align-items:center;gap:11px;padding-left:1px;">
            <div style="width:34px;height:34px;border-radius:50%;background:${c2};display:flex;align-items:center;justify-content:center;font-weight:800;font-size:0.75rem;flex-shrink:0;">${p2init}</div>
            <span style="font-weight:600;font-size:0.92rem;">${t.player2?.name || '—'}</span>
          </div>
        </div>
      </div>` + "`" + `;
  }).join('');
  return ` + "`" + `
    <div class="section">
      <div class="section-title">Teams — ${teams.length} team${teams.length!==1?'s':''}</div>
      <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px;margin-bottom:24px;">${cards}</div>
      <div style="background:rgba(251,191,36,0.08);border:1px solid rgba(251,191,36,0.2);border-radius:14px;padding:18px 22px;display:flex;align-items:center;gap:14px;">
        <span style="font-size:1.8rem;">⏳</span>
        <div>
          <p style="font-weight:700;color:#fbbf24;font-size:0.9rem;margin:0 0 3px;">Bracket coming soon</p>
          <p style="font-size:0.78rem;color:rgba(255,255,255,0.4);margin:0;">Teams are locked in — the bracket will appear once the tournament starts.</p>
        </div>
      </div>
    </div>` + "`" + `;
}

async function fetchAndRender() {
  try {
    const res = await fetch('/api/tournament/bracket');
    const data = await res.json();
    const el = document.getElementById('content');
    if (!data.tournament) {
      const teams = data.teams || [];
      const players = data.players || [];
      if (teams.length > 0) {
        el.innerHTML = renderTeamsPreview(teams);
      } else if (players.length > 0) {
        el.innerHTML = renderPlayersPreview(players);
      } else {
        el.innerHTML = ` + "`" + `<div class="empty">
          <div class="icon">🏓</div>
          <h3>Registration is open</h3>
          <p>No one has signed up yet — share the registration link to get started!</p>
        </div>` + "`" + `;
      }
      return;
    }
    const matches = data.matches || [];
    const hasRR = matches.some(m => m.round === 'rr');
    const hasKnockout = matches.some(m => m.round === 'sf' || m.round === 'final' || m.round === 'qf');
    const knockoutSize = data.tournament?.knockout_size || 4;
    el.innerHTML = (hasRR ? renderGroupStage(data) : '') +
                   (hasKnockout ? renderKnockout(matches, knockoutSize) : '');
  } catch(e) {
    document.getElementById('content').innerHTML = ` + "`" + `<div class="empty"><p>Failed to load — retrying...</p></div>` + "`" + `;
  }
}

fetchAndRender();
setInterval(fetchAndRender, 10000);
</script>
</body>
</html>`

const adminPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>Ping Pong Admin</title>
<script src="https://cdn.tailwindcss.com"></script>
</head>
<body style="min-height:100vh;background:url('/bg.jpg') center/cover no-repeat fixed;position:relative;">

<!-- Login screen -->
<div id="loginScreen" style="min-height:100vh;background:rgba(0,0,0,0.6);" class="flex items-center justify-center p-4">
  <div class="bg-white rounded-2xl shadow-lg p-8 w-full max-w-sm">
    <div class="text-center mb-6">
      <div class="text-4xl mb-2">🔐</div>
      <h1 class="text-xl font-bold text-gray-900">Admin Login</h1>
      <p class="text-sm text-gray-500 mt-1">Ping Pong Tournament</p>
    </div>
    <form id="loginForm" class="space-y-4">
      <input id="passphrase" type="password" placeholder="Admin passphrase" required
        class="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500"/>
      <div id="loginErr" class="text-red-500 text-sm hidden"></div>
      <button type="submit" id="loginBtn"
        class="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-semibold py-2 rounded-lg transition-colors">
        Login
      </button>
    </form>
  </div>
</div>

<!-- Dashboard (hidden until logged in) -->
<div id="dashboard" class="hidden">
  <header class="bg-gray-900 text-white px-6 py-4 flex justify-between items-center">
    <div class="flex items-center gap-3">
      <span class="text-2xl">🏓</span>
      <h1 class="text-lg font-bold">Ping Pong Admin</h1>
    </div>
    <div class="flex items-center gap-4">
      <a href="/bracket" target="_blank" class="text-sm text-gray-300 hover:text-white underline">Public bracket ↗</a>
      <button onclick="resetTournament()" class="text-sm bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded-lg">🗑 Reset Tournament</button>
      <button onclick="logout()" class="text-sm text-gray-400 hover:text-white">Logout</button>
    </div>
  </header>

  <div class="border-b border-gray-200 bg-white px-6">
    <nav class="flex gap-1">
      <button onclick="showTab('reg')" id="tab-reg"
        class="px-4 py-3 text-sm font-medium border-b-2 border-indigo-600 text-indigo-600">📋 Registrations</button>
      <button onclick="showTab('teams')" id="tab-teams"
        class="px-4 py-3 text-sm font-medium border-b-2 border-transparent text-gray-500 hover:text-gray-700">👥 Teams</button>
      <button onclick="showTab('bracket')" id="tab-bracket"
        class="px-4 py-3 text-sm font-medium border-b-2 border-transparent text-gray-500 hover:text-gray-700">🏆 Bracket & Scores</button>
    </nav>
  </div>

  <main class="p-6 max-w-5xl mx-auto">
    <div id="tab-content-reg"></div>
    <div id="tab-content-teams" class="hidden"></div>
    <div id="tab-content-bracket" class="hidden"></div>
  </main>
</div>

<!-- Score modal -->
<div id="scoreModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 hidden">
  <div class="bg-white rounded-2xl shadow-xl p-6 w-full max-w-md">
    <h3 class="font-bold text-gray-900 mb-1">Enter Score</h3>
    <p id="modalMatchLabel" class="text-sm text-gray-500 mb-4"></p>

    <!-- Team name column headers -->
    <div style="display:grid;grid-template-columns:64px 1fr 20px 1fr;gap:6px;align-items:center;margin-bottom:6px;">
      <div></div>
      <div id="hdrTeam1" class="text-xs font-bold text-gray-500 text-center truncate uppercase tracking-wide"></div>
      <div></div>
      <div id="hdrTeam2" class="text-xs font-bold text-gray-500 text-center truncate uppercase tracking-wide"></div>
    </div>

    <!-- Game rows -->
    <div class="space-y-2 mb-3">
      <div style="display:grid;grid-template-columns:64px 1fr 20px 1fr;gap:6px;align-items:center;">
        <span class="text-xs font-semibold text-gray-400 text-right pr-2">Game 1</span>
        <input id="g1t1" type="number" min="0" oninput="updateTally()" placeholder="—"
          class="border rounded-lg px-2 py-2 text-center text-xl font-bold focus:outline-none focus:ring-2 focus:ring-indigo-500 w-full"/>
        <span class="text-gray-300 font-bold text-center">–</span>
        <input id="g1t2" type="number" min="0" oninput="updateTally()" placeholder="—"
          class="border rounded-lg px-2 py-2 text-center text-xl font-bold focus:outline-none focus:ring-2 focus:ring-indigo-500 w-full"/>
      </div>
      <div style="display:grid;grid-template-columns:64px 1fr 20px 1fr;gap:6px;align-items:center;">
        <span class="text-xs font-semibold text-gray-400 text-right pr-2">Game 2</span>
        <input id="g2t1" type="number" min="0" oninput="updateTally()" placeholder="—"
          class="border rounded-lg px-2 py-2 text-center text-xl font-bold focus:outline-none focus:ring-2 focus:ring-indigo-500 w-full"/>
        <span class="text-gray-300 font-bold text-center">–</span>
        <input id="g2t2" type="number" min="0" oninput="updateTally()" placeholder="—"
          class="border rounded-lg px-2 py-2 text-center text-xl font-bold focus:outline-none focus:ring-2 focus:ring-indigo-500 w-full"/>
      </div>
      <div id="game3row" style="display:grid;grid-template-columns:64px 1fr 20px 1fr;gap:6px;align-items:center;opacity:0.35;">
        <span class="text-xs font-semibold text-gray-400 text-right pr-2">Game 3</span>
        <input id="g3t1" type="number" min="0" oninput="updateTally()" placeholder="—" disabled
          class="border rounded-lg px-2 py-2 text-center text-xl font-bold focus:outline-none focus:ring-2 focus:ring-indigo-500 w-full bg-gray-50"/>
        <span class="text-gray-300 font-bold text-center">–</span>
        <input id="g3t2" type="number" min="0" oninput="updateTally()" placeholder="—" disabled
          class="border rounded-lg px-2 py-2 text-center text-xl font-bold focus:outline-none focus:ring-2 focus:ring-indigo-500 w-full bg-gray-50"/>
      </div>
    </div>

    <!-- Live tally -->
    <div id="tallyDisplay" class="text-center py-2 px-3 bg-gray-50 rounded-lg mb-4 text-sm font-medium text-gray-400">
      Enter scores above — Game 3 unlocks if needed
    </div>

    <div class="flex gap-3">
      <button onclick="closeModal()" class="flex-1 py-2 border rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-50">Cancel</button>
      <button onclick="submitScore()" id="saveBtn"
        class="flex-1 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg text-sm font-medium">Save Score</button>
    </div>
    <div class="mt-3 pt-3 border-t border-gray-100">
      <p class="text-xs text-gray-400 mb-2">Or mark a team as no-show:</p>
      <div class="flex gap-2">
        <button onclick="submitForfeit('team1')" class="flex-1 text-xs py-1.5 bg-orange-50 border border-orange-200 text-orange-700 hover:bg-orange-100 rounded-lg font-medium truncate">🏳 <span id="forfeit1Label">Team 1</span> forfeits</button>
        <button onclick="submitForfeit('team2')" class="flex-1 text-xs py-1.5 bg-orange-50 border border-orange-200 text-orange-700 hover:bg-orange-100 rounded-lg font-medium truncate">🏳 <span id="forfeit2Label">Team 2</span> forfeits</button>
      </div>
    </div>
  </div>
</div>

<!-- Schedule modal -->
<div id="scheduleModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4 hidden">
  <div class="bg-white rounded-2xl shadow-xl p-6 w-full max-w-sm">
    <h3 class="font-bold text-gray-900 mb-1">📅 Schedule Match</h3>
    <p id="scheduleMatchLabel" class="text-sm text-gray-500 mb-4"></p>
    <div class="space-y-3 mb-4">
      <div>
        <label class="text-xs font-medium text-gray-600 mb-1 block">Date & Time</label>
        <input id="scheduleTime" type="datetime-local"
          class="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"/>
      </div>
      <div>
        <label class="text-xs font-medium text-gray-600 mb-1 block">Table Number (optional)</label>
        <input id="scheduleTable" type="text" placeholder="e.g. 1 or 2"
          class="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"/>
      </div>
      <label class="flex items-center gap-2 cursor-pointer select-none">
        <input id="scheduleNotify" type="checkbox" class="rounded accent-blue-600"/>
        <span class="text-sm text-gray-600">Notify players via Slack DM</span>
      </label>
    </div>
    <div class="flex gap-3">
      <button onclick="closeScheduleModal()" class="flex-1 py-2 border rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-50">Cancel</button>
      <button onclick="submitSchedule()" id="scheduleSaveBtn" class="flex-1 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium">Save & Notify</button>
    </div>
  </div>
</div>

<script>
const TOKEN_KEY = 'pp_admin_token';
let token = localStorage.getItem(TOKEN_KEY) || '';
let currentMatchID = '';
let currentTeam1Name = '';
let currentTeam2Name = '';
let swapPlayerID = '';
let swapPlayerName = '';
let allMatches = {}; // matchID → match object, populated in loadBracket()

function adminFetch(url, opts={}) {
  return fetch(url, {...opts, headers: {'Content-Type':'application/json','X-Admin-Token':token,...(opts.headers||{})}});
}

function teamName(t) {
  if (!t) return 'TBD';
  return t.player1.name + ' & ' + t.player2.name;
}

// --- Auth ---
document.getElementById('loginForm').addEventListener('submit', async e => {
  e.preventDefault();
  const pp = document.getElementById('passphrase').value;
  const btn = document.getElementById('loginBtn');
  const err = document.getElementById('loginErr');
  btn.textContent = 'Checking...'; btn.disabled = true; err.classList.add('hidden');
  try {
    const res = await fetch('/api/admin/auth', {method:'POST', headers:{'Content-Type':'application/json','X-Admin-Token':pp}});
    if (res.ok) { token = pp; localStorage.setItem(TOKEN_KEY, pp); showDashboard(); }
    else { err.textContent = 'Wrong passphrase. Try again.'; err.classList.remove('hidden'); btn.textContent='Login'; btn.disabled=false; }
  } catch { err.textContent='Network error.'; err.classList.remove('hidden'); btn.textContent='Login'; btn.disabled=false; }
});

function logout() { localStorage.removeItem(TOKEN_KEY); token=''; location.reload(); }

async function resetTournament() {
  if (!confirm('⚠️ This will DELETE all registrations, teams, matches, and scores. Are you sure?')) return;
  if (!confirm('Really reset everything? This cannot be undone.')) return;
  const res = await adminFetch('/api/admin/reset', {method:'POST'});
  if (res.ok) { alert('Tournament reset! Starting fresh.'); showTab('reg'); loadReg(); }
  else { alert('Reset failed.'); }
}

function showDashboard() {
  document.getElementById('loginScreen').classList.add('hidden');
  document.getElementById('dashboard').classList.remove('hidden');
  loadReg();
}

// --- Tabs ---
function showTab(tab) {
  ['reg','teams','bracket'].forEach(t => {
    document.getElementById('tab-content-'+t).classList.toggle('hidden', t!==tab);
    const btn = document.getElementById('tab-'+t);
    btn.className = 'px-4 py-3 text-sm font-medium border-b-2 ' + (t===tab ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700');
  });
  if (tab==='reg') loadReg();
  if (tab==='teams') loadTeams();
  if (tab==='bracket') loadBracket();
}

// --- Registrations ---
async function loadReg() {
  const el = document.getElementById('tab-content-reg');
  el.innerHTML = '<p class="text-gray-400">Loading...</p>';
  try {
    const [pr, sr] = await Promise.all([
      adminFetch('/api/admin/players').then(r=>r.json()),
      fetch('/api/tournament/status').then(r=>r.json())
    ]);
    const players = pr.players || [];
    const open = sr.registration_open;
    const odd = players.length % 2 !== 0;
    let rows = players.map((p,i) => ` + "`" + `
      <tr class="border-t border-gray-100 hover:bg-gray-50">
        <td class="p-3 text-gray-400">${i+1}</td>
        <td class="p-3 font-medium">${p.name}</td>
        <td class="p-3 text-gray-500">${p.email}</td>
        <td class="p-3 text-gray-400 text-xs">${new Date(p.registered_at).toLocaleString()}</td>
        <td class="p-3"><button onclick="deletePlayer('${p.id}','${p.name.replace(/'/g,"\\'")}' )" class="text-xs text-red-400 hover:text-red-600 px-2 py-1 rounded hover:bg-red-50 whitespace-nowrap" title="Remove player">✕ Remove</button></td>
      </tr>` + "`" + `).join('');
    el.innerHTML = ` + "`" + `
      <div class="flex flex-wrap justify-between items-start gap-3 mb-4">
        <div>
          <h2 class="text-xl font-bold text-gray-900">Registrations</h2>
          <p class="text-sm text-gray-500">${players.length} player${players.length!==1?'s':''} signed up</p>
        </div>
        <div class="flex gap-3 flex-wrap">
          <button onclick="toggleReg(${open})" class="px-4 py-2 rounded-lg text-sm font-medium transition-colors ${open?'bg-red-100 text-red-700 hover:bg-red-200':'bg-green-100 text-green-700 hover:bg-green-200'}">
            ${open ? '🔒 Close Registration' : '🔓 Open Registration'}
          </button>
          <button onclick="randomizeTeams()" ${(open || players.length < 2) ? 'disabled' : ''}
            class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-200 text-white text-sm font-medium rounded-lg transition-colors">
            🎲 Randomize into Teams
          </button>
        </div>
      </div>
      ${open ? '<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-3 mb-4 text-sm text-yellow-800">⚠️ Registration is still <strong>open</strong>. Close it before randomizing teams.</div>' : ''}
      ${odd && !open ? '<div class="bg-orange-50 border border-orange-200 rounded-lg p-3 mb-4 text-sm text-orange-800">⚠️ <strong>Odd number of players ('+players.length+')</strong> — one player will be left without a partner.</div>' : ''}
      ${players.length===0 ? '<div class="bg-white rounded-xl p-8 text-center text-gray-400 shadow">No registrations yet. Share the registration link!</div>' :
      '<div class="bg-white rounded-xl shadow overflow-hidden"><table class="w-full text-sm"><thead class="bg-gray-50"><tr><th class="text-left p-3 font-medium text-gray-600">#</th><th class="text-left p-3 font-medium text-gray-600">Name</th><th class="text-left p-3 font-medium text-gray-600">Email</th><th class="text-left p-3 font-medium text-gray-600">Registered</th><th class="p-3"></th></tr></thead><tbody>'+rows+'</tbody></table></div>'}
      <div class="mt-6 bg-yellow-50 border border-yellow-200 rounded-xl p-4">
        <p class="text-xs font-bold text-yellow-700 uppercase tracking-wide mb-2">🧪 Testing Tools</p>
        <div class="flex items-center gap-3">
          <input id="testCount" type="number" min="1" max="100" value="10" placeholder="# players"
            class="border border-gray-300 rounded-lg px-3 py-2 text-sm w-28 focus:outline-none focus:ring-2 focus:ring-yellow-400"/>
          <button onclick="generateTestPlayers()" class="px-4 py-2 bg-yellow-500 hover:bg-yellow-600 text-white text-sm font-medium rounded-lg">
            ⚡ Generate Fake Players
          </button>
          <span class="text-xs text-gray-400">Creates random players for testing bracket formats</span>
        </div>
      </div>
    ` + "`" + `;
  } catch(e) { el.innerHTML = '<p class="text-red-400">Failed to load.</p>'; }
}

async function toggleReg(isOpen) {
  const ep = isOpen ? '/api/admin/registration/close' : '/api/admin/registration/open';
  await adminFetch(ep, {method:'POST'});
  loadReg();
}

async function generateTestPlayers() {
  const count = parseInt(document.getElementById('testCount').value) || 10;
  if (count < 1 || count > 100) { alert('Enter a number between 1 and 100'); return; }
  const res = await adminFetch('/api/admin/test/generate', {method:'POST', body:JSON.stringify({count})});
  const data = await res.json();
  if (res.ok) { alert('Generated '+data.created+' players! Total: '+data.total_players); loadReg(); }
  else { alert('Error: '+(data.error||'unknown')); }
}

async function randomizeTeams() {
  if (!confirm('Randomize players into teams of 2?')) return;
  const res = await adminFetch('/api/admin/teams/randomize', {method:'POST'});
  const data = await res.json();
  if (res.ok) {
    alert('Created '+data.teams_created+' teams!'+(data.odd_player_id?' ⚠️ One player has no partner.':''));
    showTab('teams');
  } else { alert('Error: '+(data.error||'unknown')); }
}

// --- Teams ---
async function loadTeams() {
  const el = document.getElementById('tab-content-teams');
  el.innerHTML = '<p class="text-gray-400">Loading...</p>';
  try {
    const data = await adminFetch('/api/admin/teams').then(r=>r.json());
    const teams = data.teams || [];
    let cards = teams.map((t,i) => {
      const players = [t.player1, t.player2].map(p => ` + "`" + `
        <button onclick="handlePlayerClick('${p.id}','${p.name.replace(/'/g,"\\'")}',this)"
          class="player-btn w-full text-left px-3 py-2 rounded-lg mb-1 text-sm bg-gray-50 hover:bg-gray-100 transition-colors" data-pid="${p.id}">
          <span class="font-medium">${p.name}</span>
          <span class="text-xs text-gray-400 block truncate">${p.email}</span>
        </button>` + "`" + `).join('');
      return ` + "`" + `
        <div class="bg-white rounded-xl shadow p-4 border border-gray-100">
          <p class="text-xs font-bold text-gray-400 uppercase mb-3">Team ${i+1}</p>
          ${players}
        </div>` + "`" + `;
    }).join('');
    el.innerHTML = ` + "`" + `
      <div class="flex justify-between items-center mb-4">
        <div><h2 class="text-xl font-bold text-gray-900">Teams</h2><p class="text-sm text-gray-500">${teams.length} team${teams.length!==1?'s':''}</p></div>
        <div class="flex gap-3">
          <button onclick="rerandomizeTeams()" ${teams.length<2?'disabled':''} class="px-4 py-2 bg-gray-100 hover:bg-gray-200 disabled:bg-gray-50 text-gray-700 text-sm font-medium rounded-lg border border-gray-300">
            🎲 Re-randomize
          </button>
          <button onclick="generateBracket()" ${teams.length<2?'disabled':''} class="px-4 py-2 bg-green-600 hover:bg-green-700 disabled:bg-green-200 text-white text-sm font-medium rounded-lg">
            🏆 Lock Teams & Generate Bracket
          </button>
        </div>
      </div>
      <div id="swapBanner" class="hidden bg-indigo-50 border border-indigo-200 rounded-lg p-3 mb-4 text-sm text-indigo-800"></div>
      ${teams.length===0 ? '<div class="bg-white rounded-xl p-8 text-center text-gray-400 shadow">No teams yet. Go to Registrations and randomize first.</div>' :
      '<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">'+cards+'</div>'}
      <p class="text-xs text-gray-400 mt-4">💡 Click a player to select, then click another to swap them.</p>
    ` + "`" + `;
  } catch(e) { el.innerHTML = '<p class="text-red-400">Failed to load.</p>'; }
}

function handlePlayerClick(pid, pname, btn) {
  if (!swapPlayerID) {
    swapPlayerID = pid; swapPlayerName = pname;
    document.querySelectorAll('.player-btn').forEach(b => {
      b.className = b.dataset.pid === pid
        ? 'player-btn w-full text-left px-3 py-2 rounded-lg mb-1 text-sm bg-indigo-600 text-white transition-colors'
        : 'player-btn w-full text-left px-3 py-2 rounded-lg mb-1 text-sm bg-yellow-50 border border-yellow-300 hover:bg-yellow-100 transition-colors';
    });
    const banner = document.getElementById('swapBanner');
    banner.textContent = '🔄 Swapping '+pname+' — click another player to swap, or click again to cancel.';
    banner.classList.remove('hidden');
    return;
  }
  if (swapPlayerID === pid) { cancelSwap(); return; }
  doSwap(swapPlayerID, pid);
}

async function doSwap(p1, p2) {
  const res = await adminFetch('/api/admin/teams/swap', {method:'PUT', body:JSON.stringify({player1_id:p1,player2_id:p2})});
  swapPlayerID=''; swapPlayerName='';
  if (res.ok) { loadTeams(); } else { alert('Swap failed'); loadTeams(); }
}

function cancelSwap() { swapPlayerID=''; swapPlayerName=''; loadTeams(); }

async function rerandomizeTeams() {
  if (!confirm('Re-randomize all players into new random teams? Current team assignments will be replaced.')) return;
  const res = await adminFetch('/api/admin/teams/randomize', {method:'POST'});
  const data = await res.json();
  if (res.ok) {
    alert('Re-randomized into '+data.teams_created+' teams!'+(data.odd_player_id?' ⚠️ One player has no partner.':''));
    loadTeams();
  } else { alert('Error: '+(data.error||'unknown')); }
}

async function generateBracket() {
  const teams = document.querySelectorAll('[class*="Team"]').length;
  if (!confirm('Lock teams and generate the bracket?')) return;
  const res = await adminFetch('/api/admin/bracket/generate', {method:'POST'});
  const data = await res.json();
  if (res.ok) { alert('Bracket generated! '+data.rr_matches+' round-robin matches created.'); showTab('bracket'); }
  else { alert('Error: '+(data.error||'unknown')); }
}

// --- Bracket ---
async function loadBracket() {
  const el = document.getElementById('tab-content-bracket');
  el.innerHTML = '<p class="text-gray-400">Loading...</p>';
  try {
    const data = await fetch('/api/tournament/bracket').then(r=>r.json());
    const matches = data.matches || [];
    const groups = data.groups || [];
    const tourn = data.tournament;
    // Populate allMatches lookup so openModal(id) can find match details
    allMatches = {};
    matches.forEach(m => { allMatches[m.id] = m; });
    if (matches.length===0) {
      el.innerHTML = '<h2 class="text-xl font-bold text-gray-900 mb-4">Bracket & Scores</h2><div class="bg-white rounded-xl p-8 text-center text-gray-400 shadow">No bracket yet. Go to Teams tab and generate the bracket first.</div>';
      return;
    }

    const groupNames = ['A','B','C','D','E','F','G','H'];
    const numGroups = tourn?.num_groups || 1;
    const knockoutSize = tourn?.knockout_size || 4;

    function fmtGames(m) {
      if (!m.games || !m.games.length) {
        // Fallback: show game-win counts
        if (m.team1_score != null) return (m.team1_score)+'–'+(m.team2_score)+' games';
        return '';
      }
      return m.games.map(g => (g.team1_score??0)+'–'+(g.team2_score??0)).join(', ');
    }

    function matchRow(m) {
      const done = m.status==='complete';
      const canScore = m.team1 && m.team2;
      const t1 = teamName(m.team1); const t2 = teamName(m.team2);
      const timeLabel = m.scheduled_time ? formatMatchTime(m.scheduled_time) : '';
      const tableLabel = m.table_num ? ' · Table '+m.table_num : '';
      const curTime = m.scheduled_time ? m.scheduled_time.substring(0,16) : '';
      const matchLabel = (t1+' vs '+t2).replace(/'/g,"\\'");
      const scoreDetail = done ? fmtGames(m) : '';
      const gameWins = done && m.team1_score != null ? m.team1_score+'–'+m.team2_score+' games' : '';
      return ` + "`" + `
        <div class="flex items-center justify-between p-3 rounded-lg mb-2 border ${done?'bg-green-50 border-green-200':'bg-white border-gray-200'}">
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate">${t1} <span class="text-gray-400">vs</span> ${t2}</p>
            ${done ? '<p class="text-xs text-green-700 font-bold mt-0.5">'+gameWins+(scoreDetail?' <span class="font-normal text-green-600">('+scoreDetail+')</span>':'')+'</p>' : ''}
            ${timeLabel ? '<p class="text-xs text-blue-600 mt-0.5">📅 '+timeLabel+tableLabel+'</p>' : ''}
          </div>
          <div class="flex items-center gap-1.5 ml-3 flex-shrink-0">
            <button onclick="openScheduleModal('${m.id}','${matchLabel}','${curTime}','${m.table_num||''}')"
              title="Schedule match" class="px-2 py-1 text-xs ${timeLabel?'text-blue-500 bg-blue-50 hover:bg-blue-100':'text-gray-300 hover:text-gray-500 hover:bg-gray-100'} rounded-lg">📅</button>
            <button onclick="openModal('${m.id}')"
              ${canScore?'':'disabled'}
              class="px-3 py-1 text-xs font-medium bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-100 disabled:text-gray-400 text-white rounded-lg">
              ${done?'Edit':'Enter Score'}
            </button>
          </div>
        </div>` + "`" + `;
    }

    let html = '<h2 class="text-xl font-bold text-gray-900 mb-4">Bracket & Scores</h2>';

    // Group stage: one section per group
    for (let g = 0; g < numGroups; g++) {
      const groupMatches = matches.filter(m => m.round === 'rr' && m.group_number === g);
      const rounds = [...new Set(groupMatches.map(m=>m.round_number))].sort((a,b)=>a-b);
      html += ` + "`" + `<div class="mb-6"><h3 class="font-semibold text-gray-700 mb-3">Group ${groupNames[g]}</h3>` + "`" + `;
      rounds.forEach(r => {
        html += ` + "`" + `<div class="mb-3"><p class="text-xs font-bold text-gray-400 uppercase mb-2">Round ${r+1}</p>` + "`" + `;
        groupMatches.filter(m=>m.round_number===r).forEach(m => { html+=matchRow(m); });
        html += '</div>';
      });
      html += '</div>';
    }

    // QF
    const qfs = matches.filter(m=>m.round==='qf').sort((a,b)=>a.match_order-b.match_order);
    if (qfs.length) {
      html += '<div class="mb-6"><h3 class="font-semibold text-gray-700 mb-3">Quarter-finals</h3>';
      qfs.forEach(m => { html+=matchRow(m); });
      html += '</div>';
    }

    // SF
    const sfs = matches.filter(m=>m.round==='sf').sort((a,b)=>a.match_order-b.match_order);
    if (sfs.length) {
      html += '<div class="mb-6"><h3 class="font-semibold text-gray-700 mb-3">Semi-finals</h3>';
      sfs.forEach(m => { html+=matchRow(m); });
      html += '</div>';
    }

    // Final
    const final = matches.find(m=>m.round==='final');
    if (final) {
      html += '<div class="mb-6"><h3 class="font-semibold text-gray-700 mb-3">🏆 Final</h3>'+matchRow(final)+'</div>';
    }

    // Format info badge
    const formatLabel = numGroups === 1
      ? ` + "`" + `1 group • top ${knockoutSize} → knockout` + "`" + `
      : ` + "`" + `${numGroups} groups • top ${knockoutSize} → knockout` + "`" + `;
    html = ` + "`" + `<div class="flex items-center justify-between mb-4"><h2 class="text-xl font-bold text-gray-900">Bracket & Scores</h2><span class="text-xs bg-indigo-50 text-indigo-700 border border-indigo-200 rounded-full px-3 py-1 font-medium">${formatLabel}</span></div>` + "`" + ` + html;

    // ── Auto-schedule section ────────────────────────────────────
    const todayStr = new Date().toISOString().split('T')[0];
    const autoSchedHtml = ` + "`" + `<div class="mb-5 p-4 bg-sky-50 border border-sky-200 rounded-xl">
      <p class="text-sm font-bold text-sky-800 mb-2">📅 Schedule Matches</p>
      <div class="flex flex-wrap items-center gap-3">
        <input id="autoScheduleDate" type="date" value="${todayStr}"
          class="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-sky-400"/>
        <label class="flex items-center gap-1.5 text-sm text-gray-600 cursor-pointer select-none">
          <input id="autoNotify" type="checkbox" checked class="rounded accent-blue-600"/> Notify via Slack
        </label>
        <button id="autoScheduleBtn" onclick="autoSchedule()"
          class="px-4 py-1.5 bg-sky-600 hover:bg-sky-700 text-white text-sm font-medium rounded-lg">
          ⚡ Auto-Schedule All
        </button>
      </div>
      <p class="text-xs text-sky-500 mt-2">Assigns matches to 1:00 PM, 1:30 PM, 5:00 PM, 5:30 PM… slots on the selected date. Click 📅 on any match to set a custom time.</p>
    </div>` + "`" + `;

    // ── Progress bar ─────────────────────────────────────────────
    const totalPlayable = matches.filter(m => m.team1 && m.team2).length;
    const completedCount = matches.filter(m => m.status === 'complete').length;
    const pct = totalPlayable > 0 ? Math.round(completedCount/totalPlayable*100) : 0;
    const progressHtml = '<div class="mb-5 p-4 bg-white rounded-xl shadow">'
      + '<div class="flex justify-between items-center mb-2">'
      + '<span class="text-sm font-medium text-gray-700">Tournament Progress</span>'
      + '<span class="text-sm font-bold text-indigo-600">'+completedCount+' / '+totalPlayable+' matches done</span>'
      + '</div>'
      + '<div class="w-full bg-gray-100 rounded-full h-2">'
      + '<div class="bg-indigo-500 h-2 rounded-full transition-all" style="width:'+pct+'%"></div>'
      + '</div></div>';

    // ── Champion banner ──────────────────────────────────────────
    const finalM = matches.find(m=>m.round==='final');
    let champHtml = '';
    if (finalM && finalM.status==='complete' && finalM.winner_team_id) {
      const ct = finalM.winner_team_id===finalM.team1?.id ? finalM.team1 : finalM.team2;
      champHtml = '<div class="mb-5 p-5 bg-amber-50 border-2 border-amber-300 rounded-xl text-center">'
        + '<div class="text-3xl mb-1">🏆</div>'
        + '<p class="text-xs font-bold text-amber-600 uppercase tracking-wide mb-1">Tournament Champion</p>'
        + '<p class="text-xl font-bold text-amber-900">'+teamName(ct)+'</p>'
        + '</div>';
    }

    // ── Pending ready matches ────────────────────────────────────
    const roundPri = {rr:0,qf:1,sf:2,final:3};
    const pendingReady = matches
      .filter(m => m.team1 && m.team2 && m.status !== 'complete')
      .sort((a,b) => (roundPri[a.round]||0) - (roundPri[b.round]||0));
    let pendingHtml = '';
    if (pendingReady.length > 0) {
      const items = pendingReady.map(m => {
        const rl = m.round==='rr'
          ? ('Group '+groupNames[m.group_number||0]+' RR')
          : ({qf:'Quarter-final',sf:'Semi-final',final:'🏆 Final'}[m.round]||m.round.toUpperCase());
        const t1=teamName(m.team1), t2=teamName(m.team2);
        return '<div class="flex items-center justify-between p-2.5 bg-blue-50 border border-blue-100 rounded-lg mb-1.5">'
          + '<div><span class="text-xs font-bold text-blue-400 uppercase mr-2">'+rl+'</span>'
          + '<span class="text-sm font-medium text-gray-800">'+t1+' <span class="text-gray-400">vs</span> '+t2+'</span></div>'
          + '<button onclick="openModal(\''+m.id+'\')" '
          + 'class="ml-3 px-3 py-1 text-xs font-bold bg-blue-600 hover:bg-blue-700 text-white rounded-lg flex-shrink-0">Enter →</button>'
          + '</div>';
      }).join('');
      pendingHtml = '<div class="mb-5 p-4 bg-white rounded-xl shadow">'
        + '<h3 class="text-sm font-bold text-gray-700 mb-3 flex items-center gap-2">'
        + '<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:#3b82f6;animation:pulse 1.5s infinite;"></span>'
        + '⚡ Next Up — '+pendingReady.length+' match'+(pendingReady.length!==1?'es':'')+' pending'
        + '</h3>'+items+'</div>';
    }

    html = champHtml + progressHtml + pendingHtml + autoSchedHtml + html;
    el.innerHTML = html;
  } catch(e) { el.innerHTML = '<p class="text-red-400">Failed to load bracket.</p>'; }
}

function openModal(matchID) {
  const m = allMatches[matchID];
  if (!m) return;
  currentMatchID = matchID;
  currentTeam1Name = teamName(m.team1);
  currentTeam2Name = teamName(m.team2);

  document.getElementById('modalMatchLabel').textContent = currentTeam1Name + ' vs ' + currentTeam2Name;
  document.getElementById('hdrTeam1').textContent = currentTeam1Name;
  document.getElementById('hdrTeam2').textContent = currentTeam2Name;
  document.getElementById('forfeit1Label').textContent = currentTeam1Name;
  document.getElementById('forfeit2Label').textContent = currentTeam2Name;

  // Clear all inputs first
  ['g1t1','g1t2','g2t1','g2t2','g3t1','g3t2'].forEach(id => {
    document.getElementById(id).value = '';
  });

  // Populate existing game scores if re-editing
  const games = m.games || [];
  if (games[0]) {
    document.getElementById('g1t1').value = games[0].team1_score ?? '';
    document.getElementById('g1t2').value = games[0].team2_score ?? '';
  }
  if (games[1]) {
    document.getElementById('g2t1').value = games[1].team1_score ?? '';
    document.getElementById('g2t2').value = games[1].team2_score ?? '';
  }
  if (games[2]) {
    document.getElementById('g3t1').value = games[2].team1_score ?? '';
    document.getElementById('g3t2').value = games[2].team2_score ?? '';
  }

  document.getElementById('scoreModal').classList.remove('hidden');
  updateTally();
  setTimeout(() => document.getElementById('g1t1').focus(), 50);
}

function closeModal() {
  document.getElementById('scoreModal').classList.add('hidden');
}

function parseScore(id) {
  const v = document.getElementById(id).value;
  if (v === '' || v === null || v === undefined) return null;
  const n = parseInt(v);
  return isNaN(n) ? null : n;
}

function updateTally() {
  let t1Wins = 0, t2Wins = 0;

  // Score game 1 and 2 only (always visible)
  const g1t1 = parseScore('g1t1'), g1t2 = parseScore('g1t2');
  const g2t1 = parseScore('g2t1'), g2t2 = parseScore('g2t2');

  if (g1t1 !== null && g1t2 !== null && g1t1 !== g1t2) {
    if (g1t1 > g1t2) t1Wins++; else t2Wins++;
  }
  if (g2t1 !== null && g2t2 !== null && g2t1 !== g2t2) {
    if (g2t1 > g2t2) t1Wins++; else t2Wins++;
  }

  // Game 3 only needed when it's 1-1 after games 1+2
  const needsGame3 = t1Wins === 1 && t2Wins === 1;
  const game3row = document.getElementById('game3row');
  const g3t1el = document.getElementById('g3t1');
  const g3t2el = document.getElementById('g3t2');

  if (needsGame3) {
    game3row.style.opacity = '1';
    g3t1el.disabled = false;
    g3t2el.disabled = false;
    // Also tally game 3
    const g3t1 = parseScore('g3t1'), g3t2 = parseScore('g3t2');
    if (g3t1 !== null && g3t2 !== null && g3t1 !== g3t2) {
      if (g3t1 > g3t2) t1Wins++; else t2Wins++;
    }
  } else {
    game3row.style.opacity = '0.35';
    g3t1el.disabled = true;
    g3t2el.disabled = true;
    g3t1el.value = '';
    g3t2el.value = '';
  }

  const tally = document.getElementById('tallyDisplay');
  if (t1Wins === 0 && t2Wins === 0) {
    tally.textContent = 'Enter scores above — Game 3 unlocks if needed';
    tally.className = 'text-center py-2 px-3 bg-gray-50 rounded-lg mb-4 text-sm font-medium text-gray-400';
  } else if (t1Wins === 2 || t2Wins === 2) {
    const winner = t1Wins === 2 ? currentTeam1Name : currentTeam2Name;
    tally.innerHTML = '🏆 <strong>' + winner + '</strong> wins ' + t1Wins + '–' + t2Wins;
    tally.className = 'text-center py-2 px-3 bg-green-50 border border-green-200 rounded-lg mb-4 text-sm font-semibold text-green-700';
  } else {
    tally.textContent = currentTeam1Name + ': ' + t1Wins + (t1Wins===1?' game':' games') + '  ·  ' + currentTeam2Name + ': ' + t2Wins + (t2Wins===1?' game':' games');
    tally.className = 'text-center py-2 px-3 bg-blue-50 border border-blue-100 rounded-lg mb-4 text-sm font-medium text-blue-600';
  }
}

async function submitForfeit(forfeiter) {
  const fName = forfeiter === 'team1' ? currentTeam1Name : currentTeam2Name;
  if (!confirm('Mark "' + fName + '" as forfeit? They receive a 0–11 loss and the other team advances.')) return;
  const btn = document.getElementById('saveBtn');
  btn.textContent = 'Saving...'; btn.disabled = true;
  try {
    const res = await adminFetch('/api/admin/matches/'+currentMatchID+'/forfeit', {method:'POST', body:JSON.stringify({forfeiter})});
    if (res.ok) { closeModal(); loadBracket(); }
    else { const d = await res.json().catch(()=>({})); alert('Forfeit failed: '+(d.error||'unknown error')); }
  } finally { btn.textContent = 'Save Score'; btn.disabled = false; }
}

async function deletePlayer(id, name) {
  if (!confirm('Remove "' + name + '" from the tournament?\nThis cannot be undone.')) return;
  const res = await adminFetch('/api/admin/players/'+id, {method:'DELETE'});
  if (res.ok) { loadReg(); }
  else {
    const d = await res.json().catch(()=>({}));
    if (d.error === 'teams_formed') { alert('Cannot remove players after teams have been formed.\nReset the tournament first if needed.'); }
    else { alert('Failed to remove player.'); }
  }
}

// ── Schedule helpers ────────────────────────────────────────────────────
let currentScheduleMatchID = '';

function formatMatchTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleDateString('en-US', {weekday:'short',month:'short',day:'numeric'})
    + ' · ' + d.toLocaleTimeString('en-US', {hour:'numeric',minute:'2-digit',hour12:true});
}

function openScheduleModal(matchID, matchLabel, currentTime, currentTable) {
  currentScheduleMatchID = matchID;
  document.getElementById('scheduleMatchLabel').textContent = matchLabel;
  document.getElementById('scheduleTime').value = currentTime || '';
  document.getElementById('scheduleTable').value = currentTable || '';
  document.getElementById('scheduleNotify').checked = true;
  document.getElementById('scheduleModal').classList.remove('hidden');
  setTimeout(() => document.getElementById('scheduleTime').focus(), 50);
}

function closeScheduleModal() {
  document.getElementById('scheduleModal').classList.add('hidden');
}

async function submitSchedule() {
  const t = document.getElementById('scheduleTime').value;
  if (!t) { alert('Please pick a date and time.'); return; }
  const table = document.getElementById('scheduleTable').value;
  const notify = document.getElementById('scheduleNotify').checked;
  const btn = document.getElementById('scheduleSaveBtn');
  btn.textContent = 'Saving...'; btn.disabled = true;
  try {
    const res = await adminFetch('/api/admin/matches/'+currentScheduleMatchID+'/schedule', {
      method: 'PUT',
      body: JSON.stringify({scheduled_time: t, table_num: table, notify})
    });
    if (res.ok) {
      closeScheduleModal();
      loadBracket();
      if (notify) alert('Schedule saved and players notified via Slack! ✅');
    } else {
      const d = await res.json().catch(()=>({}));
      alert('Failed: ' + (d.error || 'unknown error'));
    }
  } finally { btn.textContent = 'Save & Notify'; btn.disabled = false; }
}

async function autoSchedule() {
  const date = document.getElementById('autoScheduleDate').value;
  const notify = document.getElementById('autoNotify').checked;
  if (!date) { alert('Please pick a date first.'); return; }
  const btn = document.getElementById('autoScheduleBtn');
  btn.textContent = 'Scheduling...'; btn.disabled = true;
  try {
    const res = await adminFetch('/api/admin/matches/autoschedule', {
      method: 'POST',
      body: JSON.stringify({date, notify})
    });
    const data = await res.json();
    if (res.ok) {
      loadBracket();
      alert('Scheduled ' + data.scheduled + ' matches for ' + date + (notify ? ' and notified players! 📅' : '.'));
    } else {
      alert('Error: ' + (data.error || 'unknown'));
    }
  } finally { btn.textContent = '⚡ Auto-Schedule'; btn.disabled = false; }
}

async function submitScore() {
  // Collect filled-in game scores
  const gamePairs = [['g1t1','g1t2'],['g2t1','g2t2'],['g3t1','g3t2']];
  const games = [];
  for (const [id1, id2] of gamePairs) {
    const el1 = document.getElementById(id1);
    const el2 = document.getElementById(id2);
    if (el1.disabled) continue; // game 3 disabled = not needed
    const s1 = parseScore(id1), s2 = parseScore(id2);
    if (s1 === null && s2 === null) continue; // both empty = game not played
    if (s1 === null || s2 === null) {
      alert('Please enter both scores for each game you started.');
      return;
    }
    games.push({team1_score: s1, team2_score: s2});
  }
  if (games.length === 0) {
    alert('Please enter at least one game score.');
    return;
  }
  const btn = document.getElementById('saveBtn');
  btn.textContent = 'Saving...'; btn.disabled = true;
  try {
    const res = await adminFetch('/api/admin/matches/'+currentMatchID+'/score', {
      method: 'POST',
      body: JSON.stringify({games})
    });
    if (res.ok) { closeModal(); loadBracket(); }
    else {
      const d = await res.json().catch(()=>({}));
      alert('Failed to save: ' + (d.error || 'unknown error'));
    }
  } finally { btn.textContent = 'Save Score'; btn.disabled = false; }
}

// Boot
if (token) {
  fetch('/api/admin/auth',{method:'POST',headers:{'Content-Type':'application/json','X-Admin-Token':token}})
    .then(r => { if(r.ok){showDashboard();} else {localStorage.removeItem(TOKEN_KEY);token='';} })
    .catch(() => {});
}
</script>
</body>
</html>`
