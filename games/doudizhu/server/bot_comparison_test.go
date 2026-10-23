package doudizhu

// Opt-in paired-deal comparison through production Configure/Start/BotAction/
// Apply/ValidateMatch. Openings and action traces are retained for replay.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestBotPairedDealComparison(t *testing.T) {
	if os.Getenv("POKER_BOT_COMPARISON") != "1" {
		t.Skip("opt-in paired-deal strategy evidence")
	}
	root, _ := os.Getwd()
	for {
		if _, e := os.Stat(filepath.Join(root, "go.mod")); e == nil {
			break
		}
		next := filepath.Dir(root)
		if next == root {
			t.Fatal("repository root missing")
		}
		root = next
	}
	out := filepath.Join(root, ".local/poker-selection-oct23/ai/doudizhu")
	if e := os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	mod := Module()
	opts, e := mod.Configure(4, json.RawMessage(`{"rounds":1}`))
	if e != nil {
		t.Fatal(e)
	}
	initialCtx := gameapi.Context{Players: 4, Now: time.Unix(1792700000, 0)}
	var openings []gameapi.Match
	openingPath := filepath.Join(out, "openings.json")
	if raw, e := os.ReadFile(openingPath); e == nil {
		if e = json.Unmarshal(raw, &openings); e != nil {
			t.Fatal(e)
		}
	} else {
		for i := 0; i < 32; i++ {
			m, e := mod.Start(initialCtx, opts)
			if e != nil {
				t.Fatal(e)
			}
			openings = append(openings, m)
		}
		raw, _ := json.MarshalIndent(openings, "", "  ")
		if e = os.WriteFile(openingPath, raw, 0644); e != nil {
			t.Fatal(e)
		}
	}
	wins, draws, newBombs, oldBombs, midBombs := 0, 0, 0, 0, 0
	durations := []float64{}
	records := []any{}
	for sample, opening := range openings {
		for newTeam := 0; newTeam < 2; newTeam++ {
			m := opening
			ctx := initialCtx
			trace := []any{}
			steps := 0
			for ; m.Result == nil && steps < 1500; steps++ {
				if e = mod.ValidateMatch(m, 4); e != nil {
					t.Fatal(e)
				}
				s, e := decode(m)
				if e != nil || len(m.Actors) == 0 {
					t.Fatal("invalid live state", e)
				}
				ctx.Actor = m.Actors[0]
				ctx.Version++
				ctx.Now = ctx.Now.Add(200 * time.Millisecond)
				team := 0
				if ctx.Actor != s.Landlord {
					team = 1
				}
				start := time.Now()
				var action json.RawMessage
				if team == newTeam {
					action, e = botAction(ctx, m)
					durations = append(durations, float64(time.Since(start).Microseconds())/1000)
				} else {
					action, e = baselineBotAction(ctx, m)
				}
				if e != nil || len(action) == 0 {
					t.Fatal("bot stalled", e)
				}
				var a Action
				if e = json.Unmarshal(action, &a); e != nil {
					t.Fatal(e)
				}
				if a.Type == "play" {
					bomb := false
					c, e := s.classify(a.Cards)
					if e != nil {
						t.Fatal(e)
					}
					bomb = c.Kind == "bomb" || c.Kind == "rocket"
					if bomb {
						if team == newTeam {
							newBombs++
							if len(s.Hands[ctx.Actor])-len(a.Cards) > 5 {
								midBombs++
							}
						} else {
							oldBombs++
						}
					}
				}
				trace = append(trace, map[string]any{"seat": ctx.Actor, "policy": map[bool]string{true: "new", false: "v1.30.1"}[team == newTeam], "action": a})
				m, e = mod.Apply(ctx, m, action)
				if e != nil {
					t.Fatalf("illegal action %s: %v", action, e)
				}
			}
			if m.Result == nil {
				t.Fatal("match did not terminate", sample, newTeam)
			}
			if e = mod.ValidateMatch(m, 4); e != nil {
				t.Fatal(e)
			}
			final, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			winner := -1
			if !m.Result.Draw && len(m.Result.Winners) > 0 {
				winner = 0
				if m.Result.Winners[0] != final.Landlord {
					winner = 1
				}
			}
			if winner < 0 {
				draws++
			} else if winner == newTeam {
				wins++
			}
			records = append(records, map[string]any{"opening": sample, "newTeam": newTeam, "winnerTeam": winner, "steps": steps, "trace": trace})
		}
	}
	sort.Float64s(durations)
	p95 := durations[(len(durations)-1)*95/100]
	report := map[string]any{"passed": true, "games": len(records), "openings": len(openings), "newWins": wins, "draws": draws, "newWinRate": (float64(wins) + float64(draws)/2) / float64(len(records)), "newBombs": newBombs, "oldBombs": oldBombs, "newBombsWithMoreThanFiveLeft": midBombs, "decisionP95MS": p95, "decisionMaxMS": durations[len(durations)-1], "matches": records, "limit": "paired fixed openings, not a general playing-strength guarantee; baseline uses current legal candidates"}
	raw, _ := json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(filepath.Join(out, "comparison.json"), raw, 0644); e != nil {
		t.Fatal(e)
	}
	t.Logf("%d games; new wins %d; draws %d; new bombs %d (midgame %d), old bombs %d; decision p95 %.2fms", len(records), wins, draws, newBombs, midBombs, oldBombs, p95)
}
