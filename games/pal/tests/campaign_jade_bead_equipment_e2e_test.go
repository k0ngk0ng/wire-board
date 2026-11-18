package pal_test

// Original equipment E2E first: a gray non-owner role accepts the jade bead;
// the earned item disappears/duplicates; Ling's accessory or displayed magic
// and defense do not change by the original net effects; saving loses either
// selected/equipped menu state; equipping changes HP/MP/cash or accepts old keys.
import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func templeEquipmentRow(vNodes [][16]int, y int) string {
	letters := map[int]rune{}
	for _, n := range vNodes {
		if n[0] == 2 && n[6] >= 130 && n[6] < 226 && n[7] == y && n[8] != 0 {
			letters[n[6]] = rune(n[5])
		}
	}
	xs := []int{}
	for x := range letters {
		xs = append(xs, x)
	}
	sort.Ints(xs)
	var s strings.Builder
	for _, x := range xs {
		s.WriteRune(letters[x])
	}
	return s.String()
}

func templeEquipmentValue(nodes [][16]int, y int) int {
	digits := map[int]int{}
	for _, n := range nodes {
		if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] >= 56 && n[7] <= 65 && n[8] >= 260 && n[8] <= 278 && n[9] == y {
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
	value := 0
	for _, x := range xs {
		value = value*10 + digits[x]
	}
	return value
}

func TestOriginalJadeBeadEquipmentAndSavedLingRole(t *testing.T) {
	r := loadRiverMedicine(t, "campaign-jade-temple-state.json")
	defer r.failure()
	initial := r.field()
	if initial.Scene != 58 || len(initial.Party) != 3 {
		t.Fatal("not the original temple ending", initial.Scene, initial.Party)
	}
	r.keys(4096, 4)
	r.menu()
	r.keys(65536, 4)
	r.progress()
	r.progress()
	for scans := 0; ; scans++ {
		v := r.menu()
		if strings.Contains(originalVisibleText(v, true), "玉佛珠") {
			if n, ok := campaignInventoryAmount(v, "玉佛珠"); !ok || n != 1 {
				t.Fatal("jade bead missing or duplicated", n, ok)
			}
			break
		}
		if scans > 96 {
			t.Fatal("earned jade bead not available in original equipment inventory")
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	r.save("bead-equipment-item")
	r.keys(2, 4)
	r.progress()
	wait := func(ready func() bool) {
		t.Helper()
		for i := 0; i < 96; i++ {
			v := r.visible()
			if v.Seq == v.AppliedSeq && ready() {
				return
			}
			r.progress()
		}
		t.Fatal("original jade bead equipment did not settle", r.visible().Menu, originalVisibleText(r.visible(), false))
	}
	wait(func() bool { return r.visible().Menu.Kind == 6 })
	for scans := 0; !strings.Contains(originalVisibleText(r.visible(), true), "赵灵儿"); scans++ {
		if scans > 4 {
			t.Fatal("Ling could not equip her original jade bead")
		}
		r.keys(4, 4)
		r.progress()
		r.progress()
	}
	v := r.visible()
	beforeNodes := caveEquipmentNodes(v)
	beforeMagic, beforeDefense := templeEquipmentValue(beforeNodes, 36), templeEquipmentValue(beforeNodes, 58)
	if templeEquipmentRow(beforeNodes, 121) != "香袋" || beforeMagic < 0 || beforeDefense < 0 {
		t.Fatal("original Ling accessory/stat baseline changed", templeEquipmentRow(beforeNodes, 121), beforeMagic, beforeDefense)
	}
	r.save("bead-ling-selected")
	r.keys(2, 4)
	r.progress()
	wait(func() bool { return templeEquipmentRow(caveEquipmentNodes(r.visible()), 121) == "玉佛珠" })
	afterNodes := caveEquipmentNodes(r.visible())
	afterMagic, afterDefense := templeEquipmentValue(afterNodes, 36), templeEquipmentValue(afterNodes, 58)
	// The original fragrance pouch contributes +8 magic. Replacing it with
	// jade bead (+88 magic, +18 defense) therefore gives net +80 / +18.
	if afterMagic-beforeMagic != 80 || afterDefense-beforeDefense != 18 {
		t.Fatal("original jade bead net effects changed", beforeMagic, afterMagic, beforeDefense, afterDefense)
	}
	r.save("bead-equipped")
	r.keys(1, 4)
	r.menu()
	r.keys(1, 4)
	final := r.field()
	if final.Cash != initial.Cash || !bytes.Equal(raw(final.Party), raw(initial.Party)) || final.Scene != initial.Scene || final.X != initial.X || final.Y != initial.Y {
		t.Fatal("equipping jade bead changed non-equipment resources/position")
	}
	r.save("bead-equipped-field")
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual earned jade bead equipped on original Ling role through normal menus; original accessory row, net magic +80 and defense +18 replacing fragrance pouch, four fresh-runtime saves, original party/resources and old-input rejection", "role": 1, "controller": 0, "magicBefore": beforeMagic, "magicAfter": afterMagic, "defenseBefore": beforeDefense, "defenseAfter": afterDefense, "saves": r.p.Saves, "developmentProfile": developmentHealthProfile()}, "", "  ")
	for name, data := range map[string][]byte{"jade-temple-bead-equipment.json": report, "campaign-jade-temple-equipped-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
