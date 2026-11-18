package pal_test

// Test first: the harbor rumor fails to change the boatman's dialogue, returning
// to the village misses the original collapse event, partial shared reading is
// lost on save, rewards duplicate, or a later script calls an unsupported import.
// This walkthrough uses only ordinary inputs; coordinates never reach the bot.
import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalCampaignFairyRumorAndAuntCollapseSave(t *testing.T) {
	originalAuntCollapse(t, 1)
}

func originalAuntCollapse(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalVillage(t, players)
	rumor, boatman, collapse := false, false, false
	rumorSaved, collapseSaved := false, false
	pages := []string{}
	last := uint64(0)
	save := func() {
		t.Helper()
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
		if before.Scene != after.Scene || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) {
			t.Fatal("saved rumor/collapse changed original progress or shared reading")
		}
	}
	for step := 0; step < 1536; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != last {
				last = d.ID
				pages = append(pages, d.Text)
				t.Log("fairy rumor story", v.Scene, v.X, v.Y, d.Text)
				rumor = rumor || strings.Contains(d.Text, "美丽的仙女")
				boatman = boatman || rumor && strings.Contains(d.Text, "哪有什么仙女")
				collapse = collapse || strings.Contains(d.Text, "李大娘就突然昏倒")
			}
			if rumor && !rumorSaved || collapse && !collapseSaved {
				c.Actor = 0
				if players > 1 {
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
					partial := view(t, m, g, 0).Dialogue
					if partial == nil || !partial.Read[0] || partial.Read[1] {
						t.Fatal("expected partial shared reading before save")
					}
					for actor, read := range partial.Read {
						if read != (actor == 0) {
							t.Fatal("unexpected reader before rumor save", actor, read)
						}
					}
				}
				save()
				v = view(t, m, g, 0)
				d = v.Dialogue
				if collapse {
					collapseSaved = true
				} else {
					rumorSaved = true
				}
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "field" {
			if collapse && collapseSaved {
				if !rumor || !boatman || !rumorSaved || v.Scene != 5 || v.Map != 1 || v.Cash != 550 || len(v.Party) != 1 {
					t.Fatal("original rumor/collapse chain changed", rumor, boatman, v.Scene, v.Map, v.Cash)
				}
				save()
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original harbor rumor, changed boatman conversation, village return and aunt collapse; saved reading and fresh-runtime continuation; not island travel or full campaign", "players": players, "pages": pages, "cash": v.Cash, "scene": v.Scene, "rumorHeard": rumor, "boatmanDialogueChanged": boatman, "auntCollapseTriggered": collapse, "savedPartialRumorReading": players > 1 && rumorSaved, "savedPartialCollapseReading": players > 1 && collapseSaved}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-fairy-rumor-collapse.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			x, y, npc := 1824, 1328, 0
			if v.Scene == 6 {
				x, y, npc = 1040, 1432, 125
				if rumor {
					x, y, npc = 960, 1376, 124
				}
				if boatman {
					x, y, npc = 192, 1680, 0
				}
			} else if v.Scene != 5 {
				t.Fatal("rumor walkthrough left village/harbor", v.Scene)
			}
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == npc {
					x, y = actor[3], actor[4]
				}
			}
			direction := nextInnWalk(v, x, y)
			keys := uint32(0)
			if npc != 0 {
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
				t.Fatal("rumor walkthrough has no public route", v.Scene, v.X, v.Y, x, y)
			}
			c.Actor = v.Leader
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys | []uint32{4, 8, 16, 32}[direction]})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
			if step%50 == 0 {
				t.Log("rumor walk", step, v.Scene, v.X, v.Y, x, y)
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
	t.Fatal("original rumor/collapse chain did not finish", v.Scene, v.X, v.Y, rumor, boatman, collapse)
	return m, c, g
}
