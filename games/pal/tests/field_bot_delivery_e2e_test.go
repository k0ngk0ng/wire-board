package pal_test

// Before extending autoplay: a held tray never reaches the actual guest room,
// the bot searches static scenery forever, original wine is not received,
// pending movement or shared wine dialogue is lost on save, or a menu probe
// finds no usable wine. Checkpoints use ordinary original gameplay only.
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
)

func TestOriginalFieldBotDeliversTrayAndReceivesWine(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			m, c, g := originalKitchenTray(t, players)
			c.Automated = make([]bool, players)
			for i := range c.Automated {
				c.Automated[i] = true
			}
			pages := []string{}
			last := uint64(0)
			moves, reads := 0, 0
			delivered, wine, movingSaved, readingSaved, inventory := false, false, false, false, false
			menuMoves := 0
			for frame := 0; frame < 4096; frame++ {
				v := view(t, m, g, 0)
				if d := v.Dialogue; d != nil && d.ID != last {
					last = d.ID
					pages = append(pages, d.Text)
					t.Log("delivery bot dialogue", v.Scene, v.X, v.Y, d.Text)
					delivered = delivered || strings.Contains(d.Text, "酒菜送来了")
					wine = wine || strings.Contains(d.Text, "桂花酒")
					if wine && !readingSaved {
						c.Actor = 0
						if players > 1 {
							first, err := m.BotAction(c, g)
							if err != nil || len(first) == 0 {
								t.Fatal("missing cooperative wine confirmation", err)
							}
							g, err = m.Apply(c, g, first)
							if err != nil {
								t.Fatal(err)
							}
							reads++
						}
						before := view(t, m, g, 0)
						var err error
						g, err = m.SaveControl(c, g, true)
						if err != nil {
							t.Fatal(err)
						}
						c.Now = c.Now.Add(24 * time.Hour)
						m, _ = nativeModule(t)
						g, err = m.ResumeSaved(c, g)
						if err != nil {
							t.Fatal(err)
						}
						after := view(t, m, g, 0)
						if !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || before.Cash != after.Cash {
							t.Fatal("saved wine dialogue changed")
						}
						readingSaved = true
					}
				}
				if !inventory && delivered && wine && v.Field != nil && v.Field.Actors[0][2] == 2 {
					if !movingSaved || !readingSaved || v.Cash != 500 {
						t.Fatal("bot bypassed original delivery or save", movingSaved, readingSaved, v.Cash)
					}
					c.Actor = v.Leader
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
					inventory = true
				} else if inventory {
					// The final assertion inspects the actual original inventory by
					// ordinary inputs, rather than trusting dialogue or hidden state.
					if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
						if strings.Contains(originalVisibleText(v, true), "桂花酒") {
							report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original kitchen tray checkpoint; public-view autoplay delivers to original guests, receives wine, saves movement and partial shared reading, and verifies usable wine via ordinary original inventory inputs; not full campaign", "players": players, "frames": frame, "fieldCommands": moves, "readingCommands": reads, "pages": pages, "cash": v.Cash, "usableWine": "桂花酒", "movementSaved": movingSaved, "wineReadingSaved": readingSaved}, "", "  ")
							if err := os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/field-bot-delivery-%d.json", players)), report, 0600); err != nil {
								t.Fatal(err)
							}
							return
						}
						if menuMoves >= 16 {
							t.Fatal("bot received no usable original wine", originalVisibleText(v, false))
						}
						c.Actor = v.Leader
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 32})
						menuMoves++
					}
				} else {
					for _, actor := range g.Actors {
						c.Actor = actor
						before := bytes.Clone(g.Data)
						action, err := m.BotAction(c, g)
						if err != nil || !bytes.Equal(before, g.Data) {
							t.Fatal("bot mutated source or failed", err)
						}
						if len(action) == 0 {
							continue
						}
						var proposed pal.Action
						if err = json.Unmarshal(action, &proposed); err != nil || proposed.Type != "input" && proposed.Type != "read" {
							t.Fatal("bot bypassed ordinary actions", string(action), err)
						}
						if proposed.Type == "input" {
							moves++
						} else {
							reads++
						}
						g, err = m.Apply(c, g, action)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				if !movingSaved && moves >= 12 && v.Scene == 4 {
					position := view(t, m, g, 0)
					before := bytes.Clone(g.Data)
					var saved struct {
						FieldBot json.RawMessage `json:"fieldBot"`
					}
					if err := json.Unmarshal(before, &saved); err != nil {
						t.Fatal(err)
					}
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
					var continued struct {
						FieldBot json.RawMessage `json:"fieldBot"`
					}
					if err = json.Unmarshal(g.Data, &continued); err != nil || !bytes.Equal(saved.FieldBot, continued.FieldBot) {
						t.Fatal("delivery exploration history lost", err)
					}
					after := view(t, m, g, 0)
					if position.Scene != after.Scene || position.Map != after.Map || position.X != after.X || position.Y != after.Y || position.Cash != after.Cash || !bytes.Equal(raw(position.Party), raw(after.Party)) {
						t.Fatal("saved delivery position or party changed")
					}
					movingSaved = true
				}
				c.Now = time.UnixMilli(g.SystemDeadline)
				var err error
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				if frame%100 == 0 {
					v = view(t, m, g, 0)
					t.Log("delivery bot position", frame, v.Scene, v.X, v.Y, v.Phase)
				}
			}
			if err := os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/delivery-bot-failure-state.json"), g.Data, 0600); err != nil {
				t.Fatal(err)
			}
			v := view(t, m, g, 0)
			t.Fatal("original delivery bot did not receive usable wine", v.Scene, v.X, v.Y, delivered, wine)
		})
	}
}
