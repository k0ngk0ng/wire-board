package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestOriginalShuJadeAndSevenStarSavedRepeat(t *testing.T) {
	if os.Getenv("PAL_SHU_REWARD_REPEAT") != "1" {
		t.Skip("explicit earned original jade and seven-star repeated searches")
	}
	if campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" || os.Getenv("PAL_EVIDENCE_SCOPE") != "tower-remainder-v1" {
		t.Fatal("Shu repeat requires isolated registered authority scope")
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		Name, Item   string
		Event, Scene int
	}{{"shu-tower-step-6-state.json", "玉佛珠", 2677, 161}, {"tower-seven-star-repeated-state.json", "七星剑", 2408, 148}} {
		t.Run(fmt.Sprint(q.Event), func(t *testing.T) {
			source, err := os.ReadFile(campaignEvidencePath(q.Name, 1))
			if err != nil {
				t.Fatal(err)
			}
			if err := acceptedCampaignRoot(q.Name, source, ""); err != nil {
				t.Fatal(err)
			}
			prefix := fmt.Sprint("shu-reward-repeat-v1-", q.Event)
			r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, q.Name), tables: tables, prefix: prefix, physicalAttacks: true, reserveScenes: []int{q.Scene}, milestones: map[string]string{}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
			r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
			r.reserve(-1)
			initial := r.field()
			if initial.Scene != q.Scene || len(initial.Party) != 2 || initial.Party[0].Role != 0 || initial.Party[1].Role != 2 {
				t.Fatal("Shu reward source party or scene changed", initial.Party, initial.Scene)
			}
			names := []string{q.Item}
			if q.Event == 2408 {
				names = append(names, "天师道袍")
			}
			before := trialWormInventory(r, "", names...)
			if before[q.Item] != 1 {
				t.Fatal("genuine earned Shu reward missing", q.Event, before)
			}
			r.save("before")
			counts := []map[string]int{}
			for repeat := 1; repeat <= 3; repeat++ {
				e := tables.Events[q.Event-1]
				r.search(q.Event, int(e.X), int(e.Y))
				r.progress()
				r.field()
				after := trialWormInventory(r, "", names...)
				if !bytes.Equal(raw(after), raw(before)) {
					t.Fatal("earned Shu reward repeated", q.Event, repeat, before, after)
				}
				counts = append(counts, after)
				r.save(fmt.Sprint("repeat-", repeat))
			}
			final := r.field()
			if final.Cash != initial.Cash || r.proof.Battles != 0 || !bytes.Equal(raw(initial.Party), raw(final.Party)) {
				t.Fatal("Shu reward repeats changed party, combat or wallet")
			}
			hash := sha256.Sum256(source)
			result := raw(map[string]any{"passed": true, "event": q.Event, "scene": q.Scene, "item": q.Item, "sourceCheckpoint": q.Name, "sourceSHA256": hex.EncodeToString(hash[:]), "nativeEdits": false, "existingEarnedBranch": true, "inventoryBefore": before, "inventoryAfter": counts[2], "repeatInventories": counts, "repeatSearches": 3, "cash": final.Cash, "proof": r.proof})
			for suffix, data := range map[string][]byte{"result": result, "state": r.g.Data} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
