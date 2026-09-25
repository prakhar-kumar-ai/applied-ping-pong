package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.apps.applied.dev/lib/slacklib"
)

// botRef holds the Slack bot reference for sending match notifications.
var botRef *slacklib.Bot

// defaultSlots are the match time blocks (24h HH:MM) used by auto-scheduler.
// Lunch block: 1:00pm, 1:30pm — Evening block: 5:00pm onwards.
var defaultSlots = []string{
	"13:00", "13:30",
	"17:00", "17:30", "18:00", "18:30", "19:00",
}

// tournamentTZ is the local timezone for formatting times in DMs.
const tournamentTZ = "America/Los_Angeles"

// fmtMatchTime formats a match time for display in Slack messages.
func fmtMatchTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	loc, _ := time.LoadLocation(tournamentTZ)
	return t.In(loc).Format("Mon Jan 2 at 3:04 PM")
}

// lookupSlackID returns the Slack user ID for a given email, or "" if not found.
func lookupSlackID(email string) string {
	if botRef == nil {
		return ""
	}
	client, err := botRef.Client()
	if err != nil {
		log.Printf("[Schedule] Slack client unavailable: %v", err)
		return ""
	}
	user, err := client.GetUserByEmail(email)
	if err != nil {
		log.Printf("[Schedule] no Slack user for email=%s: %v", email, err)
		return ""
	}
	return user.ID
}

// sendMatchDMs sends scheduling DMs to all 4 players in a match.
// Must be called outside store lock; takes a value copy of Match for safety.
func sendMatchDMs(m *Match) {
	if botRef == nil || m.Team1 == nil || m.Team2 == nil || m.ScheduledTime == nil {
		return
	}

	timeStr := fmtMatchTime(m.ScheduledTime)
	tableStr := ""
	if m.TableNum != "" {
		tableStr = " · Table " + m.TableNum
	}

	t1Name := m.Team1.Player1.Name + " & " + m.Team1.Player2.Name
	t2Name := m.Team2.Player1.Name + " & " + m.Team2.Player2.Name

	type entry struct {
		player   *Player
		opponent string
	}
	players := []entry{
		{m.Team1.Player1, t2Name},
		{m.Team1.Player2, t2Name},
		{m.Team2.Player1, t1Name},
		{m.Team2.Player2, t1Name},
	}

	for _, e := range players {
		slackID := lookupSlackID(e.player.Email)
		if slackID == "" {
			log.Printf("[Schedule] no Slack ID for %s (%s) — skipping DM", e.player.Name, e.player.Email)
			continue
		}

		text := fmt.Sprintf(
			"🏓 *Match Scheduled!*\nYou're playing *%s*\n📅 %s%s\n\nBring your A-game! If you can't make it, tap below and the admin will reschedule.",
			e.opponent, timeStr, tableStr,
		)
		blocks := slacklib.NewBlocks().
			AddSection(text).
			AddButton("cant_make_match", "🚫 Can't Make It", m.ID).
			Build()

		if _, err := botRef.SendMessageWithBlocks(context.Background(), slackID, blocks); err != nil {
			log.Printf("[Schedule] DM failed for %s (%s): %v", e.player.Name, slackID, err)
		} else {
			log.Printf("[Schedule] DM sent to %s (%s) for match %s", e.player.Name, slackID, m.ID)
		}
	}
}

// handleAdminScheduleMatch sets the scheduled time (and optional table) for a match.
// PUT /api/admin/matches/:id/schedule
func handleAdminScheduleMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		matchID := c.Param("id")
		var req struct {
			ScheduledTime string `json:"scheduled_time" binding:"required"` // "2006-01-02T15:04"
			TableNum      string `json:"table_num"`
			Notify        bool   `json:"notify"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[Admin] PUT /api/admin/matches/%s/schedule time=%s table=%s notify=%v",
			matchID, req.ScheduledTime, req.TableNum, req.Notify)

		loc, _ := time.LoadLocation(tournamentTZ)
		t, err := time.ParseInLocation("2006-01-02T15:04", req.ScheduledTime, loc)
		if err != nil {
			t, err = time.Parse(time.RFC3339, req.ScheduledTime)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "use format 2006-01-02T15:04 (e.g. 2026-09-22T13:00)"})
				return
			}
		}

		store.mu.Lock()
		m, ok := store.matchByID[matchID]
		if !ok {
			store.mu.Unlock()
			c.JSON(http.StatusNotFound, gin.H{"error": "match not found"})
			return
		}
		m.ScheduledTime = &t
		m.TableNum = req.TableNum
		mcopy := *m // copy for DMs outside lock
		store.mu.Unlock()

		if req.Notify {
			go sendMatchDMs(&mcopy)
		}

		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"scheduled_time": t.Format(time.RFC3339),
			"table_num":      req.TableNum,
		})
	}
}

// handleAdminAutoSchedule assigns time slots to all pending matches with both teams assigned.
// POST /api/admin/matches/autoschedule
func handleAdminAutoSchedule() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Date   string `json:"date"`   // "2026-09-22" (defaults to today)
			Notify bool   `json:"notify"` // send player DMs?
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		loc, _ := time.LoadLocation(tournamentTZ)
		if req.Date == "" {
			req.Date = time.Now().In(loc).Format("2006-01-02")
		}
		log.Printf("[Admin] POST /api/admin/matches/autoschedule date=%s notify=%v", req.Date, req.Notify)

		// Build time slot list
		var slots []time.Time
		for _, s := range defaultSlots {
			t, err := time.ParseInLocation("2006-01-02 15:04", req.Date+" "+s, loc)
			if err == nil {
				slots = append(slots, t)
			}
		}

		roundOrder := map[string]int{"rr": 0, "qf": 1, "sf": 2, "final": 3}

		store.mu.Lock()

		var pending []*Match
		for _, m := range store.matches {
			if m.Team1 != nil && m.Team2 != nil && m.Status != "complete" {
				pending = append(pending, m)
			}
		}
		sort.Slice(pending, func(i, j int) bool {
			ri, rj := roundOrder[pending[i].Round], roundOrder[pending[j].Round]
			if ri != rj {
				return ri < rj
			}
			if pending[i].GroupNumber != pending[j].GroupNumber {
				return pending[i].GroupNumber < pending[j].GroupNumber
			}
			return pending[i].MatchOrder < pending[j].MatchOrder
		})

		// Extend slots beyond predefined list if needed (30-min increments)
		for len(slots) < len(pending) {
			if len(slots) == 0 {
				break
			}
			slots = append(slots, slots[len(slots)-1].Add(30*time.Minute))
		}

		const numTables = 2
		var copies []Match
		for i, m := range pending {
			if i >= len(slots) {
				break
			}
			t := slots[i]
			m.ScheduledTime = &t
			m.TableNum = fmt.Sprintf("%d", (i%numTables)+1)
			copies = append(copies, *m)
		}
		store.mu.Unlock()

		if req.Notify {
			go func() {
				for i := range copies {
					sendMatchDMs(&copies[i])
				}
			}()
		}

		log.Printf("[Admin] auto-scheduled %d matches for %s", len(copies), req.Date)
		c.JSON(http.StatusOK, gin.H{
			"scheduled": len(copies),
			"date":      req.Date,
		})
	}
}

// RegisterScheduleHandlers sets the bot reference and wires up the "Can't Make It" Slack button.
// Call this from main.go after creating the bot and store.
func RegisterScheduleHandlers(bot *slacklib.Bot) {
	botRef = bot

	bot.Action("cant_make_match", func(ctx *slacklib.ActionContext) {
		matchID := ctx.Value
		log.Printf("[Schedule] cant_make_match: user=%s match=%s", ctx.UserID, matchID)

		store.mu.RLock()
		m := store.matchByID[matchID]
		store.mu.RUnlock()

		matchInfo := "your upcoming match"
		if m != nil && m.Team1 != nil && m.Team2 != nil {
			t1 := m.Team1.Player1.Name + " & " + m.Team1.Player2.Name
			t2 := m.Team2.Player1.Name + " & " + m.Team2.Player2.Name
			matchInfo = fmt.Sprintf("*%s vs %s*", t1, t2)
			if m.ScheduledTime != nil {
				matchInfo += " (" + fmtMatchTime(m.ScheduledTime) + ")"
			}
		}

		// Update the original message so the button shows as handled
		_ = ctx.UpdateMessage(
			"✅ Conflict noted for " + matchInfo + ". The admin will reschedule — sit tight! 👋",
		)

		// Notify admin if ADMIN_SLACK_USER is set
		if adminID := os.Getenv("ADMIN_SLACK_USER"); adminID != "" {
			msg := fmt.Sprintf(
				"⚠️ <@%s> can't make their match: %s\nReschedule at: https://applied-ping-pong.experimental.apps.applied.dev/admin",
				ctx.UserID, matchInfo,
			)
			if _, err := bot.SendDM(ctx.Context(), adminID, msg); err != nil {
				log.Printf("[Schedule] failed to DM admin (%s): %v", adminID, err)
			} else {
				log.Printf("[Schedule] admin (%s) notified about conflict for match %s", adminID, matchID)
			}
		} else {
			log.Printf("[Schedule] ADMIN_SLACK_USER not set — admin notification skipped")
		}
	})
}

// registerScheduleRoutes adds schedule API endpoints to the admin group.
func registerScheduleRoutes(admin *gin.RouterGroup) {
	admin.PUT("/matches/:id/schedule", handleAdminScheduleMatch())
	admin.POST("/matches/autoschedule", handleAdminAutoSchedule())
}
