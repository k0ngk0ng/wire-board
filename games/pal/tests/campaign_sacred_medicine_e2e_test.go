package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type sacredMedicineChecks struct {
	InventoryBefore, InventoryAfterBirth, InventoryAfterInsufficient map[string]int
	InventoryAtTrial                                                 map[string]int
	PartyOrders                                                      map[string][]int
	InsufficientChecked                                              bool
}

var sacredMedicineReadings = map[string]string{
	"medicine-delivery": "两种圣药都拿到了吗", "orb-origin": "是当年我爹娘偷走的吧",
	"orb-heir": "只有巫后娘娘才知道", "months-later": "数月后", "hot-water": "快去厨房提热水来",
	"daughter": "是个女儿", "yiru-name": "李忆如", "yue-memory": "牺牲性命换来的",
	"worm-quest": "替我收集三十六只", "worm-direction": "大理城外东南方", "escape-advice": "记得用土灵珠脱身",
	"trial-challenge": "谁！？", "anu-stops": "通通给我住手", "trial-secret": "此人看见我军的机密",
	"trial-misunderstanding": "这只是一场误会", "trial-child": "你怎么也来了", "trial-team": "你快完成老婆婆交待的事吧",
}

var sacredMedicineEntrySteps = []struct {
	Scene, Event, Dest int
	Token              string
	Search             bool
}{
	{204, 3403, 212, "", false},
	{212, 3581, 207, "", false},
	{207, 3459, 203, "", false},
	{203, 3383, 180, "", false},
	{180, 2954, 187, "", false},
	{187, 3131, 177, "", false},
	{177, 2917, 175, "", false},
	{175, 2876, 173, "worm-quest", true},
	{173, 2863, 173, "escape-advice", true},
	{173, 2855, 177, "", false},
	{177, 2916, 187, "", false},
	{187, 3132, 180, "", false},
	{180, 2955, 203, "", false},
	{203, 3385, 215, "", false},
	{215, 3646, 216, "", false},
	{216, 3658, 216, "trial-team", false},
}

func sacredMedicineTempleStairs(t *testing.T, tables *resources.Tables) map[int]map[[2]int][2]int {
	t.Helper()
	moves := map[int]map[[2]int][2]int{204: {}}
	count := 0
	for i := tables.Scenes[203].FirstEvent; i < tables.Scenes[204].FirstEvent; i++ {
		e := tables.Events[i]
		if e.State <= 0 || e.Sprite != 0 || e.TriggerMode < 4 || e.TriggerScript == 0 {
			continue
		}
		delta, done := [2]int{}, false
		for pc := int(e.TriggerScript); pc < len(tables.Scripts) && pc < int(e.TriggerScript)+80; pc++ {
			s := tables.Scripts[pc]
			if s.Opcode == 0 {
				done = true
				break
			}
			if s.Opcode == 0x6e {
				delta[0] += int(int16(s.Operands[0]))
				delta[1] += int(int16(s.Operands[1]))
			} else if s.Opcode != 0x15 && s.Opcode != 0x9 {
				break
			}
		}
		if !done || delta == [2]int{} {
			continue
		}
		areas := map[[2]int][2]int{}
		sacredTreeTouchArea(areas, [2]int{int(e.X), int(e.Y)}, [2]int{}, (int(e.TriggerMode)-4)*32+16)
		for at := range areas {
			if _, exists := moves[204][at]; !exists {
				moves[204][at] = [2]int{at[0] + delta[0], at[1] + delta[1]}
			}
		}
		count++
	}
	if count != 10 {
		t.Fatal("original present temple stair set changed", count)
	}
	return moves
}

func TestOriginalSacredMedicineBirthTrialQuestAndSave(t *testing.T) {
	if os.Getenv("PAL_SACRED_MEDICINE_ENTRY") != "1" {
		t.Skip("explicit actual sacred medicine, birth and trial entry; not full stage 22")
	}
	r, checks, source, rootHash, name, checkpoint := runSacredMedicineTrialEntry(t)
	v := sacredTreeField(r)
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "fullStage22Accepted": false, "scope": "actual earned stage21 ending, ordinary sacred medicine handover, original time skip/birth/naming and worm quest, insufficient-stock rejection, trial guards/Gaijiao conflict/Anu intervention and actual Li/Anu reunion; fresh-runtime saves; worm collection/delivery/recovery remain unaccepted", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "rootStage21SHA256": hex.EncodeToString(rootHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"sacred-medicine-trial-entry.json": report, "sacred-medicine-trial-entry-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func runSacredMedicineTrialEntry(t *testing.T) (*caiyiForestDriver, sacredMedicineChecks, []byte, [32]byte, string, string) {
	t.Helper()
	name := "campaign-old-yuhang-state.json"
	root, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-old-yuhang-state.json", root, "b191bb4254cfd8bbb9a51459d5b9da8c3a78b7aee67dca5669f0c0268f0e9ce3"); err != nil {
		t.Fatal(err)
	}
	checkpoint := os.Getenv("PAL_SACRED_MEDICINE_CHECKPOINT")
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := sacredMedicineChecks{PartyOrders: map[string][]int{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 0 || n > len(sacredMedicineEntrySteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported sacred medicine debug checkpoint", checkpoint)
			}
		}
		name = "sacred-medicine-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("sacred-medicine-"+checkpoint+"-"+kind+".json", 1))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatal(err)
			}
		}
	}
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "sacred-medicine", proof: p, tables: tables, milestones: sacredMedicineReadings,
		allowedRoles: map[int]bool{0: true, 4: true}, scriptedEnemies: map[int]bool{501: true}, reserveScenes: []int{204, 212, 207, 203, 180, 187, 177, 175, 173, 215, 216}}
	if checkpoint != "" && r.proof.InBattle && r.visible().Battle != nil {
		for _, enemy := range r.visible().Battle.Enemies {
			if enemy.Object == 501 {
				r.proof.BattleKind = 501
			}
		}
	}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}, sameSceneMoves: sacredMedicineTempleStairs(t, tables)}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("sacred-medicine-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.notice = func(v pal.View) {
		roles := []int{}
		for _, member := range v.Party {
			roles = append(roles, member.Role)
		}
		if !bytes.Equal(raw(roles), raw([]int{0})) && !bytes.Equal(raw(roles), raw([]int{0, 4})) {
			t.Fatal("original medicine/birth temporary party changed", v.Scene, roles)
		}
		label := fmt.Sprint("party-", roles)
		if !r.proof.Saved[label] {
			checks.PartyOrders[label] = roles
			save(label)
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			label = fmt.Sprint("field-", v.Scene, "-step-", r.proof.Step)
			if !r.proof.Saved[label] {
				save(label)
			}
		}
		if b := v.Battle; b != nil {
			r.battlePartySize = len(roles)
			alive, gaijiao := false, false
			for _, enemy := range b.Enemies {
				gaijiao = gaijiao || enemy.Object == 501
			}
			if r.proof.BattleKind == 501 {
				if _, _, won := campaignBattleReward(v); won {
					r.proof.BattleKind = 0
				}
			}
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive && !gaijiao {
				t.Fatal("original medicine route lost whole party", v.Party)
			}
			if !alive && gaijiao && !r.proof.Saved["gaijiao-original-defeat"] {
				save("gaijiao-original-defeat")
			}
			label = fmt.Sprint("battle-accepted-", r.proof.Battles)
			for _, member := range b.Players {
				if member.State == 2 && !r.proof.Saved[label] {
					save(label)
				}
			}
		}
	}
	t.Cleanup(func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("sacred-medicine-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	})
	items := []string{"金凤凰蛋壳", "火眼麒麟角", "水灵珠", "风灵珠", "火灵珠", "傀儡虫"}
	if checkpoint == "" {
		v := sacredTreeField(r)
		checks.InventoryBefore = r.inventoryCounts(items...)
		if v.Scene != 204 || checks.InventoryBefore["金凤凰蛋壳"] != 1 || checks.InventoryBefore["火眼麒麟角"] != 1 || checks.InventoryBefore["水灵珠"] != 1 || checks.InventoryBefore["傀儡虫"] >= 36 {
			t.Fatal("original sacred medicine entry inventory changed", v.Scene, checks.InventoryBefore)
		}
		save("step-0")
	}
	for r.proof.Step < len(sacredMedicineEntrySteps) {
		q := sacredMedicineEntrySteps[r.proof.Step]
		t.Log("original sacred medicine waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[3]int]int{}
		for i := 0; i < 8000; i++ {
			v := sacredTreeField(r)
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if q.Event == 2876 && v.Scene == 173 {
				r.keys(0, 4)
				r.progress()
				continue
			}
			if v.Scene != q.Scene {
				t.Fatal("sacred medicine route entered unexpected field", q, v.Scene, v.X, v.Y)
			}
			at := [3]int{v.Scene, v.X, v.Y}
			visits[at]++
			if visits[at] > 50 {
				t.Fatal("sacred medicine approach blocked", q, at)
			}
			x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					x, y = actor[3], actor[4]
				}
			}
			reach := 0
			if q.Search {
				reach = 32
			}
			if q.Search && fieldTestDistance(v.X, v.Y, x, y) <= reach {
				r.search(q.Event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, reach)
			}
			r.progress()
		}
		v := sacredTreeField(r)
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("original sacred medicine waypoint incomplete", q, v.Scene)
		}
		if q.Event == 2876 {
			checks.InventoryAfterBirth = r.inventoryCounts(items...)
			if checks.InventoryAfterBirth["金凤凰蛋壳"] != 0 || checks.InventoryAfterBirth["火眼麒麟角"] != 0 || checks.InventoryAfterBirth["水灵珠"] != 1 || checks.InventoryAfterBirth["风灵珠"] != 1 || checks.InventoryAfterBirth["火灵珠"] != 1 || checks.InventoryAfterBirth["傀儡虫"] != checks.InventoryBefore["傀儡虫"] || len(v.Party) != 1 || v.Party[0].Role != 0 {
				t.Fatal("original medicine consumption, birth party or earned orbs changed", checks.InventoryAfterBirth, v.Party)
			}
			save("birth-and-medicine-verified")
		}
		if q.Event == 2863 {
			checks.InventoryAfterInsufficient = r.inventoryCounts(items...)
			if !bytes.Equal(raw(checks.InventoryAfterBirth), raw(checks.InventoryAfterInsufficient)) {
				t.Fatal("insufficient original worm delivery consumed inventory", checks.InventoryAfterInsufficient)
			}
			checks.InsufficientChecked = true
			save("insufficient-worms-preserved")
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := sacredTreeField(r)
	checks.InventoryAtTrial = r.inventoryCounts(items...)
	if v.Scene != 216 || v.Map != 213 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 4 || !checks.InsufficientChecked || r.proof.Battles != r.proof.Wins+r.proof.ScriptedFinishes {
		t.Fatal("actual trial entry, party or ordinary battles incomplete", v.Scene, v.Party, checks, r.proof)
	}
	for name := range sacredMedicineReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original sacred medicine milestone missing", name, r.proof.Seen)
		}
	}
	save("trial-entry-complete")
	return r, checks, source, rootHash, name, checkpoint
}
