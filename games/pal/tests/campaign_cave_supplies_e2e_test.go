package pal_test

// Test first using earned original supplies through original menus. Failures:
// a partly injured/low-mana role cannot be selected after party reorder; item
// use invents recovery, changes the wrong role/maxima/cash, consumes twice or
// fails to consume; selected role and actual recovery do not survive a fresh
// runtime; a stale control uses a second dose. No native stats are edited.
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

type caveSuppliesProof struct {
	Passed bool             `json:"passed"`
	Saves  int              `json:"saves"`
	Uses   []map[string]any `json:"uses"`
}

func prepareOriginalCaveSupplies(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, label string) (gameapi.Module, gameapi.Context, gameapi.Match, caveSuppliesProof) {
	t.Helper()
	proof := caveSuppliesProof{}
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"cave-supplies-failure-state.json": g.Data, "cave-supplies-failure-view.json": raw(view(t, m, g, 0))} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), b, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	tick := func() {
		t.Helper()
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	input := func(keys uint32) {
		t.Helper()
		v := view(t, m, g, 0)
		c.Actor = 0
		g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
	}
	wait := func(why string, ready func(pal.View) bool) pal.View {
		t.Helper()
		for i := 0; i < 96; i++ {
			v := view(t, m, g, 0)
			if v.Seq == v.AppliedSeq && ready(v) {
				return v
			}
			tick()
		}
		t.Fatal("original supply menu did not settle", why, originalVisibleText(view(t, m, g, 0), false))
		return pal.View{}
	}
	save := func(name string) {
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
		normalize := func(v pal.View) pal.View {
			v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
			return v
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(view(t, m, g, 0)))) {
			t.Fatal("original supply save changed menu/recovery", name)
		}
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("supply save accepted old confirmation")
		}
		proof.Saves++
		if err := os.WriteFile(campaignEvidencePath("hidden-dragon-supply-"+label+"-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, mana := range []bool{false, true} {
		initial := view(t, m, g, 0)
		target := -1
		stat := func(p pal.PartyMember) (int, int) {
			if mana {
				return p.MP, p.MaxMP
			}
			return p.HP, p.MaxHP
		}
		for i, p := range initial.Party {
			n, max := stat(p)
			if p.HP <= 0 || max <= 0 || n >= max {
				continue
			}
			if target < 0 {
				target = i
			} else {
				old, omax := stat(initial.Party[target])
				if n*omax < old*max {
					target = i
				}
			}
		}
		if target < 0 {
			continue
		}
		item, slug := "金创药", "hp"
		if mana {
			item, slug = "还神丹", "mp"
		}
		role := initial.Party[target]
		before, _ := stat(role)
		input(2048)
		wait("supply inventory", func(v pal.View) bool { return v.Menu.Kind == 5 })
		input(65536)
		tick()
		wait("inventory beginning", func(v pal.View) bool { return v.Menu.Kind == 5 })
		amount := 0
		for scans := 0; ; scans++ {
			v := view(t, m, g, 0)
			if strings.Contains(originalVisibleText(v, true), item) {
				var ok bool
				amount, ok = campaignInventoryAmount(v, item)
				if !ok || amount < 1 {
					t.Fatal("earned supply amount invalid", item, amount)
				}
				break
			}
			if scans > 96 {
				t.Fatal("earned cave supply unavailable", item, originalVisibleText(v, true), originalVisibleText(v, false), v.Menu)
			}
			input(32)
			tick()
			wait("supply cursor", func(v pal.View) bool { return v.Menu.Kind == 5 })
		}
		input(2)
		wait("original item role selector", func(v pal.View) bool { return v.Menu.Kind == 7 })
		for scans := 0; ; scans++ {
			v := view(t, m, g, 0)
			if strings.Contains(originalVisibleText(v, true), role.Name) {
				break
			}
			if scans > 8 {
				t.Fatal("original supply target not selectable", role.Name)
			}
			input(4)
			tick()
			wait("supply role cursor", func(v pal.View) bool { return v.Menu.Kind == 7 })
		}
		save(slug + "-selected")
		input(2)
		v := wait("original recovery effect", func(v pal.View) bool {
			for _, p := range v.Party {
				if p.Role == role.Role {
					n, _ := stat(p)
					return n > before
				}
			}
			return false
		})
		after := 0
		for i, p := range v.Party {
			old := initial.Party[i]
			if p.MaxHP != old.MaxHP || p.MaxMP != old.MaxMP || p.Controller != 0 || p.Role != old.Role {
				t.Fatal("supply changed maxima/party ownership", v.Party)
			}
			if p.Role == role.Role {
				after, _ = stat(p)
				if mana && p.HP != old.HP || !mana && p.MP != old.MP {
					t.Fatal("supply changed the wrong resource", p)
				}
			} else if p.HP != old.HP || p.MP != old.MP {
				t.Fatal("supply affected another original role", p)
			}
		}
		if v.Cash != initial.Cash {
			t.Fatal("original supply spent cash")
		}
		save(slug + "-recovered")
		v = view(t, m, g, 0)
		if v.Menu.Kind == 7 {
			input(1)
			wait("supply inventory return", func(v pal.View) bool { return v.Menu.Kind == 5 })
		}
		v = wait("consumed supply inventory", func(v pal.View) bool { return v.Menu.Kind == 5 })
		left, found := campaignInventoryAmount(v, item)
		if !found {
			left = 0
		}
		if left != amount-1 {
			t.Fatal("original supply not consumed exactly once", item, amount, left)
		}
		input(1)
		wait("supply field return", func(v pal.View) bool { return v.Phase == "field" })
		proof.Uses = append(proof.Uses, map[string]any{"name": item, "role": role.Role, "before": before, "after": after, "amountBefore": amount, "amountAfter": left})
	}
	proof.Passed = true
	b, _ := json.MarshalIndent(proof, "", "  ")
	if err := os.WriteFile(campaignEvidencePath("cave-supplies-"+label+".json", 1), b, 0600); err != nil {
		t.Fatal(err)
	}
	return m, c, g, proof
}

func TestOriginalCaveSuppliesAndPersonalSave(t *testing.T) {
	b, err := os.ReadFile(campaignEvidencePath("hidden-dragon-stair-8-state.json", 1))
	if err != nil {
		t.Fatal("run the original snake battle first", err)
	}
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; view(t, m, g, 0).Phase != "field"; i++ {
		if i > 512 {
			t.Fatal("original snake aftermath did not reach field")
		}
		v = view(t, m, g, 0)
		if v.Dialogue != nil {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _, _, proof := prepareOriginalCaveSupplies(t, m, c, g, "manual-verification")
	if len(proof.Uses) == 0 {
		t.Fatal("original supply verification exercised no actual recovery")
	}
}
