package pal_test

// Test first with ordinary original equipment menus. Failure cases: a solo
// player cannot select both original roles; the original inventory lacks its
// earned swords; confirming equips the wrong role or duplicates a weapon;
// the original equipment row/attack display fails to update; resource bindings
// corrupt the equipment background; selected/equipped C menu locals are lost
// on fresh-runtime resume; equipping spends HP, mana/cash or accepts old input.
import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type caveEquipmentProof struct {
	Passed  bool             `json:"passed"`
	Saves   int              `json:"saves"`
	Weapons []map[string]any `json:"weapons"`
}

func caveEquipmentNodes(v pal.View) [][16]int {
	nodes := [][16]int{}
	if v.Raster != nil {
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
			nodes = append(nodes, n)
		}
		visit(v.Raster.Root)
	} else {
		for _, op := range v.Overlay {
			if op.Kind == 0 {
				nodes = append(nodes, [16]int{0: 2, 5: op.ID, 6: op.X, 7: op.Y, 8: op.Color})
			}
			if op.Kind == 1 {
				nodes = append(nodes, [16]int{0: 1, 5: op.Source, 6: op.ID, 7: op.Frame, 8: op.X, 9: op.Y})
			}
		}
	}
	return nodes
}

func caveEquippedWeapon(v pal.View) string {
	letters := map[int]rune{}
	for _, n := range caveEquipmentNodes(v) {
		if n[0] == 2 && n[6] >= 130 && n[6] < 226 && n[7] == 77 && n[8] != 0 {
			letters[n[6]] = rune(n[5])
		}
	}
	keys := []int{}
	for x := range letters {
		keys = append(keys, x)
	}
	sort.Ints(keys)
	var out strings.Builder
	for _, x := range keys {
		out.WriteRune(letters[x])
	}
	return out.String()
}

func caveEquipmentAttack(v pal.View) int {
	digits := map[int]int{}
	for _, n := range caveEquipmentNodes(v) {
		if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] >= 56 && n[7] <= 65 && n[8] >= 260 && n[8] <= 278 && n[9] == 14 {
			digits[n[8]] = n[7] - 56
		}
	}
	if len(digits) == 0 {
		return -1
	}
	xs := []int{}
	for x := range digits {
		xs = append(xs, x)
	}
	sort.Ints(xs)
	n := 0
	for _, x := range xs {
		n = n*10 + digits[x]
	}
	return n
}

type caveWeaponChoice struct {
	name, roleName, id string
	role               int
}

func equipOriginalCaveGear(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match) (gameapi.Module, gameapi.Context, gameapi.Match, caveEquipmentProof) {
	return equipOriginalCaveWeapons(t, m, c, g, []caveWeaponChoice{{"铁剑", "李逍遥", "li-iron", 0}, {"越女剑", "林月如", "yue-sword", 2}})
}

func equipOriginalCaveWeapons(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, choices []caveWeaponChoice) (gameapi.Module, gameapi.Context, gameapi.Match, caveEquipmentProof) {
	t.Helper()
	proof := caveEquipmentProof{}
	initial := view(t, m, g, 0)
	defer func() {
		if t.Failed() {
			if err := os.WriteFile(campaignEvidencePath("cave-equipment-failure-state.json", 1), g.Data, 0600); err != nil {
				t.Log(err)
			}
			if err := os.WriteFile(campaignEvidencePath("cave-equipment-failure-view.json", 1), raw(view(t, m, g, 0)), 0600); err != nil {
				t.Log(err)
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
	wait := func(label string, ready func(pal.View) bool) pal.View {
		t.Helper()
		for i := 0; i < 96; i++ {
			v := view(t, m, g, 0)
			if v.Seq == v.AppliedSeq && ready(v) {
				return v
			}
			tick()
		}
		v := view(t, m, g, 0)
		t.Fatal("original equipment menu did not settle", label, v.Menu, originalVisibleText(v, false))
		return v
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
			t.Fatal("equipment save changed original menu", name)
		}
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("equipment accepted pre-save input")
		}
		proof.Saves++
		if err := os.WriteFile(campaignEvidencePath("cave-equipment-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, weapon := range choices {
		input(4096)
		wait("equipment inventory", func(v pal.View) bool { return v.Phase == "menu" && v.Menu.Kind == 5 })
		input(65536)
		tick()
		wait("equipment inventory beginning", func(v pal.View) bool { return v.Menu.Kind == 5 })
		for scans := 0; ; scans++ {
			v := view(t, m, g, 0)
			if strings.Contains(originalVisibleText(v, true), weapon.name) {
				if n, ok := campaignInventoryAmount(v, weapon.name); !ok || n != 1 {
					t.Fatal("earned sword amount changed", weapon.name, n, ok)
				}
				break
			}
			if scans > 64 {
				t.Fatal("earned sword not selectable", weapon.name, originalVisibleText(v, false))
			}
			input(32)
			tick()
			wait("inventory cursor", func(v pal.View) bool { return v.Phase == "menu" && v.Menu.Kind == 5 })
		}
		input(2)
		v := wait("equipment role selector", func(v pal.View) bool { return v.Phase == "menu" && v.Menu.Kind == 6 })
		for i, p := range v.Party {
			if p.Role == weapon.role {
				for j := 0; j < i; j++ {
					input(4)
					tick()
				}
				break
			}
		}
		v = wait("original selected role", func(v pal.View) bool {
			return v.Menu.Kind == 6 && strings.Contains(originalVisibleText(v, true), weapon.roleName)
		})
		before := caveEquipmentAttack(v)
		if before < 0 {
			t.Fatal("equipment attack value was not visibly drawn")
		}
		save(weapon.id + "-selected")
		input(2)
		v = wait("equipped weapon row", func(v pal.View) bool {
			return v.Menu.Kind == 6 && strings.Contains(caveEquippedWeapon(v), weapon.name) && caveEquipmentAttack(v) != before
		})
		after := caveEquipmentAttack(v)
		save(weapon.id + "-equipped")
		proof.Weapons = append(proof.Weapons, map[string]any{"role": weapon.role, "name": weapon.name, "attackBefore": before, "attackAfter": after, "controller": 0})
		input(1)
		wait("equipment inventory return", func(v pal.View) bool { return v.Menu.Kind == 5 })
		input(1)
		wait("field return", func(v pal.View) bool { return v.Phase == "field" })
	}
	v := view(t, m, g, 0)
	if v.Cash != initial.Cash || !bytes.Equal(raw(v.Party), raw(initial.Party)) || v.Scene != initial.Scene || v.X != initial.X || v.Y != initial.Y {
		t.Fatal("equipping changed original non-equipment resources", initial.Party, v.Party)
	}
	proof.Passed = true
	b, _ := json.MarshalIndent(proof, "", "  ")
	name := "cave-equipment-" + choices[0].id + ".json"
	if initial.Scene == 40 {
		name = "cave-equipment.json"
		if err := os.WriteFile(campaignEvidencePath("hidden-dragon-prepared-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(campaignEvidencePath(name, 1), b, 0600); err != nil {
		t.Fatal(err)
	}
	return m, c, g, proof
}

func TestOriginalCaveEquipmentAndPersonalSave(t *testing.T) {
	b, err := os.ReadFile(campaignEvidencePath("campaign-tournament-state.json", 1))
	if err != nil {
		t.Fatal("run the stage-4 E2E first", err)
	}
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	equipOriginalCaveGear(t, m, c, g)
}
