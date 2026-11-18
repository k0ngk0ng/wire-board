package pal_test

// Test first: the next original quest cannot start, its reward duplicates after
// saved partial reading, old controls cross the inn exit, the next original map
// traps an unsupported native call, or fresh-runtime resume changes the village.
// Every waypoint below is test walkthrough data, never runtime bot knowledge.
import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalCampaignMarketQuestAndVillageSave(t *testing.T) {
	originalVillage(t, 1)
}

func originalVillage(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalDrunkardWine(t, players)
	door, inside, quest, saved, leavingDoor, leavingInside := false, false, false, false, false, false
	pages := []string{}
	for step := 0; step < 512; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			pages = append(pages, d.Text)
			t.Log("market quest", v.Scene, v.X, v.Y, v.Cash, d.Text)
			quest = quest || strings.Contains(d.Text, "菜市场")
			if v.Cash == 550 && !saved {
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
				if v.Cash != 550 || !reflect.DeepEqual(before.Dialogue, v.Dialogue) || before.Epoch == v.Epoch {
					t.Fatal("market quest reward/reading changed across saved continuation")
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
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 1})
		} else if v.Phase == "field" {
			if v.Scene == 5 {
				if !quest || !saved || v.Cash != 550 || v.Map != 1 || len(v.Party) != 1 {
					t.Fatal("original inn exit/market quest state changed", quest, saved, v.Cash, v.Map)
				}
				before := v
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
				if after.Scene != before.Scene || after.Map != before.Map || after.X != before.X || after.Y != before.Y || after.Cash != before.Cash || !reflect.DeepEqual(after.Party, before.Party) {
					t.Fatal("fresh-runtime village resume changed original progress")
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original GameMain through wine quest, aunt market assignment and 50-cash reward, shared saved partial reading, original village transition and fresh-runtime saved continuation; not full campaign", "players": players, "pages": pages, "scene": after.Scene, "map": after.Map, "x": after.X, "y": after.Y, "cash": after.Cash}, "", "  ")
				if err = os.WriteFile(campaignEvidencePath("campaign-market-village.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			targetX, targetY := 992, 1472
			search := false
			if !quest || !saved {
				if v.Scene == 2 {
					if v.X == 528 && v.Y == 1208 {
						door = true
					}
					if door && v.X >= 544 && v.Y <= 1184 {
						inside = true
					}
					targetX, targetY = 528, 1208
					if door && !inside {
						targetX, targetY = 560, 1176
					} else if inside {
						targetX, targetY, search = 704, 1072, true
					}
				}
			} else if v.Scene == 2 {
				if v.X == 560 && v.Y == 1176 {
					leavingInside = true
				}
				if leavingInside && v.Y >= 1200 {
					leavingDoor = true
				}
				targetX, targetY = 560, 1176
				if leavingInside && !leavingDoor {
					targetX, targetY = 528, 1208
				} else if leavingDoor {
					targetX, targetY = 688, 1288
				}
			} else if v.Scene == 4 {
				targetX, targetY = 976, 1704
			} else {
				t.Fatal("walkthrough left original inn unexpectedly", v.Scene)
			}
			direction := nextInnWalk(v, targetX, targetY)
			keys := uint32(0)
			if search && fieldTestDistance(v.X, v.Y, targetX, targetY) <= 32 {
				direction, keys = 2, 2
			}
			if direction < 0 {
				t.Fatal("original market quest has no visible route", v.Scene, v.X, v.Y)
			}
			c.Actor = v.Leader
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys | []uint32{4, 8, 16, 32}[direction]})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
			if step%20 == 0 {
				t.Log("village walk", v.Scene, v.X, v.Y, "toward", targetX, targetY)
			}
		}
		var err error
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("original market quest did not reach village")
	return m, c, g
}

func fieldTestDistance(x, y, tx, ty int) int {
	dx, dy := x-tx, y-ty
	return max(dx, -dx) + 2*max(dy, -dy)
}
