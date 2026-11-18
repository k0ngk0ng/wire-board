package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestOriginalOldYuhangClothRepeatedDialogueAndSavedNoDuplication(t *testing.T) {
	if os.Getenv("PAL_OLD_YUHANG_REWARDS") != "1" {
		t.Skip("explicit original earned handkerchief repeat dialogue acceptance")
	}
	if campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("old Yuhang rewards require the original registered current-core profile")
	}
	name := "old-yuhang-field-269-step-3-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptedCampaignRoot(name, source, ""); err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables, prefix: "old-yuhang-rewards-v1", allowedRoles: map[int]bool{0: true}, reserveScenes: []int{269}, milestones: map[string]string{}, physicalAttacks: true, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	r.reserve(-1)
	initial := r.field()
	if initial.Scene != 269 || len(initial.Party) != 1 || initial.Party[0].Role != 0 || initial.Party[0].Controller != 0 {
		t.Fatal("old Yuhang reward source lost the original solo party")
	}
	items := []string{"凤纹手绢", "木剑", "水灵珠", "金凤凰蛋壳"}
	before := r.inventoryCounts(items...)
	if before["凤纹手绢"] != 1 || before["木剑"] != 1 || before["水灵珠"] != 0 || before["金凤凰蛋壳"] != 0 {
		t.Fatal("old Yuhang cloth was not normally earned before repeat dialogue", before)
	}
	r.save("before-repeat")
	readings := []int{}
	for repeat := 1; repeat <= 3; repeat++ {
		prior := r.proof.Readings
		for attempt := 0; attempt < 30 && r.proof.Readings == prior; attempt++ {
			r.search(4674, 848, 360)
			r.progress()
			r.field()
		}
		if r.proof.Readings <= prior || !bytes.Equal(raw(before), raw(r.inventoryCounts(items...))) || r.field().Cash != initial.Cash {
			t.Fatal("original grandmother repeat dialogue did not retain the single cloth", repeat)
		}
		readings = append(readings, r.proof.Readings-prior)
		r.save("repeat-" + string(rune('0'+repeat)))
	}
	final := r.field()
	if r.proof.Battles != 0 || !bytes.Equal(raw(initial.Party), raw(final.Party)) || r.proof.Saves != 4 {
		t.Fatal("original cloth repeat verification changed party or normal field")
	}
	hash := sha256.Sum256(source)
	result := raw(map[string]any{"passed": true, "scope": "three actual original grandmother repeat conversations after normally earned cloth, exactly one cloth and unchanged wooden sword/no premature orb or egg, four fresh-runtime saves and stale input rejection; no native edits", "event": 4674, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceSHA256": hex.EncodeToString(hash[:]), "repeatConversations": 3, "readingsPerRepeat": readings, "inventoryBefore": before, "inventoryAfter": r.inventoryCounts(items...), "cashBefore": initial.Cash, "cashAfter": final.Cash, "proof": r.proof, "nativeEdits": false})
	for suffix, data := range map[string][]byte{"result": result, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath("old-yuhang-rewards-v1-"+suffix+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
