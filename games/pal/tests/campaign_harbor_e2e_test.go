package pal_test

// Test first: the real village/harbor transition fails; merchants cannot be
// searched; shared reading is lost on save; the unavailable shrimp is granted
// or charged anyway; a fresh runtime changes the harbor scene or quest state.
// Coordinates below are walkthrough input, never runtime bot knowledge.
import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalCampaignHarborShrimpAndBoatmanSave(t *testing.T) {
	m, c, g := originalVillage(t, 1)
	pages := []string{}
	last := uint64(0)
	shrimp, boatman, saved := false, false, false
	for step := 0; step < 1024; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != last {
				last = d.ID
				pages = append(pages, d.Text)
				t.Log("harbor story", v.Scene, v.X, v.Y, d.Text)
				shrimp = shrimp || strings.Contains(d.Text, "没得买")
				boatman = boatman || strings.Contains(d.Text, "一条鱼") && strings.Contains(d.Text, "没捞到")
			}
			if shrimp && !saved {
				c.Actor = 0
				g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
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
				d = v.Dialogue
				if !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || v.Cash != 550 {
					t.Fatal("saved shrimp reading or wallet changed")
				}
				saved = true
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "field" {
			if shrimp && boatman && saved {
				if v.Scene != 6 || v.Map != 2 || v.Cash != 550 || len(v.Party) != 1 {
					t.Fatal("original harbor state changed", v.Scene, v.Map, v.Cash)
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
				if before.Scene != after.Scene || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || !bytes.Equal(raw(before.Party), raw(after.Party)) {
					t.Fatal("saved harbor state changed")
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original village-to-harbor market visit, fish seller's unavailable shrimp and boatman's rough-sea account, three-human partial reading/fresh-runtime saves; no shrimp purchase or full campaign acceptance", "players": 3, "pages": pages, "scene": after.Scene, "map": after.Map, "cash": after.Cash, "shrimpUnavailable": true, "boatmanMet": true, "partialReadingSaved": saved}, "", "  ")
				if err = os.WriteFile(campaignEvidencePath("campaign-harbor-shrimp.json", 3), report, 0600); err != nil {
					t.Fatal(err)
				}
				return
			}
			x, y := 1824, 1328
			search := false
			if v.Scene == 6 {
				x, y = 752, 1448
				actorID := 128
				if shrimp {
					x, y, actorID = 960, 1376, 124
				}
				for _, actor := range v.Field.Actors {
					if actor[0] == 1 && actor[1] == actorID {
						x, y = actor[3], actor[4]
					}
				}
				search = true
			} else if v.Scene != 5 {
				t.Fatal("walkthrough left village/harbor", v.Scene)
			}
			direction := nextInnWalk(v, x, y)
			keys := uint32(0)
			if search {
				same := func(px, py int) bool { return px/32 == x/32 && py/16 == y/16 && (px%32 != 0) == (x%32 != 0) }
				for d, delta := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
					u, w := delta[0], delta[1]
					dx, dy := (u-w)*16, (u+w)*8
					px, py := v.X, v.Y
					if v.Field.Cells[(w+8)*17+u+8]&1 == 0 {
						px, py = px+dx, py+dy
					}
					matched := same(px, py)
					for i := 0; i < 4; i++ {
						matched = matched || same(px+dx, py+dy) || same(px, py+dy*2) || same(px+dx*2, py)
						px, py = px+dx, py+dy
					}
					if matched {
						direction, keys = d, 2
						break
					}
				}
			}
			if direction < 0 {
				t.Fatal("harbor walkthrough has no public route", v.Scene, v.X, v.Y, x, y)
			}
			c.Actor = v.Leader
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys | []uint32{4, 8, 16, 32}[direction]})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
			if step%25 == 0 {
				t.Log("harbor walk", step, v.Scene, v.X, v.Y, x, y)
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
	t.Fatal("original harbor visit did not finish", v.Scene, v.X, v.Y, shrimp, boatman)
}
