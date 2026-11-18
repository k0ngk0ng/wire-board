package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalFinalMazeFalseJadeChestAndSavedNoGrant(t *testing.T) {
	if os.Getenv("PAL_FINAL_EMPTY_CHEST") != "1" {
		t.Skip("explicit original false jade chest")
	}
	originalFalseJadeChest(t, "finale-field-293-step-1-state.json", "final-maze-empty-v1", 293, 5049, [2]int{752, 904}, []int{1, 0, 4})
}

func TestOriginalTowerFalseJadeChestAndSavedNoGrant(t *testing.T) {
	if os.Getenv("PAL_TOWER_EMPTY_CHEST") != "1" {
		t.Skip("explicit original tower false jade chest")
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != "tower-remainder-v1" {
		t.Fatal("tower false jade requires isolated evidence scope")
	}
	originalFalseJadeChest(t, "tower-upper-step-3-state.json", "tower-remainder-empty-v1", 167, 2764, [2]int{336, 616}, []int{0, 2})
}

func originalFalseJadeChest(t *testing.T, name, prefix string, scene, event int, position [2]int, roles []int) {
	t.Helper()
	if campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("false jade chest requires registered authority profile")
	}
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
	e := tables.Events[event-1]
	if int(e.X) != position[0] || int(e.Y) != position[1] || e.Sprite != 10 || e.TriggerScript != 37479 || tables.Scripts[37482].Opcode != 0x1f || tables.Scripts[37482].Operands != [3]uint16{} {
		t.Fatal("original false jade chest changed")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables, prefix: prefix, physicalAttacks: true, allowedRoles: map[int]bool{}, battlePartySize: len(roles), reserveScenes: []int{scene}, milestones: map[string]string{}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	for _, role := range roles {
		r.allowedRoles[role] = true
	}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	reserveTrialRewardObjects(r, -1)
	forceInputs := 0
	r.manualBattle = func(v pal.View) ([]byte, bool) {
		a, ok := caiyiForestPhysicalAttack(v)
		if !ok {
			return nil, true
		}
		if v.Battle.State == 1 && v.Battle.Menu == 0 {
			a.Direction, a.Keys = 4, 32768
			forceInputs++
		}
		return raw(a), true
	}
	noticed := false
	battles := []map[string]any{}
	r.notice = func(v pal.View) {
		if !noticed && strings.Contains(hiddenNoticeText(v), "获得紫青玉蓉膏") {
			noticed = true
			r.save("notice")
		}
		if r.proof.InBattle && v.Battle == nil {
			battles = append(battles, map[string]any{"cashBefore": r.proof.BattleCash, "cashAfter": v.Cash, "rewardCash": r.proof.RewardCash})
			r.w.known = nil
		}
	}
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-failure-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	initial := r.field()
	if initial.Scene != scene {
		t.Fatal("false jade source left original scene")
	}
	if len(initial.Party) != len(roles) {
		t.Fatal("false jade party size changed")
	}
	for i, role := range roles {
		if initial.Party[i].Role != role {
			t.Fatal("false jade party order changed")
		}
	}
	before := trialWormInventory(r, "", "紫菁玉蓉膏")["紫菁玉蓉膏"]
	r.save("unpicked")
	visits := map[[2]int]int{}
	for step := 0; step < 3600 && !noticed; step++ {
		v := r.field()
		at := [2]int{v.X, v.Y}
		visits[at]++
		if v.Scene != scene || visits[at] > 40 {
			t.Fatal("false jade original search approach blocked", at)
		}
		if scene == 167 && [2]int{v.X, v.Y} != [2]int{320, 624} {
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, 320, 624, 0)
		} else if fieldTestDistance(v.X, v.Y, position[0], position[1]) <= 96 {
			r.search(event, position[0], position[1])
		} else {
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, position[0], position[1], 96)
		}
		r.progress()
	}
	if !noticed {
		t.Fatal("false jade original notice absent")
	}
	r.field()
	r.save("collected")
	for repeat := 0; repeat < 3; repeat++ {
		r.search(event, position[0], position[1])
		r.progress()
		r.field()
	}
	after := trialWormInventory(r, "", "紫菁玉蓉膏")["紫菁玉蓉膏"]
	if before != after || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 {
		t.Fatal("false jade chest granted item or battle did not finish", before, after)
	}
	opened := false
	for _, actor := range r.field().Field.Actors {
		opened = opened || actor[0] == 1 && actor[1] == event && actor[2] == 10 && actor[5] == 1
	}
	if !opened {
		t.Fatal("false jade original open frame missing")
	}
	r.save("repeated")
	final := r.field()
	rewardCash := 0
	for _, row := range battles {
		rewardCash += row["rewardCash"].(int)
	}
	if int64(final.Cash)-int64(initial.Cash) != int64(rewardCash) || len(battles) != r.proof.Battles {
		t.Fatal("false jade battle wallet changed", fmt.Sprint(battles))
	}
	hash := sha256.Sum256(source)
	result := raw(map[string]any{"passed": true, "event": event, "scene": scene, "position": position, "sourceCheckpoint": name, "sourceSHA256": hex.EncodeToString(hash[:]), "nativeEdits": false, "originalGrantPC": 37482, "originalItemID": 0, "notification": true, "itemBefore": before, "itemAfter": after, "repeatSearches": 3, "cashInitial": initial.Cash, "cashFinal": final.Cash, "battleCashChanges": battles, "forceInputs": forceInputs, "proof": r.proof})
	for suffix, data := range map[string][]byte{"result": result, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
