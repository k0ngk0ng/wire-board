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

type templePastChecks struct {
	InventoryBefore, InventoryWithStaff, InventoryAfter map[string]int
	PartyOrders                                         map[string][]int
	DisguiseSaved                                       bool
	StaffRepeatInputs                                   int
	WaterBattles, WaterAcceptedSaves                    int
	LastWaterBattle                                     int
	ChasePositions                                      map[string][]int
	RescueBattles                                       int
}

var templePastReadings = map[string]string{
	"temple-prayer": "保佑我族，早日脱离苦难", "temple-reason": "麒麟老公公叫我们来这",
	"queen-question": "你可是真心爱着灵儿", "ten-years": "我要你回到十年前",
	"past-entry": "咦．．这是哪里", "rescue-princess": "竟然连小公主也不放过",
	"rescue-thanks": "谢谢你替我们解围", "nanshao": "这里是黑苗族的都城",
	"grandmother-flies": "嘟儿．．飞吧", "disguise": "还满好看的",
	"boat": "你就搭我们的船一道回去吧", "palace-crowd": "杀妖孽、清君侧",
	"stone-protests": "你带兵包围王宫更不是大臣所应为", "jail-guard": "要传话给巫后娘娘",
	"queen-prison": "您的冤情谁来替您洗清", "staff-quest": "请你先替我取回我的天蛇杖",
	"lotus-password": "长老应该不会为难你的", "stone-allows": "娘娘的蛇杖就放在此地下",
	"staff": "取得天蛇杖", "staff-delivered": "我拿到天蛇杖了",
	"queen-love": "我仍然像当初一样爱着他", "king-distrust": "你真的要背叛我吗",
	"stand-with-queen": "你们想动她，先过我这一关", "water-plot": "你竟然真的在培养邪魔兽",
	"water-revives": "水魔兽遇水则生", "last-stand": "可恶～无路可逃了",
	"queen-entrusts": "请你能代我好好照顾她", "queen-sacrifice": "与我一同化为尘土吧",
	"queen-gone": "娘娘．．．？！",
}

var templePastSteps = []struct {
	Scene, Event, Dest int
	Token              string
	Party              []int
	Search             bool
}{
	{207, 3462, 212, "", []int{4, 0}, false},
	{212, 3582, 204, "", []int{4, 0}, false},
	{204, 3405, 247, "past-entry", []int{0}, false},
	{247, 4351, 247, "disguise", []int{0}, false},
	{247, 4365, 255, "boat", []int{0}, false},
	{255, 4466, 249, "", []int{0}, false},
	{249, 4394, 249, "stone-protests", []int{0}, false},
	{249, 4371, 256, "", []int{0}, false},
	{256, 4487, 257, "", []int{0}, false},
	{257, 4531, 248, "jail-guard", []int{0}, false},
	{248, 4368, 248, "lotus-password", []int{0}, false},
	{248, 4366, 257, "", []int{0}, false},
	{257, 4529, 256, "", []int{0}, false},
	{256, 4488, 258, "", []int{0}, false},
	{258, 4551, 250, "", []int{0}, false},
	{250, 4396, 262, "", []int{0}, false},
	{262, 4563, 263, "", []int{0}, false},
	{263, 4569, 263, "stone-allows", []int{0}, true},
	{263, 4568, 264, "", []int{0}, false},
	{264, 4588, 266, "", []int{0}, false},
	{266, 4649, 266, "staff", []int{0}, true},
	{266, 4647, 264, "", []int{0}, false},
	{264, 4587, 263, "", []int{0}, false},
	{263, 4567, 262, "", []int{0}, false},
	{262, 4562, 250, "", []int{0}, false},
	{250, 4395, 258, "", []int{0}, false},
	{258, 4550, 256, "", []int{0}, false},
	{256, 4487, 257, "", []int{0}, false},
	{257, 4531, 248, "", []int{0}, false},
	{248, 4368, 254, "queen-love", []int{0, 3}, true},
	{254, 4464, 253, "stand-with-queen", []int{3, 0}, false},
	{253, 4443, 253, "water-revives", []int{3, 0}, false},
	{253, 4440, 251, "", []int{3, 0}, false},
	{251, 4412, 267, "queen-gone", []int{0}, false},
}

func templePastStairs(t *testing.T, tables *resources.Tables) map[int]map[[2]int][2]int {
	t.Helper()
	moves := map[int]map[[2]int][2]int{204: {}}
	rows := []map[string]any{}
	for i := tables.Scenes[203].FirstEvent; i < tables.Scenes[204].FirstEvent; i++ {
		e := tables.Events[i]
		if e.State <= 0 || e.Sprite != 0 || e.TriggerMode < 4 || e.TriggerScript == 0 {
			continue
		}
		delta, supported, done := [2]int{}, true, false
		for pc := int(e.TriggerScript); pc < len(tables.Scripts) && pc < int(e.TriggerScript)+80; pc++ {
			s := tables.Scripts[pc]
			if s.Opcode == 0 {
				done = true
				break
			}
			switch s.Opcode {
			case 0x15, 0x9:
			case 0x6e:
				delta[0] += int(int16(s.Operands[0]))
				delta[1] += int(int16(s.Operands[1]))
			default:
				supported = false
			}
			if !supported {
				break
			}
		}
		if !supported || !done || delta == [2]int{} {
			continue
		}
		entry := [2]int{int(e.X), int(e.Y)}
		areas := map[[2]int][2]int{}
		radius := (int(e.TriggerMode)-4)*32 + 16
		sacredTreeTouchArea(areas, entry, [2]int{}, radius)
		for at := range areas {
			if _, exists := moves[204][at]; !exists {
				moves[204][at] = [2]int{at[0] + delta[0], at[1] + delta[1]}
			}
		}
		rows = append(rows, map[string]any{"scene": 204, "event": int(i) + 1, "script": e.TriggerScript, "entry": entry, "radius": radius, "originalRelativeMovement": delta})
	}
	if len(rows) != 10 {
		t.Fatal("original temple stair reference is incomplete", len(rows))
	}
	if err := os.WriteFile(campaignEvidencePath("temple-past-stair-reference.json", 1), raw(map[string]any{"scope": "test-only original relative stair trigger areas; ordinary movement triggers all native displacement; no production bot or native coordinate writes", "stairs": rows}), 0600); err != nil {
		t.Fatal(err)
	}
	return moves
}

func TestOriginalTemplePastPalaceStaffQueenAndWaterBeastSave(t *testing.T) {
	if os.Getenv("PAL_TEMPLE_PAST") != "1" {
		t.Skip("explicit actual temple and ten-years-ago full-story acceptance")
	}
	name := "campaign-dali-kirin-state.json"
	root, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-dali-kirin-state.json", root, "bee94e72df9c30d16baefe276ddd14f8c907b93b3daeee9804ad6dcac72d2c2d"); err != nil {
		t.Fatal(err)
	}
	checkpoint := os.Getenv("PAL_TEMPLE_PAST_CHECKPOINT")
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := templePastChecks{PartyOrders: map[string][]int{}, ChasePositions: map[string][]int{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 0 || n > len(templePastSteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported temple debug checkpoint", checkpoint)
			}
		}
		name = "temple-past-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("temple-past-"+checkpoint+"-"+kind+".json", 1))
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
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "temple-past", proof: p, tables: tables, milestones: templePastReadings,
		allowedRoles: map[int]bool{0: true, 3: true, 4: true}, reserveScenes: []int{207, 212, 204, 247, 255, 249, 256, 257, 248, 258, 250, 262, 263, 264, 266, 254, 253, 251, 252}}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	r.w.sameSceneMoves = templePastStairs(t, tables)
	settled, spellScans := 0, 0
	r.manualBattle = func(v pal.View) ([]byte, bool) {
		b := v.Battle
		if b == nil {
			return nil, false
		}
		water := false
		for _, enemy := range b.Enemies {
			water = water || enemy.Object == 547
		}
		if !water {
			return nil, false
		}
		if b.Phase != 0 || b.State == 0 || v.Seq != v.AppliedSeq {
			return nil, true
		}
		if settled > 0 {
			settled--
			return nil, true
		}
		action, ok := caiyiForestPhysicalAttack(v)
		if !ok {
			return nil, true
		}
		spell, cost := "天剑", 18
		if b.Current >= 0 && b.Current < len(v.Party) && v.Party[b.Current].Role == 3 {
			spell, cost = "雷神", 68
		}
		if b.Current >= 0 && b.Current < len(v.Party) && v.Party[b.Current].MP >= cost {
			if b.State == 1 && b.Menu == 0 {
				if b.Selected != 1 {
					action.Direction, action.Keys = 1, 8
				} else {
					action.Direction, action.Keys = 4, 2
				}
			} else if b.State == 1 && b.Menu == 1 {
				if originalVisibleText(v, true) == spell {
					action.Direction, action.Keys = 4, 2
					spellScans = 0
				} else {
					spellScans++
					if spellScans > 64 {
						t.Fatal("original player spell unavailable", originalVisibleText(v, true), originalVisibleText(v, false))
					}
					action.Direction, action.Keys = 3, 32
				}
			} else if (b.State == 2 || b.State == 4) && b.Action == 3 {
				action.Keys = 2
			}
		}
		settled = 2
		return raw(action), true
	}

	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("temple-past-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	observations := 0
	r.notice = func(v pal.View) {
		observations++
		roles := []int{}
		for _, member := range v.Party {
			roles = append(roles, member.Role)
		}
		valid := false
		for _, expected := range [][]int{{4, 0}, {0}, {0, 3}, {3, 0}, {3}} {
			valid = valid || bytes.Equal(raw(roles), raw(expected))
		}
		if !valid {
			t.Fatal("original past temporary party changed", r.proof.Step, v.Scene, roles)
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
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && (actor[1] == 4441 || actor[1] == 4410) {
					at := fmt.Sprint(v.Scene, ":", actor[3], ":", actor[4])
					if len(checks.ChasePositions) < 16 {
						checks.ChasePositions[at] = []int{v.Scene, actor[3], actor[4]}
					}
					if !r.proof.Saved["water-chase"] {
						save("water-chase")
					}
				}
			}
		}
		for _, draw := range v.Draws {
			if v.Scene == 247 && draw.Kind == 0 && draw.A == 563 && draw.C == 0 && !checks.DisguiseSaved {
				checks.DisguiseSaved = true
				save("disguise-active")
			}
		}
		if v.Scene == 252 && v.Phase == "script" && !r.proof.Saved["sacrifice-flight"] {
			save("sacrifice-flight")
		}
		if b := v.Battle; b != nil {
			r.battlePartySize = len(roles)
			alive := false
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive {
				t.Fatal("original past battle defeated the whole party", v.Scene, v.Party)
			}
			if !r.proof.InBattle && v.Scene == 247 {
				checks.RescueBattles++
			}
			for _, enemy := range b.Enemies {
				if enemy.Object != 547 {
					continue
				}
				if !bytes.Equal(raw(roles), raw([]int{3, 0})) || v.Scene != 253 && v.Scene != 251 {
					t.Fatal("original water beast battle party or scene changed", v.Scene, roles)
				}
				number := r.proof.Battles
				if !r.proof.InBattle {
					number++
				}
				if checks.LastWaterBattle != number {
					checks.LastWaterBattle = number
					checks.WaterBattles++
				}
				label := fmt.Sprint("water-accepted-", number)
				for _, member := range b.Players {
					if member.State == 2 && !r.proof.Saved[label] {
						checks.WaterAcceptedSaves++
						save(label)
					}
				}
			}
			if observations%1000 == 0 {
				t.Log("temple past public battle", observations, r.proof.Step, v.Scene, v.Party, b)
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("temple-past-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	items := []string{"天蛇杖", "火灵珠", "火眼麒麟角", "风灵珠", "水灵珠", "金凤凰蛋壳"}
	if checkpoint == "" {
		sacredTreeField(r)
		checks.InventoryBefore = r.inventoryCounts(items...)
		if checks.InventoryBefore["天蛇杖"] != 0 || checks.InventoryBefore["水灵珠"] != 0 || checks.InventoryBefore["金凤凰蛋壳"] != 0 || checks.InventoryBefore["火灵珠"] != 1 || checks.InventoryBefore["风灵珠"] != 1 || checks.InventoryBefore["火眼麒麟角"] != 1 {
			t.Fatal("original temple entry sacred inventory changed", checks.InventoryBefore)
		}
		save("step-0")
	}
	for r.proof.Step < len(templePastSteps) {
		q := templePastSteps[r.proof.Step]
		t.Log("original temple past waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[3]int]int{}
		for i := 0; i < 8000; i++ {
			v := sacredTreeField(r)
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if q.Event == 4412 && v.Scene == 252 {
				found := false
				for _, actor := range v.Field.Actors {
					if actor[0] == 1 && actor[1] == 4439 {
						r.g = r.w.walk(t, r.m, &r.c, r.g, v, actor[3], actor[4], 0)
						found = true
						break
					}
				}
				if !found {
					r.keys(0, 4)
				}
				r.progress()
				continue
			}
			if v.Scene != q.Scene {
				t.Fatal("original temple past route entered an unexpected field", q, v.Scene, v.X, v.Y)
			}
			at := [3]int{v.Scene, v.X, v.Y}
			visits[at]++
			if visits[at] > 50 {
				t.Fatal("original temple past approach blocked", q, at)
			}
			x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
			searchReach := 96
			if q.Event == 4368 && q.Search {
				searchReach = 32
			}
			if q.Search && fieldTestDistance(v.X, v.Y, x, y) <= searchReach {
				r.search(q.Event, x, y)
			} else {
				reach := 0
				if q.Search {
					reach = searchReach
				}
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, reach)
			}
			r.progress()
		}
		v := sacredTreeField(r)
		roles := []int{}
		for _, member := range v.Party {
			roles = append(roles, member.Role)
		}
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] || !bytes.Equal(raw(roles), raw(q.Party)) {
			t.Fatal("original temple past waypoint or party incomplete", q, v.Scene, roles)
		}
		if q.Event == 4649 {
			checks.InventoryWithStaff = r.inventoryCounts(items...)
			if checks.InventoryWithStaff["天蛇杖"] != 1 {
				t.Fatal("original underground pedestal did not grant one staff", checks.InventoryWithStaff)
			}
			save("staff-collected")
			for checks.StaffRepeatInputs < 3 {
				r.keys(2, 4)
				r.progress()
				sacredTreeField(r)
				checks.StaffRepeatInputs++
			}
			if !bytes.Equal(raw(checks.InventoryWithStaff), raw(r.inventoryCounts(items...))) {
				t.Fatal("original staff duplicated after repeat confirmation")
			}
			save("staff-no-duplication")
		}
		if q.Dest == 254 && r.inventoryCounts(items...)["天蛇杖"] != 0 {
			t.Fatal("original staff was not consumed on delivery")
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := sacredTreeField(r)
	checks.InventoryAfter = r.inventoryCounts(items...)
	if !bytes.Equal(raw(checks.InventoryAfter), raw(checks.InventoryBefore)) {
		t.Fatal("original past entry to Yuhang changed sacred inventory or retained the delivered staff", checks.InventoryBefore, checks.InventoryAfter)
	}
	for name := range templePastReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original temple past milestone missing", name, r.proof.Seen)
		}
	}
	if v.Scene != 267 || v.Map != 5 || len(v.Party) != 1 || v.Party[0].Role != 0 || r.proof.Battles != r.proof.Wins || checks.RescueBattles != 2 || !checks.DisguiseSaved || checks.StaffRepeatInputs != 3 || checks.WaterBattles < 1 || checks.WaterAcceptedSaves < 1 || len(checks.PartyOrders) != 5 || len(checks.ChasePositions) < 2 || !r.proof.Saved["water-chase"] || !r.proof.Saved["sacrifice-flight"] {
		t.Fatal("original temple and past ending incomplete", v.Scene, v.Map, r.proof, checks)
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "waterBattleInputPolicy": "test-only ordinary queen thunder and Li sky-sword menu selections, physical attacks when mana is insufficient; no native edits", "scope": "original temple and past transition, grandmother/princess rescue, disguise/boat, palace, underground staff and actual delivery, queen joining/order, water beast/field revival chase, queen sacrifice and phoenix ride to old Yuhang; ordinary inputs and fresh-runtime saves, no native edits or premature water orb/egg grant", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "rootStage19SHA256": hex.EncodeToString(rootHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-temple-past.json": report, "campaign-temple-past-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
