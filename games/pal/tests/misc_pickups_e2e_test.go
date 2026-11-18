package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalYueSwordAndDeerTrapPickupWithSavedRepeat(t *testing.T) {
	mode := os.Getenv("PAL_MISC_PICKUP")
	if mode == "" {
		t.Skip("explicit original dropped sword or deer trap")
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != "misc-rewards-v1" || campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("miscellaneous pickup requires isolated authority profile")
	}
	specs := map[string]struct {
		Source, Item, Message string
		Scene, Event, ItemID  int
	}{"sword": {"yueru-injury-state.json", "越女剑", "拾起地上的宝剑", 22, 399, 173}, "trap": {"campaign-white-river-fishing-state.json", "捕兽夹", "收起捕兽夹", 49, 798, 285}}
	q, ok := specs[mode]
	if !ok {
		t.Fatal("unsupported original pickup", mode)
	}
	source, err := os.ReadFile(campaignEvidencePath(q.Source, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptedCampaignRoot(q.Source, source, ""); err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	e := tables.Events[q.Event-1]
	prefix := fmt.Sprint("misc-pickup-v1-", q.Event)
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, q.Source), tables: tables, prefix: prefix, allowedRoles: map[int]bool{0: true, 1: true, 2: true}, physicalAttacks: true, reserveScenes: []int{22, 49, 50, 52}, milestones: map[string]string{}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	r.battlePartySize = len(r.visible().Party)
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	r.reserve(-1)
	r.manualBattle = func(v pal.View) ([]byte, bool) {
		a, ok := caiyiForestPhysicalAttack(v)
		if !ok {
			return nil, true
		}
		if v.Battle.State == 1 && v.Battle.Menu == 0 {
			a.Direction, a.Keys = 4, 32768
		}
		return raw(a), true
	}
	active, noticed := false, false
	battles := []map[string]any{}
	r.notice = func(v pal.View) {
		if active && !noticed && strings.Contains(hiddenNoticeText(v)+originalVisibleText(v, false), q.Message) {
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
	r.save("source")
	if mode == "sword" {
		ready := false
		for step := 0; step < 1400; step++ {
			v := r.field()
			visible := false
			for _, actor := range v.Field.Actors {
				visible = visible || actor[0] == 1 && actor[1] == 399
			}
			if len(v.Party) == 2 && v.Party[1].Role == 1 && visible {
				ready = true
				break
			}
			if v.Scene != 22 {
				t.Fatal("original injury recovery left encounter")
			}
			r.search(404, int(tables.Events[403].X), int(tables.Events[403].Y))
			r.progress()
		}
		if !ready {
			t.Fatal("original Ling recovery did not enable dropped sword")
		}
		r.save("original-recovery-complete")
	} else {
		for _, door := range []struct{ Event, Scene int }{{871, 50}, {801, 49}} {
			r.reserve(door.Event)
			target := tables.Events[door.Event-1]
			arrived := false
			for step := 0; step < 1600; step++ {
				v := r.field()
				if v.Scene == door.Scene {
					arrived = true
					break
				}
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(target.X), int(target.Y), 0)
				r.progress()
			}
			if !arrived {
				t.Fatal("original trap approach portal blocked", door.Event)
			}
			r.reserve(-1)
			r.save(fmt.Sprint("door-", door.Event, "-arrived"))
		}
	}
	initial := r.field()
	if initial.Scene != q.Scene {
		t.Fatal("original pickup source scene changed", q.Event, initial.Scene)
	}
	before := trialWormInventory(r, "", q.Item)[q.Item]
	if before != 0 {
		t.Fatal("original pickup already in bag", q.Event, before)
	}
	r.save("unpicked")
	active = true
	visits := map[[2]int]int{}
	for step := 0; step < 2400 && !noticed; step++ {
		v := r.field()
		at := [2]int{v.X, v.Y}
		visits[at]++
		if v.Scene != q.Scene || visits[at] > 50 {
			t.Fatal("original pickup approach blocked", q.Event, at)
		}
		if fieldTestDistance(v.X, v.Y, int(e.X), int(e.Y)) <= 96 {
			r.search(q.Event, int(e.X), int(e.Y))
		} else {
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(e.X), int(e.Y), 96)
		}
		r.progress()
	}
	if !noticed {
		t.Fatal("original pickup notice absent", q.Event, q.Message)
	}
	r.field()
	r.save("collected")
	after := trialWormInventory(r, "", q.Item)[q.Item]
	if after != 1 {
		t.Fatal("original pickup amount changed", q.Event, before, after)
	}
	for repeat := 0; repeat < 3; repeat++ {
		r.search(q.Event, int(e.X), int(e.Y))
		r.progress()
		r.field()
	}
	finalAmount := trialWormInventory(r, "", q.Item)[q.Item]
	if finalAmount != 1 {
		t.Fatal("original pickup duplicated after save", q.Event, finalAmount)
	}
	r.save("repeated")
	final := r.field()
	if r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 || !bytes.Equal(raw(initial.Party), raw(final.Party)) && r.proof.Battles == 0 {
		t.Fatal("original pickup party or battle continuation changed")
	}
	rewardCash := 0
	for _, battle := range battles {
		rewardCash += battle["rewardCash"].(int)
	}
	if len(battles) != r.proof.Battles {
		t.Fatal("original pickup battle ledger incomplete")
	}
	hash := sha256.Sum256(source)
	result := raw(map[string]any{"passed": true, "event": q.Event, "scene": q.Scene, "position": [2]int{int(e.X), int(e.Y)}, "item": q.Item, "itemID": q.ItemID, "sourceCheckpoint": q.Source, "sourceSHA256": hex.EncodeToString(hash[:]), "nativeEdits": false, "notification": true, "message": q.Message, "amountBefore": before, "amountAfter": after, "amountAfterRepeat": finalAmount, "repeatSearches": 3, "cashInitial": initial.Cash, "cashFinal": final.Cash, "battleCashChanges": battles, "allRouteRewardCash": rewardCash, "proof": r.proof})
	for suffix, data := range map[string][]byte{"result": result, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
