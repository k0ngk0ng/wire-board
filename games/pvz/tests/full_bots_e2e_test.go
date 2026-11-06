package pvz_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func fullBotRecord(t *testing.T, label string, value any) {
	t.Helper()
	out := filepath.Join("..", "..", "..", ".local/pvz/full-bots")
	if e := os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": label, "result": value}, "", "  ")
	name := strings.ReplaceAll(t.Name(), "/", "-") + ".json"
	if e := os.WriteFile(filepath.Join(out, name), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func fullBotRuntime(t *testing.T, m gameapi.Match) engine.State {
	t.Helper()
	var v struct {
		Runtime engine.State `json:"runtime"`
	}
	if e := json.Unmarshal(m.Data, &v); e != nil {
		t.Fatal(e)
	}
	return v.Runtime
}
func TestPVZFullBotsNaturalModule(t *testing.T) {
	for _, scene := range []string{"day", "night", "pool", "fog", "roof", "roofnight"} {
		for _, n := range []int{2, 4} {
			t.Run(fmt.Sprintf("%s-%d", scene, n), func(t *testing.T) {
				mod := pvz.Module()
				mode := "duel"
				if n == 4 {
					mode = "team"
				}
				raw, _ := json.Marshal(map[string]any{"scene": scene, "mode": mode, "roundSeconds": 180, "prepSeconds": 10})
				now := time.Unix(1800000000, 0)
				auto := make([]bool, n)
				for i := range auto {
					auto[i] = true
				}
				ctx := gameapi.Context{Players: n, Actor: -1, Now: now, Version: 1, RequestedVersion: 1, Automated: auto}
				match, e := mod.Start(ctx, raw)
				if e != nil {
					t.Fatal(e)
				}
				trace := []any{}
				seen := map[int]bool{}
				actions := make([]int, n)
				rows := map[int]bool{}
				manualSeq := int64(-1)
				lastTick := int64(0)
				for frame := 0; frame < 8100 && match.Result == nil; frame++ {
					s := fullBotRuntime(t, match)
					seen[s.Round] = true
					if s.Tick < lastTick {
						t.Fatal("clock moved backwards")
					}
					lastTick = s.Tick
					if frame == 450 {
						before := engine.Digest(s)
						now = now.Add(time.Hour)
						ctx.Now = now
						match, e = mod.Restore(ctx, match)
						if e != nil {
							t.Fatal(e)
						}
						if engine.Digest(fullBotRuntime(t, match)) != before {
							t.Fatal("restore caught up offline or changed rules")
						}
					}
					if frame == 600 {
						before := engine.Digest(fullBotRuntime(t, match))
						ctx.Actor = 0
						match, e = mod.Reclaim(ctx, match)
						if e != nil {
							t.Fatal(e)
						}
						if engine.Digest(fullBotRuntime(t, match)) != before {
							t.Fatal("reclaim changed resources/clock")
						}
						auto[0] = false
						manualSeq = fullBotRuntime(t, match).Players[0].Seq
					}
					if frame == 650 {
						if fullBotRuntime(t, match).Players[0].Seq != manualSeq {
							t.Fatal("idle manual seat was automatically controlled")
						}
						auto[0] = true
					}
					for actor := range n {
						if !auto[actor] {
							continue
						}
						ctx.Actor = actor
						ctx.Now = now
						before := append([]byte(nil), match.Data...)
						a, err := mod.BotAction(ctx, match)
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(before, []byte(match.Data)) {
							t.Fatal("proposal mutated match")
						}
						if len(a) == 0 {
							continue
						}
						next, err := mod.Apply(ctx, match, a)
						if err != nil {
							t.Fatalf("scene=%s actor=%d illegal bot action %s: %v", scene, actor, a, err)
						}
						match = next
						actions[actor]++
						var v engine.Action
						_ = json.Unmarshal(a, &v)
						if v.Type == "plant" || v.Type == "spawn" {
							rows[v.Row] = true
						}
						trace = append(trace, map[string]any{"frame": frame, "actor": actor, "action": v})
					}
					ctx.Actor = -1
					now = now.Add(50 * time.Millisecond)
					ctx.Now = now
					match, e = mod.OnTimer(ctx, match)
					if e != nil {
						t.Fatalf("scene=%s frame=%d timer: %v", scene, frame, e)
					}
				}
				final := fullBotRuntime(t, match)
				if match.Result == nil || len(seen) != 2 {
					t.Fatalf("two-round natural match did not finish: phase=%s tick=%d rounds=%v", final.Phase, final.Tick, seen)
				}
				if e = match.Validate(n); e != nil {
					t.Fatal(e)
				}
				for i, count := range actions {
					if count < 2 {
						t.Fatalf("seat%d never participated: %v", i, actions)
					}
				}
				fullBotRecord(t, "production Module; natural resources, actors, legal actions and 50ms clock; test-only restore wall-clock jump and explicit manual interval", map[string]any{"scene": scene, "players": n, "actions": actions, "rowsUsed": rows, "trace": trace, "final": final, "outcome": match.Result})
			})
		}
	}
}

func fullBotFixture(t *testing.T, scene string) engine.State {
	t.Helper()
	s, e := engine.New(engine.Options{Scene: scene, Mode: "duel", RoundSeconds: 600, PrepSeconds: 10}, 2, "full-bot-fixture-"+t.Name())
	if e != nil {
		t.Fatal(e)
	}
	if e = engine.Step(&s, 200); e != nil {
		t.Fatal(e)
	}
	s.Players[0].Money = 100000
	s.Players[1].Money = 100000
	return s
}
func fullBotFixtureAct(t *testing.T, s *engine.State, actor int, a engine.Action) {
	t.Helper()
	if a.Kind != "" && !containsBotCard(s.Players[actor].Deck, a.Kind) {
		s.Players[actor].Deck = append(s.Players[actor].Deck, a.Kind)
	}
	a.Epoch = s.Epoch
	a.Seq = s.Players[actor].Seq + 1
	s.Players[actor].Cooldowns[a.Kind] = 0
	if e := engine.Apply(s, actor, a); e != nil {
		t.Fatalf("fixture %+v: %v", a, e)
	}
}
func containsBotCard(deck []string, kind string) bool {
	for _, s := range deck {
		if s == kind {
			return true
		}
	}
	return false
}
func fullBotProposal(t *testing.T, s *engine.State, actor int) *engine.Action {
	t.Helper()
	before := engine.Digest(*s)
	a := engine.BotAction(s, actor)
	if engine.Digest(*s) != before {
		t.Fatal("bot mutated its source")
	}
	again := engine.BotAction(s, actor)
	if !reflect.DeepEqual(a, again) {
		t.Fatal("same visible state produced different actions")
	}
	if a == nil {
		t.Fatal("legal targeted card had no strategy")
	}
	copy := engine.Clone(*s)
	if e := engine.Apply(&copy, actor, *a); e != nil {
		t.Fatalf("proposal is illegal %+v: %v", a, e)
	}
	return a
}
func TestPVZFullBotsEveryPlantCard(t *testing.T) {
	for _, spec := range engine.GetCatalog().Plants {
		t.Run(spec.Kind, func(t *testing.T) {
			scene := "day"
			row := 1
			if spec.Mushroom || spec.Kind == "gravebuster" {
				scene = "night"
			}
			if spec.WaterOnly {
				scene = "pool"
				row = 2
				if spec.Mushroom {
					scene = "fog"
				}
			}
			s := fullBotFixture(t, scene)
			s.Players[0].Deck = []string{}
			if spec.UpgradeFrom != "" {
				if spec.Kind == "cobcannon" {
					fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "kernelpult", Row: row, Col: 2})
					fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "kernelpult", Row: row, Col: 3})
				} else {
					fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: spec.UpgradeFrom, Row: row, Col: 2})
				}
			}
			if spec.Kind == "coffeebean" {
				fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "puffshroom", Row: row, Col: 2})
			}
			s.Players[0].Deck = []string{spec.Kind}
			s.Players[0].ImitaterKind = "peashooter"
			enemyKind, enemyRow := "zombie", 1
			if spec.Kind == "tanglekelp" {
				enemyKind, enemyRow = "snorkel", row
			}
			if spec.Kind == "blover" {
				enemyKind = "balloon"
			}
			fullBotFixtureAct(t, &s, 1, engine.Action{Type: "spawn", Kind: enemyKind, Row: enemyRow})
			s.Zombies[len(s.Zombies)-1].X = 4500
			a := fullBotProposal(t, &s, 0)
			if a.Type != "plant" || a.Kind != spec.Kind {
				t.Fatalf("single-card policy ignored %s: %+v", spec.Kind, a)
			}
			fullBotRecord(t, "controlled single-card strategy fixture; sufficient money and legitimate upgrade bases, not a natural economy game", map[string]any{"kind": spec.Kind, "scene": scene, "proposal": a})
		})
	}
}
func TestPVZFullBotsEveryZombieCard(t *testing.T) {
	for _, spec := range engine.GetCatalog().Zombies {
		t.Run(spec.Kind, func(t *testing.T) {
			scene := "day"
			if spec.WaterOnly {
				scene = "pool"
			}
			if spec.Kind == "zomboss" {
				scene = "roofnight"
			}
			s := fullBotFixture(t, scene)
			s.Players[1].Deck = []string{spec.Kind}
			if spec.RequiresIce {
				s.NextID++
				s.Terrain = append(s.Terrain, engine.Terrain{ID: s.NextID, Kind: "ice", Owner: 1, Row: 1, FromX: 0, ToX: 10000, Until: s.Tick + 1000})
			}
			if spec.Target == "cell" {
				s.Players[0].Deck = []string{"sunflower"}
				fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "sunflower", Row: 1, Col: 2})
			}
			a := fullBotProposal(t, &s, 1)
			if a.Type != "spawn" || a.Kind != spec.Kind {
				t.Fatalf("single-card policy ignored %s: %+v", spec.Kind, a)
			}
			fullBotRecord(t, "controlled single-card strategy fixture; sufficient money and required scene/ice/target, not a natural economy game", map[string]any{"kind": spec.Kind, "scene": scene, "proposal": a})
		})
	}
}
func TestPVZFullBotsCannonAndFogInformation(t *testing.T) {
	t.Run("cannon", func(t *testing.T) {
		s := fullBotFixture(t, "day")
		s.Players[0].Deck = []string{"kernelpult", "cobcannon"}
		fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "kernelpult", Row: 1, Col: 1})
		fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "kernelpult", Row: 1, Col: 2})
		fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "cobcannon", Row: 1, Col: 1})
		for i := range s.Plants {
			if s.Plants[i].Kind == "cobcannon" {
				s.Plants[i].ReadyAt = s.Tick
				s.Plants[i].ChargeMS = s.Tick * engine.TickMS
			}
		}
		s.Players[0].Deck = []string{"cobcannon"}
		fullBotFixtureAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 1})
		s.Zombies[len(s.Zombies)-1].X = 6000
		a := fullBotProposal(t, &s, 0)
		if a.Type != "ability" {
			t.Fatal("charged cannon never fired", a)
		}
		fullBotRecord(t, "controlled charged-cannon fixture", a)
	})
	t.Run("fog", func(t *testing.T) {
		s := fullBotFixture(t, "fog")
		s.Players[0].Deck = []string{"puffshroom", "sunshroom", "plantern"}
		s.Players[1].Deck = []string{"zombie"}
		fullBotFixtureAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0})
		a := engine.ProjectState(s, 0)
		s.Zombies[0].Row = 4
		b := engine.ProjectState(s, 0)
		if len(a.Zombies) != 0 || len(b.Zombies) != 0 {
			t.Fatal("fixture hidden unit leaked")
		}
		pa, pb := engine.BotAction(&a, 0), engine.BotAction(&b, 0)
		if !reflect.DeepEqual(pa, pb) {
			t.Fatal("hidden position changed bot decision", pa, pb)
		}
		fullBotRecord(t, "two viewer-equivalent projected states; hidden zombie changed rows", map[string]any{"a": pa, "b": pb})
	})
}
func TestPVZFullBotsSourceArtifact(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "engine", "bot.go"))
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	fullBotRecord(t, "source identity for replay results", map[string]any{"botSHA256": hex.EncodeToString(sum[:]), "command": "go test -json ./games/pvz/tests -run '^TestPVZFullBots' -count=1"})
}

func TestPVZFullBotsDoNotBuildImpossibleSupportChain(t *testing.T) {
	s := fullBotFixture(t, "roof")
	s.Players[0].Deck = []string{"spikeweed", "flowerpot"}
	before := engine.Digest(s)
	if a := engine.BotAction(&s, 0); a != nil {
		t.Fatalf("roof spikeweed cannot become legal after a pot: %+v", a)
	}
	if engine.Digest(s) != before {
		t.Fatal("idle policy mutated state")
	}
	fullBotRecord(t, "controlled impossible terrain/prerequisite chain", map[string]any{"digest": before})
}
