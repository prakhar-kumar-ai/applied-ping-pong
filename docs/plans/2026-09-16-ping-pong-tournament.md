# Ping Pong Tournament Implementation Plan

**Goal:** Build a full ping pong doubles tournament web app: employees register via a shared link, admins pair them into teams, generate a round-robin + knockout bracket, and manually enter scores.

**Architecture:** Go backend (Gin + MySQL) serves three React pages — `/` (registration), `/bracket` (public read-only), `/admin` (passphrase-protected dashboard). All tournament logic lives in `tournament.go`. Frontend uses pathname-based routing in `App.tsx` with no router library.

---

## Task 1: MySQL DB connection + schema migration

**Task description**
Create `tournament.go` with the MySQL connection helper (copied from the platform pattern), the schema migration function (all 5 tables), and the Go structs for every data type. Wire DB init into `main.go`.

**Files:**
- Create: `tournament.go`
- Modify: `main.go`

**Step 1: Verify vendor packages exist**

Run:
```
ls vendor/github.com/go-sql-driver/mysql && ls vendor/cloud.google.com/go/cloudsqlconn
```
Expected: both directories exist (they do — MySQL is already enabled in project.toml).

**Step 2: Create `tournament.go` with DB types, connection, and migration**

```go
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	mysql_driver "github.com/go-sql-driver/mysql"
)

// --- Go structs ---

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

type Match struct {
	ID           string    `json:"id"`
	TournamentID string    `json:"tournament_id"`
	Team1        *Team     `json:"team1"`
	Team2        *Team     `json:"team2"`
	Team1Score   *int      `json:"team1_score"`
	Team2Score   *int      `json:"team2_score"`
	WinnerTeamID *string   `json:"winner_team_id"`
	Status       string    `json:"status"` // pending | complete
	Round        string    `json:"round"`  // rr | sf | final
	MatchOrder   int       `json:"match_order"`
	RoundNumber  int       `json:"round_number"`
	CreatedAt    time.Time `json:"created_at"`
}

type Tournament struct {
	ID        string    `json:"id"`
	Format    string    `json:"format"` // round_robin
	Status    string    `json:"status"` // draft | active | complete
	CreatedAt time.Time `json:"created_at"`
}

// --- UUID helper ---

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// --- DB connection ---

func initTournamentDB(ctx context.Context) (*sql.DB, error) {
	dbUser := os.Getenv("MYSQL_DB_USER")
	dbName := os.Getenv("MYSQL_DB_NAME")
	if dbUser == "" || dbName == "" {
		return nil, fmt.Errorf("[TournamentDB] missing MYSQL_DB_USER or MYSQL_DB_NAME")
	}

	instanceConnectionName := os.Getenv("MYSQL_INSTANCE_CONNECTION_NAME")
	if instanceConnectionName == "" {
		dsn := fmt.Sprintf("%s@tcp(localhost:3306)/%s?parseTime=true", dbUser, dbName)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("[TournamentDB] open local: %w", err)
		}
		if err := db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("[TournamentDB] ping local: %w", err)
		}
		log.Printf("[TournamentDB] connected locally to %s", dbName)
		return db, nil
	}

	dialer, err := cloudsqlconn.NewDialer(ctx,
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
	if err != nil {
		return nil, fmt.Errorf("[TournamentDB] dialer: %w", err)
	}

	mysql_driver.RegisterDialContext("cloudsql", func(ctx context.Context, addr string) (net.Conn, error) {
		return dialer.Dial(ctx, instanceConnectionName)
	})

	dsn := fmt.Sprintf("%s@cloudsql(%s)/%s?parseTime=true", dbUser, instanceConnectionName, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("[TournamentDB] open cloud: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("[TournamentDB] ping cloud: %w", err)
	}
	log.Printf("[TournamentDB] connected via Cloud SQL IAM auth")
	return db, nil
}

// --- Migrations ---

func migrateTournamentDB(db *sql.DB) error {
	log.Printf("[TournamentDB] running migrations...")

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tt_settings (
			` + "`key`" + ` VARCHAR(100) PRIMARY KEY,
			value VARCHAR(255) NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tt_players (
			id           VARCHAR(36) PRIMARY KEY,
			name         VARCHAR(255) NOT NULL,
			email        VARCHAR(255) NOT NULL UNIQUE,
			registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tt_teams (
			id         VARCHAR(36) PRIMARY KEY,
			player1_id VARCHAR(36) NOT NULL,
			player2_id VARCHAR(36) NOT NULL,
			locked     TINYINT(1) NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tt_tournaments (
			id         VARCHAR(36) PRIMARY KEY,
			format     VARCHAR(50) NOT NULL DEFAULT 'round_robin',
			status     VARCHAR(50) NOT NULL DEFAULT 'draft',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tt_matches (
			id             VARCHAR(36) PRIMARY KEY,
			tournament_id  VARCHAR(36) NOT NULL,
			team1_id       VARCHAR(36),
			team2_id       VARCHAR(36),
			team1_score    INT,
			team2_score    INT,
			winner_team_id VARCHAR(36),
			status         VARCHAR(50) NOT NULL DEFAULT 'pending',
			round          VARCHAR(10) NOT NULL,
			match_order    INT NOT NULL DEFAULT 0,
			round_number   INT NOT NULL DEFAULT 0,
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Seed default settings
		`INSERT IGNORE INTO tt_settings (` + "`key`" + `, value) VALUES ('registration_open', 'true')`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("[TournamentDB] migration failed: %w\nSQL: %s", err, stmt)
		}
	}
	log.Printf("[TournamentDB] migrations complete")
	return nil
}
```

**Step 3: Wire DB init into `main.go`**

In `main.go`, import `context` (already present) and add after the existing logger setup, before `registerSlackHandlers`:

```go
// In main(), after logger setup:
db, err := initTournamentDB(context.Background())
if err != nil {
    logger.Warn("tournament DB unavailable", zap.Error(err))
    db = nil
}
if db != nil {
    if err := migrateTournamentDB(db); err != nil {
        logger.Fatal("tournament DB migration failed", zap.Error(err))
    }
    defer db.Close()
}
```

Then pass `db` to `registerAPIRoutes`:
```go
registerAPIRoutes(r, bot, anaheimClient, db)
```

Also update the `registerAPIRoutes` signature in `api.go`:
```go
func registerAPIRoutes(r *gin.Engine, bot *slacklib.Bot, anaheimClient *anaheim.Client, db *sql.DB) {
```
Add `"database/sql"` to imports in `api.go`.

**Step 4: Verify build**

Run: `go build ./...`
Expected: no errors.

---

## Task 2: Public tournament API routes

**Task description**
Add the three public API endpoints to `tournament.go`: tournament status, player registration, and bracket read. These have no auth requirement.

**Files:**
- Modify: `tournament.go`
- Modify: `api.go`

**Step 1: Add public route handlers to `tournament.go`**

```go
// ---- Public API handlers ----

// getSetting reads a value from tt_settings. Returns defaultVal if not found.
func getSetting(db *sql.DB, key, defaultVal string) string {
	var val string
	err := db.QueryRow("SELECT value FROM tt_settings WHERE `key` = ?", key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

func setSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		"INSERT INTO tt_settings (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = ?",
		key, value, value)
	return err
}

func handleTournamentStatus(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] GET /api/tournament/status")

		regOpen := getSetting(db, "registration_open", "true") == "true"

		var playerCount int
		db.QueryRow("SELECT COUNT(*) FROM tt_players").Scan(&playerCount)

		var tournamentStatus string
		err := db.QueryRow("SELECT status FROM tt_tournaments ORDER BY created_at DESC LIMIT 1").Scan(&tournamentStatus)
		if err != nil {
			tournamentStatus = "none"
		}

		c.JSON(200, gin.H{
			"registration_open": regOpen,
			"player_count":      playerCount,
			"tournament_status": tournamentStatus,
		})
	}
}

func handleRegister(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] POST /api/register")

		var req struct {
			Name  string `json:"name" binding:"required"`
			Email string `json:"email" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[Register] bad request: %v", err)
			c.JSON(400, gin.H{"error": "name and email are required"})
			return
		}

		log.Printf("[Register] attempt: name=%s email=%s", req.Name, req.Email)

		// Guard: registration closed?
		if getSetting(db, "registration_open", "true") != "true" {
			log.Printf("[Register] rejected: registration closed")
			c.JSON(409, gin.H{"error": "registration_closed"})
			return
		}

		// Guard: duplicate email?
		var existing string
		err := db.QueryRow("SELECT id FROM tt_players WHERE email = ?", req.Email).Scan(&existing)
		if err == nil {
			log.Printf("[Register] rejected: duplicate email %s", req.Email)
			c.JSON(409, gin.H{"error": "already_registered"})
			return
		}

		id := newUUID()
		_, err = db.Exec(
			"INSERT INTO tt_players (id, name, email) VALUES (?, ?, ?)",
			id, req.Name, req.Email)
		if err != nil {
			log.Printf("[Register] insert error: %v", err)
			c.JSON(500, gin.H{"error": "failed to register"})
			return
		}

		log.Printf("[Register] success: id=%s name=%s email=%s", id, req.Name, req.Email)
		c.JSON(201, gin.H{"success": true, "id": id})
	}
}

func handleGetBracket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] GET /api/tournament/bracket")

		// Load tournament
		var tournID, tournStatus string
		err := db.QueryRow(
			"SELECT id, status FROM tt_tournaments ORDER BY created_at DESC LIMIT 1",
		).Scan(&tournID, &tournStatus)
		if err != nil {
			c.JSON(200, gin.H{"tournament": nil, "teams": []interface{}{}, "matches": []interface{}{}})
			return
		}

		// Load teams with players
		teams, err := loadTeams(db)
		if err != nil {
			log.Printf("[GetBracket] load teams error: %v", err)
			c.JSON(500, gin.H{"error": "failed to load teams"})
			return
		}

		// Load matches
		matches, err := loadMatches(db, tournID, teams)
		if err != nil {
			log.Printf("[GetBracket] load matches error: %v", err)
			c.JSON(500, gin.H{"error": "failed to load matches"})
			return
		}

		c.JSON(200, gin.H{
			"tournament": gin.H{"id": tournID, "status": tournStatus},
			"teams":      teams,
			"matches":    matches,
		})
	}
}

// loadTeams returns all teams with their players populated.
func loadTeams(db *sql.DB) ([]Team, error) {
	rows, err := db.Query(`
		SELECT t.id, t.locked, t.created_at,
		       p1.id, p1.name, p1.email, p1.registered_at,
		       p2.id, p2.name, p2.email, p2.registered_at
		FROM tt_teams t
		JOIN tt_players p1 ON t.player1_id = p1.id
		JOIN tt_players p2 ON t.player2_id = p2.id
		ORDER BY t.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var t Team
		var p1, p2 Player
		if err := rows.Scan(
			&t.ID, &t.Locked, &t.CreatedAt,
			&p1.ID, &p1.Name, &p1.Email, &p1.RegisteredAt,
			&p2.ID, &p2.Name, &p2.Email, &p2.RegisteredAt,
		); err != nil {
			return nil, err
		}
		t.Player1 = &p1
		t.Player2 = &p2
		teams = append(teams, t)
	}
	return teams, nil
}

// loadMatches returns all matches for a tournament, with team objects populated.
func loadMatches(db *sql.DB, tournamentID string, teams []Team) ([]Match, error) {
	// Build team lookup map
	teamMap := map[string]*Team{}
	for i := range teams {
		teamMap[teams[i].ID] = &teams[i]
	}

	rows, err := db.Query(`
		SELECT id, team1_id, team2_id, team1_score, team2_score,
		       winner_team_id, status, round, match_order, round_number, created_at
		FROM tt_matches
		WHERE tournament_id = ?
		ORDER BY round_number ASC, match_order ASC
	`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		var team1ID, team2ID sql.NullString
		m.TournamentID = tournamentID
		if err := rows.Scan(
			&m.ID, &team1ID, &team2ID,
			&m.Team1Score, &m.Team2Score, &m.WinnerTeamID,
			&m.Status, &m.Round, &m.MatchOrder, &m.RoundNumber, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		if team1ID.Valid {
			if t, ok := teamMap[team1ID.String]; ok {
				m.Team1 = t
			}
		}
		if team2ID.Valid {
			if t, ok := teamMap[team2ID.String]; ok {
				m.Team2 = t
			}
		}
		matches = append(matches, m)
	}
	return matches, nil
}
```

**Step 2: Register public routes in `api.go`**

Inside `registerAPIRoutes`, after existing routes, add:
```go
// Tournament public routes
if db != nil {
    api.GET("/tournament/status", handleTournamentStatus(db))
    api.POST("/register", handleRegister(db))
    api.GET("/tournament/bracket", handleGetBracket(db))
}
```

**Step 3: Add missing imports to `tournament.go`**

The file needs these imports at the top (add to the import block):
```go
"github.com/gin-gonic/gin"
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: no errors.

---

## Task 3: Admin auth middleware + admin player routes

**Task description**
Add the admin passphrase middleware and the first two admin routes: auth check and player list. All admin routes are mounted under `/api/admin` with the middleware applied.

**Files:**
- Modify: `tournament.go`
- Modify: `api.go`

**Step 1: Add admin middleware and handlers to `tournament.go`**

```go
// ---- Admin middleware ----

func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		passphrase := os.Getenv("ADMIN_PASSPHRASE")
		if passphrase == "" {
			log.Printf("[AdminAuth] WARNING: ADMIN_PASSPHRASE not set, blocking all admin requests")
			c.AbortWithStatusJSON(503, gin.H{"error": "admin not configured"})
			return
		}
		token := c.GetHeader("X-Admin-Token")
		if token != passphrase {
			log.Printf("[AdminAuth] unauthorized attempt, token mismatch")
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		log.Printf("[AdminAuth] authorized")
		c.Next()
	}
}

// ---- Admin handlers ----

func handleAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Middleware already validated the token. Just return ok.
		log.Printf("[AdminAuth] POST /api/admin/auth - success")
		c.JSON(200, gin.H{"ok": true})
	}
}

func handleAdminGetPlayers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] GET /api/admin/players")
		rows, err := db.Query(
			"SELECT id, name, email, registered_at FROM tt_players ORDER BY registered_at ASC")
		if err != nil {
			log.Printf("[Admin] get players error: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var players []Player
		for rows.Next() {
			var p Player
			if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.RegisteredAt); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			players = append(players, p)
		}
		if players == nil {
			players = []Player{}
		}
		log.Printf("[Admin] returning %d players", len(players))
		c.JSON(200, gin.H{"players": players})
	}
}
```

**Step 2: Register admin routes in `api.go`**

```go
// Admin routes (require passphrase)
if db != nil {
    admin := api.Group("/admin")
    admin.Use(adminAuthMiddleware())
    admin.POST("/auth", handleAdminAuth())
    admin.GET("/players", handleAdminGetPlayers(db))
    // More admin routes added in subsequent tasks
}
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: no errors.

---

## Task 4: Registration open/close + team randomization routes

**Task description**
Add admin routes to open/close registration and to randomize registered players into random teams of 2.

**Files:**
- Modify: `tournament.go`
- Modify: `api.go`

**Step 1: Add registration control + randomize handlers to `tournament.go`**

```go
import "math/rand"  // add to imports

func handleAdminCloseRegistration(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/registration/close")
		if err := setSetting(db, "registration_open", "false"); err != nil {
			log.Printf("[Admin] close registration error: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[Admin] registration closed")
		c.JSON(200, gin.H{"registration_open": false})
	}
}

func handleAdminOpenRegistration(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/registration/open")
		if err := setSetting(db, "registration_open", "true"); err != nil {
			log.Printf("[Admin] open registration error: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[Admin] registration opened")
		c.JSON(200, gin.H{"registration_open": true})
	}
}

func handleAdminRandomizeTeams(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/teams/randomize")

		// Fetch all players
		rows, err := db.Query("SELECT id FROM tt_players ORDER BY registered_at ASC")
		if err != nil {
			log.Printf("[Admin] randomize: fetch players error: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var playerIDs []string
		for rows.Next() {
			var id string
			rows.Scan(&id)
			playerIDs = append(playerIDs, id)
		}

		log.Printf("[Admin] randomizing %d players into teams", len(playerIDs))

		// Shuffle
		rand.Shuffle(len(playerIDs), func(i, j int) {
			playerIDs[i], playerIDs[j] = playerIDs[j], playerIDs[i]
		})

		// Delete existing (unlocked) teams
		db.Exec("DELETE FROM tt_teams WHERE locked = 0")

		// Pair into teams of 2
		var teamIDs []string
		for i := 0; i+1 < len(playerIDs); i += 2 {
			tid := newUUID()
			_, err := db.Exec(
				"INSERT INTO tt_teams (id, player1_id, player2_id) VALUES (?, ?, ?)",
				tid, playerIDs[i], playerIDs[i+1])
			if err != nil {
				log.Printf("[Admin] randomize: insert team error: %v", err)
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			teamIDs = append(teamIDs, tid)
		}

		oddPlayer := ""
		if len(playerIDs)%2 != 0 {
			oddPlayer = playerIDs[len(playerIDs)-1]
		}

		log.Printf("[Admin] created %d teams, odd player: %s", len(teamIDs), oddPlayer)
		c.JSON(200, gin.H{
			"teams_created": len(teamIDs),
			"odd_player_id": oddPlayer,
		})
	}
}
```

**Step 2: Add GET teams + swap player handlers**

```go
func handleAdminGetTeams(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] GET /api/admin/teams")
		teams, err := loadTeams(db)
		if err != nil {
			log.Printf("[Admin] get teams error: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if teams == nil {
			teams = []Team{}
		}
		log.Printf("[Admin] returning %d teams", len(teams))
		c.JSON(200, gin.H{"teams": teams})
	}
}

func handleAdminSwapPlayers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Player1ID string `json:"player1_id" binding:"required"`
			Player2ID string `json:"player2_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[Admin] PUT /api/admin/teams/swap player1=%s player2=%s", req.Player1ID, req.Player2ID)

		// Find which team each player is in
		var team1ID, team1Slot string
		err := db.QueryRow(
			`SELECT id, CASE WHEN player1_id = ? THEN 'player1' ELSE 'player2' END
			 FROM tt_teams WHERE player1_id = ? OR player2_id = ?`,
			req.Player1ID, req.Player1ID, req.Player1ID,
		).Scan(&team1ID, &team1Slot)
		if err != nil {
			log.Printf("[Admin] swap: player1 not found in any team: %v", err)
			c.JSON(404, gin.H{"error": "player1 not in any team"})
			return
		}

		var team2ID, team2Slot string
		err = db.QueryRow(
			`SELECT id, CASE WHEN player1_id = ? THEN 'player1' ELSE 'player2' END
			 FROM tt_teams WHERE player1_id = ? OR player2_id = ?`,
			req.Player2ID, req.Player2ID, req.Player2ID,
		).Scan(&team2ID, &team2Slot)
		if err != nil {
			log.Printf("[Admin] swap: player2 not found in any team: %v", err)
			c.JSON(404, gin.H{"error": "player2 not in any team"})
			return
		}

		// Swap: update each team's slot
		col1 := "player1_id"
		if team1Slot == "player2" {
			col1 = "player2_id"
		}
		col2 := "player1_id"
		if team2Slot == "player2" {
			col2 = "player2_id"
		}

		if _, err := db.Exec(fmt.Sprintf("UPDATE tt_teams SET %s = ? WHERE id = ?", col1), req.Player2ID, team1ID); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if _, err := db.Exec(fmt.Sprintf("UPDATE tt_teams SET %s = ? WHERE id = ?", col2), req.Player1ID, team2ID); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		log.Printf("[Admin] swap complete: teams %s <-> %s", team1ID, team2ID)
		c.JSON(200, gin.H{"success": true})
	}
}
```

**Step 3: Register new routes in `api.go` admin group**

Add to the existing admin group block:
```go
admin.POST("/registration/close", handleAdminCloseRegistration(db))
admin.POST("/registration/open", handleAdminOpenRegistration(db))
admin.POST("/teams/randomize", handleAdminRandomizeTeams(db))
admin.GET("/teams", handleAdminGetTeams(db))
admin.PUT("/teams/swap", handleAdminSwapPlayers(db))
```

**Step 4: Add `math/rand` to imports in `tournament.go`**

**Step 5: Verify build**

Run: `go build ./...`
Expected: no errors.

---

## Task 5: Bracket generation + score entry

**Task description**
Add the two most complex routes: bracket generation (round-robin scheduling algorithm) and score entry (with automatic knockout slot assignment when all RR matches finish).

**Files:**
- Modify: `tournament.go`
- Modify: `api.go`

**Step 1: Add bracket generation handler to `tournament.go`**

```go
// generateRRSchedule generates round-robin match pairings.
// Returns list of (team1Index, team2Index, roundNumber) tuples.
type rrPairing struct {
	Team1Idx    int
	Team2Idx    int
	RoundNumber int
}

func generateRRSchedule(n int) []rrPairing {
	hasBye := n%2 != 0
	if hasBye {
		n++ // add a bye slot
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
				continue // skip bye matches
			}
			pairings = append(pairings, rrPairing{t1, t2, round})
		}
		// Rotate: fix teams[0], rotate teams[1:]
		last := teams[n-1]
		copy(teams[2:], teams[1:n-1])
		teams[1] = last
	}
	return pairings
}

func handleAdminGenerateBracket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[Admin] POST /api/admin/bracket/generate")

		// Load teams
		teams, err := loadTeams(db)
		if err != nil || len(teams) < 2 {
			log.Printf("[Admin] generate bracket: not enough teams (have %d): %v", len(teams), err)
			c.JSON(400, gin.H{"error": "need at least 2 teams"})
			return
		}

		log.Printf("[Admin] generating bracket for %d teams", len(teams))

		// Lock all teams
		db.Exec("UPDATE tt_teams SET locked = 1")

		// Create or get tournament
		var tournID string
		err = db.QueryRow("SELECT id FROM tt_tournaments ORDER BY created_at DESC LIMIT 1").Scan(&tournID)
		if err != nil {
			tournID = newUUID()
			db.Exec("INSERT INTO tt_tournaments (id, format, status) VALUES (?, 'round_robin', 'active')",
				tournID)
			log.Printf("[Admin] created tournament id=%s", tournID)
		} else {
			db.Exec("UPDATE tt_tournaments SET status = 'active' WHERE id = ?", tournID)
		}

		// Delete existing matches
		db.Exec("DELETE FROM tt_matches WHERE tournament_id = ?", tournID)

		// Generate RR pairings
		pairings := generateRRSchedule(len(teams))
		log.Printf("[Admin] generated %d RR matches", len(pairings))

		// Group pairings by round for match_order
		roundCounts := map[int]int{}
		for _, p := range pairings {
			order := roundCounts[p.RoundNumber]
			matchID := newUUID()
			_, err := db.Exec(`
				INSERT INTO tt_matches
				  (id, tournament_id, team1_id, team2_id, status, round, match_order, round_number)
				VALUES (?, ?, ?, ?, 'pending', 'rr', ?, ?)`,
				matchID, tournID, teams[p.Team1Idx].ID, teams[p.Team2Idx].ID, order, p.RoundNumber)
			if err != nil {
				log.Printf("[Admin] insert RR match error: %v", err)
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			roundCounts[p.RoundNumber]++
		}

		// Create SF and Final placeholder matches (no teams yet)
		sf1ID, sf2ID, finalID := newUUID(), newUUID(), newUUID()
		baseRound := len(teams) // knockout rounds come after RR rounds
		db.Exec(`INSERT INTO tt_matches (id, tournament_id, status, round, match_order, round_number)
			VALUES (?, ?, 'pending', 'sf', 0, ?)`, sf1ID, tournID, baseRound)
		db.Exec(`INSERT INTO tt_matches (id, tournament_id, status, round, match_order, round_number)
			VALUES (?, ?, 'pending', 'sf', 1, ?)`, sf2ID, tournID, baseRound)
		db.Exec(`INSERT INTO tt_matches (id, tournament_id, status, round, match_order, round_number)
			VALUES (?, ?, 'pending', 'final', 0, ?)`, finalID, tournID, baseRound+1)

		log.Printf("[Admin] bracket generated: %d RR matches + 3 knockout slots", len(pairings))
		c.JSON(200, gin.H{
			"tournament_id": tournID,
			"rr_matches":    len(pairings),
			"teams":         len(teams),
		})
	}
}
```

**Step 2: Add score entry handler to `tournament.go`**

```go
func handleAdminEnterScore(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		matchID := c.Param("id")
		var req struct {
			Team1Score int `json:"team1_score"`
			Team2Score int `json:"team2_score"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[Admin] POST /api/admin/matches/%s/score: %d - %d", matchID, req.Team1Score, req.Team2Score)

		// Load match
		var m struct {
			tournamentID string
			team1ID      sql.NullString
			team2ID      sql.NullString
			round        string
		}
		err := db.QueryRow(
			"SELECT tournament_id, team1_id, team2_id, round FROM tt_matches WHERE id = ?", matchID,
		).Scan(&m.tournamentID, &m.team1ID, &m.team2ID, &m.round)
		if err != nil {
			log.Printf("[Admin] enter score: match not found %s: %v", matchID, err)
			c.JSON(404, gin.H{"error": "match not found"})
			return
		}

		// Determine winner
		var winnerID *string
		if m.team1ID.Valid && m.team2ID.Valid {
			if req.Team1Score > req.Team2Score {
				winnerID = &m.team1ID.String
			} else if req.Team2Score > req.Team1Score {
				winnerID = &m.team2ID.String
			}
			// draw: winner stays nil
		}

		_, err = db.Exec(`
			UPDATE tt_matches
			SET team1_score = ?, team2_score = ?, winner_team_id = ?, status = 'complete'
			WHERE id = ?`,
			req.Team1Score, req.Team2Score, winnerID, matchID)
		if err != nil {
			log.Printf("[Admin] enter score: update error: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// If this was an RR match, check if all RR matches are done → assign SF slots
		if m.round == "rr" {
			if err := maybeAssignKnockoutTeams(db, m.tournamentID); err != nil {
				log.Printf("[Admin] knockout assignment error: %v", err)
				// Non-fatal — score was saved
			}
		}

		// If this was a SF match and complete, assign winner to final
		if m.round == "sf" {
			if err := maybeAssignFinalTeams(db, m.tournamentID); err != nil {
				log.Printf("[Admin] final assignment error: %v", err)
			}
		}

		log.Printf("[Admin] score saved for match %s", matchID)
		c.JSON(200, gin.H{"success": true})
	}
}

// maybeAssignKnockoutTeams checks if all RR matches are complete and, if so,
// assigns the top 4 teams (by points) to the SF slots.
func maybeAssignKnockoutTeams(db *sql.DB, tournamentID string) error {
	// Count pending RR matches
	var pending int
	db.QueryRow(
		"SELECT COUNT(*) FROM tt_matches WHERE tournament_id = ? AND round = 'rr' AND status = 'pending'",
		tournamentID,
	).Scan(&pending)

	if pending > 0 {
		log.Printf("[Knockout] %d RR matches still pending, skipping SF assignment", pending)
		return nil
	}

	log.Printf("[Knockout] all RR matches complete, computing standings...")

	// Compute standings: for each team, sum wins (2pts) from RR matches
	rows, err := db.Query(`
		SELECT team_id, SUM(pts) AS total_pts,
		       SUM(gf) AS goals_for, SUM(ga) AS goals_against
		FROM (
			SELECT team1_id AS team_id,
			       CASE WHEN winner_team_id = team1_id THEN 2
			            WHEN winner_team_id IS NULL AND status = 'complete' THEN 1
			            ELSE 0 END AS pts,
			       COALESCE(team1_score, 0) AS gf,
			       COALESCE(team2_score, 0) AS ga
			FROM tt_matches
			WHERE tournament_id = ? AND round = 'rr' AND status = 'complete' AND team1_id IS NOT NULL
			UNION ALL
			SELECT team2_id AS team_id,
			       CASE WHEN winner_team_id = team2_id THEN 2
			            WHEN winner_team_id IS NULL AND status = 'complete' THEN 1
			            ELSE 0 END AS pts,
			       COALESCE(team2_score, 0) AS gf,
			       COALESCE(team1_score, 0) AS ga
			FROM tt_matches
			WHERE tournament_id = ? AND round = 'rr' AND status = 'complete' AND team2_id IS NOT NULL
		) t
		GROUP BY team_id
		ORDER BY total_pts DESC, (SUM(gf) - SUM(ga)) DESC
		LIMIT 4
	`, tournamentID, tournamentID)
	if err != nil {
		return fmt.Errorf("standings query: %w", err)
	}
	defer rows.Close()

	type standing struct {
		teamID string
		pts    int
	}
	var standings []standing
	for rows.Next() {
		var s standing
		var gf, ga int
		rows.Scan(&s.teamID, &s.pts, &gf, &ga)
		standings = append(standings, s)
	}

	if len(standings) < 4 {
		log.Printf("[Knockout] only %d teams in standings, need 4 for SF", len(standings))
		return nil
	}

	// SF1: 1st vs 4th, SF2: 2nd vs 3rd
	log.Printf("[Knockout] assigning SF slots: sf1=%s vs %s, sf2=%s vs %s",
		standings[0].teamID, standings[3].teamID,
		standings[1].teamID, standings[2].teamID)

	// Get SF match IDs (order 0 and 1)
	var sf1ID, sf2ID string
	db.QueryRow(
		"SELECT id FROM tt_matches WHERE tournament_id = ? AND round = 'sf' AND match_order = 0",
		tournamentID,
	).Scan(&sf1ID)
	db.QueryRow(
		"SELECT id FROM tt_matches WHERE tournament_id = ? AND round = 'sf' AND match_order = 1",
		tournamentID,
	).Scan(&sf2ID)

	db.Exec("UPDATE tt_matches SET team1_id = ?, team2_id = ? WHERE id = ?",
		standings[0].teamID, standings[3].teamID, sf1ID)
	db.Exec("UPDATE tt_matches SET team1_id = ?, team2_id = ? WHERE id = ?",
		standings[1].teamID, standings[2].teamID, sf2ID)

	log.Printf("[Knockout] SF slots assigned")
	return nil
}

// maybeAssignFinalTeams assigns SF winners to the final match once both SFs are done.
func maybeAssignFinalTeams(db *sql.DB, tournamentID string) error {
	var pendingSF int
	db.QueryRow(
		"SELECT COUNT(*) FROM tt_matches WHERE tournament_id = ? AND round = 'sf' AND status = 'pending'",
		tournamentID,
	).Scan(&pendingSF)
	if pendingSF > 0 {
		return nil
	}

	// Get SF winners
	rows, err := db.Query(
		"SELECT winner_team_id FROM tt_matches WHERE tournament_id = ? AND round = 'sf' ORDER BY match_order ASC",
		tournamentID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var winners []string
	for rows.Next() {
		var w sql.NullString
		rows.Scan(&w)
		if w.Valid {
			winners = append(winners, w.String)
		}
	}

	if len(winners) < 2 {
		return nil
	}

	var finalID string
	db.QueryRow(
		"SELECT id FROM tt_matches WHERE tournament_id = ? AND round = 'final'", tournamentID,
	).Scan(&finalID)

	db.Exec("UPDATE tt_matches SET team1_id = ?, team2_id = ? WHERE id = ?",
		winners[0], winners[1], finalID)

	log.Printf("[Knockout] Final assigned: %s vs %s", winners[0], winners[1])
	return nil
}
```

**Step 3: Register the new admin routes in `api.go`**

Add to the admin group:
```go
admin.POST("/bracket/generate", handleAdminGenerateBracket(db))
admin.POST("/matches/:id/score", handleAdminEnterScore(db))
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: no errors.

---

## Task 6: Frontend routing — pathname-based page switching

**Task description**
Update `App.tsx` to detect the current URL pathname and render the correct full-page component (`Register`, `Bracket`, or `AdminDashboard`) instead of the sidebar layout. The existing sidebar/Home remains untouched — it just won't render on the 3 new paths.

**Files:**
- Modify: `frontend/src/App.tsx`

**Step 1: Replace App.tsx content**

```tsx
import { useState, useEffect, createContext, useContext } from 'react'
import Sidebar from './components/Sidebar'
import Home from './components/Home'
import IntegrationBar from './components/IntegrationBar'
import Register from './components/Register'
import Bracket from './components/Bracket'
import AdminDashboard from './components/AdminDashboard'

export type Tab = 'home'

interface BotStatus {
  ready: boolean
  message: string
}

const StatusContext = createContext<BotStatus>({ ready: false, message: 'Loading...' })

export function useStatus() {
  return useContext(StatusContext)
}

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('home')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [status, setStatus] = useState<BotStatus>({ ready: false, message: 'Loading...' })

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const response = await fetch('/slack/status')
        const data = await response.json()
        setStatus({ ready: data.ready, message: data.message })
      } catch {
        setStatus({ ready: false, message: 'Failed to connect to server' })
      }
    }
    fetchStatus()
    const interval = setInterval(fetchStatus, 10000)
    return () => clearInterval(interval)
  }, [])

  const path = window.location.pathname

  // Full-page routes (no sidebar)
  if (path === '/bracket') return <Bracket />
  if (path === '/admin') return <AdminDashboard />
  if (path === '/' || path === '/register') return <Register />

  // Default: existing sidebar layout
  return (
    <StatusContext.Provider value={status}>
      <div className="flex h-screen bg-gray-100">
        <Sidebar activeTab={activeTab} setActiveTab={setActiveTab} isOpen={sidebarOpen} onToggle={() => setSidebarOpen(v => !v)} />
        <main className="flex-1 overflow-auto flex flex-col">
          <IntegrationBar />
          <div className="flex-1 overflow-auto p-6">
            <Home />
          </div>
          <div className="text-right px-6 py-2 text-xs text-gray-600 font-semibold">
            Made with the{' '}
            <a href="https://agentic-app-platform.experimental.staging.apps.applied.dev" target="_blank" rel="noopener noreferrer" className="underline hover:text-gray-600">
              Agentic App Builder
            </a>
          </div>
        </main>
      </div>
    </StatusContext.Provider>
  )
}

export default App
```

---

## Task 7: Register page component

**Task description**
Create the employee-facing registration page: a clean centered card with name + email fields.

**Files:**
- Create: `frontend/src/components/Register.tsx`

**Step 1: Create Register.tsx**

```tsx
import { useState, useEffect } from 'react'

type PageState = 'loading' | 'form' | 'success' | 'already_registered' | 'closed' | 'error'

export default function Register() {
  const [pageState, setPageState] = useState<PageState>('loading')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    fetch('/api/tournament/status')
      .then(r => r.json())
      .then(data => {
        if (!data.registration_open) {
          setPageState('closed')
        } else {
          setPageState('form')
        }
      })
      .catch(() => setPageState('form')) // show form on error
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !email.trim()) return
    setSubmitting(true)
    setErrorMsg('')

    try {
      const res = await fetch('/api/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), email: email.trim() }),
      })
      const data = await res.json()

      if (res.status === 201) {
        setPageState('success')
      } else if (data.error === 'already_registered') {
        setPageState('already_registered')
      } else if (data.error === 'registration_closed') {
        setPageState('closed')
      } else {
        setErrorMsg(data.error || 'Something went wrong. Please try again.')
      }
    } catch {
      setErrorMsg('Network error. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-green-50 to-emerald-100 flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-lg p-8 w-full max-w-md">
        <div className="text-center mb-6">
          <div className="text-5xl mb-3">🏓</div>
          <h1 className="text-2xl font-bold text-gray-900">Applied Ping Pong Tournament</h1>
          <p className="text-gray-500 mt-1 text-sm">Doubles tournament — register to join!</p>
        </div>

        {pageState === 'loading' && (
          <div className="text-center text-gray-400 py-8">Loading...</div>
        )}

        {pageState === 'form' && (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Full Name</label>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="Jane Smith"
                required
                className="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-green-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Applied Email</label>
              <input
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="jane@applied.dev"
                required
                className="w-full border border-gray-300 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-green-500"
              />
            </div>
            {errorMsg && (
              <p className="text-red-500 text-sm">{errorMsg}</p>
            )}
            <button
              type="submit"
              disabled={submitting}
              className="w-full bg-green-600 hover:bg-green-700 disabled:bg-green-300 text-white font-semibold py-2 px-4 rounded-lg transition-colors"
            >
              {submitting ? 'Registering...' : 'Register Me! 🎾'}
            </button>
          </form>
        )}

        {pageState === 'success' && (
          <div className="text-center py-6">
            <div className="text-4xl mb-3">🎉</div>
            <h2 className="text-xl font-bold text-gray-900">You're in!</h2>
            <p className="text-gray-500 mt-2">We'll announce teams and match schedules soon. Stay tuned!</p>
          </div>
        )}

        {pageState === 'already_registered' && (
          <div className="text-center py-6">
            <div className="text-4xl mb-3">✅</div>
            <h2 className="text-xl font-bold text-gray-900">Already registered!</h2>
            <p className="text-gray-500 mt-2">You're already signed up. See you on the court!</p>
          </div>
        )}

        {pageState === 'closed' && (
          <div className="text-center py-6">
            <div className="text-4xl mb-3">🔒</div>
            <h2 className="text-xl font-bold text-gray-900">Registration is closed</h2>
            <p className="text-gray-500 mt-2">
              Sign-ups have ended. Check the{' '}
              <a href="/bracket" className="text-green-600 underline">live bracket</a> for match progress!
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
```

---

## Task 8: Bracket page component

**Task description**
Create the public `/bracket` page showing the round-robin standings table and the knockout bracket tree. Polls every 10 seconds.

**Files:**
- Create: `frontend/src/components/Bracket.tsx`

**Step 1: Create Bracket.tsx**

```tsx
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
```

---

## Task 9: Admin Dashboard — passphrase gate + tabs

**Task description**
Create the admin dashboard shell with passphrase login, localStorage session, and the three-tab layout. The sub-components (AdminRegistrations, AdminTeams, AdminBracket) are created in the next task.

**Files:**
- Create: `frontend/src/components/AdminDashboard.tsx`
- Create: `frontend/src/components/AdminRegistrations.tsx`
- Create: `frontend/src/components/AdminTeams.tsx`
- Create: `frontend/src/components/AdminBracket.tsx`

**Step 1: Create AdminDashboard.tsx**

```tsx
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
```

---

## Task 10: Admin sub-components (Registrations, Teams, Bracket)

**Files:**
- Create: `frontend/src/components/AdminRegistrations.tsx`
- Create: `frontend/src/components/AdminTeams.tsx`
- Create: `frontend/src/components/AdminBracket.tsx`

**Step 1: Create AdminRegistrations.tsx**

```tsx
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
      <div className="flex justify-between items-center mb-4">
        <div>
          <h2 className="text-xl font-bold text-gray-900">Registrations</h2>
          <p className="text-sm text-gray-500">{players.length} player{players.length !== 1 ? 's' : ''} signed up</p>
        </div>
        <div className="flex gap-3">
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
```

**Step 2: Create AdminTeams.tsx**

```tsx
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

  const allPlayers = teams.flatMap(t => [t.player1, t.player2])

  const handlePlayerClick = async (clickedPlayer: Player) => {
    if (!swapState) {
      setSwapState({ playerID: clickedPlayer.id, playerName: clickedPlayer.name })
      return
    }
    if (swapState.playerID === clickedPlayer.id) {
      setSwapState(null)
      return
    }
    // Perform swap
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
          🔄 Swapping <strong>{swapState.playerName}</strong> — now click the player you want to swap them with. Click the same player to cancel.
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
                  <span className="text-xs text-gray-400 ml-1 block truncate">{p.email}</span>
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
      <p className="text-xs text-gray-400 mt-4">💡 Click a player to start a swap, then click another player to complete the swap.</p>
    </div>
  )
}
```

**Step 3: Create AdminBracket.tsx**

```tsx
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
  const [teams, setTeams] = useState<Team[]>([])
  const [matches, setMatches] = useState<Match[]>([])
  const [loading, setLoading] = useState(true)
  const [scoreModal, setScoreModal] = useState<Match | null>(null)
  const [score1, setScore1] = useState('')
  const [score2, setScore2] = useState('')
  const [saving, setSaving] = useState(false)

  const fetchBracket = () =>
    fetch('/api/tournament/bracket')
      .then(r => r.json())
      .then(d => { setTeams(d.teams || []); setMatches(d.matches || []); setLoading(false) })

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
  const rounds = [...new Set(rrMatches.map(m => m.round_number))].sort()
  const sfMatches = matches.filter(m => m.round === 'sf').sort((a, b) => a.match_order - b.match_order)
  const finalMatch = matches.find(m => m.round === 'final')

  const MatchRow = ({ m }: { m: Match }) => (
    <div className={`flex items-center justify-between p-3 rounded-lg mb-2 border ${m.status === 'complete' ? 'bg-green-50 border-green-200' : 'bg-white border-gray-200'}`}>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{teamName(m.team1)} <span className="text-gray-400">vs</span> {teamName(m.team2)}</p>
        {m.status === 'complete' && (
          <p className="text-xs text-green-700 font-bold">{m.team1_score} — {m.team2_score}</p>
        )}
      </div>
      <button
        onClick={() => openScoreModal(m)}
        disabled={!m.team1 || !m.team2}
        className="ml-3 px-3 py-1 text-xs font-medium bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-100 disabled:text-gray-400 text-white rounded-lg"
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
          No bracket yet. Go to Teams tab and generate the bracket first.
        </div>
      )}

      {rounds.length > 0 && (
        <div className="mb-6">
          <h3 className="font-semibold text-gray-700 mb-3">Round Robin Matches</h3>
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
                <input type="number" min="0" value={score1} onChange={e => setScore1(e.target.value)}
                  className="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500" />
              </div>
              <div>
                <label className="text-xs font-medium text-gray-600 mb-1 block">{teamName(scoreModal.team2)}</label>
                <input type="number" min="0" value={score2} onChange={e => setScore2(e.target.value)}
                  className="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500" />
              </div>
            </div>
            <div className="flex gap-3">
              <button onClick={() => setScoreModal(null)} className="flex-1 py-2 border rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-50">Cancel</button>
              <button onClick={submitScore} disabled={saving || score1 === '' || score2 === ''}
                className="flex-1 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-200 text-white rounded-lg text-sm font-medium">
                {saving ? 'Saving...' : 'Save Score'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
```

---

## Task 11: Deploy and verify

**Task description**
Deploy the app and verify the full flow works end-to-end: registration, admin login, team randomization, bracket generation, and score entry.

**Step 1: Invoke the `apps-platform` skill to deploy**

Run the deployment flow using `apps-platform app deploy`.

**Step 2: Spawn a log-tailing subagent**

After deploy completes, spawn a subagent running `apps-platform app logs` and look for:
- `[TournamentDB] connected via Cloud SQL IAM auth`
- `[TournamentDB] migrations complete`
- `[API] GET /api/tournament/status` (from registration page load)
- `[Register]` entries when registrations happen

**Step 3: Test registration flow**
1. Open `https://your-app-url/` in a browser
2. Fill in name + email, click "Register Me!"
3. Expect: success message appears
4. Try the same email again → expect: "Already registered" state

**Step 4: Test admin flow**
1. Open `https://your-app-url/admin`
2. Enter the `ADMIN_PASSPHRASE` you set
3. Expect: dashboard loads with 3 tabs
4. Go to Registrations tab → see registered players
5. Click "Close Registration", then "Randomize into Teams"
6. Go to Teams tab → see generated teams, try clicking players to swap
7. Click "Lock Teams & Generate Bracket"
8. Go to Bracket tab → see all RR matches
9. Click "Enter Score" on a match, enter scores, submit
10. Check `/bracket` in a new tab to confirm public view updates

---

## Task 12: Walk the user through the app

**Summary**
The ping pong tournament app is a three-page web app. Employees open the registration link, enter their name and Applied email, and get a confirmation. You (as admin) open `/admin`, log in with your passphrase, see everyone who signed up, randomize them into doubles teams, tweak pairings if needed, generate the bracket, and enter scores after each match. A public `/bracket` page shows the live standings and knockout matches, auto-refreshing every 10 seconds.

**To use it:**
1. **Share the registration link** — paste `https://your-app-url/` in your Slack channel
2. **Close registration** — go to `/admin` → Registrations tab → click "Close Registration"
3. **Make teams** — click "Randomize into Teams", then go to Teams tab to make any swaps
4. **Generate the bracket** — click "Lock Teams & Generate Bracket"
5. **Enter scores** — go to Bracket tab, click "Enter Score" on each match after it finishes
6. **Watch the bracket** — share `https://your-app-url/bracket` so everyone can follow along

**Setup required:**
- Set the `ADMIN_PASSPHRASE` environment variable in the Apps Platform config before deploying

**If something looks wrong:**
- Check `/admin` → Registrations to confirm players are saved
- Check app logs for `[TournamentDB]` and `[Register]` lines
- If the bracket isn't updating, try refreshing the `/bracket` page
