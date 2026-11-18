package pal_test

// Before quest-item autoplay: a visible request is ignored; the selected item
// is wrong/disabled; another owner controls the shared item; saving loses the
// item plan/menu/reading; wine is consumed twice; the NPC fails to leave; old
// input is accepted after continuing. All setup uses ordinary original input.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalFieldBotUsesQuestWineAndSaves(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			m, c, g := originalFacingWineRequest(t, players)
			c.Automated = make([]bool, players)
			for i := range c.Automated {
				c.Automated[i] = true
			}
			last, oldEpoch := uint64(0), uint64(0)
			requestFrame := -1
			opened, confirmed, menuSaved, readSaved, promised, checking := false, false, false, false, false, false
			reclaimed := false
			menuNavigation := 0
			pendingSaved, pendingFrame := false, -1
			pages := []string{}
			for frame := 0; frame < 1024; frame++ {
				v := view(t, m, g, 0)
				if d := v.Dialogue; d != nil && d.ID != last {
					last = d.ID
					pages = append(pages, d.Text)
					t.Log("wine bot dialogue", d.Text)
					if requestFrame < 0 && strings.Contains(d.Text, "醉道士") && strings.Contains(d.Text, "酒") {
						requestFrame = frame
					}
					promised = promised || strings.Contains(d.Text, "山神庙")
					if strings.Contains(d.Text, "你怎么喝光了") && !readSaved {
						c.Actor = 0
						if players > 1 {
							a, err := m.BotAction(c, g)
							if err != nil || len(a) == 0 {
								t.Fatal(err)
							}
							g, err = m.Apply(c, g, a)
							if err != nil {
								t.Fatal(err)
							}
						}
						before := view(t, m, g, 0)
						var err error
						g, err = m.SaveControl(c, g, true)
						if err != nil {
							t.Fatal(err)
						}
						c.Now = c.Now.Add(time.Hour)
						m, _ = nativeModule(t)
						g, err = m.ResumeSaved(c, g)
						if err != nil {
							t.Fatal(err)
						}
						after := view(t, m, g, 0)
						if !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || after.Cash != 500 {
							t.Fatal("saved bot wine reading changed")
						}
						readSaved = true
					}
				}
				if requestFrame >= 0 && frame-requestFrame > 256 && !opened {
					t.Fatal("bot ignored the visible wine request")
				}
				if pendingSaved && frame-pendingFrame > 128 && !confirmed {
					t.Fatal("bot did not recover the cancelled menu input after save")
				}
				if requestFrame >= 0 && !opened && !reclaimed && v.Phase == "field" {
					// Manually step away after the observed request, then hand back
					// the same quest. The bot must recover its approach and facing.
					c.Actor = v.Leader
					c.Automated[c.Actor] = false
					var before struct {
						FieldBot struct {
							Wine json.RawMessage `json:"wine"`
						} `json:"fieldBot"`
					}
					if err := json.Unmarshal(g.Data, &before); err != nil {
						t.Fatal(err)
					}
					var err error
					g, err = m.Reclaim(c, g)
					if err != nil {
						t.Fatal(err)
					}
					own := view(t, m, g, 0)
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: own.Epoch, Seq: own.Seq + 1, Direction: 2, Keys: 16})
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: own.Epoch, Seq: own.Seq + 2, Direction: 4})
					c.Now = time.UnixMilli(g.SystemDeadline + 150)
					g, err = m.OnTimer(c, g)
					if err != nil {
						t.Fatal(err)
					}
					after := view(t, m, g, 0)
					var continued struct {
						FieldBot struct {
							Wine json.RawMessage `json:"wine"`
						} `json:"fieldBot"`
					}
					if err = json.Unmarshal(g.Data, &continued); err != nil || !bytes.Equal(before.FieldBot.Wine, continued.FieldBot.Wine) || after.X != v.X+16 || after.Y != v.Y-8 {
						t.Fatal("manual reclaim changed the quest or failed to move", err, v.X, v.Y, after.X, after.Y)
					}
					c.Automated[c.Actor] = true
					reclaimed = true
					v = after
				}
				if opened && v.Phase == "menu" && v.Menu.Kind == 5 && strings.Contains(originalVisibleText(v, true), "桂花酒") && !menuSaved {
					before := v
					var stored struct {
						FieldBot json.RawMessage `json:"fieldBot"`
					}
					if err := json.Unmarshal(g.Data, &stored); err != nil {
						t.Fatal(err)
					}
					var taskBefore struct {
						FieldBot struct {
							Wine struct {
								Scene, NPC                       int
								Wanted, Checked, Available, Menu bool
							} `json:"wine"`
						} `json:"fieldBot"`
					}
					if err := json.Unmarshal(g.Data, &taskBefore); err != nil {
						t.Fatal(err)
					}
					oldEpoch = v.Epoch
					var err error
					g, err = m.SaveControl(c, g, true)
					if err != nil {
						t.Fatal(err)
					}
					// SaveControl persists a fresh scan after clearing pending input.
					if err = json.Unmarshal(g.Data, &stored); err != nil {
						t.Fatal(err)
					}
					c.Now = c.Now.Add(24 * time.Hour)
					m, _ = nativeModule(t)
					g, err = m.ResumeSaved(c, g)
					if err != nil {
						t.Fatal(err)
					}
					after := view(t, m, g, 0)
					var restored struct {
						FieldBot json.RawMessage `json:"fieldBot"`
					}
					if err = json.Unmarshal(g.Data, &restored); err != nil || !bytes.Equal(stored.FieldBot, restored.FieldBot) || !bytes.Equal(raw(before.Menu), raw(after.Menu)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) || !bytes.Equal(raw(before.Overlay), raw(after.Overlay)) {
						t.Fatal("saved bot item plan or selected wine changed", err)
					}
					var taskAfter struct {
						FieldBot struct {
							Wine struct {
								Scene, NPC                       int
								Wanted, Checked, Available, Menu bool
							} `json:"wine"`
						} `json:"fieldBot"`
					}
					if err = json.Unmarshal(g.Data, &taskAfter); err != nil || !bytes.Equal(raw(taskBefore), raw(taskAfter)) {
						t.Fatal("saved task semantics changed", err)
					}
					menuSaved = true
				}
				if !checking && promised && readSaved && v.Phase == "field" {
					if !confirmed || !menuSaved || !pendingSaved || v.Cash != 500 {
						t.Fatal("original wine quest was bypassed", confirmed, menuSaved, v.Cash)
					}
					for _, npc := range v.Field.Actors {
						if npc[0] == 1 && npc[1] == 63 {
							t.Fatal("wine bot did not complete original NPC departure")
						}
					}
					c.Actor = v.Leader
					before := bytes.Clone(g.Data)
					if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: oldEpoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4, Keys: 2})); err == nil || !bytes.Equal(before, g.Data) {
						t.Fatal("old wine input accepted")
					}
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
					checking = true
				} else if checking {
					if v.Phase == "menu" && v.Menu.Kind == 5 {
						if strings.Contains(originalVisibleText(v, false), "桂花酒") {
							t.Fatal("bot wine was not consumed")
						}
						if menuNavigation == 0 {
							t.Fatal("bot never navigated from the unrelated inventory selection")
						}
						if !strings.Contains(originalVisibleText(v, false), "大蒜") {
							t.Fatal("bot consumed the unrelated item")
						}
						report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original wine plus ordinarily acquired garlic; public-view bot reads request, recovers after manual reclaim/step away, navigates from unrelated item to wine and completes original temple promise/NPC departure; fresh-runtime saves cover an accepted unconsumed menu arrow, selected wine, task and partial shared reading; not full campaign", "players": players, "frames": frame, "pages": pages, "cash": v.Cash, "itemMenuSaved": menuSaved, "readingSaved": readSaved, "wineConsumed": true, "oldInputRejected": true, "manualReclaimRecovered": reclaimed, "itemNavigationCommands": menuNavigation, "pendingMenuInputSaved": pendingSaved, "unrelatedItemPreserved": "大蒜"}, "", "  ")
						if err := os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/field-bot-wine-%d.json", players)), report, 0600); err != nil {
							t.Fatal(err)
						}
						return
					}
				} else {
					for _, actor := range g.Actors {
						c.Actor = actor
						before := bytes.Clone(g.Data)
						a, err := m.BotAction(c, g)
						if err != nil || !bytes.Equal(before, g.Data) {
							t.Fatal("bot mutated source or failed", err)
						}
						if len(a) == 0 {
							continue
						}
						var proposal pal.Action
						if err = json.Unmarshal(a, &proposal); err != nil || proposal.Type != "input" && proposal.Type != "read" {
							t.Fatal("bot bypassed input", string(a), err)
						}
						if proposal.Type == "input" && proposal.Keys == 2048 {
							opened = true
						}
						if proposal.Type == "input" && (proposal.Keys == 32 || proposal.Keys == 8) && v.Phase == "menu" && v.Menu.Kind == 5 {
							menuNavigation++
						}
						if proposal.Type == "input" && proposal.Keys&2 != 0 && v.Phase == "menu" && v.Menu.Kind == 5 && strings.Contains(originalVisibleText(v, true), "桂花酒") {
							confirmed = true
						}
						g, err = m.Apply(c, g, a)
						if err != nil {
							t.Fatal(err)
						}
						if !pendingSaved && proposal.Type == "input" && (proposal.Keys == 8 || proposal.Keys == 32) && v.Phase == "menu" && v.Menu.Kind == 5 {
							before := view(t, m, g, 0)
							g, err = m.SaveControl(c, g, true)
							if err != nil {
								t.Fatal(err)
							}
							c.Now = c.Now.Add(time.Hour)
							m, _ = nativeModule(t)
							g, err = m.ResumeSaved(c, g)
							if err != nil {
								t.Fatal(err)
							}
							after := view(t, m, g, 0)
							if !bytes.Equal(raw(before.Menu), raw(after.Menu)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
								t.Fatal("pending menu save moved the original selection")
							}
							pendingSaved, pendingFrame = true, frame
						}
					}
				}
				c.Now = time.UnixMilli(g.SystemDeadline)
				var err error
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
			}
			v := view(t, m, g, 0)
			t.Fatal("wine bot did not complete original quest", v.Scene, v.X, v.Y, opened, confirmed)
		})
	}
}

func originalFacingWineRequest(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalGuestWine(t, players)
	balcony, downstairs, faced := false, false, false
	lobbyFloor := false
	garlic, kitchenDoor, inside, exitDoor, returned := false, false, false, false, false
	inventory, cursorReady := false, false
	pickupIssued := false
	cooldown := 0
	menuSteps := 0
	for step := 0; step < 256; step++ {
		v := view(t, m, g, 0)
		c.Actor = v.Leader
		if d := v.Dialogue; d != nil {
			garlic = garlic || strings.Contains(d.Text, "得到大蒜")
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" {
			keys := uint32(1)
			if inventory && !strings.Contains(originalVisibleText(v, true), "大蒜") {
				keys = 32
				menuSteps++
				if menuSteps > 4 {
					// Confirm the search's visible result before trusting the
					// setup, then retry ordinary search if it did not land.
					keys, inventory, pickupIssued, menuSteps = 1, false, false, 0
				}
			} else if inventory {
				cursorReady, garlic = true, true
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
			if keys == 1 {
				cooldown = 2
			}
		} else if v.Phase == "field" {
			if cooldown > 0 {
				cooldown--
				c.Now = time.UnixMilli(g.SystemDeadline + 200)
				var err error
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				continue
			}
			if v.Scene == 4 {
				balcony = balcony || (v.X == 1296 && v.Y == 1368)
				downstairs = downstairs || (v.X >= 1248 && v.Y >= 1456)
				returned = returned || garlic
				lobbyFloor = lobbyFloor || (downstairs && v.X <= 1168 && v.Y >= 1584)
			}
			if cursorReady && faced && v.Field.Direction == 0 {
				return m, c, g
			}
			if pickupIssued && !garlic && !inventory {
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
				inventory = true
			} else {
				x, y := 1200, 1128
				if v.Scene == 4 {
					x, y = 1296, 1368
					if balcony && !downstairs {
						x, y = 1168, 1368
					} else if downstairs && !returned {
						x, y = 992, 1472
						if !lobbyFloor {
							x, y = 1136, 1624
						}
					} else if returned {
						x, y = 1024, 1680
					}
				} else if downstairs {
					if !garlic {
						if v.X == 528 && v.Y == 1208 {
							kitchenDoor = true
						}
						if kitchenDoor && v.X >= 544 && v.Y <= 1184 {
							inside = true
						}
						x, y = 496, 1112
						if !kitchenDoor {
							x, y = 528, 1208
						} else if !inside {
							x, y = 560, 1176
						}
					} else {
						if v.X == 560 && v.Y == 1176 {
							exitDoor = true
						}
						x, y = 560, 1176
						if exitDoor {
							x, y = 688, 1288
						}
					}
				}
				direction := nextInnWalk(v, x, y)
				abs := func(n int) int { return max(n, -n) }
				keys := uint32(0)
				if downstairs && v.Scene == 2 && !garlic && inside && abs(v.X-496)+2*abs(v.Y-1112) <= 32 {
					// Search runs after attempted movement. A close vertical
					// offset alone does not put a hidden item in search range.
					same := func(px, py int) bool { return px/32 == 496/32 && py/16 == 1112/16 && (px%32 != 0) == (496%32 != 0) }
					for d, delta := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
						u, w := delta[0], delta[1]
						dx, dy := (u-w)*16, (u+w)*8
						px, py := v.X, v.Y
						if v.Field.Cells[(w+8)*17+u+8]&1 == 0 {
							px, py = px+dx, py+dy
						}
						if same(px, py) || same(px+dx, py+dy) {
							direction, keys, pickupIssued = d, 2, true
							break
						}
					}
				}
				if returned && abs(v.X-1024)+2*abs(v.Y-1680) == 32 {
					direction, faced = 0, true
				}
				if direction < 0 {
					t.Fatal("actual wine checkpoint has no public route", v.Scene, v.X, v.Y, garlic, returned)
				}
				if direction < 4 {
					keys |= []uint32{4, 8, 16, 32}[direction]
				}
				if step%100 == 0 {
					t.Log("wine checkpoint walk", step, v.Scene, v.X, v.Y, x, y, kitchenDoor, inside, keys)
				}
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("ordinary wine-request checkpoint failed", v.Scene, v.X, v.Y, balcony, downstairs, lobbyFloor, garlic, returned, faced, inventory, cursorReady)
	return m, c, g
}

// An empty original inventory must not keep the bot navigating forever or
// repeatedly reopening the same unsuccessful probe. No inventory edits.
func TestOriginalFieldBotMissingWineResumesExploration(t *testing.T) {
	m, c, g := originalKitchenTray(t, 1)
	outside, door, faced := false, false, false
	for step := 0; step < 256; step++ {
		v := view(t, m, g, 0)
		c.Actor = v.Leader
		if v.Dialogue != nil {
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		} else if v.Phase == "field" {
			outside = outside || v.Scene == 4
			if outside && v.X == 1040 && v.Y == 1672 && v.Field.Direction == 0 {
				faced = true
				break
			}
			x, y := 560, 1176
			if !outside && v.X == 560 && v.Y == 1176 {
				door = true
			}
			if !outside && door {
				x, y = 688, 1288
			}
			if outside {
				x, y = 1040, 1672
			}
			d := nextInnWalk(v, x, y)
			if outside && v.X == 1040 && v.Y == 1672 {
				d = 0
			}
			if d < 0 {
				t.Fatal("missing-wine checkpoint has no original route")
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: d, Keys: []uint32{4, 8, 16, 32}[d]})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !faced {
		t.Fatal("failed to prepare real missing-wine checkpoint")
	}
	c.Automated = []bool{true}
	opened, closed, moved := false, false, false
	last := uint64(0)
	navigation, reopened := 0, 0
	closedX, closedY := 0, 0
	for frame := 0; frame < 512; frame++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil && d.ID != last {
			last = d.ID
			t.Log("missing-wine request", d.Text)
		}
		if v.Phase == "menu" && v.Menu.Kind == 5 {
			if !opened {
				t.Log("empty public inventory", originalVisibleText(v, false))
			}
			opened = true
			if strings.Contains(originalVisibleText(v, false), "桂花酒") {
				t.Fatal("empty original checkpoint unexpectedly has wine")
			}
		}
		if opened && v.Phase == "field" && !closed {
			closed = true
			closedX, closedY = v.X, v.Y
			var state struct {
				FieldBot struct {
					Wine struct{ Checked, Available, Menu bool } `json:"wine"`
				} `json:"fieldBot"`
			}
			if err := json.Unmarshal(g.Data, &state); err != nil || !state.FieldBot.Wine.Checked || state.FieldBot.Wine.Available || state.FieldBot.Wine.Menu {
				t.Fatal("missing wine plan did not finish its probe", err)
			}
		}
		if closed && v.Phase == "field" && (v.X != closedX || v.Y != closedY) {
			moved = true
		}
		if moved && frame >= 400 {
			if reopened != 0 || v.Cash != 500 {
				t.Fatal("missing wine reopens or consumes unrelated resources", reopened, v.Cash)
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original inn with no wine, public request and empty inventory, bounded ordinary menu probe and resumed exploration; no resource edits", "menuNavigationCommands": navigation, "reopenedAfterMissing": reopened, "explorationResumed": moved, "cash": v.Cash}, "", "  ")
			if err := os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/field-bot-wine-missing.json"), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		for _, actor := range g.Actors {
			c.Actor = actor
			a, err := m.BotAction(c, g)
			if err != nil {
				t.Fatal(err)
			}
			if len(a) == 0 {
				continue
			}
			var action pal.Action
			if err = json.Unmarshal(a, &action); err != nil {
				t.Fatal(err)
			}
			if action.Keys&(8|32) != 0 && v.Phase == "menu" {
				navigation++
				if navigation > 32 {
					t.Fatal("bot navigates indefinitely in an empty inventory", originalVisibleText(v, false))
				}
			}
			if closed && action.Keys == 2048 {
				reopened++
			}
			g, err = m.Apply(c, g, a)
			if err != nil {
				t.Fatal(err)
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("missing wine blocked original exploration", opened, closed, moved)
}
