package main

// Google Sheets persistence + live two-way sync.
//
//   - Every mutation writes the full state to the sheet (write-through) so a redeploy /
//     restart never loses data: the "State" tab holds the authoritative JSON snapshot,
//     while "Teams", "Matches" and "Standings" are human-readable views.
//   - On boot the store is restored from the "State" tab first (before Secret Manager / GCS /
//     the hardcoded Friday fallback).
//   - Admins can edit player names/emails in "Teams" and per-game scores in "Matches"; a
//     background poller (and POST /api/admin/sheets/pull) applies those edits with exactly
//     the same rules as the admin API. Blank score cells never un-complete a match and rows
//     are matched by ID, so nothing can be deleted from the sheet side.
//
// Uses the Sheets REST API v4 directly through net/http + encoding/json with the runtime
// service account token (golang.org/x/oauth2/google) — no new Go modules.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	sheetsScope   = "https://www.googleapis.com/auth/spreadsheets"
	sheetsAPIBase = "https://sheets.googleapis.com/v4/spreadsheets/"

	tabTeams     = "Teams"
	tabMatches   = "Matches"
	tabStandings = "Standings"
	tabState     = "State"

	// Google caps a cell at 50,000 characters; the JSON snapshot is split across cells.
	stateChunkSize = 40000
)

var (
	teamsHeader = []string{"team_id", "group", "team_no", "player1_id", "player1_name", "player1_email", "player2_id", "player2_name", "player2_email"}
	// g1..g3 are the editable per-game point scores; everything else is rewritten by the app.
	matchesHeader   = []string{"match_id", "stage", "group", "round", "order", "team1_id", "team1", "team2_id", "team2", "g1_t1", "g1_t2", "g2_t1", "g2_t2", "g3_t1", "g3_t2", "games_t1", "games_t2", "winner", "status", "scheduled_time", "table"}
	standingsHeader = []string{"group", "rank", "team", "played", "won", "lost", "points", "game_diff"}
	groupLetters    = "ABCDEFGH"
)

var (
	sheetsTokenOnce sync.Once
	sheetsTokenSrc  oauth2.TokenSource
	sheetsTokenErr  error

	// sheetsIO serialises pushes and pulls so a pull never reads a half-written sheet.
	sheetsIO sync.Mutex

	sheetsStatusMu    sync.Mutex
	sheetsLastPush    time.Time
	sheetsLastPull    time.Time
	sheetsLastError   string
	sheetsTabsChecked bool
	sheetsPollerOnce  sync.Once
	// lastLocalMutation is bumped on every app-side save; the poller backs off right after it
	// so it never re-applies a stale sheet value over an edit that is still being written.
	lastLocalMutation time.Time
)

func sheetsID() string { return strings.TrimSpace(os.Getenv("SHEETS_SPREADSHEET_ID")) }

func sheetsEnabled() bool { return sheetsID() != "" }

func sheetsURL() string {
	if !sheetsEnabled() {
		return ""
	}
	return "https://docs.google.com/spreadsheets/d/" + sheetsID() + "/edit"
}

func sheetsPollInterval() time.Duration {
	if v := os.Getenv("SHEETS_POLL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n <= 0 {
				return 0
			}
			return time.Duration(n) * time.Second
		}
	}
	return 30 * time.Second
}

func setSheetsError(err error) {
	sheetsStatusMu.Lock()
	defer sheetsStatusMu.Unlock()
	if err == nil {
		sheetsLastError = ""
	} else {
		sheetsLastError = err.Error()
	}
}

// ---- REST helpers ----

func sheetsToken(ctx context.Context) (string, error) {
	sheetsTokenOnce.Do(func() {
		sheetsTokenSrc, sheetsTokenErr = google.DefaultTokenSource(context.Background(), sheetsScope)
	})
	if sheetsTokenErr != nil {
		return "", sheetsTokenErr
	}
	tok, err := sheetsTokenSrc.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

// sheetsCall performs one Sheets API request. path is appended to the spreadsheet URL
// (e.g. "/values:batchGet?ranges=Teams"). A nil out skips response decoding.
func sheetsCall(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	tok, err := sheetsToken(ctx)
	if err != nil {
		return fmt.Errorf("sheets token: %w", err)
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, sheetsAPIBase+sheetsID()+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sheets %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		msg := string(data)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return fmt.Errorf("sheets %s %s: status %d: %s", method, strings.SplitN(path, "?", 2)[0], resp.StatusCode, msg)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// ensureSheetTabs creates any missing tabs (once per process).
func ensureSheetTabs(ctx context.Context) error {
	sheetsStatusMu.Lock()
	done := sheetsTabsChecked
	sheetsStatusMu.Unlock()
	if done {
		return nil
	}
	var meta struct {
		Sheets []struct {
			Properties struct {
				Title string `json:"title"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if err := sheetsCall(ctx, "GET", "?fields=sheets.properties.title", nil, &meta); err != nil {
		return err
	}
	have := map[string]bool{}
	for _, s := range meta.Sheets {
		have[s.Properties.Title] = true
	}
	var requests []map[string]interface{}
	for _, want := range []string{tabTeams, tabMatches, tabStandings, tabState} {
		if !have[want] {
			requests = append(requests, map[string]interface{}{
				"addSheet": map[string]interface{}{"properties": map[string]interface{}{"title": want}},
			})
		}
	}
	if len(requests) > 0 {
		if err := sheetsCall(ctx, "POST", ":batchUpdate", map[string]interface{}{"requests": requests}, nil); err != nil {
			return err
		}
		log.Printf("[Sheets] created %d missing tab(s)", len(requests))
	}
	sheetsStatusMu.Lock()
	sheetsTabsChecked = true
	sheetsStatusMu.Unlock()
	return nil
}

// ---- Row builders (store lock must be held by the caller) ----

func groupLetterOf(teamID string) string {
	for g, ids := range store.groupAssignments {
		for _, id := range ids {
			if id == teamID && g < len(groupLetters) {
				return string(groupLetters[g])
			}
		}
	}
	return ""
}

func cellInt(p *int) interface{} {
	if p == nil {
		return ""
	}
	return *p
}

func buildTeamsRows() [][]interface{} {
	rows := [][]interface{}{toIface(teamsHeader)}
	for i, t := range store.teams {
		if t == nil {
			continue
		}
		row := []interface{}{t.ID, groupLetterOf(t.ID), i + 1}
		for _, p := range []*Player{t.Player1, t.Player2} {
			if p != nil {
				row = append(row, p.ID, p.Name, p.Email)
			} else {
				row = append(row, "", "", "")
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func stageOrder(round string) int {
	switch round {
	case "rr":
		return 0
	case "qf":
		return 1
	case "sf":
		return 2
	case "final":
		return 3
	}
	return 9
}

func buildMatchesRows() [][]interface{} {
	sorted := make([]*Match, 0, len(store.matches))
	for _, m := range store.matches {
		if m != nil {
			sorted = append(sorted, m)
		}
	}
	// Stable ordering: stage, group, round, order.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && matchLess(sorted[j], sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	rows := [][]interface{}{toIface(matchesHeader)}
	for _, m := range sorted {
		group := ""
		if m.Round == "rr" && m.GroupNumber >= 0 && m.GroupNumber < len(groupLetters) {
			group = string(groupLetters[m.GroupNumber])
		}
		t1ID, t2ID := "", ""
		if m.Team1 != nil {
			t1ID = m.Team1.ID
		}
		if m.Team2 != nil {
			t2ID = m.Team2.ID
		}
		row := []interface{}{m.ID, m.Round, group, m.RoundNumber + 1, m.MatchOrder + 1, t1ID, teamLabel(m.Team1), t2ID, teamLabel(m.Team2)}
		for g := 0; g < 3; g++ {
			if g < len(m.Games) {
				row = append(row, cellInt(m.Games[g].Team1Score), cellInt(m.Games[g].Team2Score))
			} else {
				row = append(row, "", "")
			}
		}
		winner := ""
		if m.WinnerTeamID != nil {
			if t := store.teamByID[*m.WinnerTeamID]; t != nil {
				winner = teamLabel(t)
			}
		}
		sched := ""
		if m.ScheduledTime != nil {
			sched = m.ScheduledTime.Format(time.RFC3339)
		}
		row = append(row, cellInt(m.Team1Score), cellInt(m.Team2Score), winner, m.Status, sched, m.TableNum)
		rows = append(rows, row)
	}
	return rows
}

func matchLess(a, b *Match) bool {
	if stageOrder(a.Round) != stageOrder(b.Round) {
		return stageOrder(a.Round) < stageOrder(b.Round)
	}
	if a.GroupNumber != b.GroupNumber {
		return a.GroupNumber < b.GroupNumber
	}
	if a.RoundNumber != b.RoundNumber {
		return a.RoundNumber < b.RoundNumber
	}
	return a.MatchOrder < b.MatchOrder
}

func buildStandingsRows() [][]interface{} {
	rows := [][]interface{}{toIface(standingsHeader)}
	for g, entries := range computeGroupStandings() {
		letter := ""
		if g < len(groupLetters) {
			letter = string(groupLetters[g])
		}
		for _, e := range entries {
			played, won := 0, 0
			for _, m := range store.matches {
				if m == nil || m.Round != "rr" || m.Status != "complete" || m.Team1 == nil || m.Team2 == nil {
					continue
				}
				if m.Team1.ID != e.team.ID && m.Team2.ID != e.team.ID {
					continue
				}
				played++
				if m.WinnerTeamID != nil && *m.WinnerTeamID == e.team.ID {
					won++
				}
			}
			rows = append(rows, []interface{}{letter, e.rank + 1, teamLabel(e.team), played, won, played - won, e.pts, e.gd})
		}
	}
	return rows
}

func teamLabel(t *Team) string {
	if t == nil {
		return "TBD"
	}
	n1, n2 := "?", "?"
	if t.Player1 != nil {
		n1 = t.Player1.Name
	}
	if t.Player2 != nil {
		n2 = t.Player2.Name
	}
	return n1 + " & " + n2
}

func toIface(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// ---- Push (app → sheet) ----

// pushStateToSheet rewrites all four tabs. data is the JSON snapshot already produced by
// saveStateWithFallback; the readable tabs are built from the store under a read lock.
func pushStateToSheet(ctx context.Context, data []byte) error {
	if !sheetsEnabled() {
		return nil
	}
	sheetsIO.Lock()
	defer sheetsIO.Unlock()

	if err := ensureSheetTabs(ctx); err != nil {
		setSheetsError(err)
		return err
	}

	store.mu.RLock()
	teamsRows := buildTeamsRows()
	matchesRows := buildMatchesRows()
	standingsRows := buildStandingsRows()
	store.mu.RUnlock()

	stateRows := [][]interface{}{
		{"saved_at", time.Now().UTC().Format(time.RFC3339)},
		{"note", "Machine-readable snapshot used to restore the app on restart. Edit names in the Teams tab and scores in the Matches tab instead of touching this."},
	}
	js := string(data)
	for i := 0; i < len(js); i += stateChunkSize {
		end := i + stateChunkSize
		if end > len(js) {
			end = len(js)
		}
		stateRows = append(stateRows, []interface{}{fmt.Sprintf("json_%02d", len(stateRows)-2), js[i:end]})
	}

	clearBody := map[string]interface{}{"ranges": []string{tabTeams, tabMatches, tabStandings, tabState}}
	if err := sheetsCall(ctx, "POST", "/values:batchClear", clearBody, nil); err != nil {
		setSheetsError(err)
		return err
	}
	updateBody := map[string]interface{}{
		"valueInputOption": "RAW",
		"data": []map[string]interface{}{
			{"range": tabTeams + "!A1", "values": teamsRows},
			{"range": tabMatches + "!A1", "values": matchesRows},
			{"range": tabStandings + "!A1", "values": standingsRows},
			{"range": tabState + "!A1", "values": stateRows},
		},
	}
	if err := sheetsCall(ctx, "POST", "/values:batchUpdate", updateBody, nil); err != nil {
		setSheetsError(err)
		return err
	}
	setSheetsError(nil)
	sheetsStatusMu.Lock()
	sheetsLastPush = time.Now()
	sheetsStatusMu.Unlock()
	log.Printf("[Sheets] pushed %d teams, %d matches, %d bytes of state", len(teamsRows)-1, len(matchesRows)-1, len(data))
	return nil
}

// ---- Load on boot (sheet → store) ----

// loadStateFromSheet restores the store from the State tab. Returns true when restored.
func loadStateFromSheet(ctx context.Context) bool {
	if !sheetsEnabled() {
		return false
	}
	sheetsIO.Lock()
	defer sheetsIO.Unlock()

	if err := ensureSheetTabs(ctx); err != nil {
		log.Printf("[Sheets] load skipped — %v", err)
		setSheetsError(err)
		return false
	}
	var resp struct {
		Values [][]string `json:"values"`
	}
	if err := sheetsCall(ctx, "GET", "/values/"+url.PathEscape(tabState+"!A1:B200"), nil, &resp); err != nil {
		log.Printf("[Sheets] load failed — %v", err)
		setSheetsError(err)
		return false
	}
	var sb strings.Builder
	for _, row := range resp.Values {
		if len(row) >= 2 && strings.HasPrefix(row[0], "json") {
			sb.WriteString(row[1])
		}
	}
	if sb.Len() == 0 {
		log.Printf("[Sheets] State tab is empty — nothing to restore")
		return false
	}
	var snap persistedState
	if err := json.Unmarshal([]byte(sb.String()), &snap); err != nil {
		log.Printf("[Sheets] State tab JSON invalid — %v", err)
		setSheetsError(fmt.Errorf("state json: %w", err))
		return false
	}
	if len(snap.Players) == 0 && len(snap.Teams) == 0 {
		log.Printf("[Sheets] State tab snapshot is empty — nothing to restore")
		return false
	}
	if !restoreSnap(snap) {
		return false
	}
	log.Printf("[Sheets] state restored from Google Sheet (saved %s)", snap.SavedAt.Format(time.RFC3339))
	setSheetsError(nil)
	return true
}

// ---- Pull (sheet edits → store) ----

type sheetPullResult struct {
	NameChanges  int      `json:"name_changes"`
	ScoreChanges int      `json:"score_changes"`
	Warnings     []string `json:"warnings"`
}

// syncFromSheet applies human edits from the Teams and Matches tabs. Only known IDs are
// touched; blank score cells are ignored; invalid rows are reported, never applied.
func syncFromSheet(ctx context.Context) (sheetPullResult, error) {
	res := sheetPullResult{Warnings: []string{}}
	if !sheetsEnabled() {
		return res, fmt.Errorf("sheets not configured (SHEETS_SPREADSHEET_ID unset)")
	}
	sheetsIO.Lock()
	defer sheetsIO.Unlock()

	var resp struct {
		ValueRanges []struct {
			Range  string     `json:"range"`
			Values [][]string `json:"values"`
		} `json:"valueRanges"`
	}
	q := "/values:batchGet?ranges=" + url.QueryEscape(tabTeams) + "&ranges=" + url.QueryEscape(tabMatches)
	if err := sheetsCall(ctx, "GET", q, nil, &resp); err != nil {
		setSheetsError(err)
		return res, err
	}
	var teamsRows, matchesRows [][]string
	for _, vr := range resp.ValueRanges {
		switch {
		case strings.HasPrefix(vr.Range, tabTeams):
			teamsRows = vr.Values
		case strings.HasPrefix(vr.Range, tabMatches):
			matchesRows = vr.Values
		}
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	// Teams: columns by header name so reordering columns in the sheet is harmless.
	if len(teamsRows) > 1 {
		col := headerIndex(teamsRows[0])
		for _, row := range teamsRows[1:] {
			tid := cellAt(row, col["team_id"])
			t := store.teamByID[tid]
			if t == nil {
				continue
			}
			for _, side := range []struct {
				p       *Player
				nameCol string
				mailCol string
			}{{t.Player1, "player1_name", "player1_email"}, {t.Player2, "player2_name", "player2_email"}} {
				if side.p == nil {
					continue
				}
				if name := strings.TrimSpace(cellAt(row, col[side.nameCol])); name != "" && name != side.p.Name {
					log.Printf("[Sheets] player %s renamed %q → %q (from sheet)", side.p.ID, side.p.Name, name)
					side.p.Name = name
					res.NameChanges++
				}
				if email := strings.TrimSpace(cellAt(row, col[side.mailCol])); email != "" && email != side.p.Email {
					delete(store.playerByEmail, side.p.Email)
					side.p.Email = email
					store.playerByEmail[email] = side.p
					res.NameChanges++
				}
			}
		}
	}

	// Matches: per-game scores.
	if len(matchesRows) > 1 {
		col := headerIndex(matchesRows[0])
		for _, row := range matchesRows[1:] {
			mid := cellAt(row, col["match_id"])
			m := store.matchByID[mid]
			if m == nil {
				continue
			}
			var games []GameScore
			incomplete := false
			for g := 1; g <= 3; g++ {
				a := strings.TrimSpace(cellAt(row, col[fmt.Sprintf("g%d_t1", g)]))
				b := strings.TrimSpace(cellAt(row, col[fmt.Sprintf("g%d_t2", g)]))
				if a == "" && b == "" {
					continue
				}
				ai, errA := strconv.Atoi(a)
				bi, errB := strconv.Atoi(b)
				if errA != nil || errB != nil {
					incomplete = true
					break
				}
				games = append(games, GameScore{Team1Score: intPtr(ai), Team2Score: intPtr(bi)})
			}
			label := teamLabel(m.Team1) + " vs " + teamLabel(m.Team2)
			if incomplete {
				res.Warnings = append(res.Warnings, label+": a game has only one score filled in — not applied")
				continue
			}
			if len(games) == 0 || gamesEqual(games, m.Games) {
				continue
			}
			if m.Team1 == nil || m.Team2 == nil {
				res.Warnings = append(res.Warnings, label+": teams not assigned yet — score not applied")
				continue
			}
			t1, t2, err := validateGames(games)
			if err != nil {
				res.Warnings = append(res.Warnings, label+": "+err.Error()+" — not applied")
				continue
			}
			applyMatchScore(m, games, t1, t2)
			log.Printf("[Sheets] score applied from sheet: %s → %d–%d games", label, t1, t2)
			res.ScoreChanges++
		}
	}

	sheetsStatusMu.Lock()
	sheetsLastPull = time.Now()
	sheetsStatusMu.Unlock()
	setSheetsError(nil)
	return res, nil
}

func headerIndex(header []string) map[string]int {
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	return idx
}

// cellAt returns row[i] or "" when the column is missing (Sheets trims trailing blanks).
func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func gamesEqual(a, b []GameScore) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Team1Score == nil || b[i].Team1Score == nil || a[i].Team2Score == nil || b[i].Team2Score == nil {
			return false
		}
		if *a[i].Team1Score != *b[i].Team1Score || *a[i].Team2Score != *b[i].Team2Score {
			return false
		}
	}
	return true
}

// ---- Poller ----

func startSheetsPoller() {
	if !sheetsEnabled() {
		log.Printf("[Sheets] SHEETS_SPREADSHEET_ID not set — Google Sheets sync disabled")
		return
	}
	interval := sheetsPollInterval()
	if interval == 0 {
		log.Printf("[Sheets] poller disabled (SHEETS_POLL_SECONDS=0); use POST /api/admin/sheets/pull")
		return
	}
	sheetsPollerOnce.Do(func() {
		go func() {
			log.Printf("[Sheets] poller started (every %s) — %s", interval, sheetsURL())
			for {
				time.Sleep(interval)
				sheetsStatusMu.Lock()
				recent := time.Since(lastLocalMutation) < 10*time.Second
				sheetsStatusMu.Unlock()
				if recent {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
				res, err := syncFromSheet(ctx)
				cancel()
				if err != nil {
					log.Printf("[Sheets] poll failed: %v", err)
					continue
				}
				for _, w := range res.Warnings {
					log.Printf("[Sheets] poll warning: %s", w)
				}
				if res.NameChanges+res.ScoreChanges > 0 {
					log.Printf("[Sheets] poll applied %d name and %d score change(s) from the sheet", res.NameChanges, res.ScoreChanges)
					go saveStateWithFallback(context.Background())
				}
			}
		}()
	})
}

// ---- Admin endpoints ----

func sheetsStatusJSON() map[string]interface{} {
	sheetsStatusMu.Lock()
	defer sheetsStatusMu.Unlock()
	fmtTime := func(t time.Time) interface{} {
		if t.IsZero() {
			return nil
		}
		return t.UTC().Format(time.RFC3339)
	}
	return map[string]interface{}{
		"enabled":      sheetsEnabled(),
		"url":          sheetsURL(),
		"poll_seconds": int(sheetsPollInterval() / time.Second),
		"last_push":    fmtTime(sheetsLastPush),
		"last_pull":    fmtTime(sheetsLastPull),
		"last_error":   sheetsLastError,
	}
}

func registerSheetsRoutes(admin *gin.RouterGroup) {
	admin.GET("/sheets/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, sheetsStatusJSON())
	})
	admin.POST("/sheets/push", func(c *gin.Context) {
		if !sheetsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sheets not configured"})
			return
		}
		saveStateWithFallback(context.Background())
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": sheetsStatusJSON()})
	})
	admin.POST("/sheets/pull", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		res, err := syncFromSheet(ctx)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		if res.NameChanges+res.ScoreChanges > 0 {
			go saveStateWithFallback(context.Background())
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "result": res, "status": sheetsStatusJSON()})
	})

	// Full JSON backup / restore — the same shape the State tab and Secret Manager use.
	admin.GET("/export", func(c *gin.Context) {
		store.mu.RLock()
		snap := persistedState{
			Players:          store.players,
			Teams:            store.teams,
			Tournament:       store.tournament,
			Matches:          store.matches,
			GroupAssignments: store.groupAssignments,
			RegistrationOpen: store.registrationOpen,
			SavedAt:          time.Now(),
		}
		store.mu.RUnlock()
		c.Header("Content-Disposition", "attachment; filename=ping-pong-state.json")
		c.JSON(http.StatusOK, snap)
	})
	admin.POST("/import", handleAdminImport())
}

// handleAdminImport restores a full snapshot. It accepts either the export shape
// (players/teams/matches/group_assignments) or the public /api/tournament/bracket shape
// (teams/matches/groups, no players) — players and group assignments are derived when absent.
// The request is refused unless it carries at least as many matches as the current store, so a
// stale or partial file can never silently shrink the tournament.
func handleAdminImport() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			persistedState
			Groups [][]*Team `json:"groups"`
			Force  bool      `json:"force"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		snap := req.persistedState
		if len(snap.Teams) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "snapshot has no teams"})
			return
		}
		if len(snap.Players) == 0 {
			seen := map[string]bool{}
			for _, t := range snap.Teams {
				for _, p := range []*Player{t.Player1, t.Player2} {
					if p != nil && !seen[p.ID] {
						seen[p.ID] = true
						snap.Players = append(snap.Players, p)
					}
				}
			}
		}
		if len(snap.GroupAssignments) == 0 && len(req.Groups) > 0 {
			for _, grp := range req.Groups {
				ids := make([]string, 0, len(grp))
				for _, t := range grp {
					if t != nil {
						ids = append(ids, t.ID)
					}
				}
				snap.GroupAssignments = append(snap.GroupAssignments, ids)
			}
		}
		store.mu.RLock()
		curMatches := len(store.matches)
		store.mu.RUnlock()
		if len(snap.Matches) < curMatches && !req.Force {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "snapshot has fewer matches than the live store — refusing to shrink data (send force:true to override)",
				"current": curMatches, "snapshot": len(snap.Matches),
			})
			return
		}
		if !restoreSnap(snap) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "snapshot could not be applied"})
			return
		}
		log.Printf("[Admin] imported snapshot: %d players, %d teams, %d matches", len(snap.Players), len(snap.Teams), len(snap.Matches))
		saveStateWithFallback(context.Background())
		c.JSON(http.StatusOK, gin.H{"ok": true, "players": len(snap.Players), "teams": len(snap.Teams), "matches": len(snap.Matches), "sheets": sheetsStatusJSON()})
	}
}
