package pal_test

// E2E first: the actual rewarded orb is not selectable, saving loses its item
// cursor, use does not execute the scene's original escape script, the orb is
// consumed/duplicated, or escape/save changes resources and accepts old keys.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalEarthOrbSelectedSaveAndEscape(t *testing.T) {
	r := &bloodPoolDriver{riverMedicineDriver: loadRiverMedicine(t, "campaign-blood-pool-state.json"), proof: bloodPoolProof{Saved: map[string]bool{}, Open: map[int]bool{}}}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"earth-orb-failure-state.json": r.g.Data, "earth-orb-failure-view.json": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	initial := r.field()
	if initial.Scene != 60 || len(initial.Party) != 3 {
		t.Fatal("earth orb source is not the original king ending", initial.Scene, initial.Party)
	}
	r.selectOrb(true)
	r.save("earth-orb-selected")
	r.keys(2, 4)
	r.progress()
	after := r.field()
	if after.Scene != 65 || after.Map != 54 || after.Cash != initial.Cash || !bytes.Equal(raw(after.Party), raw(initial.Party)) {
		t.Fatal("original earth-orb escape changed resources or destination", initial.Scene, after.Scene, after.Map, after.Cash, after.Party)
	}
	r.save("earth-orb-escaped")
	r.selectOrb(false)
	r.keys(1, 4)
	r.field()
	r.save("earth-orb-escape-complete")
	source, err := os.ReadFile(campaignEvidencePath("campaign-blood-pool-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "actual rewarded earth orb selected through ordinary original inventory, fresh-runtime selected-item save, ordinary confirm runs original scene escape into the original tomb exterior, orb remains exactly one, party/cash preserved, old input rejected", "scene": after.Scene, "map": after.Map, "party": after.Party, "cash": after.Cash, "saves": r.proof.Saves, "orbAmountAfterUse": 1}, "", "  ")
	for name, data := range map[string][]byte{"campaign-earth-orb-escaped-state.json": r.g.Data, "campaign-earth-orb-escaped.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *bloodPoolDriver) selectOrb(selected bool) {
	r.t.Helper()
	r.field()
	r.keys(2048, 4)
	r.inventoryReady()
	r.keys(65536, 4)
	r.progress()
	r.progress()
	for scans := 0; scans < 128; scans++ {
		v := r.inventoryReady()
		if amount, found := campaignInventoryAmount(v, "土灵珠"); found && (!selected || strings.Contains(originalVisibleText(v, true), "土灵珠")) {
			if amount != 1 {
				r.t.Fatal("original earth orb missing/duplicated", amount)
			}
			return
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	r.t.Fatal("original earth orb not selectable", originalVisibleText(r.visible(), false))
}

func (r *bloodPoolDriver) inventoryReady() pal.View {
	r.t.Helper()
	for i := 0; i < 128; i++ {
		v := r.visible()
		if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			return v
		}
		r.progress()
	}
	r.t.Fatal("original earth-orb inventory did not settle", r.visible().Menu)
	return pal.View{}
}
