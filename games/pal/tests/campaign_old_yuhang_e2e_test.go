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

type oldYuhangChecks struct {
	InventoryBefore, InventoryWithCloth, InventoryAfterWrongUse  map[string]int
	InventoryAfterCloth, InventoryBeforeExchange, InventoryAfter map[string]int
	PartyOrders                                                  map[string][]int
	WrongUseChecked, ClothUsed                                   bool
	RepeatInputs                                                 int
}

var oldYuhangReadings = map[string]string{
	"ling-warning": "你不要过来", "ling-fire": "火来", "cloth-quest": "请你拿著这条手绢到港口边",
	"cloth-granted": "凤纹手绢", "cloth-recognized": "你怎么会有这条手绢", "lingyue": "贫道正是灵月",
	"grandmother-healed": "你的脚应该没事了", "healing-lesson": "救人，而非与人争斗的",
	"harbor-farewell": "我们就此告别", "dream-spell": "回魂仙梦", "aunt-search": "我家的小李子",
	"xiulan-search": "大娘叫我来找李家哥哥", "toy-chest": "箱中有若干童玩物", "child-guards": "不许动我的东西",
	"sword-demand": "你把你的宝剑给我", "wooden-sword-agreement": "我给你一把木剑",
	"wooden-sword-request": "不是说好要给我一把木剑吗", "exchange": "好～交换", "water-orb": "得到水灵珠",
	"return-present": "我回来了吗", "anu-reunion": "娘娘终于放你回来了", "egg-shell": "得到金凤凰蛋壳",
	"month-passed": "你一失踪就是一个月", "ling-concern": "希望灵儿不会有事才好",
}

var oldYuhangSteps = []struct {
	Scene, Event, Dest int
	Token              string
	Search             bool
}{
	{267, 4657, 269, "", false},
	{269, 4676, 269, "ling-fire", false},
	{269, 4674, 269, "cloth-granted", true},
	{269, 4673, 267, "", false},
	{267, 4656, 270, "", false},
	{270, 4681, 268, "", false},
	{268, 4664, 274, "", false},
	{274, 4725, 274, "aunt-search", true},
	{274, 4720, 268, "", false},
	{268, 4665, 270, "", false},
	{270, 4680, 267, "", false},
	{267, 4662, 267, "xiulan-search", true},
	{267, 4661, 267, "wooden-sword-agreement", true},
	{267, 4663, 204, "ling-concern", true},
}

func oldYuhangUseCloth(r *caiyiForestDriver, label string) {
	r.t.Helper()
	r.riverMedicineDriver.inventory("凤纹手绢", true)
	r.save(label)
	r.keys(2, 4)
	r.progress()
	for i := 0; i < 128; i++ {
		v := r.visible()
		if v.Phase != "menu" {
			return
		}
		if v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			r.keys(1, 4)
		}
		r.progress()
	}
	r.t.Fatal("original handkerchief use did not leave the inventory", r.visible().Menu)
}

func TestOriginalOldYuhangHandkerchiefWoodenSwordWaterOrbAndSave(t *testing.T) {
	if os.Getenv("PAL_OLD_YUHANG") != "1" {
		t.Skip("explicit actual old Yuhang and water-orb story acceptance")
	}
	name := "campaign-temple-past-state.json"
	root, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-temple-past-state.json", root, "1ce17774473cf98aad2810bcc644ba1d1d0035893c9163b715b0623e79ebe566"); err != nil {
		t.Fatal(err)
	}
	checkpoint := os.Getenv("PAL_OLD_YUHANG_CHECKPOINT")
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := oldYuhangChecks{PartyOrders: map[string][]int{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 0 || n > len(oldYuhangSteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported old Yuhang debug checkpoint", checkpoint)
			}
		}
		name = "old-yuhang-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("old-yuhang-"+checkpoint+"-"+kind+".json", 1))
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
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "old-yuhang", proof: p, tables: tables, milestones: oldYuhangReadings,
		allowedRoles: map[int]bool{0: true, 4: true}, reserveScenes: []int{267, 268, 269, 270, 274}}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("old-yuhang-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.notice = func(v pal.View) {
		roles := []int{}
		for _, member := range v.Party {
			roles = append(roles, member.Role)
		}
		if !bytes.Equal(raw(roles), raw([]int{0})) && !bytes.Equal(raw(roles), raw([]int{0, 4})) {
			t.Fatal("old Yuhang original party changed", v.Scene, roles)
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
		if v.Scene == 267 && v.Phase == "script" && r.proof.Seen["water-orb"] && !r.proof.Saved["orb-return-transition"] {
			save("orb-return-transition")
		}
		if b := v.Battle; b != nil {
			r.battlePartySize = len(roles)
			if len(v.Party) != 1 || v.Party[0].HP == 0 {
				t.Fatal("old Yuhang battle party defeated or changed", v.Party)
			}
			label = fmt.Sprint("battle-accepted-", r.proof.Battles)
			for _, member := range b.Players {
				if member.State == 2 && !r.proof.Saved[label] {
					save(label)
				}
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("old-yuhang-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	items := []string{"凤纹手绢", "木剑", "水灵珠", "金凤凰蛋壳", "火灵珠", "风灵珠", "火眼麒麟角"}
	if checkpoint == "" {
		v := sacredTreeField(r)
		checks.InventoryBefore = r.inventoryCounts(items...)
		if v.Scene != 267 || checks.InventoryBefore["木剑"] != 1 || checks.InventoryBefore["凤纹手绢"] != 0 || checks.InventoryBefore["水灵珠"] != 0 || checks.InventoryBefore["金凤凰蛋壳"] != 0 {
			t.Fatal("old Yuhang starting field or original inventory changed", v.Scene, checks.InventoryBefore)
		}
		save("step-0")
	}
	for r.proof.Step < len(oldYuhangSteps) {
		q := oldYuhangSteps[r.proof.Step]
		t.Log("original old Yuhang waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[3]int]int{}
		for i := 0; i < 8000; i++ {
			v := sacredTreeField(r)
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if checks.ClothUsed && q.Event == 4725 {
				r.keys(0, 4)
				r.progress()
				continue
			}
			if v.Scene != q.Scene {
				t.Fatal("old Yuhang entered an unexpected field", q, v.Scene, v.X, v.Y)
			}
			at := [3]int{v.Scene, v.X, v.Y}
			visits[at]++
			if visits[at] > 50 {
				t.Fatal("old Yuhang approach blocked", q, at)
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
				if q.Event == 4674 && (v.X != 832 || v.Y != 368) {
					r.g = r.w.walk(t, r.m, &r.c, r.g, v, 832, 368, 0)
					r.progress()
					continue
				}
				r.search(q.Event, x, y)
				if q.Event == 4725 {
					sacredTreeField(r)
					oldYuhangUseCloth(r, "cloth-selected-at-merchant")
					checks.ClothUsed = true
				}
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, reach)
			}
			r.progress()
		}
		v := sacredTreeField(r)
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("old Yuhang waypoint incomplete", q, v.Scene, r.proof.Seen)
		}
		if q.Event == 4674 {
			checks.InventoryWithCloth = r.inventoryCounts(items...)
			if checks.InventoryWithCloth["凤纹手绢"] != 1 || checks.InventoryWithCloth["水灵珠"] != 0 || checks.InventoryWithCloth["金凤凰蛋壳"] != 0 {
				t.Fatal("old Yuhang cloth grant or premature orb/egg", checks.InventoryWithCloth)
			}
			save("cloth-collected")
			oldYuhangUseCloth(r, "cloth-selected-away")
			sacredTreeField(r)
			checks.InventoryAfterWrongUse = r.inventoryCounts(items...)
			if !bytes.Equal(raw(checks.InventoryWithCloth), raw(checks.InventoryAfterWrongUse)) || r.proof.Seen["cloth-recognized"] {
				t.Fatal("old Yuhang handkerchief worked away from merchant", checks.InventoryAfterWrongUse)
			}
			checks.WrongUseChecked = true
			save("cloth-wrong-place-preserved")
		}
		if q.Event == 4725 {
			checks.InventoryAfterCloth = r.inventoryCounts(items...)
			if checks.InventoryAfterCloth["凤纹手绢"] != 0 || checks.InventoryAfterCloth["木剑"] != 1 || checks.InventoryAfterCloth["水灵珠"] != 0 || checks.InventoryAfterCloth["金凤凰蛋壳"] != 0 {
				t.Fatal("old Yuhang cloth delivery inventory changed", checks.InventoryAfterCloth)
			}
		}
		if q.Event == 4661 {
			checks.InventoryBeforeExchange = r.inventoryCounts(items...)
			if !bytes.Equal(raw(checks.InventoryAfterCloth), raw(checks.InventoryBeforeExchange)) {
				t.Fatal("old Yuhang chest granted orb before actual exchange")
			}
			save("exchange-not-yet-made")
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := sacredTreeField(r)
	checks.InventoryAfter = r.inventoryCounts(items...)
	for _, name := range []string{"火灵珠", "风灵珠", "火眼麒麟角"} {
		if checks.InventoryBefore[name] != 1 || checks.InventoryAfter[name] != 1 {
			t.Fatal("old Yuhang changed an earned sacred item", name, checks)
		}
	}
	if v.Scene != 204 || v.Map != 175 || checks.InventoryAfter["凤纹手绢"] != 0 || checks.InventoryAfter["木剑"] != 0 || checks.InventoryAfter["水灵珠"] != 1 || checks.InventoryAfter["金凤凰蛋壳"] != 1 || !checks.WrongUseChecked || !checks.ClothUsed || r.proof.Wins != r.proof.Battles || len(checks.PartyOrders) != 2 || !r.proof.Saved["orb-return-transition"] {
		t.Fatal("old Yuhang ending, inventory, party or battle incomplete", v.Scene, v.Map, checks, r.proof)
	}
	for checks.RepeatInputs < 3 {
		r.keys(2, 4)
		r.progress()
		sacredTreeField(r)
		checks.RepeatInputs++
	}
	if !bytes.Equal(raw(checks.InventoryAfter), raw(r.inventoryCounts(items...))) {
		t.Fatal("old Yuhang exchange or egg grant duplicated after confirmation")
	}
	save("returned-no-duplication")
	for name := range oldYuhangReadings {
		if !r.proof.Seen[name] {
			t.Fatal("old Yuhang milestone missing", name, r.proof.Seen)
		}
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "actual old Yuhang child/grandmother, wrong-place and actual merchant handkerchief use, Lingyue/healing/farewell, Xiulan/chest/young Li, ordinary wooden-sword consumption and water-orb exchange, present Anu reunion and egg shell; ordinary inputs and fresh-runtime saves, no native data or state edits", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "rootStage20SHA256": hex.EncodeToString(rootHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-old-yuhang.json": report, "campaign-old-yuhang-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
