package pal_test

// Test first: returning to the inn misses the sick-aunt scene; doctor/child
// dialogue traps an unsupported call; partial reading or event mutations are
// lost on fresh-runtime resume; stale controls cross a save; the original party
// or wallet changes. Walkthrough positions below never inform runtime bots.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalCampaignDoctorAndIslandMedicineSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			originalIslandMedicineRequest(t, players)
		})
	}
}

func originalIslandMedicineRequest(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalAuntCollapse(t, players)
	doctor, medicine, care := false, false, false
	doctorSaved, medicineSaved := false, false
	pages := []string{}
	last := uint64(0)
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		old := pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 0, Keys: 4}
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			if v.Dialogue == nil || !v.Dialogue.Read[0] || v.Dialogue.Read[1] {
				t.Fatal("medicine save lacks partial shared reading")
			}
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("unexpected reader before medicine save", actor, read)
				}
			}
		}
		before := v
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
			t.Fatal("medicine continuation changed original progress or shared reading")
		}
		c.Actor = after.Leader
		if _, err = m.Apply(c, g, raw(old)); err == nil {
			t.Fatal("medicine continuation accepted pre-save controls")
		}
	}
	for step := 0; step < 1024; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != last {
				last = d.ID
				pages = append(pages, d.Text)
				t.Log("medicine story", v.Scene, v.X, v.Y, d.Text)
				doctor = doctor || strings.Contains(d.Text, "病已经拖很久")
				medicine = medicine || strings.Contains(d.Text, "仙灵岛") && strings.Contains(d.Text, "求药")
				care = care || strings.Contains(d.Text, "麻烦你照顾")
			}
			if doctor && !doctorSaved || medicine && !medicineSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				if medicine {
					medicineSaved = true
				} else {
					doctorSaved = true
				}
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "field" {
			if doctor && medicine && care && doctorSaved && medicineSaved {
				if v.Scene != 3 || v.Map != 12 || v.Cash != 550 || len(v.Party) != 1 || v.Party[0].Role != 0 {
					t.Fatal("original sick-aunt scene or party changed", v.Scene, v.Map, v.Cash, v.Party)
				}
				save(false)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original village return, sick-aunt room, doctor diagnosis and Xiaohu island medicine request; saved reading, fresh-runtime continuation and stale input rejection; not island travel or full campaign", "players": players, "pages": pages, "cash": v.Cash, "scene": v.Scene, "map": v.Map, "doctorHeard": doctor, "islandMedicineRequested": medicine, "auntCareArranged": care, "doctorReadingSaved": doctorSaved, "medicineReadingSaved": medicineSaved, "partialReadingSaved": players > 1, "staleControlsRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-island-medicine.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			x, y, npc := 1296, 872, 0
			switch v.Scene {
			case 5:
			case 4:
				x, y = 1328, 1448
			case 3:
				x, y, npc = 1424, 728, 36
				if doctor {
					x, y, npc = 1472, 736, 37
				}
			default:
				t.Fatal("medicine walkthrough left original route", v.Scene, v.X, v.Y)
			}
			g = campaignWalkTo(t, m, &c, g, v, x, y, npc)
			if step%50 == 0 {
				t.Log("medicine walk", step, v.Scene, v.X, v.Y, x, y)
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
	t.Fatal("original medicine request did not finish", v.Scene, v.X, v.Y, doctor, medicine, care)
	return m, c, g
}

func campaignWalkTo(t *testing.T, m gameapi.Module, c *gameapi.Context, g gameapi.Match, v pal.View, x, y, npc int) gameapi.Match {
	t.Helper()
	v = view(t, m, g, v.Leader)
	for _, actor := range v.Field.Actors {
		if actor[0] == 1 && actor[1] == npc {
			x, y = actor[3], actor[4]
		}
	}
	// Do not leave the intended scene while walking toward a distant waypoint.
	// Only the destination's trigger footprint is part of this walkthrough.
	routed := v
	field := *v.Field
	field.Cells = bytes.Clone(field.Cells)
	for i := 0; i < 289; i++ {
		u, w := i%17-8, i/17-8
		px, py := v.X+(u-w)*16, v.Y+(u+w)*8
		if field.Cells[i]&2 != 0 && fieldTestDistance(px, py, x, y) > 96 {
			field.Cells[i] |= 1
		}
	}
	routed.Field = &field
	direction := nextInnWalk(routed, x, y)
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
		t.Fatal("campaign walkthrough has no public route", v.Scene, v.X, v.Y, x, y)
	}
	c.Actor = v.Leader
	g = apply(t, m, *c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys | []uint32{4, 8, 16, 32}[direction]})
	return apply(t, m, *c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
}
