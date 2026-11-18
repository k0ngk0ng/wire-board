package pal_test

// Test first through actual GameMain: using the quest item on the wrong seat,
// losing the selected item or partial reading on save, skipping the original
// drunkard quest, consuming wine twice, retaining a vanished NPC, or accepting
// pre-save menu inputs. Walkthrough coordinates are test inputs, never bot data.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Public glyph observations only. Follow the displayed raster root rather than
// unrelated retained backup surfaces; item grids are drawn by original C code.
func originalVisibleText(v pal.View, selectedOnly bool) string {
	var text strings.Builder
	add := func(code, color int) {
		if color != 0 && (!selectedOnly || color >= 249 && color <= 254) {
			text.WriteRune(rune(code))
		}
	}
	if v.Raster == nil {
		for _, op := range v.Overlay {
			if op.Kind == 0 {
				add(op.ID, op.Color)
			}
		}
	} else {
		seen := map[int]bool{}
		var visit func(int)
		visit = func(id int) {
			if id <= 0 || id > len(v.Raster.Nodes) || seen[id] {
				return
			}
			seen[id] = true
			n := v.Raster.Nodes[id-1]
			visit(n[1])
			visit(n[2])
			if n[0] == 2 {
				add(n[5], n[8])
			}
		}
		visit(v.Raster.Root)
	}
	return text.String()
}

func TestOriginalCampaignDrunkardWineUseAndSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) { originalDrunkardWine(t, players) })
	}
}

func originalDrunkardWine(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalGuestWine(t, players)
	balcony, downstairs, faced, opening, used, promised, saved, verifying := false, false, false, false, false, false, false, false
	pages := []string{}
	oldEpoch := uint64(0)
	for step := 0; step < 512; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			pages = append(pages, d.Text)
			t.Log("drunkard story", v.Scene, v.X, v.Y, d.Text)
			promised = promised || strings.Contains(d.Text, "山神庙")
			if strings.Contains(d.Text, "你怎么喝光了") && !saved {
				oldEpoch = v.Epoch
				c.Actor = 0
				if players > 1 {
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
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
				v = view(t, m, g, 0)
				if !reflect.DeepEqual(before.Dialogue, v.Dialogue) || v.Cash != 500 || v.Epoch == before.Epoch {
					t.Fatal("wine-use continuation lost partial original reading or resources")
				}
				d = v.Dialogue
				saved = true
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" {
			c.Actor = v.Leader
			if !opening {
				// Close the verified inventory checkpoint before walking.
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 1})
			} else if verifying && v.Menu.Kind == 5 {
				if strings.Contains(originalVisibleText(v, false), "桂花酒") {
					t.Fatal("original wine still appears after the completed quest")
				}
				if !promised || !saved || v.Cash != 500 {
					t.Fatal("original drunkard quest did not finish", promised, saved, v.Cash)
				}
				before := bytes.Clone(g.Data)
				if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: oldEpoch, Seq: v.Seq + 1, Direction: 4, Keys: 2, ControlAt: v.ControlAt})); err == nil || !bytes.Equal(before, g.Data) {
					t.Fatal("pre-save wine command affected the continued story")
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original GameMain, mapped inventory selection/use, drunkard wine quest and original temple promise, saved reading across a fresh runtime, vanished NPC and consumed wine; not full campaign", "players": c.Players, "pages": pages, "cash": v.Cash, "wineConsumed": true, "drunkardVanished": true, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-drunkard-wine.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			} else if !used && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
				keys := uint32(32)
				if strings.Contains(originalVisibleText(v, true), "桂花酒") {
					wrong := c
					wrong.Actor = 1
					before := bytes.Clone(g.Data)
					if _, err := m.Apply(wrong, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 4, Keys: 2, ControlAt: v.ControlAt})); err == nil || !bytes.Equal(before, g.Data) {
						t.Fatal("nonleader used the shared quest item")
					}
					keys, used = 2, true
				}
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
			}
		} else if v.Phase == "field" {
			c.Actor = v.Leader
			if used && promised && saved {
				for _, actor := range v.Field.Actors {
					if actor[0] == 1 && actor[1] == 63 {
						t.Fatal("original drunkard did not vanish after promising to teach swordplay")
					}
				}
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
				verifying = true
			} else if !used {
				targetX, targetY := 1200, 1128 // Leave the original guest room.
				if v.Scene == 4 {
					if v.X == 1296 && v.Y == 1368 {
						balcony = true
					}
					if v.X >= 1248 && v.Y >= 1456 {
						downstairs = true
					}
					targetX, targetY = 1296, 1368
					if balcony && !downstairs {
						targetX, targetY = 1168, 1368
					} else if downstairs {
						targetX, targetY = 1024, 1680
					}
				}
				abs := func(n int) int { return max(n, -n) }
				near := downstairs && abs(v.X-1024)+2*abs(v.Y-1680) == 32
				if near && faced {
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
					opening = true
				} else {
					direction := nextInnWalk(v, targetX, targetY)
					if near {
						direction, faced = 0, true // Face the original NPC from its northeast.
					}
					if direction < 0 {
						t.Fatal("original drunkard has no visible walking route", v.Scene, v.X, v.Y)
					}
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: []uint32{4, 8, 16, 32}[direction]})
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
					if step%20 == 0 {
						t.Log("drunkard walk", v.Scene, v.X, v.Y, "toward", targetX, targetY)
					}
				}
			}
		}
		var err error
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("original wine use did not complete drunkard quest", used, promised, saved)
	return m, c, g
}
