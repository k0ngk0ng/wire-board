package bubbleparty

// Failure scenarios (recorded before adding this E2E harness):
// - any of the 112 source maps or 2/4/6/8-seat configurations cannot start;
// - computer actions bypass validation, mutate simulation directly, stop before
//   a natural result, or repeatedly request impossible airborne bomb placement;
// - selection, countdown, native movement, boxes, terrain, items and water do
//   not advance through the same paths used by production Apply/OnTimer;
// - save/restore changes clocks, sequences, controls, positions, mounts or map
//   objects; restored play cannot continue to a natural result;
// - timeout invents winners, missing seats in ranks/scores, or mixed winner
//   teams; invalid positions/abilities/entities escape persisted validation.
// The only accelerated component is wall-clock time. No result, death, item,
// player position, board, phase or match deadline is injected by this harness.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type nativeIdleSeat struct {
	Seat         int   `json:"seat"`
	X            int   `json:"x"`
	Y            int   `json:"y"`
	InitialExits int   `json:"initialExits"`
	Alive        bool  `json:"aliveAtEnd"`
	Inputs       int64 `json:"inputs"`
	Crates       int   `json:"crates"`
}
type nativeMatchReport struct {
	UnmovedSeats                []nativeIdleSeat `json:"unmovedSeats,omitempty"`
	InvalidAirborneBombRequests int              `json:"invalidAirborneBombRequests"`
	Map                         string           `json:"map"`
	Players                     int              `json:"players"`
	Seed                        uint64           `json:"seed"`
	ElapsedMS                   int64            `json:"elapsedMS"`
	Inputs                      int              `json:"inputs"`
	Restores                    int              `json:"restores"`
	Bombs                       int              `json:"bombs"`
	Pushes                      int              `json:"pushes"`
	Pickups                     int              `json:"pickups"`
	MountPickups                int              `json:"mountPickups"`
	Dismounts                   int              `json:"dismounts"`
	AirborneBombRequests        int              `json:"airborneBombRequests"`
	LongestAirborneIdleMS       int64            `json:"longestAirborneIdleMS"`
	MovedSeats                  int              `json:"movedSeats"`
	Timeout                     bool             `json:"timeout"`
	Draw                        bool             `json:"draw"`
	Winners                     []int            `json:"winners"`
	Reason                      string           `json:"reason"`
}

func nativeFullmatchRestore(t *testing.T, s *State, players int) *State {
	t.Helper()
	before, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restore(gameapi.Context{Players: players, Now: time.UnixMilli(s.ServerAt)}, m)
	if err != nil {
		t.Fatal("restore", err)
	}
	again, err := decode(restored)
	if err != nil {
		t.Fatal("restored decode", err)
	}
	after, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("restore changed live simulation state")
	}
	return again
}

func runNativeFullmatch(t *testing.T, mapID string, players int, seed uint64) nativeMatchReport {
	t.Helper()
	report := nativeMatchReport{Map: mapID, Players: players, Seed: seed}
	ctx := gameapi.Context{Players: players, Now: time.UnixMilli(1_700_000_000_000)}
	m, err := startWithSeed(ctx, mustJSON(Options{Map: mapID, RoundSeconds: 180}), seed)
	if err != nil {
		t.Fatal(err)
	}
	// Selection uses the public bot/apply boundary. The match loop below avoids
	// repeated entire-state JSON parsing while keeping input validation and the
	// exact production Apply ordering for every computer decision.
	for seat := 0; seat < players; seat++ {
		ctx.Actor = seat
		action, err := botAction(ctx, m)
		if err != nil || len(action) == 0 {
			t.Fatalf("selection seat%d: %v", seat, err)
		}
		m, err = apply(ctx, m, action)
		if err != nil {
			t.Fatal("select apply", err)
		}
	}
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhaseCountdown {
		t.Fatal("selection did not produce countdown", s.Phase)
	}
	s.onTimer(s.StartAt)
	if s.Phase != PhasePlaying {
		t.Fatal("countdown did not start play")
	}
	started, deadline := s.StartAt, s.MatchEndsAt
	moved := make([]bool, players)
	lastX, lastY := make([]float64, players), make([]float64, players)
	airIdle := make([]int64, players)
	initialExits := make([]int, players)
	for i, p := range s.Players {
		lastX[i], lastY[i] = float64(p.X)+p.OffsetX, float64(p.Y)+p.OffsetY
		for _, d := range directions {
			if s.canWalk(&p, p.X+d[0], p.Y+d[1]) && s.nativePassage(p.X, p.Y, d[0], d[1], false) {
				initialExits[i]++
			}
		}
	}
	eventID := 0
	checkpointed, lateCheckpointed := false, false
	for tick := 0; s.Result == nil && tick < 2000; tick++ {
		now := s.ServerAt + FrameMS
		for seat := 0; seat < players; seat++ {
			if s.Result != nil {
				break
			}
			p := &s.Players[seat]
			if !p.Alive {
				continue
			}
			move, bomb := botDecision(s, p, now)
			if mountPassesBoxes(p.Mount) && bomb {
				report.AirborneBombRequests++
				if !s.canPlaceBomb(p, now) {
					report.InvalidAirborneBombRequests++
					t.Fatalf("airborne bot requested impossible bomb at %d,%d", p.X, p.Y)
				}
			}
			raw := mustJSON(Action{Type: "input", Epoch: s.Epoch, Seq: p.Seq + 1, Move: move, Bomb: bomb})
			ctx.Actor, ctx.Now = seat, time.UnixMilli(now)
			action, err := validAction(ctx, s, raw)
			if err != nil {
				t.Fatalf("input seat%d tick%d: %v", seat, tick, err)
			}
			s.advance(now)
			if s.Result == nil && s.Players[seat].Alive {
				s.applyInput(&s.Players[seat], action, now)
				report.Inputs++
			}
		}
		s.onTimer(now)
		for _, event := range s.Events {
			if event.ID <= eventID {
				continue
			}
			switch event.Type {
			case "place":
				report.Bombs++
			case "push":
				report.Pushes++
			case "item":
				report.Pickups++
				if mountKind(event.Kind) != "" {
					report.MountPickups++
				}
			case "dismount":
				report.Dismounts++
			}
		}
		eventID = s.EventID
		for i, p := range s.Players {
			x, y := float64(p.X)+p.OffsetX, float64(p.Y)+p.OffsetY
			changed := math.Abs(x-lastX[i]) > 1e-8 || math.Abs(y-lastY[i]) > 1e-8
			if changed {
				moved[i] = true
			}
			if p.Alive && mountPassesBoxes(p.Mount) && !changed {
				airIdle[i] += FrameMS
				if airIdle[i] > report.LongestAirborneIdleMS {
					report.LongestAirborneIdleMS = airIdle[i]
				}
			} else {
				airIdle[i] = 0
			}
			lastX[i], lastY[i] = x, y
			if !nativeInBounds(p.X, p.Y) || math.Abs(p.OffsetX) > .500001 || math.Abs(p.OffsetY) > .500001 || !validPlayerAbilities(s.RulesVersion, &p) {
				t.Fatalf("invalid player state seat%d tick%d %+v", i, tick, p)
			}
		}
		if s.Result == nil && (!checkpointed && s.ServerAt >= started+500 || !lateCheckpointed && s.ServerAt >= started+60000) {
			s = nativeFullmatchRestore(t, s, players)
			report.Restores++
			if !checkpointed {
				checkpointed = true
			} else {
				lateCheckpointed = true
			}
		}
	}
	if s.Result == nil || s.Phase != PhaseFinished {
		t.Fatal("natural match failed to finish before allowed deadline")
	}
	if s.StartAt != started || s.MatchEndsAt != deadline {
		t.Fatal("play changed match deadlines")
	}
	if !checkpointed {
		t.Fatal("match finished without live restore coverage")
	}
	// Final production envelope validation also verifies serialized entity limits.
	final, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decode(final); err != nil {
		t.Fatal("final snapshot", err)
	}
	r := s.Result
	if !r.Team || len(r.Ranks) != players || len(r.Scores) != players {
		t.Fatal("incomplete result", r)
	}
	for i, p := range s.Players {
		if r.Scores[i] != p.Eliminations*100+p.Crates*10 {
			t.Fatal("score does not follow recorded actions")
		}
	}
	if r.Draw {
		if len(r.Winners) != 0 {
			t.Fatal("draw has winners")
		}
		for _, rank := range r.Ranks {
			if rank != 1 {
				t.Fatal("draw rank differs")
			}
		}
	} else {
		if len(r.Winners) != players/2 {
			t.Fatal("incomplete winning team")
		}
		winningTeam := -1
		seen := map[int]bool{}
		for _, seat := range r.Winners {
			if seat < 0 || seat >= players || seen[seat] || r.Ranks[seat] != 1 {
				t.Fatal("invalid winner")
			}
			seen[seat] = true
			if winningTeam < 0 {
				winningTeam = s.Players[seat].Team
			}
			if s.Players[seat].Team != winningTeam {
				t.Fatal("mixed winning teams")
			}
		}
		if s.aliveCount(1-winningTeam) != 0 {
			t.Fatal("winning result while opponents alive")
		}
	}
	report.Bombs = s.BombID
	report.ElapsedMS = s.ServerAt - started
	report.Timeout = s.ServerAt >= deadline
	report.Draw, report.Winners = r.Draw, r.Winners
	for i, didMove := range moved {
		if didMove {
			report.MovedSeats++
		} else {
			p := s.Players[i]
			report.UnmovedSeats = append(report.UnmovedSeats, nativeIdleSeat{Seat: i, X: p.X, Y: p.Y, InitialExits: initialExits[i], Alive: p.Alive, Inputs: p.Seq, Crates: p.Crates})
		}
	}
	var details struct {
		Reason string `json:"reason"`
	}
	if err = json.Unmarshal(r.Details, &details); err != nil {
		t.Fatal(err)
	}
	report.Reason = details.Reason
	if report.Timeout && s.aliveCount(0) > 0 && s.aliveCount(1) > 0 && !r.Draw {
		t.Fatal("timeout with survivors invented winner")
	}
	t.Logf("NATIVE_FULLMATCH %s", mustJSON(report))
	return report
}

func TestNativeFullmatchAllClassicMaps(t *testing.T) {
	if testing.Short() {
		t.Skip("full 112-map gameplay matrix; run without -short")
	}
	ids := classicMapIDs()
	if len(ids) != 112 {
		t.Fatal("incomplete map catalog")
	}
	for _, id := range ids {
		for _, players := range []int{2, 4, 6, 8} {
			id, players := id, players
			t.Run(fmt.Sprintf("%s/%d", id, players), func(t *testing.T) {
				runNativeFullmatch(t, id, players, hash64(20020313, fmt.Sprintf("%s/%d", id, players)))
			})
		}
	}
}
