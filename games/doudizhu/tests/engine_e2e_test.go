package doudizhu_test

// End-to-end module traces authored before the game. HTTP and browser tests
// complement these deterministic actor/view/serialization lifecycle checks.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func view(t *testing.T, m gameapi.Match, p int) map[string]any {
	t.Helper()
	v, e := game.Module().View(m, p)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
func TestNaturalMatchesE2E(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, doubling := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/double=%v", n, doubling), func(t *testing.T) {
				module := game.Module()
				raw := json.RawMessage(fmt.Sprintf(`{"rounds":3,"personalDoubling":%v}`, doubling))
				opts, e := module.Configure(n, raw)
				if e != nil {
					t.Fatal(e)
				}
				ctx := gameapi.Context{Actor: -1, Players: n, Now: time.Unix(1791900000, 0)}
				m, e := module.Start(ctx, opts)
				if e != nil {
					t.Fatal(e)
				}
				trace := []any{}
				steps := 0
				for m.Result == nil && steps < 2400 {
					if e := module.ValidateMatch(m, n); e != nil {
						t.Fatal(e)
					}
					observer := view(t, m, -1)
					if len(observer["hand"].([]any)) != 0 {
						t.Fatal("spectator received a hand")
					}
					if _, ok := observer["hands"]; ok {
						t.Fatal("private state was projected")
					}
					if observer["phase"] == "bid" && len(observer["bottom"].([]any)) != 0 {
						t.Fatal("bottom leaked before auction")
					}
					if len(m.Actors) == 0 {
						t.Fatal("live game has no next actor")
					}
					p := m.Actors[0]
					ctx.Actor = p
					ctx.Version++
					ctx.Now = ctx.Now.Add(250 * time.Millisecond)
					action, e := module.BotAction(ctx, m)
					if e != nil || len(action) == 0 {
						t.Fatalf("bot stalled at step %d: %s %v", steps, m.Data, e)
					}
					old := append([]byte{}, m.Data...)
					next, e := module.Apply(ctx, m, action)
					if e != nil {
						t.Fatalf("bot returned illegal action %s at %s: %v", action, m.Data, e)
					}
					if string(m.Data) != string(old) {
						t.Fatal("Apply mutated input snapshot")
					}
					if steps%17 == 0 {
						b, _ := json.Marshal(next)
						var restored gameapi.Match
						_ = json.Unmarshal(b, &restored)
						if !reflect.DeepEqual(view(t, next, p), view(t, restored, p)) {
							t.Fatal("restart altered public/private view")
						}
						next = restored
					}
					trace = append(trace, map[string]any{"actor": p, "action": json.RawMessage(action), "phase": view(t, next, p)["phase"]})
					m = next
					steps++
				}
				if m.Result == nil {
					t.Fatal("natural match did not finish")
				}
				if e := m.Validate(n); e != nil {
					t.Fatal(e)
				}
				sum := 0
				for _, s := range m.Result.Scores {
					sum += s
				}
				if sum != 0 {
					t.Fatal("points not zero-sum", m.Result.Scores)
				}
				v := view(t, m, 0)
				if len(v["roundResults"].([]any)) != 3 {
					t.Fatal("did not play all three rounds")
				}
				out := filepath.Join("..", "..", "..", ".local", "poker", "doudizhu")
				_ = os.MkdirAll(out, 0755)
				b, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "players": n, "personalDoubling": doubling, "steps": steps, "trace": trace, "result": m.Result}, "", "  ")
				if e := os.WriteFile(filepath.Join(out, fmt.Sprintf("natural-%d-%v.json", n, doubling)), b, 0600); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
func TestOptionsAndPrivateDecisionBoundaryE2E(t *testing.T) {
	module := game.Module()
	for _, raw := range []string{`{"mode":"shanghai4"}`, `{"rounds":0}`, `{"turnSeconds":1}`, `{"hands":[[]]}`, `{"rounds":1} {}`} {
		if _, e := module.Configure(3, json.RawMessage(raw)); e == nil {
			t.Fatal("accepted invalid options", raw)
		}
	}
	for _, n := range []int{2, 5} {
		if _, e := module.Configure(n, json.RawMessage(`{}`)); e == nil {
			t.Fatal("accepted unsupported capacity")
		}
	}
	opts, _ := module.Configure(4, json.RawMessage(`{}`))
	if _, e := module.Start(gameapi.Context{Players: 3, Now: time.Now()}, opts); e == nil {
		t.Fatal("four-player table started with three players")
	}
	ctx := gameapi.Context{Players: 4, Actor: -1, Now: time.Now()}
	m, e := module.Start(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	v := view(t, m, m.Actors[0])
	window := int(v["window"].(float64))
	other := (m.Actors[0] + 1) % 4
	for _, p := range []int{-1, other} {
		bad := ctx
		bad.Actor = p
		if _, e = module.Apply(bad, m, json.RawMessage(fmt.Sprintf(`{"type":"bid","window":%d,"bid":3}`, window))); e == nil {
			t.Fatal("out of turn/spectator bid accepted")
		}
	}
	ctx.Actor = m.Actors[0]
	if _, e = module.Apply(ctx, m, json.RawMessage(fmt.Sprintf(`{"type":"bid","window":%d,"bid":3}`, window-1))); e == nil {
		t.Fatal("stale window accepted")
	}
	m, e = module.Apply(ctx, m, json.RawMessage(fmt.Sprintf(`{"type":"bid","window":%d,"bid":3}`, window)))
	if e != nil {
		t.Fatal(e)
	}
	v = view(t, m, ctx.Actor)
	if len(v["hand"].([]any)) != 33 || len(v["bottom"].([]any)) != 8 {
		t.Fatal("landlord did not take eight public bottom cards")
	}
	hidden := view(t, m, other)
	if len(hidden["hand"].([]any)) != 25 {
		t.Fatal("farmer hand count wrong")
	}
	for _, p := range []int{-1, other} {
		vv := view(t, m, p)
		control := vv["controls"].(map[string]any)
		if len(control["moves"].([]any)) != 0 {
			t.Fatal("another actor's legal moves leaked")
		}
	}
}

// New restore-hardening boundary: inconsistent bidding state must be rejected
// before it can become a negative landlord index or an impossible dealt hand.
func TestRejectsInconsistentSavedAuctionE2E(t *testing.T) {
	module := game.Module()
	opts, _ := module.Configure(3, json.RawMessage(`{}`))
	m, e := module.Start(gameapi.Context{Players: 3, Now: time.Now()}, opts)
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"bid_without_landlord", func(s map[string]any) { s["bid"] = 1 }},
		{"bid_steps_without_bidder", func(s map[string]any) { s["bidSteps"] = 1 }},
		{"uneven_initial_hands", func(s map[string]any) {
			h := s["hands"].([]any)
			a, b := h[0].([]any), h[1].([]any)
			h[0] = append(a, b[0])
			h[1] = b[1:]
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s map[string]any
			_ = json.Unmarshal(m.Data, &s)
			c.mutate(s)
			b, _ := json.Marshal(s)
			bad := m
			bad.Data = b
			if _, e := module.View(bad, 0); e == nil {
				t.Fatal("accepted corrupt saved auction")
			}
		})
	}
}

// Labelled final-hand milestone: personal doubling changes settlement amounts,
// never the single-hand coalition's winners or platform team-rating outcome.
func TestPersonalDoublingKeepsAllFarmersWinnersE2E(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			mod := game.Module()
			ctx := gameapi.Context{Players: n, Actor: 1, Now: time.Unix(1700000000, 0)}
			m, e := mod.Start(ctx, json.RawMessage(`{"rounds":1,"personalDoubling":true}`))
			if e != nil {
				t.Fatal(e)
			}
			var s game.State
			_ = json.Unmarshal(m.Data, &s)
			s.Phase = "play"
			s.Turn = 1
			s.Landlord = 0
			s.Bid = 3
			s.BidSteps = 1
			s.Bids[0] = 3
			s.BidDone[0] = true
			s.PlayCounts[0] = 1
			s.DoubleFactors[1] = 2
			s.Played = []int{}
			for p := 0; p < n; p++ {
				s.Hands[p] = []int{p}
				s.Due[p] = 0
			}
			s.Due[1] = ctx.Now.Add(time.Minute).UnixMilli()
			size := 54
			if n == 4 {
				size = 108
			}
			for id := n; id < size; id++ {
				s.Played = append(s.Played, id)
			}
			m.Data, _ = json.Marshal(s)
			m.Actors = []int{1}
			m.TimeoutActors = []int{1}
			m.Attention = []int{1}
			m.Deadline = s.Due[1]
			if _, e = mod.View(m, 1); e != nil {
				t.Fatal(e)
			}
			a, _ := json.Marshal(game.Action{Type: "play", Window: s.Window, Cards: []int{1}})
			next, e := mod.Apply(ctx, m, a)
			if e != nil {
				t.Fatal(e)
			}
			if next.Result == nil || len(next.Result.Winners) != n-1 || !next.Result.Team {
				t.Fatalf("personal stakes split the winning coalition: %+v", next.Result)
			}
			for p := 1; p < n; p++ {
				if next.Result.Ranks[p] != 1 {
					t.Fatal("winning farmer ranked as loser")
				}
			}
			if next.Result.Scores[1] != 2*next.Result.Scores[2] {
				t.Fatal("personal score multiplier lost")
			}
		})
	}
}
