package pal_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalCampaignLeavesInnBedroom(t *testing.T) {
	m, _ := nativeModule(t)
	c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0)}
	g := originalInn(t, m, &c)
	start := view(t, m, g, 0)
	events := []map[string]any{}
	for step := 0; step < 96; step++ {
		v := view(t, m, g, 0)
		events = append(events, map[string]any{"scene": v.Scene, "map": v.Map, "phase": v.Phase, "x": v.X, "y": v.Y, "dialogue": v.Dialogue})
		t.Log(v.Scene, v.Map, v.Phase, v.X, v.Y)
		if v.Phase == "field" && v.Scene != start.Scene {
			before := v
			var e error
			g, e = m.SaveControl(c, g, true)
			if e != nil {
				t.Fatal(e)
			}
			c.Now = c.Now.Add(time.Hour)
			m, _ = nativeModule(t)
			g, e = m.ResumeSaved(c, g)
			if e != nil {
				t.Fatal(e)
			}
			after := view(t, m, g, 0)
			if after.Scene != before.Scene || after.X != before.X || after.Y != before.Y || after.Cash != before.Cash || after.Epoch == before.Epoch {
				t.Fatal("continued inn exit changed original progress")
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original GameMain, three-human reading and movement from inn bedroom to original next scene, fresh-runtime save continuation; not full campaign", "events": events, "continuedScene": after.Scene}, "", "  ")
			if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/campaign-inn-exit.json"), report, 0600); e != nil {
				t.Fatal(e)
			}
			return
		}
		if v.Dialogue != nil {
			for actor, read := range v.Dialogue.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
				}
			}
		} else if v.Phase == "field" {
			direction := nextInnWalk(v, 1152, 384)
			if direction < 0 {
				t.Fatal("original exit has no visible route", v.X, v.Y)
			}
			c.Actor = v.Leader
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: []uint32{4, 8, 16, 32}[direction]})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
		}
		var e error
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("original bedroom exit did not advance the campaign")
}

func TestOriginalCampaignInnGuestsRewardSurvivesSave(t *testing.T) {
	originalInnGuests(t)
}

func originalInnGuests(t *testing.T, seats ...int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	players := 1
	if len(seats) != 0 {
		players = seats[0]
	}
	m, _ := nativeModule(t)
	c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
	g := originalInn(t, m, &c)
	events := []map[string]any{}
	savedReward, rewardDialogue, pages := false, false, 0
	for step := 0; step < 384; step++ {
		v := view(t, m, g, 0)
		if v.Dialogue != nil {
			events = append(events, map[string]any{"scene": v.Scene, "x": v.X, "y": v.Y, "text": v.Dialogue.Text, "cash": v.Cash})
			t.Log(v.Scene, v.X, v.Y, v.Cash, v.Dialogue.Text)
			pages++
			rewardDialogue = rewardDialogue || strings.Contains(v.Dialogue.Text, "赏银不会少你的")
			if v.Cash == 500 && !savedReward {
				c.Actor = 0
				if players > 1 {
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
				}
				before := view(t, m, g, 0)
				var e error
				g, e = m.SaveControl(c, g, true)
				if e != nil {
					t.Fatal(e)
				}
				c.Now = c.Now.Add(24 * time.Hour)
				m, _ = nativeModule(t)
				g, e = m.ResumeSaved(c, g)
				if e != nil {
					t.Fatal(e)
				}
				v = view(t, m, g, 0)
				if !reflect.DeepEqual(v.Dialogue, before.Dialogue) || v.Cash != 500 || v.Epoch == before.Epoch {
					t.Fatal("reward/partial reading changed after original story resume")
				}
				savedReward = true
			}
			for actor, read := range v.Dialogue.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
				}
			}
		} else if v.Phase == "field" {
			if savedReward {
				if v.Scene != 4 || v.Cash != 500 || !rewardDialogue || pages != 10 || len(v.Party) != 1 {
					t.Fatal("original guest storyline/reward changed", v.Scene, v.Cash, rewardDialogue, pages)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original GameMain, shared inn guest storyline, original 500-cash reward, saved reward reading and fresh-runtime continuation without duplicate reward; not full campaign", "players": players, "pages": pages, "events": events, "cash": v.Cash, "party": v.Party}, "", "  ")
				if e := os.WriteFile(campaignEvidencePath("campaign-inn-guests.json", players), report, 0600); e != nil {
					t.Fatal(e)
				}
				return m, c, g
			}
			targetX, targetY := 1152, 384
			if v.Scene == 4 {
				targetX, targetY = 1264, 1352
			} else if v.Scene != 2 {
				t.Fatal("walkthrough unexpectedly left original inn", v.Scene)
			}
			direction := nextInnWalk(v, targetX, targetY)
			if direction < 0 {
				t.Fatal("original guest checkpoint has no visible route")
			}
			c.Actor = v.Leader
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: []uint32{4, 8, 16, 32}[direction]})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
		}
		var e error
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("original inn guests did not reach their first reward")
	return m, c, g
}

func TestOriginalCampaignKitchenTraySavedContinuation(t *testing.T) {
	originalKitchenTray(t)
}

func originalKitchenTray(t *testing.T, seats ...int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalInnGuests(t, seats...)
	stage := 0
	descended := false
	kitchenDoor, kitchenInside := false, false
	pages := []string{}
	for step := 0; step < 768; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			pages = append(pages, d.Text)
			t.Log("story", stage, v.Scene, v.X, v.Y, d.Text)
			switch {
			case strings.Contains(d.Text, "那个臭要饭的赶走"):
				stage = 1
			case strings.Contains(d.Text, "没酒喝我一步也走不动"):
				stage = 2
			case strings.Contains(d.Text, "端上楼去"):
				stage = 3
			case strings.Contains(d.Text, "赶快端去给楼上的客官"):
				stage = 4
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "field" {
			if v.Scene == 4 && v.X >= 1248 && v.Y >= 1456 {
				descended = true
			}
			if v.Scene == 2 && v.X == 528 && v.Y == 1208 {
				kitchenDoor = true
			}
			if kitchenDoor && v.Scene == 2 && v.X >= 544 && v.Y <= 1184 {
				kitchenInside = true
			}
			if stage == 4 && v.Field != nil && v.Field.Actors[0][2] == 208 {
				if v.Scene != 2 || v.Cash != 500 || len(v.Party) != 1 {
					t.Fatal("original kitchen storyline altered party or reward", v.Scene, v.Cash, v.Party)
				}
				before := v
				var e error
				g, e = m.SaveControl(c, g, true)
				if e != nil {
					t.Fatal(e)
				}
				c.Now = c.Now.Add(24 * time.Hour)
				m, _ = nativeModule(t)
				g, e = m.ResumeSaved(c, g)
				if e != nil {
					t.Fatal(e)
				}
				after := view(t, m, g, 0)
				if after.X != before.X || after.Y != before.Y || after.Scene != before.Scene || after.Cash != 500 || after.Field == nil || after.Field.Actors[0][2] != 208 || !reflect.DeepEqual(after.Party, before.Party) {
					t.Fatal("saved original tray state changed across runtimes")
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original GameMain after inn guest reward, aunt and drunkard interaction, kitchen transition and original tray sprite, shared reading and fresh-runtime save; not full campaign", "players": c.Players, "pages": pages, "scene": after.Scene, "cash": after.Cash, "partySprite": after.Field.Actors[0][2]}, "", "  ")
				if e = os.WriteFile(campaignEvidencePath("campaign-kitchen-tray.json", c.Players), report, 0600); e != nil {
					t.Fatal(e)
				}
				return m, c, g
			}
			targetX, targetY := 1136, 1624
			switch stage {
			case 1:
				targetX, targetY = 1024, 1680
			case 2:
				targetX, targetY = 992, 1472
				if v.Scene == 2 {
					targetX, targetY = 704, 1072
					if !kitchenDoor {
						targetX, targetY = 528, 1208
					} else if !kitchenInside {
						targetX, targetY = 560, 1176
					}
				}
			case 3, 4:
				targetX, targetY = 656, 1144
			}
			if stage == 0 && !descended {
				targetX, targetY = 1168, 1368
			}
			direction := nextInnWalk(v, targetX, targetY)
			keys := uint32(0)
			abs := func(n int) int { return max(n, -n) }
			search := stage == 1 || (stage == 2 && v.Scene == 2) || stage >= 3
			if search && abs(targetX-v.X)+2*abs(targetY-v.Y) <= 32 {
				direction = 3
				if targetX < v.X && targetY >= v.Y {
					direction = 0
				} else if targetX < v.X && targetY < v.Y {
					direction = 1
				} else if targetY < v.Y {
					direction = 2
				}
				keys = 2
			}
			if direction < 0 {
				t.Fatal("walkthrough checkpoint has no visible route", stage, v.Scene, v.X, v.Y)
			}
			c.Actor = v.Leader
			keys |= []uint32{4, 8, 16, 32}[direction]
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
			if step%20 == 0 {
				t.Log("walk", stage, v.Scene, v.X, v.Y, "toward", targetX, targetY)
			}
		}
		var e error
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("original kitchen storyline did not reach tray pickup", stage)
	return m, c, g
}

func TestOriginalCampaignGuestWineInventorySave(t *testing.T) {
	originalGuestWine(t)
}

func originalGuestWine(t *testing.T, seats ...int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalKitchenTray(t, seats...)
	door, outside, upstairs, delivered, explained, menuRequested := false, false, false, false, false, false
	balcony := false
	menuMoves := 0
	pages := []string{}
	for step := 0; step < 512; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			pages = append(pages, d.Text)
			t.Log("wine story", v.Scene, v.X, v.Y, d.Text)
			delivered = delivered || strings.Contains(d.Text, "酒菜送来了")
			explained = explained || strings.Contains(d.Text, "桂花酒")
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" {
			var selected strings.Builder
			for _, op := range v.Overlay {
				if op.Kind == 0 && op.Color >= 249 && op.Color <= 254 {
					selected.WriteRune(rune(op.ID))
				}
			}
			if v.Raster != nil {
				for _, node := range v.Raster.Nodes {
					if node[0] == 2 && node[8] >= 249 && node[8] <= 254 {
						selected.WriteRune(rune(node[5]))
					}
				}
			}
			found := v.Menu.Kind == 5 && strings.Contains(selected.String(), "桂花酒")
			if found {
				if !delivered || !explained || v.Cash != 500 {
					t.Fatal("original wine was not received from the actual guest storyline")
				}
				before := v
				var e error
				g, e = m.SaveControl(c, g, true)
				if e != nil {
					t.Fatal(e)
				}
				c.Now = c.Now.Add(time.Hour)
				m, _ = nativeModule(t)
				g, e = m.ResumeSaved(c, g)
				if e != nil {
					t.Fatal(e)
				}
				after := view(t, m, g, 0)
				if !reflect.DeepEqual(before.Menu, after.Menu) || !reflect.DeepEqual(before.Overlay, after.Overlay) || !reflect.DeepEqual(before.Raster, after.Raster) || before.Cash != after.Cash || !reflect.DeepEqual(before.Party, after.Party) {
					t.Fatal("saved nonempty original inventory changed across runtimes")
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original GameMain, carry tray from kitchen to guest room, original wine dialogue, visible enabled wine selection, shared reading and fresh-runtime continuation of nonempty inventory including exact drawing commands; not full campaign", "players": c.Players, "pages": pages, "scene": after.Scene, "menu": after.Menu, "selectedLabel": "桂花酒", "cash": after.Cash}, "", "  ")
				if e = os.WriteFile(campaignEvidencePath("campaign-guest-wine.json", c.Players), report, 0600); e != nil {
					t.Fatal(e)
				}
				return m, c, g
			}
			if v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
				if menuMoves > 16 {
					t.Fatal("original inventory has no enabled wine selection", selected.String())
				}
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 32})
				menuMoves++
			}
		} else if v.Phase == "field" {
			c.Actor = v.Leader
			if delivered && explained && v.Field != nil && v.Field.Actors[0][2] == 2 {
				if !menuRequested {
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
					menuRequested = true
				}
			} else {
				if v.Scene == 4 {
					outside = true
					if v.X >= 1120 && v.Y <= 1376 {
						upstairs = true
					}
					if v.X == 1296 && v.Y == 1368 {
						balcony = true
					}
				}
				if !outside && v.X == 560 && v.Y == 1176 {
					door = true
				}
				targetX, targetY := 560, 1176
				if !outside && door {
					targetX, targetY = 688, 1288
				} else if v.Scene == 4 && !upstairs {
					targetX, targetY = 1248, 1456
				} else if v.Scene == 4 && !balcony {
					targetX, targetY = 1296, 1368
				} else if v.Scene == 4 {
					targetX, targetY = 1456, 1400
				} else if outside && v.Scene == 2 {
					targetX, targetY = 1264, 1096
				}
				direction := nextInnWalk(v, targetX, targetY)
				if direction < 0 {
					t.Fatal("original serving route has no visible continuation", v.Scene, v.X, v.Y)
				}
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: []uint32{4, 8, 16, 32}[direction]})
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
				if step%20 == 0 {
					t.Log("wine walk", v.Scene, v.X, v.Y, "toward", targetX, targetY)
				}
			}
		}
		var e error
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("original guest storyline did not reach wine inventory")
	return m, c, g
}

func campaignEvidencePath(name string, players int) string {
	if players != 3 {
		name = fmt.Sprintf("%s-%d.json", strings.TrimSuffix(name, ".json"), players)
	}
	dir := filepath.Join("..", "..", "..", ".local/pal/verification")
	if variant := campaignVariant(); variant != "" {
		dir = filepath.Join(dir, "runtime-candidates", variant)
	}
	if profile := developmentHealthProfile(); profile != "" {
		dir = filepath.Join(dir, "development-health", profile)
	}
	if scope := os.Getenv("PAL_EVIDENCE_SCOPE"); scope != "" {
		if len(scope) > 64 || !regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`).MatchString(scope) {
			panic("invalid campaign evidence scope")
		}
		dir = filepath.Join(dir, "scopes", scope)
	}
	return filepath.Join(dir, name)
}

func nextInnWalk(v pal.View, targetX, targetY int) int {
	if v.Field == nil {
		return -1
	}
	type node struct{ u, w, first, steps int }
	queue := []node{{0, 0, -1, 0}}
	seen := map[[2]int]bool{{0, 0}: true}
	best, score := -1, int(^uint(0)>>1)
	r := v.Field.Radius
	abs := func(n int) int { return max(n, -n) }
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		if p.first >= 0 {
			x, y := v.X+(p.u-p.w)*16, v.Y+(p.u+p.w)*8
			distance := (abs(x-targetX) + abs(y-targetY)*2) * 100
			if distance+p.steps < score {
				best, score = p.first, distance+p.steps
			}
		}
		for direction, delta := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
			u, w := p.u+delta[0], p.w+delta[1]
			if abs(u) > r || abs(w) > r || seen[[2]int{u, w}] || v.Field.Cells[(w+r)*(2*r+1)+u+r]&1 != 0 {
				continue
			}
			seen[[2]int{u, w}] = true
			first := p.first
			if first < 0 {
				first = direction
			}
			queue = append(queue, node{u, w, first, p.steps + 1})
		}
	}
	return best
}
