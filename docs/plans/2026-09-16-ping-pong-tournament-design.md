# Ping Pong Doubles Tournament App — Design Doc
**Date:** 2026-09-16

## Overview
A web app for Applied employees to register for an internal ping pong doubles tournament. Admins manage registrations, pair players into teams, generate brackets, and enter scores. The app lives at a shareable URL pasted into Slack.

## Users
- **Players** — Applied employees who open the link, register, and check the live bracket
- **Admins** (2–3 people) — manage the full tournament lifecycle via a passphrase-protected dashboard

---

## Pages

### 1. Registration Page (`/`)
Public, no login required.
- Clean centered card: tournament name, deadline subtitle
- Fields: Full Name, Applied Email
- Submit → success state ("You're in! 🎉")
- Guards: duplicate email → "Already registered", registration closed → "Registration is closed"
- Live counter: "N players registered so far"

### 2. Live Bracket/Standings (`/bracket`)
Public, read-only.
- Shows tournament format (bracket tree or standings table)
- Updates as admins enter scores
- No score entry controls

### 3. Admin Dashboard (`/admin`)
Passphrase-protected (set via `ADMIN_PASSPHRASE` env var). Session persists in localStorage.

**Three sequential tabs:**

#### Tab 1: Registrations
- Table: Name | Email | Registered At
- "Close Registration" toggle (disables public sign-up)
- "Randomize into Teams" button (appears after closing registration)
- Flags odd player count with a warning

#### Tab 2: Teams
- Team cards showing Player A & Player B per team
- Swap/edit controls to reassign players between teams
- "Lock Teams & Generate Bracket" button

#### Tab 3: Bracket / Score Entry
- Same bracket/standings view as `/bracket` but with score entry controls
- Click any completed or scheduled match → modal → enter team scores → submit
- Bracket updates immediately on score submission

---

## Tournament Format Logic

Auto-selected at bracket generation time based on team count:

| Teams | Format |
|-------|--------|
| 4–8 | Round Robin → Top 4 knockout (Semifinals + Final) |
| 9+ | EPL-style league table |

**Round Robin scoring:** Win = 2pts, Loss = 0pts  
**EPL scoring:** Win = 3pts, Draw = 1pt, Loss = 0pts. Tiebreaker: score differential.

---

## Data Model (MySQL)

```sql
players      — id, name, email, registered_at
teams        — id, player1_id, player2_id, locked (bool), created_at
tournaments  — id, format (round_robin|epl), status (draft|active|complete), created_at
matches      — id, tournament_id, team1_id, team2_id, team1_score, team2_score,
               winner_team_id, status (pending|complete), round, match_order, created_at
```

---

## Auth
- Admin passphrase stored in `ADMIN_PASSPHRASE` env var
- Frontend sends passphrase with admin API requests via `X-Admin-Token` header
- Backend validates header on all `/api/admin/*` routes
- No player auth — email is trusted on submission

---

## Key Decisions
- **No OAuth** — name + email entry, trusted
- **No calendar invites** — deferred to v2
- **No Slack bot** — URL only for v1
- **Format auto-selected** — no manual format picker, based on team count
- **Odd player count** — flagged but not blocked; admin resolves manually
