package main

import (
	cryptorand "crypto/rand"
	"fmt"
	"log"
	mathrand "math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---- Data types ----

type Player struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	RegisteredAt time.Time `json:"registered_at"`
}

type Team struct {
	ID        string    `json:"id"`
	Player1   *Player   `json:"player1"`
	Player2   *Player   `json:"player2"`
	Locked    bool      `json:"locked"`
	CreatedAt time.Time `json:"created_at"`
}

// GameScore holds the point score for one game (leg) within a match.
type GameScore struct {
	Team1Score *int `json:"team1_score"`
	Team2Score *int `json:"team2_score"`
}

// intPtr returns a pointer to an int — helper for setting *int fields inline.
func intPtr(v int) *int { n := v; return &n }

type Match struct {
	ID            string      `json:"id"`
	TournamentID  string      `json:"tournament_id"`
	Team1         *Team       `json:"team1"`
	Team2         *Team       `json:"team2"`
	Team1Score    *int        `json:"team1_score"`   // number of games won by team 1
	Team2Score    *int        `json:"team2_score"`   // number of games won by team 2
	Games         []GameScore `json:"games"`         // per-game point scores (best of 3)
	WinnerTeamID  *string     `json:"winner_team_id"`
	Status        string      `json:"status"`       // pending | complete
	Round         string      `json:"round"`        // rr | qf | sf | final
	MatchOrder    int         `json:"match_order"`  // ordering within a round
	RoundNumber   int         `json:"round_number"` // for RR: which round of the group; for knockout: 0
	GroupNumber   int         `json:"group_number"` // for RR: group index (0-based); -1 for knockout
	ScheduledTime *time.Time  `json:"scheduled_time"` // when this match is scheduled to be played
	TableNum      string      `json:"table_num"`      // which table (e.g. "1", "2")
	CreatedAt     time.Time   `json:"created_at"`
}

type Tournament struct {
	ID           string    `json:"id"`
	Format       string    `json:"format"`
	Status       string    `json:"status"`       // active | complete
	NumGroups    int       `json:"num_groups"`
	KnockoutSize int       `json:"knockout_size"` // 4 = SF+Final, 8 = QF+SF+Final
	CreatedAt    time.Time `json:"created_at"`
}

// ---- In-memory store ----

type TournamentStore struct {
	mu               sync.RWMutex
	registrationOpen bool
	players          []*Player
	playerByEmail    map[string]*Player
	playerByID       map[string]*Player
	teams            []*Team
	teamByID         map[string]*Team
	tournament       *Tournament
	matches          []*Match
	matchByID        map[string]*Match
	groupAssignments [][]string // groupAssignments[g] = ordered list of team IDs in group g
}

func newStore() *TournamentStore {
	return &TournamentStore{
		registrationOpen: true,
		playerByEmail:    make(map[string]*Player),
		playerByID:       make(map[string]*Player),
		teamByID:         make(map[string]*Team),
		matchByID:        make(map[string]*Match),
	}
}

var store *TournamentStore

// ---- UUID helper ----

func newUUID() string {
	b := make([]byte, 16)
	_, _ = cryptorand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ---- Format engine ----

// determineFormat decides group count and knockout bracket size based on team count.
//
//	≤ 8 teams  → 1 group,  top 4 advance → SF + Final
//	9-12 teams → 3 groups, top 1 each + best runner-up (4 total) → SF + Final
//	13+ teams  → 4 groups, top 2 each (8 total) → QF + SF + Final
func determineFormat(numTeams int) (numGroups, knockoutSize int) {
	switch {
	case numTeams <= 8:
		return 1, 4
	case numTeams <= 11:
		return 3, 4
	default: // 12+ teams → 4 groups, QF+SF+Final
		return 4, 8
	}
}

// splitGroupSizes distributes n items into g groups as evenly as possible.
// Larger groups come first.
func splitGroupSizes(n, g int) []int {
	base := n / g
	extra := n % g
	sizes := make([]int, g)
	for i := range sizes {
		sizes[i] = base
		if i < extra {
			sizes[i]++
		}
	}
	return sizes
}

// ---- Admin middleware ----

func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		passphrase := os.Getenv("ADMIN_PASSPHRASE")
		if passphrase == "" {
			log.Printf("[AdminAuth] WARNING: ADMIN_PASSPHRASE not set")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "admin not configured"})
			return
		}
		token := c.GetHeader("X-Admin-Token")
		if token != passphrase {
			log.Printf("[AdminAuth] unauthorized attempt")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// ---- Public handlers ----

func handleTournamentStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		store.mu.RLock()
		defer store.mu.RUnlock()
		tournStatus := "none"
		if store.tournament != nil {
			tournStatus = store.tournament.Status
		}
		c.JSON(http.StatusOK, gin.H{
			"registration_open": store.registrationOpen,
			"player_count":      len(store.players),
			"tournament_status": tournStatus,
		})
	}
}

func handleRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name  string `json:"name" binding:"required"`
			Email string `json:"email" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and email are required"})
			return
		}
		log.Printf("[Register] attempt: name=%s email=%s", req.Name, req.Email)

		store.mu.Lock()
		defer store.mu.Unlock()

		if !store.registrationOpen {
			c.JSON(http.StatusConflict, gin.H{"error": "registration_closed"})
			return
		}
		if _, exists := store.playerByEmail[req.Email]; exists {
			c.JSON(http.StatusConflict, gin.H{"error": "already_registered"})
			return
		}

		p := &Player{ID: newUUID(), Name: req.Name, Email: req.Email, RegisteredAt: time.Now()}
		store.players = append(store.players, p)
		store.playerByEmail[p.Email] = p
		store.playerByID[p.ID] = p

		log.Printf("[Register] success: id=%s name=%s", p.ID, p.Name)
		c.JSON(http.StatusCreated, gin.H{"success": true, "id": p.ID})
	}
}

func handleGetBracket() gin.HandlerFunc {
	return func(c *gin.Context) {
		store.mu.RLock()
		defer store.mu.RUnlock()

		if store.tournament == nil {
			// Return players and teams before bracket is generated so public page can show them
			players := store.players
			if players == nil {
				players = []*Player{}
			}
			teams := store.teams
			if teams == nil {
				teams = []*Team{}
			}
			c.JSON(http.StatusOK, gin.H{
				"tournament": nil,
				"players":    players,
				"teams":      teams,
				"matches":    []interface{}{},
				"groups":     []interface{}{},
			})
			return
		}

		// Build groups: for each group, ordered list of Team objects
		groupTeams := make([][]*Team, len(store.groupAssignments))
		for g, tids := range store.groupAssignments {
			groupTeams[g] = make([]*Team, 0, len(tids))
			for _, tid := range tids {
				if t := store.teamByID[tid]; t != nil {
					groupTeams[g] = append(groupTeams[g], t)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"tournament": store.tournament,
			"teams":      store.teams,
			"matches":    store.matches,
			"groups":     groupTeams,
		})
	}
}

// ---- Admin handlers ----

func handleAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func handleAdminGetPlayers() gin.HandlerFunc {
	return func(c *gin.Context) {
		store.mu.RLock()
		defer store.mu.RUnlock()
		players := make([]*Player, len(store.players))
		copy(players, store.players)
		c.JSON(http.StatusOK, gin.H{"players": players})
	}
}

func handleAdminCloseRegistration() gin.HandlerFunc {
	return func(c *gin.Context) {
		store.mu.Lock()
		store.registrationOpen = false
		store.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"registration_open": false})
	}
}

func handleAdminOpenRegistration() gin.HandlerFunc {
	return func(c *gin.Context) {
		store.mu.Lock()
		store.registrationOpen = true
		store.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"registration_open": true})
	}
}

func handleAdminRandomizeTeams() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/teams/randomize")
		store.mu.Lock()
		defer store.mu.Unlock()

		if len(store.players) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need at least 2 players"})
			return
		}

		ids := make([]string, len(store.players))
		for i, p := range store.players {
			ids[i] = p.ID
		}
		mathrand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })

		store.teams = nil
		store.teamByID = make(map[string]*Team)

		var newTeams []*Team
		for i := 0; i+1 < len(ids); i += 2 {
			t := &Team{
				ID:        newUUID(),
				Player1:   store.playerByID[ids[i]],
				Player2:   store.playerByID[ids[i+1]],
				CreatedAt: time.Now(),
			}
			newTeams = append(newTeams, t)
			store.teamByID[t.ID] = t
		}
		store.teams = newTeams

		oddPlayerID := ""
		if len(ids)%2 != 0 {
			oddPlayerID = ids[len(ids)-1]
		}

		log.Printf("[Admin] created %d teams", len(newTeams))
		c.JSON(http.StatusOK, gin.H{"teams_created": len(newTeams), "odd_player_id": oddPlayerID})
	}
}

func handleAdminGetTeams() gin.HandlerFunc {
	return func(c *gin.Context) {
		store.mu.RLock()
		defer store.mu.RUnlock()
		teams := make([]*Team, len(store.teams))
		copy(teams, store.teams)
		c.JSON(http.StatusOK, gin.H{"teams": teams})
	}
}

func handleAdminSwapPlayers() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Player1ID string `json:"player1_id" binding:"required"`
			Player2ID string `json:"player2_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		defer store.mu.Unlock()

		var team1, team2 *Team
		var slot1, slot2 string

		for _, t := range store.teams {
			if t.Player1 != nil && t.Player1.ID == req.Player1ID {
				team1 = t; slot1 = "p1"
			} else if t.Player2 != nil && t.Player2.ID == req.Player1ID {
				team1 = t; slot1 = "p2"
			}
			if t.Player1 != nil && t.Player1.ID == req.Player2ID {
				team2 = t; slot2 = "p1"
			} else if t.Player2 != nil && t.Player2.ID == req.Player2ID {
				team2 = t; slot2 = "p2"
			}
		}

		if team1 == nil || team2 == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not in any team"})
			return
		}

		p1 := store.playerByID[req.Player1ID]
		p2 := store.playerByID[req.Player2ID]

		if slot1 == "p1" {
			team1.Player1 = p2
		} else {
			team1.Player2 = p2
		}
		if slot2 == "p1" {
			team2.Player1 = p1
		} else {
			team2.Player2 = p1
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// ---- Round-robin scheduling ----

type rrPairing struct {
	Team1Idx    int
	Team2Idx    int
	RoundNumber int
}

func generateRRSchedule(n int) []rrPairing {
	hasBye := n%2 != 0
	if hasBye {
		n++
	}
	teams := make([]int, n)
	for i := range teams {
		teams[i] = i
	}
	var pairings []rrPairing
	for round := 0; round < n-1; round++ {
		for i := 0; i < n/2; i++ {
			t1 := teams[i]
			t2 := teams[n-1-i]
			if hasBye && (t1 == n-1 || t2 == n-1) {
				continue
			}
			pairings = append(pairings, rrPairing{t1, t2, round})
		}
		last := teams[n-1]
		copy(teams[2:], teams[1:n-1])
		teams[1] = last
	}
	return pairings
}

// ---- Bracket generation ----

func handleAdminGenerateBracket() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/bracket/generate")

		store.mu.Lock()
		defer store.mu.Unlock()

		if len(store.teams) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need at least 2 teams"})
			return
		}

		numGroups, knockoutSize := determineFormat(len(store.teams))
		log.Printf("[Admin] format: %d teams → %d groups, knockout=%d", len(store.teams), numGroups, knockoutSize)

		// Lock all teams
		for _, t := range store.teams {
			t.Locked = true
		}

		// Create tournament record
		tournID := newUUID()
		store.tournament = &Tournament{
			ID:           tournID,
			Format:       "group_knockout",
			Status:       "active",
			NumGroups:    numGroups,
			KnockoutSize: knockoutSize,
			CreatedAt:    time.Now(),
		}

		// Clear old matches
		store.matches = nil
		store.matchByID = make(map[string]*Match)

		// Shuffle teams and assign to groups
		teamsCopy := make([]*Team, len(store.teams))
		copy(teamsCopy, store.teams)
		mathrand.Shuffle(len(teamsCopy), func(i, j int) { teamsCopy[i], teamsCopy[j] = teamsCopy[j], teamsCopy[i] })

		sizes := splitGroupSizes(len(teamsCopy), numGroups)
		groups := make([][]*Team, numGroups)
		store.groupAssignments = make([][]string, numGroups)

		idx := 0
		for g, size := range sizes {
			groups[g] = teamsCopy[idx : idx+size]
			store.groupAssignments[g] = make([]string, size)
			for i, t := range groups[g] {
				store.groupAssignments[g][i] = t.ID
			}
			idx += size
		}

		// Generate RR matches within each group
		rrCount := 0
		for g, groupTeams := range groups {
			pairings := generateRRSchedule(len(groupTeams))
			roundCounts := map[int]int{}
			for _, p := range pairings {
				order := roundCounts[p.RoundNumber]
				m := &Match{
					ID:           newUUID(),
					TournamentID: tournID,
					Team1:        groupTeams[p.Team1Idx],
					Team2:        groupTeams[p.Team2Idx],
					Status:       "pending",
					Round:        "rr",
					MatchOrder:   order,
					RoundNumber:  p.RoundNumber,
					GroupNumber:  g,
					CreatedAt:    time.Now(),
				}
				store.matches = append(store.matches, m)
				store.matchByID[m.ID] = m
				roundCounts[p.RoundNumber]++
				rrCount++
			}
		}

		// Create knockout placeholder matches
		if knockoutSize >= 8 {
			// QF: 4 matches
			for i := 0; i < 4; i++ {
				m := &Match{ID: newUUID(), TournamentID: tournID, Status: "pending", Round: "qf", MatchOrder: i, GroupNumber: -1, CreatedAt: time.Now()}
				store.matches = append(store.matches, m)
				store.matchByID[m.ID] = m
			}
		}
		// SF: 2 matches
		for i := 0; i < 2; i++ {
			m := &Match{ID: newUUID(), TournamentID: tournID, Status: "pending", Round: "sf", MatchOrder: i, GroupNumber: -1, CreatedAt: time.Now()}
			store.matches = append(store.matches, m)
			store.matchByID[m.ID] = m
		}
		// Final: 1 match
		final := &Match{ID: newUUID(), TournamentID: tournID, Status: "pending", Round: "final", MatchOrder: 0, GroupNumber: -1, CreatedAt: time.Now()}
		store.matches = append(store.matches, final)
		store.matchByID[final.ID] = final

		log.Printf("[Admin] bracket generated: %d groups, %d RR matches, knockout=%d", numGroups, rrCount, knockoutSize)
		c.JSON(http.StatusOK, gin.H{
			"tournament_id": tournID,
			"rr_matches":    rrCount,
			"num_groups":    numGroups,
			"knockout_size": knockoutSize,
			"teams":         len(store.teams),
		})
	}
}

// ---- Score entry ----

func handleAdminEnterScore() gin.HandlerFunc {
	return func(c *gin.Context) {
		matchID := c.Param("id")
		var req struct {
			Games []GameScore `json:"games"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.Games) == 0 || len(req.Games) > 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "must provide 1–3 games"})
			return
		}

		// Validate each game and tally wins.
		t1Wins, t2Wins := 0, 0
		for i, g := range req.Games {
			if g.Team1Score == nil || g.Team2Score == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("game %d: both scores are required", i+1)})
				return
			}
			if *g.Team1Score < 0 || *g.Team2Score < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("game %d: scores cannot be negative", i+1)})
				return
			}
			if *g.Team1Score == *g.Team2Score {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("game %d: scores cannot be tied", i+1)})
				return
			}
			if *g.Team1Score > *g.Team2Score {
				t1Wins++
			} else {
				t2Wins++
			}
		}

		// One team must have won 2 games (best of 3).
		if t1Wins < 2 && t2Wins < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "match not decided — a team needs 2 game wins"})
			return
		}

		log.Printf("[Admin] POST /api/admin/matches/%s/score: %d games played, t1Wins=%d t2Wins=%d", matchID, len(req.Games), t1Wins, t2Wins)

		store.mu.Lock()
		defer store.mu.Unlock()

		m, ok := store.matchByID[matchID]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "match not found"})
			return
		}

		wasComplete := m.Status == "complete"

		m.Games = req.Games
		m.Team1Score = &t1Wins
		m.Team2Score = &t2Wins
		m.Status = "complete"

		if m.Team1 != nil && m.Team2 != nil {
			if t1Wins > t2Wins {
				wid := m.Team1.ID
				m.WinnerTeamID = &wid
			} else {
				wid := m.Team2.ID
				m.WinnerTeamID = &wid
			}
		}

		// On first completion (pending→complete) always re-seed downstream stages.
		// On re-edit (complete→complete) only re-seed if downstream matches haven't
		// been played yet — prevents corrupting results already in progress.
		if !wasComplete {
			switch m.Round {
			case "rr":
				maybeAssignFromRR()
			case "qf":
				maybeAssignSFFromQF()
			case "sf":
				maybeAssignFinalFromSF()
			}
		} else {
			log.Printf("[Admin] score re-edit on match %s — checking downstream safety", matchID)
			switch m.Round {
			case "rr":
				knockoutStarted := false
				for _, km := range store.matches {
					if (km.Round == "qf" || km.Round == "sf" || km.Round == "final") && km.Status == "complete" {
						knockoutStarted = true
						break
					}
				}
				if !knockoutStarted {
					maybeAssignFromRR()
					log.Printf("[Admin] re-ran RR seeding after score re-edit (knockout not yet started)")
				}
			case "qf":
				sfStarted := false
				for _, sm := range store.matches {
					if sm.Round == "sf" && sm.Status == "complete" {
						sfStarted = true
						break
					}
				}
				if !sfStarted {
					maybeAssignSFFromQF()
					log.Printf("[Admin] re-ran QF→SF seeding after score re-edit")
				}
			case "sf":
				finalDone := false
				for _, fm := range store.matches {
					if fm.Round == "final" && fm.Status == "complete" {
						finalDone = true
						break
					}
				}
				if !finalDone {
					maybeAssignFinalFromSF()
					log.Printf("[Admin] re-ran SF→Final seeding after score re-edit")
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "was_edit": wasComplete})
	}
}

// ---- Standings helper ----

type standingEntry struct {
	team     *Team
	pts      int
	gd       int
	gf       int
	groupIdx int
	rank     int // 0 = 1st in group, 1 = 2nd, etc.
}

// computeGroupStandings returns standings per group based on completed RR matches.
// Must be called with store lock held.
func computeGroupStandings() [][]*standingEntry {
	if store.tournament == nil {
		return nil
	}
	numGroups := store.tournament.NumGroups
	result := make([][]*standingEntry, numGroups)

	for g := 0; g < numGroups; g++ {
		teamMap := map[string]*standingEntry{}
		for _, tid := range store.groupAssignments[g] {
			if t := store.teamByID[tid]; t != nil {
				teamMap[tid] = &standingEntry{team: t, groupIdx: g}
			}
		}

		for _, m := range store.matches {
			if m.Round != "rr" || m.GroupNumber != g || m.Status != "complete" {
				continue
			}
			if m.Team1 == nil || m.Team2 == nil {
				continue
			}
			s1, s2 := 0, 0
			if m.Team1Score != nil {
				s1 = *m.Team1Score
			}
			if m.Team2Score != nil {
				s2 = *m.Team2Score
			}
			st1 := teamMap[m.Team1.ID]
			st2 := teamMap[m.Team2.ID]
			if st1 == nil || st2 == nil {
				continue
			}
			st1.gd += s1 - s2; st1.gf += s1
			st2.gd += s2 - s1; st2.gf += s2
			if m.WinnerTeamID != nil && *m.WinnerTeamID == m.Team1.ID {
				st1.pts += 2
			} else if m.WinnerTeamID != nil && *m.WinnerTeamID == m.Team2.ID {
				st2.pts += 2
			} else {
				st1.pts++; st2.pts++
			}
		}

		entries := make([]*standingEntry, 0, len(teamMap))
		for _, e := range teamMap {
			entries = append(entries, e)
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].pts != entries[j].pts {
				return entries[i].pts > entries[j].pts
			}
			if entries[i].gd != entries[j].gd {
				return entries[i].gd > entries[j].gd
			}
			return entries[i].gf > entries[j].gf
		})
		for i, e := range entries {
			e.rank = i
		}
		result[g] = entries
	}
	return result
}

// maybeAssignFromRR checks if all RR matches are done and seeds the knockout stage.
// Must be called with store.mu held (write).
func maybeAssignFromRR() {
	for _, m := range store.matches {
		if m.Round == "rr" && m.Status == "pending" {
			return // still pending matches
		}
	}

	log.Printf("[Knockout] all RR complete — seeding knockout")
	gs := computeGroupStandings()
	numGroups := store.tournament.NumGroups
	knockoutSize := store.tournament.KnockoutSize

	// Helper: find a match by round and order
	findMatch := func(round string, order int) *Match {
		for _, m := range store.matches {
			if m.Round == round && m.MatchOrder == order {
				return m
			}
		}
		return nil
	}

	if knockoutSize == 4 {
		// Collect advancing teams
		var advancing []*standingEntry

		if numGroups == 1 {
			// Single group: top 4
			for i := 0; i < 4 && i < len(gs[0]); i++ {
				advancing = append(advancing, gs[0][i])
			}
		} else {
			// Multiple groups (3): top 1 from each group
			for g := 0; g < numGroups; g++ {
				if len(gs[g]) > 0 {
					advancing = append(advancing, gs[g][0])
				}
			}
			// Best runner-up across groups
			var runners []*standingEntry
			for g := 0; g < numGroups; g++ {
				if len(gs[g]) > 1 {
					runners = append(runners, gs[g][1])
				}
			}
			sort.Slice(runners, func(i, j int) bool {
				if runners[i].pts != runners[j].pts {
					return runners[i].pts > runners[j].pts
				}
				return runners[i].gd > runners[j].gd
			})
			if len(runners) > 0 {
				advancing = append(advancing, runners[0])
			}
		}

		// Global seed order: sort advancing teams by pts then GD
		sort.Slice(advancing, func(i, j int) bool {
			if advancing[i].pts != advancing[j].pts {
				return advancing[i].pts > advancing[j].pts
			}
			return advancing[i].gd > advancing[j].gd
		})

		// Seed 1v4, 2v3 into SF
		sf1 := findMatch("sf", 0)
		sf2 := findMatch("sf", 1)
		if sf1 != nil && len(advancing) >= 2 {
			sf1.Team1 = advancing[0].team
			if len(advancing) >= 4 {
				sf1.Team2 = advancing[3].team
			} else {
				sf1.Team2 = advancing[len(advancing)-1].team
			}
		}
		if sf2 != nil && len(advancing) >= 3 {
			sf2.Team1 = advancing[1].team
			sf2.Team2 = advancing[2].team
		}
		log.Printf("[Knockout] SF seeded with %d teams (knockoutSize=4)", len(advancing))

	} else { // knockoutSize == 8
		// 4 groups: top 2 from each
		var leaders, runners []*standingEntry
		for g := 0; g < numGroups; g++ {
			if len(gs[g]) > 0 {
				leaders = append(leaders, gs[g][0])
			}
			if len(gs[g]) > 1 {
				runners = append(runners, gs[g][1])
			}
		}
		// Sort leaders and runners independently by pts/GD
		rankSlice := func(s []*standingEntry) {
			sort.Slice(s, func(i, j int) bool {
				if s[i].pts != s[j].pts {
					return s[i].pts > s[j].pts
				}
				return s[i].gd > s[j].gd
			})
		}
		rankSlice(leaders)
		rankSlice(runners)
		seeded := append(leaders, runners...) // seeds 1-4 = leaders, 5-8 = runners

		// Standard QF seeding: 1v8, 2v7, 3v6, 4v5
		pairings := [][2]int{{0, 7}, {1, 6}, {2, 5}, {3, 4}}
		for i, pair := range pairings {
			qf := findMatch("qf", i)
			if qf != nil {
				if pair[0] < len(seeded) {
					qf.Team1 = seeded[pair[0]].team
				}
				if pair[1] < len(seeded) {
					qf.Team2 = seeded[pair[1]].team
				}
			}
		}
		log.Printf("[Knockout] QF seeded with %d teams (knockoutSize=8)", len(seeded))
	}
}

// maybeAssignSFFromQF seeds SF matches once all QF matches are complete.
// Must be called with store.mu held (write).
func maybeAssignSFFromQF() {
	var qfs [4]*Match
	for _, m := range store.matches {
		if m.Round == "qf" && m.MatchOrder >= 0 && m.MatchOrder < 4 {
			qfs[m.MatchOrder] = m
			if m.Status != "complete" {
				return // not all QF done
			}
		}
	}

	log.Printf("[Knockout] all QF complete — seeding SF")

	var sf1, sf2 *Match
	for _, m := range store.matches {
		if m.Round == "sf" && m.MatchOrder == 0 {
			sf1 = m
		}
		if m.Round == "sf" && m.MatchOrder == 1 {
			sf2 = m
		}
	}
	// SF1: QF1w vs QF2w (top quarter)
	// SF2: QF3w vs QF4w (bottom quarter)
	if sf1 != nil {
		if qfs[0] != nil && qfs[0].WinnerTeamID != nil {
			sf1.Team1 = store.teamByID[*qfs[0].WinnerTeamID]
		}
		if qfs[1] != nil && qfs[1].WinnerTeamID != nil {
			sf1.Team2 = store.teamByID[*qfs[1].WinnerTeamID]
		}
	}
	if sf2 != nil {
		if qfs[2] != nil && qfs[2].WinnerTeamID != nil {
			sf2.Team1 = store.teamByID[*qfs[2].WinnerTeamID]
		}
		if qfs[3] != nil && qfs[3].WinnerTeamID != nil {
			sf2.Team2 = store.teamByID[*qfs[3].WinnerTeamID]
		}
	}
}

// maybeAssignFinalFromSF seeds the Final once both SF matches are complete.
// Must be called with store.mu held (write).
func maybeAssignFinalFromSF() {
	var sf1, sf2 *Match
	for _, m := range store.matches {
		if m.Round == "sf" && m.MatchOrder == 0 {
			sf1 = m
		}
		if m.Round == "sf" && m.MatchOrder == 1 {
			sf2 = m
		}
	}
	if sf1 == nil || sf2 == nil || sf1.Status != "complete" || sf2.Status != "complete" {
		return
	}
	var finalMatch *Match
	for _, m := range store.matches {
		if m.Round == "final" {
			finalMatch = m
			break
		}
	}
	if finalMatch == nil {
		return
	}
	if sf1.WinnerTeamID != nil {
		finalMatch.Team1 = store.teamByID[*sf1.WinnerTeamID]
	}
	if sf2.WinnerTeamID != nil {
		finalMatch.Team2 = store.teamByID[*sf2.WinnerTeamID]
	}
	log.Printf("[Knockout] Final seeded from SF results")
}

// ---- Delete player ----

func handleAdminDeletePlayer() gin.HandlerFunc {
	return func(c *gin.Context) {
		playerID := c.Param("id")
		log.Printf("[Admin] DELETE /api/admin/players/%s", playerID)

		store.mu.Lock()
		defer store.mu.Unlock()

		if len(store.teams) > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "teams_formed", "message": "Cannot remove players after teams have been formed"})
			return
		}

		p, ok := store.playerByID[playerID]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		delete(store.playerByEmail, p.Email)
		delete(store.playerByID, playerID)
		for i, pl := range store.players {
			if pl.ID == playerID {
				store.players = append(store.players[:i], store.players[i+1:]...)
				break
			}
		}

		log.Printf("[Admin] deleted player: id=%s name=%s (remaining: %d)", playerID, p.Name, len(store.players))
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ---- Forfeit match ----

func handleAdminForfeitMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		matchID := c.Param("id")
		var req struct {
			Forfeiter string `json:"forfeiter"` // "team1" or "team2"
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Forfeiter != "team1" && req.Forfeiter != "team2" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "forfeiter must be 'team1' or 'team2'"})
			return
		}
		log.Printf("[Admin] POST /api/admin/matches/%s/forfeit: forfeiter=%s", matchID, req.Forfeiter)

		store.mu.Lock()
		defer store.mu.Unlock()

		m, ok := store.matchByID[matchID]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "match not found"})
			return
		}
		if m.Team1 == nil || m.Team2 == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "both teams must be assigned before marking a forfeit"})
			return
		}

		wasComplete := m.Status == "complete"

		if req.Forfeiter == "team1" {
			// team1 forfeits: loses 0–2 games (11-0 per game for the winner)
			m.Games = []GameScore{
				{Team1Score: intPtr(0), Team2Score: intPtr(11)},
				{Team1Score: intPtr(0), Team2Score: intPtr(11)},
			}
			s1, s2 := 0, 2
			m.Team1Score = &s1
			m.Team2Score = &s2
			wid := m.Team2.ID
			m.WinnerTeamID = &wid
		} else {
			// team2 forfeits: loses 0–2 games
			m.Games = []GameScore{
				{Team1Score: intPtr(11), Team2Score: intPtr(0)},
				{Team1Score: intPtr(11), Team2Score: intPtr(0)},
			}
			s1, s2 := 2, 0
			m.Team1Score = &s1
			m.Team2Score = &s2
			wid := m.Team1.ID
			m.WinnerTeamID = &wid
		}
		m.Status = "complete"

		if !wasComplete {
			switch m.Round {
			case "rr":
				maybeAssignFromRR()
			case "qf":
				maybeAssignSFFromQF()
			case "sf":
				maybeAssignFinalFromSF()
			}
		}

		log.Printf("[Admin] forfeit applied to match %s, forfeiter=%s", matchID, req.Forfeiter)
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// ---- Seed (bulk bootstrap) ----

// handleAdminSeed populates players and teams in one shot without individual registration.
// Only works on an empty store — returns 409 if data already exists.
func handleAdminSeed() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Teams []struct {
				Player1Name  string `json:"player1_name"`
				Player1Email string `json:"player1_email"`
				Player2Name  string `json:"player2_name"`
				Player2Email string `json:"player2_email"`
			} `json:"teams"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.Teams) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no teams provided"})
			return
		}

		store.mu.Lock()
		defer store.mu.Unlock()

		if len(store.players) > 0 || len(store.teams) > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "store already has data — reset first"})
			return
		}

		now := time.Now()
		for _, t := range req.Teams {
			p1 := &Player{ID: newUUID(), Name: t.Player1Name, Email: t.Player1Email, RegisteredAt: now}
			p2 := &Player{ID: newUUID(), Name: t.Player2Name, Email: t.Player2Email, RegisteredAt: now}
			store.players = append(store.players, p1, p2)
			store.playerByEmail[p1.Email] = p1
			store.playerByID[p1.ID] = p1
			store.playerByEmail[p2.Email] = p2
			store.playerByID[p2.ID] = p2
			team := &Team{ID: newUUID(), Player1: p1, Player2: p2, CreatedAt: now}
			store.teams = append(store.teams, team)
			store.teamByID[team.ID] = team
		}
		store.registrationOpen = false

		log.Printf("[Admin] seed: populated %d teams (%d players)", len(store.teams), len(store.players))
		c.JSON(http.StatusOK, gin.H{"ok": true, "teams": len(store.teams), "players": len(store.players)})
	}
}

// ---- Reset ----

func handleAdminReset() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/reset")
		store.mu.Lock()
		store.registrationOpen = true
		store.players = nil
		store.playerByEmail = make(map[string]*Player)
		store.playerByID = make(map[string]*Player)
		store.teams = nil
		store.teamByID = make(map[string]*Team)
		store.tournament = nil
		store.matches = nil
		store.matchByID = make(map[string]*Match)
		store.groupAssignments = nil
		store.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ---- Test data generation ----

var testFirstNames = []string{
	"Alice", "Bob", "Charlie", "Diana", "Ethan", "Fiona", "George", "Hannah",
	"Ivan", "Julia", "Kevin", "Laura", "Mike", "Nina", "Oscar", "Priya",
	"Quinn", "Rachel", "Sam", "Tina", "Uma", "Victor", "Wendy", "Xavier",
	"Yara", "Zack", "Aria", "Ben", "Chloe", "David", "Emma", "Frank",
	"Grace", "Henry", "Iris", "Jack", "Kate", "Leo", "Maya", "Noah",
	"Olivia", "Peter", "Rose", "Stefan", "Tracy", "Umar", "Vera", "Will",
}

var testLastNames = []string{
	"Smith", "Johnson", "Chen", "Patel", "Kim", "Garcia", "Rodriguez", "Lee",
	"Wang", "Martinez", "Anderson", "Taylor", "Thomas", "Moore", "Jackson",
	"Martin", "Thompson", "White", "Lopez", "Wilson", "Brown", "Davis",
	"Miller", "Jones", "Williams", "Clark", "Lewis", "Walker", "Hall", "Allen",
	"Young", "Scott", "Green", "Adams", "Baker", "Nelson", "Hill", "Rivera",
}

func handleAdminGenerateTestPlayers() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Count int `json:"count"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Count < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "count must be a positive integer"})
			return
		}
		if req.Count > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max 100 at once"})
			return
		}
		log.Printf("[Admin] POST /api/admin/test/generate: count=%d", req.Count)

		store.mu.Lock()
		defer store.mu.Unlock()

		created := 0
		for attempts := 0; created < req.Count && attempts < req.Count*20; attempts++ {
			fn := testFirstNames[mathrand.Intn(len(testFirstNames))]
			ln := testLastNames[mathrand.Intn(len(testLastNames))]
			email := fmt.Sprintf("%s.%s.%04d@applied.dev",
				strings.ToLower(fn), strings.ToLower(ln), mathrand.Intn(9999)+1)

			if _, exists := store.playerByEmail[email]; exists {
				continue
			}
			p := &Player{
				ID:           newUUID(),
				Name:         fn + " " + ln,
				Email:        email,
				RegisteredAt: time.Now().Add(time.Duration(created) * time.Second),
			}
			store.players = append(store.players, p)
			store.playerByEmail[p.Email] = p
			store.playerByID[p.ID] = p
			created++
		}

		log.Printf("[Admin] generated %d test players (total now: %d)", created, len(store.players))
		c.JSON(http.StatusOK, gin.H{"created": created, "total_players": len(store.players)})
	}
}

// ---- Seed data ----

// seedTournamentData pre-populates the store with the real tournament players and teams.
// It only runs if the store is empty — safe to call on every startup.
func seedTournamentData() {
	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.players) > 0 {
		log.Printf("[Seed] store already has data (%d players), skipping", len(store.players))
		return
	}

	type pd struct{ name, email string }
	playerDefs := []pd{
		{"Pranjal Sinha", "pranjal.sinha@applied.co"},
		{"Ameet Chhatwal", "ameet.chhatwal@applied.co"},
		{"Sanjit Kalapatapu", "sanjit.kalapatapu@applied.co"},
		{"Colby Cheung", "colby.cheung@applied.co"},
		{"Yash Kshirsagar", "yash.kshirsagar@applied.co"},
		{"Sean Nelson", "sean.nelson@applied.co"},
		{"Sneha Ganesh", "sneha.ganesh@applied.co"},
		{"Bahar Kholdi-Sabeti", "bahar.kholdi-sabeti@applied.co"},
		{"Cameron Stewart", "cameron.stewart@applied.co"},
		{"Heather Wattles", "heather.wattles@applied.co"},
		{"Isiah Montalvo", "isiah.montalvo@applied.co"},
		{"Kesha Srivatsan", "kesha.srivatsan@applied.co"},
		{"Harris Muhammad", "harris.muhammad@applied.co"},
		{"Hridayesh Joshi", "hridayesh.joshi@applied.co"},
		{"David Yi Yang", "david.yi.yang@applied.co"},
		{"Prakhar Kumar", "prakhar.kumar@applied.co"},
		{"Kaushek Kumar", "kaushek.kumar@applied.co"},
		{"Seonghyun Park", "seonghyun.park@applied.co"},
		{"Yanzhao Yang", "yanzhao.yang@applied.co"},
		{"Punit Tulpule", "punit.tulpule@applied.co"},
		{"Monica Hsu", "monica.hsu@applied.co"},
		{"Aditya J", "aditya.j@applied.co"},
		{"Vivrd Prasanna", "vivrd.prasanna@applied.co"},
		{"Avinash Divecha", "avinash.divecha@applied.co"},
	}

	byName := map[string]*Player{}
	for _, def := range playerDefs {
		p := &Player{
			ID:           newUUID(),
			Name:         def.name,
			Email:        def.email,
			RegisteredAt: time.Now(),
		}
		store.players = append(store.players, p)
		store.playerByEmail[p.Email] = p
		store.playerByID[p.ID] = p
		byName[def.name] = p
	}

	type td struct{ p1, p2 string }
	teamDefs := []td{
		{"Pranjal Sinha", "Ameet Chhatwal"},
		{"Sanjit Kalapatapu", "Colby Cheung"},
		{"Yash Kshirsagar", "Sean Nelson"},
		{"Sneha Ganesh", "Bahar Kholdi-Sabeti"},
		{"Cameron Stewart", "Heather Wattles"},
		{"Isiah Montalvo", "Kesha Srivatsan"},
		{"Harris Muhammad", "Hridayesh Joshi"},
		{"David Yi Yang", "Prakhar Kumar"},
		{"Kaushek Kumar", "Seonghyun Park"},
		{"Yanzhao Yang", "Punit Tulpule"},
		{"Monica Hsu", "Aditya J"},
		{"Vivrd Prasanna", "Avinash Divecha"},
	}

	for _, def := range teamDefs {
		p1, p2 := byName[def.p1], byName[def.p2]
		if p1 == nil || p2 == nil {
			log.Printf("[Seed] WARNING: player not found: %q / %q", def.p1, def.p2)
			continue
		}
		t := &Team{
			ID:        newUUID(),
			Player1:   p1,
			Player2:   p2,
			Locked:    true,
			CreatedAt: time.Now(),
		}
		store.teams = append(store.teams, t)
		store.teamByID[t.ID] = t
	}

	store.registrationOpen = false
	log.Printf("[Seed] seeded %d players, %d teams — registration closed", len(store.players), len(store.teams))
}

// ---- Route registration ----

func registerTournamentRoutes(api *gin.RouterGroup) {
	api.GET("/tournament/status", handleTournamentStatus())
	api.POST("/register", handleRegister())
	api.GET("/tournament/bracket", handleGetBracket())

	admin := api.Group("/admin")
	admin.Use(adminAuthMiddleware())
	admin.POST("/auth", handleAdminAuth())
	admin.GET("/players", handleAdminGetPlayers())
	admin.POST("/registration/close", handleAdminCloseRegistration())
	admin.POST("/registration/open", handleAdminOpenRegistration())
	admin.POST("/teams/randomize", handleAdminRandomizeTeams())
	admin.GET("/teams", handleAdminGetTeams())
	admin.PUT("/teams/swap", handleAdminSwapPlayers())
	admin.POST("/bracket/generate", handleAdminGenerateBracket())
	admin.POST("/matches/:id/score", handleAdminEnterScore())
	admin.POST("/matches/:id/forfeit", handleAdminForfeitMatch())
	admin.DELETE("/players/:id", handleAdminDeletePlayer())
	admin.POST("/reset", handleAdminReset())
	admin.POST("/seed", handleAdminSeed())
	admin.POST("/test/generate", handleAdminGenerateTestPlayers())
	registerScheduleRoutes(admin)

	log.Printf("[Tournament] routes registered")
}
