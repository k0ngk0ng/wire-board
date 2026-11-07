package server

// Failure inventory (written before implementation):
// - Shared victory/loss awards only some seats, or no-winner loss is skipped.
// - A client supplies a policy/result; a partial or competitive result scores.
// - Existing cooperative matches, old saved matches, bots, game-owned computer
//   players or practice matches gain points after an upgrade/rematch.
// - Manual autoplay silently revokes the promised rating eligibility.
// - Close, idle expiry (including paused expiry and startup cleanup) bypass loss;
//   platform cleanup fabricates a game Result or rewrites the module state.
// - Cancellation during first loading or administrative interruption charges loss;
//   reconnect/restart resets the started flag, frozen policy or roster.
// - Failed snapshot/ledger writes publish a result; retries/restart/archive/rematch
//   write the same match twice or revive a previous match's policy.
// The fixture is deliberately a tiny cooperative game, not a Contra gameplay
// test. Authenticated HTTP drives its two legal progress actions. Only idle time
// is accelerated, through the actual scheduler and persisted LastActive value.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const coopCampaignTestPolicy = "coop_campaign_v1"

// Failure found by the real Contra victory browser fixture: independent-room
// summaries returned before exposing the frozen policy/liability/eligibility,
// even while SQLite retained them. False values must also reach clients so a
// loading/practice room cannot inherit a previous actionable warning.
func TestCoopCampaignRoomSummaryHTTP(t *testing.T) {
	for _, practice := range []bool{false, true} {
		t.Run(fmt.Sprintf("practice-%v", practice), func(t *testing.T) {
			h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{Win: true, Practice: practice}, 2, false, true)
			check := func(started bool) {
				for _, person := range h.people {
					room := current(person)
					if room["ratingPolicy"] != coopCampaignTestPolicy || room["ratingStarted"] != started || room["ratingEligible"] != !practice {
						t.Fatalf("missing/incorrect frozen challenge summary: policy=%v started=%v eligible=%v", room["ratingPolicy"], room["ratingStarted"], room["ratingEligible"])
					}
				}
			}
			check(false)
			h.loaded()
			check(true)
			h.action("advance", 200)
			check(true)
			h.action("finish", 200)
			if practice {
				h.record(0, "cleared", "practice")
			} else {
				h.record(20, "cleared", "")
			}
		})
	}
}

type coopCampaignOptions struct {
	Win              bool
	Practice         bool
	Computer         bool
	TerminalPractice bool
}

// Reflective opt-in lets the red test compile against the pre-feature API.
func coopCampaignSetField(target any, name string, value any) {
	f := reflect.ValueOf(target).Elem().FieldByName(name)
	if f.IsValid() {
		f.Set(reflect.ValueOf(value).Convert(f.Type()))
	}
}

func coopCampaignModule(policy string, options coopCampaignOptions) gameapi.Module {
	m := acceptanceModule()
	m.MinPlayers, m.MaxPlayers, m.Realtime = 1, 2, true
	coopCampaignSetField(&m, "RatingPolicy", policy)
	m.Configure = func(_ int, raw json.RawMessage) (json.RawMessage, error) {
		if len(raw) > 0 && string(raw) != "{}" && string(raw) != "null" {
			return nil, errors.New("fixture has no client options")
		}
		return json.RawMessage(`{}`), nil
	}
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		actors := make([]int, c.Players)
		for i := range actors {
			actors[i] = i
		}
		v := gameapi.Match{Schema: 1, Data: json.RawMessage(`{"progress":0}`), Actors: actors, Attention: []int{}, HasComputerPlayers: options.Computer}
		coopCampaignSetField(&v, "Unrated", options.Practice)
		return v, nil
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var a struct {
			Type string `json:"type"`
		}
		var data struct {
			Progress int `json:"progress"`
		}
		if json.Unmarshal(raw, &a) != nil || json.Unmarshal(v.Data, &data) != nil || c.Actor != 0 {
			return v, errors.New("invalid fixture action")
		}
		switch a.Type {
		case "advance":
			data.Progress++
		case "pause":
			v.Paused = true
		case "resume":
			v.Paused = false
		case "computer":
			v.HasComputerPlayers = true
		case "clear-computer":
			v.HasComputerPlayers = false
		case "finish", "partial", "competitive":
			if data.Progress != 1 {
				return v, errors.New("challenge is not complete")
			}
			ranks, winners := make([]int, c.Players), []int{}
			for i := range ranks {
				ranks[i] = 1
				if options.Win {
					winners = append(winners, i)
				}
			}
			if a.Type == "partial" {
				winners = []int{0}
			}
			v.Result = &gameapi.Result{Cooperative: a.Type != "competitive", Ranks: ranks, Winners: winners, Unrated: options.TerminalPractice}
			v.Actors, v.Attention, v.Paused = nil, nil, false
		default:
			return v, errors.New("unknown fixture action")
		}
		v.Data, _ = json.Marshal(data)
		return v, nil
	}
	m.View = func(v gameapi.Match, _ int) (any, error) {
		var data any
		err := json.Unmarshal(v.Data, &data)
		return data, err
	}
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }
	m.Reclaim = func(_ gameapi.Context, v gameapi.Match) (gameapi.Match, error) { return v, nil }
	m.OnTimer = func(_ gameapi.Context, v gameapi.Match) (gameapi.Match, error) { return v, nil }
	return m
}

type coopCampaignHTTP struct {
	t      *testing.T
	s      *Server
	ts     *httptest.Server
	cfg    Config
	files  fstest.MapFS
	people []*testClient
	trace  []any
	id     string
}

func newCoopCampaignHTTP(t *testing.T, policy string, options coopCampaignOptions, humans int, bot, loading bool) *coopCampaignHTTP {
	t.Helper()
	h := &coopCampaignHTTP{t: t, cfg: Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{coopCampaignModule(policy, options)}, ClientLoading: loading}, files: fstest.MapFS{"index.html": {Data: []byte("coop campaign HTTP")}}}
	h.open()
	t.Cleanup(func() {
		h.ts.Close()
		h.s.Close()
		out := filepath.Join("..", "..", ".local", "coop-campaign", "evidence")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "authenticated HTTP and SQLite; fixture game only", "trace": h.trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	for i := 0; i < humans; i++ {
		c := newClient(t, h.ts.URL)
		c.register(fmt.Sprintf("合作积分%d", i))
		h.people = append(h.people, c)
	}
	a := h.people[0]
	capacity := humans
	if bot {
		capacity++
	}
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "合作计分", "capacity": capacity}, 201)
	h.id = r["id"].(string)
	for _, c := range h.people[1:] {
		c.command(current(a), "join", nil, 200)
	}
	if bot {
		a.command(current(a), "add_bot", nil, 200)
	}
	for _, c := range h.people {
		c.command(current(c), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	h.trace = append(h.trace, current(a))
	return h
}

func (h *coopCampaignHTTP) open() {
	h.t.Helper()
	var err error
	h.s, err = New(h.cfg, h.files)
	if err != nil {
		h.t.Fatal(err)
	}
	stopBotTicker(h.s)
	h.ts = httptest.NewServer(h.s.Handler())
	for _, c := range h.people {
		c.base = h.ts.URL
	}
}
func (h *coopCampaignHTTP) restart(policy string) {
	h.t.Helper()
	h.ts.Close()
	h.s.Close()
	coopCampaignSetField(&h.cfg.Modules[0], "RatingPolicy", policy)
	h.open()
}
func (h *coopCampaignHTTP) action(kind string, status int) {
	h.t.Helper()
	a := h.people[0]
	a.command(current(a), "action", map[string]any{"type": kind}, status)
}
func (h *coopCampaignHTTP) loaded() {
	h.t.Helper()
	g := loadInfo(h.t, h.people[0])
	token, build := g["token"].(string), g["build"].(string)
	for i, c := range h.people {
		for _, kind := range []string{"load_begin", "load_ready"} {
			code, data := loadingCommand(c, current(c), kind, token, build, fmt.Sprintf("client-%d", i))
			if code != 200 {
				h.t.Fatal(code, data)
			}
		}
	}
	if current(h.people[0])["loading"] != nil {
		h.t.Fatal("loading remained")
	}
}
func (h *coopCampaignHTTP) record(delta int, outcome, reason string) MatchRecord {
	h.t.Helper()
	r := current(h.people[0])
	h.trace = append(h.trace, r)
	result, ok := r["result"].(map[string]any)
	if !ok {
		h.t.Fatal("missing archive", r)
	}
	if result["rated"] != (delta != 0) || result["outcome"] != outcome {
		h.t.Fatalf("wrong outcome/rating: %+v", result)
	}
	if reason != "" && result["unratedReason"] != reason {
		h.t.Fatalf("wrong unrated reason: %+v", result)
	}
	raw, _ := json.Marshal(result)
	var record MatchRecord
	_ = json.Unmarshal(raw, &record)
	if len(record.Players) != len(r["seats"].([]any)) {
		h.t.Fatal("roster lost", record)
	}
	if delta != 0 && result["ratingPolicy"] != coopCampaignTestPolicy {
		h.t.Fatal("policy not archived", result)
	}
	for _, p := range record.Players {
		if p.RatingDelta != delta || p.Won != (outcome == "cleared" && !record.Unrated) {
			h.t.Fatalf("wrong player result: %+v", p)
		}
		if delta != 0 && p.Rank != 1 {
			h.t.Fatal("cooperative rank changed", p)
		}
	}
	var count, total int
	if err := h.s.db.QueryRow("SELECT COUNT(*),COALESCE(SUM(delta),0) FROM rating_ledger WHERE match_id=?", record.ID).Scan(&count, &total); err != nil {
		h.t.Fatal(err)
	}
	want := 0
	if delta != 0 {
		want = len(record.Players)
	}
	if count != want || total != want*delta {
		h.t.Fatal("wrong ledger", count, total)
	}
	return record
}

func TestCoopCampaignSharedHTTP(t *testing.T) {
	for _, n := range []int{1, 2} {
		for _, win := range []bool{false, true} {
			t.Run(fmt.Sprintf("players-%d-win-%t", n, win), func(t *testing.T) {
				h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{Win: win}, n, false, true)
				h.action("finish", 400)
				h.loaded()
				if n == 2 {
					setAutoPlay(h.people[1], current(h.people[1]), true, 200)
				}
				h.action("advance", 200)
				if n == 2 {
					h.action("partial", 400)
				}
				h.action("competitive", 400)
				h.restart("") // Removing the module opt-in cannot change an existing promise.
				a := h.people[0]
				request := map[string]any{"type": "action", "action": map[string]any{"type": "finish"}, "version": current(a)["version"], "nonce": randomID(12)}
				a.post("/api/rooms/"+h.id, request, 200)
				delta, outcome := -10, "failed"
				if win {
					delta, outcome = 20, "cleared"
				}
				first := h.record(delta, outcome, "")
				a.post("/api/rooms/"+h.id, request, 200)
				h.restart("")
				h.record(delta, outcome, "")
				a.command(current(a), "rematch", nil, 200)
				for _, c := range h.people {
					c.command(current(c), "ready", nil, 200)
				}
				a.command(current(a), "start", nil, 200)
				h.loaded()
				h.action("advance", 200)
				h.action("finish", 200)
				if current(a)["result"].(map[string]any)["rated"] != false {
					t.Fatal("new match inherited retired policy")
				}
				var rows int
				_ = h.s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger WHERE match_id=?", first.ID).Scan(&rows)
				if rows != n {
					t.Fatal("old match rerated", rows)
				}
			})
		}
	}
}

func TestCoopCampaignEligibilityHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, policy, reason string
		options              coopCampaignOptions
		bot, old             bool
	}{
		{"bot", coopCampaignTestPolicy, "computer", coopCampaignOptions{Win: true}, true, false},
		{"owned-computer", coopCampaignTestPolicy, "computer", coopCampaignOptions{Win: true, Computer: true}, false, false},
		{"practice", coopCampaignTestPolicy, "practice", coopCampaignOptions{Win: true, Practice: true}, false, false},
		{"terminal-practice", coopCampaignTestPolicy, "practice", coopCampaignOptions{Win: true, TerminalPractice: true}, false, false},
		{"legacy", "", "cooperative_unrated", coopCampaignOptions{Win: true}, false, false},
		{"old-start", "", "cooperative_unrated", coopCampaignOptions{Win: true}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCoopCampaignHTTP(t, tc.policy, tc.options, 1, tc.bot, false)
			if tc.old {
				h.restart(coopCampaignTestPolicy)
			}
			h.action("advance", 200)
			h.action("finish", 200)
			h.record(0, "cleared", tc.reason)
		})
	}
	t.Run("computer-ever", func(t *testing.T) {
		h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{Win: true}, 2, false, false)
		h.action("computer", 200)
		h.action("clear-computer", 200)
		h.action("advance", 200)
		h.action("finish", 200)
		h.record(0, "cleared", "computer")
	})
}

func TestCoopCampaignAbandonHTTP(t *testing.T) {
	for _, mode := range []string{"close", "idle", "paused-idle", "restart-idle", "restart-paused-idle"} {
		t.Run(mode, func(t *testing.T) {
			h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{}, 2, false, true)
			h.loaded()
			h.action("advance", 200)
			paused := strings.Contains(mode, "paused")
			if paused {
				h.action("pause", 200)
			}
			h.s.mu.Lock()
			before, _ := json.Marshal(h.s.rooms[h.id].Module)
			h.s.mu.Unlock()
			outcome := "abandoned"
			if mode == "close" {
				a := h.people[0]
				h.people[1].command(current(a), "close", nil, 400)
				a.command(current(a), "close", nil, 200)
			} else {
				outcome = "expired"
				age := 25 * time.Hour
				if paused {
					age = 31 * 24 * time.Hour
				}
				h.s.mu.Lock()
				h.s.rooms[h.id].LastActive = time.Now().Add(-age).Unix()
				err := h.s.save(h.s.rooms[h.id])
				h.s.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(mode, "restart") {
					h.restart("")
				} else {
					h.s.mu.Lock()
					h.s.expireIdleRooms(time.Now())
					h.s.mu.Unlock()
				}
			}
			h.record(-10, outcome, "")
			h.s.mu.Lock()
			after, _ := json.Marshal(h.s.rooms[h.id].Module)
			h.s.mu.Unlock()
			if string(after) != string(before) {
				t.Fatal("platform termination fabricated module terminal state", string(before), string(after))
			}
			h.restart("")
			h.record(-10, outcome, "")
		})
	}
}

func TestCoopCampaignCancellationAndAtomicityHTTP(t *testing.T) {
	t.Run("loading-close", func(t *testing.T) {
		h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{}, 2, false, true)
		h.restart(coopCampaignTestPolicy)
		a := h.people[0]
		a.command(current(a), "close", nil, 200)
		h.record(0, "cancelled", "not_started")
	})
	t.Run("loading-expiry", func(t *testing.T) {
		h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{}, 2, false, true)
		h.s.mu.Lock()
		h.s.expireIdleRooms(time.Now().Add(25 * time.Hour))
		h.s.mu.Unlock()
		h.record(0, "cancelled", "not_started")
	})
	t.Run("admin", func(t *testing.T) {
		h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{}, 2, false, false)
		root := newClient(t, h.ts.URL)
		root.post("/api/login", map[string]any{"name": "ModuleRoot", "password": "module-root-test-123"}, 200)
		root.post("/api/admin/rooms/"+h.id+"/close", map[string]any{}, 200)
		h.record(0, "interrupted", "administrative")
	})
	for _, mode := range []string{"finish", "close", "idle"} {
		t.Run("rollback-"+mode, func(t *testing.T) {
			h := newCoopCampaignHTTP(t, coopCampaignTestPolicy, coopCampaignOptions{Win: true}, 2, false, false)
			h.action("advance", 200)
			before := current(h.people[0])
			if _, err := h.s.db.Exec("CREATE TRIGGER reject_coop_ledger BEFORE INSERT ON rating_ledger BEGIN SELECT RAISE(ABORT,'coop ledger failure'); END"); err != nil {
				t.Fatal(err)
			}
			if mode == "finish" {
				h.action("finish", 500)
			} else if mode == "close" {
				h.people[0].command(before, "close", nil, 500)
			} else {
				h.s.mu.Lock()
				h.s.expireIdleRooms(time.Now().Add(25 * time.Hour))
				h.s.mu.Unlock()
			}
			if !reflect.DeepEqual(before, current(h.people[0])) {
				t.Fatal("failed transaction published terminal state")
			}
			var count int
			_ = h.s.db.QueryRow("SELECT COUNT(*) FROM match_history").Scan(&count)
			if count != 0 {
				t.Fatal("failed ledger retained archive", count)
			}
			if _, err := h.s.db.Exec("DROP TRIGGER reject_coop_ledger"); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "finish":
				h.action("finish", 200)
				h.record(20, "cleared", "")
			case "close":
				h.people[0].command(current(h.people[0]), "close", nil, 200)
				h.record(-10, "abandoned", "")
			case "idle":
				h.s.mu.Lock()
				h.s.expireIdleRooms(time.Now().Add(25 * time.Hour))
				h.s.mu.Unlock()
				h.record(-10, "expired", "")
			}
		})
	}
}
