package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const stateSecretID = "applied-ping-pong-tournament-state"

// persistedState is the full snapshot of the store serialized to Secret Manager.
type persistedState struct {
	Players          []*Player   `json:"players"`
	Teams            []*Team     `json:"teams"`
	Tournament       *Tournament `json:"tournament,omitempty"`
	Matches          []*Match    `json:"matches"`
	GroupAssignments [][]string  `json:"group_assignments"`
	RegistrationOpen bool        `json:"registration_open"`
	SavedAt          time.Time   `json:"saved_at"`
}

var (
	smOnce   sync.Once
	smClient *secretmanager.Client
	smErr    error
)

func getSmClient() (*secretmanager.Client, error) {
	smOnce.Do(func() {
		smClient, smErr = secretmanager.NewClient(context.Background())
		if smErr != nil {
			log.Printf("[Persist] failed to init Secret Manager client: %v", smErr)
		} else {
			log.Printf("[Persist] Secret Manager client initialized")
		}
	})
	return smClient, smErr
}

func smProjectID() string {
	if p := os.Getenv("PROJECT_ID"); p != "" {
		return p
	}
	return ""
}

func stateSecretParent() string {
	return fmt.Sprintf("projects/%s/secrets/%s", smProjectID(), stateSecretID)
}

// saveState serializes the full store to Secret Manager.
// Safe to call from a goroutine (fire and forget).
func saveState(ctx context.Context) {
	project := smProjectID()
	if project == "" {
		log.Printf("[Persist] no PROJECT_ID env var — skipping save (local dev mode)")
		return
	}

	client, err := getSmClient()
	if err != nil {
		log.Printf("[Persist] save skipped — no SM client: %v", err)
		return
	}

	// Snapshot current state under read lock
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

	data, err := json.Marshal(snap)
	if err != nil {
		log.Printf("[Persist] marshal error: %v", err)
		return
	}

	secretParent := stateSecretParent()

	// Try adding a new version; create the secret first if it doesn't exist yet.
	_, addErr := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  secretParent,
		Payload: &secretmanagerpb.SecretPayload{Data: data},
	})
	if addErr != nil {
		if status.Code(addErr) == codes.NotFound {
			log.Printf("[Persist] secret not found — creating it")
			_, createErr := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
				Parent:   fmt.Sprintf("projects/%s", project),
				SecretId: stateSecretID,
				Secret: &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_Automatic_{
							Automatic: &secretmanagerpb.Replication_Automatic{},
						},
					},
				},
			})
			if createErr != nil {
				log.Printf("[Persist] failed to create secret: %v", createErr)
				return
			}
			// Retry after creation
			_, addErr = client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
				Parent:  secretParent,
				Payload: &secretmanagerpb.SecretPayload{Data: data},
			})
		}
		if addErr != nil {
			log.Printf("[Persist] failed to add secret version: %v", addErr)
			return
		}
	}

	log.Printf("[Persist] state saved (%d bytes, %d players, %d teams, %d matches)",
		len(data), len(snap.Players), len(snap.Teams), len(snap.Matches))
}

// loadState tries to restore the store from the latest Secret Manager version.
// Returns true if state was successfully loaded (caller should skip seedTournamentData).
func loadState(ctx context.Context) bool {
	project := smProjectID()
	if project == "" {
		log.Printf("[Persist] no PROJECT_ID — skipping load (local dev mode)")
		return false
	}

	client, err := getSmClient()
	if err != nil {
		log.Printf("[Persist] load skipped — no SM client: %v", err)
		return false
	}

	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%s/versions/latest", stateSecretParent()),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Printf("[Persist] no saved state found — will seed fresh data")
		} else {
			log.Printf("[Persist] failed to access secret version: %v — will seed fresh data", err)
		}
		return false
	}

	var snap persistedState
	if err := json.Unmarshal(result.Payload.Data, &snap); err != nil {
		log.Printf("[Persist] failed to unmarshal state: %v — will seed fresh data", err)
		return false
	}

	if len(snap.Players) == 0 {
		log.Printf("[Persist] loaded state is empty — will seed fresh data")
		return false
	}

	// Restore store under write lock
	store.mu.Lock()
	defer store.mu.Unlock()

	store.players = snap.Players
	store.teams = snap.Teams
	store.tournament = snap.Tournament
	store.matches = snap.Matches
	store.groupAssignments = snap.GroupAssignments
	store.registrationOpen = snap.RegistrationOpen

	// Rebuild player lookup maps
	store.playerByEmail = make(map[string]*Player)
	store.playerByID = make(map[string]*Player)
	for _, p := range store.players {
		if p != nil {
			store.playerByEmail[p.Email] = p
			store.playerByID[p.ID] = p
		}
	}

	// Rebuild team map and patch player pointers to canonical instances
	store.teamByID = make(map[string]*Team)
	for _, t := range store.teams {
		if t == nil {
			continue
		}
		store.teamByID[t.ID] = t
		if t.Player1 != nil {
			if canon := store.playerByID[t.Player1.ID]; canon != nil {
				t.Player1 = canon
			}
		}
		if t.Player2 != nil {
			if canon := store.playerByID[t.Player2.ID]; canon != nil {
				t.Player2 = canon
			}
		}
	}

	// Rebuild match map and patch team pointers to canonical instances
	store.matchByID = make(map[string]*Match)
	for _, m := range store.matches {
		if m == nil {
			continue
		}
		store.matchByID[m.ID] = m
		if m.Team1 != nil {
			if canon := store.teamByID[m.Team1.ID]; canon != nil {
				m.Team1 = canon
			}
		}
		if m.Team2 != nil {
			if canon := store.teamByID[m.Team2.ID]; canon != nil {
				m.Team2 = canon
			}
		}
	}

	log.Printf("[Persist] state restored from Secret Manager (saved at %s): %d players, %d teams, %d matches, tournament=%v",
		snap.SavedAt.Format(time.RFC3339),
		len(store.players), len(store.teams), len(store.matches),
		store.tournament != nil)
	return true
}

// ---- GCS fallback persistence ----
// Used when Secret Manager write access is unavailable.
// Writes state.json to gs://{project}-ping-pong-state/state.json

func gcsBucket() string {
	return fmt.Sprintf("%s-ping-pong-state", smProjectID())
}

func gcsObjectURL() string {
	return fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=state.json", gcsBucket())
}

func gcsReadURL() string {
	return fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/state.json?alt=media", gcsBucket())
}

func gcsToken(ctx context.Context) (string, error) {
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/devstorage.read_write")
	if err != nil {
		return "", err
	}
	tok, err := ts.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func saveStateGCS(ctx context.Context, data []byte) error {
	tok, err := gcsToken(ctx)
	if err != nil {
		return fmt.Errorf("GCS token: %w", err)
	}
	// Ensure bucket exists (create if needed)
	createBucketURL := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b?project=%s", smProjectID())
	bucketBody, _ := json.Marshal(map[string]interface{}{
		"name":         gcsBucket(),
		"location":     "US",
		"storageClass": "STANDARD",
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", createBucketURL, bytes.NewReader(bucketBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
		// 200 = created, 409 = already exists — both are fine
	}

	// Upload state
	req2, err := http.NewRequestWithContext(ctx, "POST", gcsObjectURL(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req2.Header.Set("Authorization", "Bearer "+tok)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return fmt.Errorf("GCS upload: %w", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		body, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("GCS upload status %d: %s", resp2.StatusCode, string(body)[:min(200, len(body))])
	}
	return nil
}

func loadStateGCS(ctx context.Context) ([]byte, error) {
	tok, err := gcsToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCS token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", gcsReadURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GCS read: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, nil // not found
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GCS read status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// saveStateWithFallback saves to Google Sheets (primary, see sheets.go), then Secret Manager,
// then GCS. Callers fire it as a goroutine right after a mutation; the passed-in context is
// usually the HTTP request's, which Gin cancels as soon as the response is written, so the
// writes run on their own background context with a timeout instead.
func saveStateWithFallback(_ context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sheetsStatusMu.Lock()
	lastLocalMutation = time.Now()
	sheetsStatusMu.Unlock()

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

	data, err := json.Marshal(snap)
	if err != nil {
		log.Printf("[Persist] marshal error: %v", err)
		return
	}

	if sheetsEnabled() {
		if err := pushStateToSheet(ctx, data); err != nil {
			log.Printf("[Persist] Google Sheets save failed: %v", err)
		}
	}

	project := smProjectID()
	if project == "" {
		return
	}

	client, err := getSmClient()
	if err != nil {
		log.Printf("[Persist] SM client error, trying GCS: %v", err)
	}

	// Try Secret Manager
	smOK := false
	if client != nil {
		secretParent := stateSecretParent()
		_, addErr := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
			Parent:  secretParent,
			Payload: &secretmanagerpb.SecretPayload{Data: data},
		})
		if addErr == nil {
			smOK = true
			log.Printf("[Persist] saved to Secret Manager (%d bytes)", len(data))
		} else {
			log.Printf("[Persist] SM write failed (%v), falling back to GCS", status.Code(addErr))
		}
	}

	if !smOK {
		if gcsErr := saveStateGCS(ctx, data); gcsErr != nil {
			log.Printf("[Persist] GCS save also failed: %v", gcsErr)
		} else {
			log.Printf("[Persist] saved to GCS bucket %s (%d bytes)", gcsBucket(), len(data))
		}
	}
}

// loadStateWithFallback tries Secret Manager then GCS.
func loadStateWithFallback(ctx context.Context) bool {
	// Try Secret Manager first
	if loadState(ctx) {
		return true
	}
	// Try GCS fallback
	project := smProjectID()
	if project == "" {
		return false
	}
	data, err := loadStateGCS(ctx)
	if err != nil {
		log.Printf("[Persist] GCS load error: %v", err)
		return false
	}
	if data == nil {
		log.Printf("[Persist] no GCS state found")
		return false
	}
	var snap persistedState
	if err := json.Unmarshal(data, &snap); err != nil {
		log.Printf("[Persist] GCS unmarshal error: %v", err)
		return false
	}
	if len(snap.Players) == 0 {
		return false
	}
	return restoreSnap(snap)
}

// restoreSnap applies a persistedState snapshot to the store (must not hold lock).
func restoreSnap(snap persistedState) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.players = snap.Players
	store.teams = snap.Teams
	store.tournament = snap.Tournament
	store.matches = snap.Matches
	store.groupAssignments = snap.GroupAssignments
	store.registrationOpen = snap.RegistrationOpen

	store.playerByEmail = make(map[string]*Player)
	store.playerByID = make(map[string]*Player)
	for _, p := range store.players {
		if p != nil {
			store.playerByEmail[p.Email] = p
			store.playerByID[p.ID] = p
		}
	}
	store.teamByID = make(map[string]*Team)
	for _, t := range store.teams {
		if t == nil {
			continue
		}
		store.teamByID[t.ID] = t
		if t.Player1 != nil {
			if c := store.playerByID[t.Player1.ID]; c != nil {
				t.Player1 = c
			}
		}
		if t.Player2 != nil {
			if c := store.playerByID[t.Player2.ID]; c != nil {
				t.Player2 = c
			}
		}
	}
	store.matchByID = make(map[string]*Match)
	for _, m := range store.matches {
		if m == nil {
			continue
		}
		store.matchByID[m.ID] = m
		if m.Team1 != nil {
			if c := store.teamByID[m.Team1.ID]; c != nil {
				m.Team1 = c
			}
		}
		if m.Team2 != nil {
			if c := store.teamByID[m.Team2.ID]; c != nil {
				m.Team2 = c
			}
		}
	}
	log.Printf("[Persist] snapshot restored (saved %s): %d players, %d teams, %d matches",
		snap.SavedAt.Format(time.RFC3339), len(store.players), len(store.teams), len(store.matches))
	return true
}
