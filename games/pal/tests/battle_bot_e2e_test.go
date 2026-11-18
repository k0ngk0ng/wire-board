package pal_test

// Test before implementation. Original battle bot failure cases: no command
// for one of the supported seat counts; another seat's turn is submitted;
// polling before native consumption floods duplicate edges; takeover confirms
// a human's unfinished item/magic choice; save/reclaim retains queued inputs or
// changes HP, rewards or the current command; mixed seats cannot finish combat.
// Uses the original PAL_StartBattle fixture and only ordinary module actions.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalBattleBotsCompleteWithSavedManualTakeover(t *testing.T) {
	for _, players := range []int{1} {
		for _, mode := range []string{"all-bots", "takeover", "mixed"} {
			mixed := mode == "mixed"
			if mixed && players == 1 {
				continue
			}
			t.Run(fmt.Sprintf("seats-%d/%s", players, mode), func(t *testing.T) {
				m := originalBattleModule(t)
				c := gameapi.Context{Players: players, Now: time.Unix(1800000000, 0), Automated: make([]bool, players)}
				for actor := range c.Automated {
					c.Automated[actor] = !mixed || actor != 0
				}
				g, e := m.Start(c, raw(map[string]any{}))
				if e != nil {
					t.Fatal(e)
				}
				tick := func() {
					t.Helper()
					c.Now = time.UnixMilli(g.SystemDeadline)
					g, e = m.OnTimer(c, g)
					if e != nil {
						t.Fatal(e)
					}
				}
				botCommands := make([]int, players)
				manualCommands, saved, reclaimed := 0, false, false
				manualStep := -1
				for step := 0; step < 1536; step++ {
					v := view(t, m, g, 0)
					if v.Cash == 3 {
						break
					}
					for actor := 0; actor < players; actor++ {
						c.Actor = actor
						before := bytes.Clone(g.Data)
						action, err := m.BotAction(c, g)
						if err != nil || !bytes.Equal(before, g.Data) {
							t.Fatal("bot mutated its source or failed", err)
						}
						if !slices.Contains(g.Actors, actor) {
							if len(action) != 0 {
								t.Fatal("bot commands a teammate or animation", actor, string(action))
							}
							continue
						}
						own := view(t, m, g, actor)
						if own.Seq != own.AppliedSeq {
							if len(action) != 0 {
								t.Fatal("bot runs ahead of its pending input")
							}
							continue
						}
						if !c.Automated[actor] {
							// A human still uses the ordinary original attack menu.
							action = raw(pal.Action{Type: "input", Epoch: own.Epoch, ControlAt: own.ControlAt, Seq: own.Seq + 1, Direction: 4, Keys: 2})
							manualCommands++
						} else if len(action) == 0 {
							t.Fatal("bot has no ordinary battle command", actor, v.Battle)
						} else {
							botCommands[actor]++
						}
						var proposed pal.Action
						if err = json.Unmarshal(action, &proposed); err != nil || proposed.Type != "input" || proposed.Seq != view(t, m, g, actor).Seq+1 {
							t.Fatal("bot bypasses ordinary input", string(action), err)
						}
						g, err = m.Apply(c, g, action)
						if err != nil {
							t.Fatal("bot action rejected", string(action), err)
						}
						if duplicate, err := m.BotAction(c, g); err != nil || len(duplicate) != 0 {
							t.Fatal("bot queued another edge before native consumption", string(duplicate), err)
						}
						if !saved {
							before := view(t, m, g, actor)
							g, err = m.SaveControl(c, g, true)
							if err != nil {
								t.Fatal(err)
							}
							if paused, err := m.BotAction(c, g); err != nil || len(paused) != 0 {
								t.Fatal("paused bot advances the battle", err)
							}
							c.Now = c.Now.Add(time.Hour)
							// Restore into a fresh runtime, as after a server restart.
							m = originalBattleModule(t)
							g, err = m.ResumeSaved(c, g)
							if err != nil {
								t.Fatal(err)
							}
							after := view(t, m, g, actor)
							if !reflect.DeepEqual(before.Battle, after.Battle) || before.Cash != after.Cash || before.Epoch == after.Epoch || after.Seq != 0 {
								t.Fatal("save changed combat or retained a queued bot command")
							}
							saved = true
						} else if mode == "takeover" && !reclaimed && c.Automated[actor] {
							before := view(t, m, g, actor)
							g, err = m.Reclaim(c, g)
							if err != nil {
								t.Fatal(err)
							}
							after := view(t, m, g, actor)
							if !reflect.DeepEqual(before.Battle, after.Battle) || before.Epoch == after.Epoch || after.Seq != 0 {
								t.Fatal("manual reclaim changes the pending original command")
							}
							c.Automated[actor] = false
							keys := uint32(2)
							if after.Battle.State == 1 {
								keys = 1024 // Human chooses defense at the original root.
							}
							g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: after.Epoch, ControlAt: after.ControlAt, Seq: 1, Direction: 4, Keys: keys})
							manualCommands++
							manualStep = step
							reclaimed = true
						}
					}
					tick()
					if manualStep == step {
						for actor := range c.Automated {
							c.Automated[actor] = !mixed || actor != 0
						}
					}
					if e = m.ValidateMatch(g, players); e != nil {
						t.Fatal(e)
					}
				}
				v := view(t, m, g, 0)
				if v.Cash != 3 || !saved || (mode == "takeover" && (!reclaimed || manualStep < 0 || manualCommands == 0)) || (mode == "mixed" && manualCommands == 0) || (mode == "all-bots" && manualCommands != 0) {
					t.Fatal("mixed/saved bot battle did not finish with original reward", v.Cash, botCommands, manualCommands, saved, reclaimed)
				}
				for actor, commands := range botCommands {
					if (!mixed || actor != 0) && commands == 0 {
						t.Fatal("supported bot seat never played", actor)
					}
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "isolated original battle, ordinary bot and human actions, queued-input backpressure, fresh-runtime saved continuation; not full campaign AI", "players": players, "mode": mode, "botCommands": botCommands, "manualCommands": manualCommands, "manualReclaim": reclaimed, "cash": v.Cash}, "", "  ")
				if e = os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/battle-bots-%d-%s.json", players, mode)), report, 0600); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestOriginalBattleBotRecoversUnfinishedHumanMenu(t *testing.T) {
	m := originalBattleModule(t)
	c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0)}
	g, e := m.Start(c, raw(map[string]any{}))
	if e != nil {
		t.Fatal(e)
	}
	tick := func() {
		t.Helper()
		c.Now = time.UnixMilli(g.SystemDeadline)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
	}
	until := func(predicate func(pal.View) bool) {
		t.Helper()
		for step := 0; step < 512; step++ {
			if predicate(view(t, m, g, 0)) {
				return
			}
			tick()
		}
		t.Fatal("original takeover checkpoint unavailable")
	}
	until(func(v pal.View) bool { return v.Battle != nil && v.Battle.State == 1 && v.Battle.Current == 0 })
	v := view(t, m, g, 0)
	g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 0})
	until(func(v pal.View) bool { return v.Battle.Selected == 3 })
	v = view(t, m, g, 0)
	g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2})
	until(func(v pal.View) bool { return v.Battle.Menu == 4 })
	// The platform explicitly marks the returning seat as automated. Keep
	// that production contract when recording accepted bot menu inputs.
	c.Automated = []bool{true}
	cancelled, selectedHealing := false, false
	healingObject := -1
	for step := 0; step < 128; step++ {
		v = view(t, m, g, 0)
		if v.Battle.Current == 1 {
			p := v.Battle.Players[0]
			if !cancelled || !(p.Action == 2 || p.Action == 3 && healingObject > 0 && p.Object == healingObject) {
				t.Fatal("takeover committed unfinished human misc choice", v.Battle.Players[0])
			}
			return
		}
		bot, err := m.BotAction(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if len(bot) != 0 {
			var a pal.Action
			if err := json.Unmarshal(bot, &a); err != nil {
				t.Fatal(err)
			}
			if v.Battle.Menu == 4 && a.Keys == 1 {
				cancelled = true
			}
			// The original fixture has a wounded ally. A newly selected,
			// visible 气疗术 is a legal takeover decision after cancellation.
			if cancelled && v.Battle.Menu == 1 && strings.Contains(originalVisibleText(v, true), "气疗术") && a.Keys == 2 {
				selectedHealing = true
			}
			if selectedHealing && v.Battle.State == 3 && v.Battle.Action == 3 && a.Keys == 2 {
				healingObject = v.Battle.Object
			}
			g, e = m.Apply(c, g, bot)
			if e != nil {
				t.Fatal(e)
			}
		}
		tick()
	}
	t.Fatal("bot could not recover unfinished human submenu", v.Battle, v.Party)
}
