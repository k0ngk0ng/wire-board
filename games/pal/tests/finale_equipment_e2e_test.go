package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func finaleEquipEarnedPoisonOrb(r *caiyiForestDriver) {
	r.t.Helper()
	initial := r.field()
	before := trialWormInventory(r, "", "五毒珠")
	if before["五毒珠"] != 1 {
		r.t.Fatal("actual earned poison orb is not in the original inventory", before)
	}
	r.keys(4096, 4)
	settle := func(ready func(pal.View) bool) {
		r.t.Helper()
		for i := 0; i < 128; i++ {
			v := r.visible()
			if v.Seq == v.AppliedSeq && ready(v) {
				return
			}
			r.progress()
		}
		r.t.Fatal("original poison-orb equipment menu did not settle", r.visible().Menu)
	}
	settle(func(v pal.View) bool { return v.Menu.Kind == 5 })
	r.keys(65536, 4)
	r.progress()
	r.progress()
	for i := 0; !strings.Contains(originalVisibleText(r.visible(), true), "五毒珠"); i++ {
		if i > 256 {
			r.t.Fatal("actual earned poison orb is missing from original equipment selection")
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	r.save("poison-orb-item-selected")
	r.keys(2, 4)
	settle(func(v pal.View) bool { return v.Menu.Kind == 6 })
	for i := 0; !strings.Contains(originalVisibleText(r.visible(), true), "李逍遥"); i++ {
		if i > 3 {
			r.t.Fatal("original poison-orb equipment role selector did not reach Li")
		}
		r.keys(4, 4)
		r.progress()
		r.progress()
	}
	weapon := templeEquipmentRow(caveEquipmentNodes(r.visible()), 77)
	previous := templeEquipmentRow(caveEquipmentNodes(r.visible()), 121)
	r.save("poison-orb-role-selected")
	r.keys(2, 4)
	settle(func(v pal.View) bool { return templeEquipmentRow(caveEquipmentNodes(v), 121) == "五毒珠" })
	r.save("poison-orb-equipped")
	for i := 0; i < 12; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			break
		}
		r.keys(1, 4)
		r.progress()
		r.progress()
	}
	v := r.field()
	after := trialWormInventory(r, "", "五毒珠", previous)
	if after["五毒珠"] != 0 || v.Cash != initial.Cash || v.Scene != initial.Scene || v.X != initial.X || v.Y != initial.Y {
		r.t.Fatal("original poison-orb equip changed unrelated field state", before, after, initial.Cash, v.Cash)
	}
	r.save("poison-orb-equipped-field")
	report := raw(map[string]any{"passed": true, "scope": "actual earned poison orb equipped on Li through original item/role menus and four fresh-runtime saves; no direct equipment/stat writes", "beforeInventory": before, "afterInventory": after, "previousAccessory": previous, "actualWeapon": weapon, "equippedAccessory": "五毒珠", "party": v.Party})
	if err := os.WriteFile(campaignEvidencePath("finale-poison-orb-equipment.json", 1), report, 0600); err != nil {
		r.t.Fatal(err)
	}
}

func TestOriginalFinaleEarnedPoisonOrbEquipmentAndSave(t *testing.T) {
	if os.Getenv("PAL_FINALE_EQUIPMENT") != "1" {
		t.Skip("explicit actual pre-palace earned protective equipment and saved menus")
	}
	name := "finale-step-9-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "15b4089530dce9769b94e03807b1dfc78e4af7ea8aa9caf1ca7d9771ef504fa2" {
		t.Fatal("pre-palace equipment source is not the actual first route arrival")
	}
	p := caiyiForestProof{}
	checks := finaleChecks{}
	for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
		data, err := os.ReadFile(campaignEvidencePath("finale-step-9-"+kind+".json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "finale-prepared", proof: p,
		allowedRoles: map[int]bool{0: true, 1: true, 4: true}, allowedMenus: map[int]bool{6: true}, milestones: map[string]string{}}
	finaleEquipEarnedPoisonOrb(r)
	checks.PoisonOrbPrepared = true
	for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks)} {
		if err := os.WriteFile(campaignEvidencePath("finale-prepared-"+kind+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
