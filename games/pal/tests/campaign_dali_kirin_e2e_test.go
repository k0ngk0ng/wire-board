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

type daliKirinChecks struct {
	InventoryBefore, InventoryAfter map[string]int
	PartyOrders                     map[string][]int
	KirinObserved                   bool
	KirinAcceptedSaved              bool
	RewardOrder                     []string
	RepeatInputs                    int
}

var daliKirinReadings = map[string]string{
	"gate-denied": "没有族长的准许，不得擅入",
	"li-waits":    "你先在这里等我一下", "anu-home": "我回来了",
	"war-council": "对于刚才所订定的策略", "poison-beasts": "使用五毒兽来作战",
	"cave-request": "人家是想进麒麟洞一趟", "water-stolen": "水灵珠自从十二年前被盗后",
	"war-cost": "没有人愿意战争", "prisoner": "鬼鬼祟祟的在洞口附近窥探",
	"reunited": "有了！快跟我来", "cave-plan": "差不多啦，跟我走就对了",
	"guard-lured":   "这里我暂时替你守着",
	"kirin-awakens": "吵死啦", "fire-orb": "获得火灵珠", "human-form": "它变成人了耶",
	"ling-lineage": "晚辈的妻子乃女娲族裔", "kirin-horn": "获得火眼麒麟角",
	"sacred-medicine": "能帮助她们提早脱胎换骨", "temple-instruction": "务必到女娲神殿去一趟",
}

var daliKirinSteps = []struct {
	Scene, Event, Dest int
	Token              string
	Party              []int
	Search             bool
}{
	{180, 2955, 203, "", []int{0, 4}, false},
	{203, 3384, 207, "", []int{0, 4}, false},
	{207, 3472, 207, "gate-denied", []int{0, 4}, true},
	{207, 3461, 206, "", []int{0, 4}, false},
	{206, 3442, 206, "war-cost", []int{4}, false},
	{206, 3443, 207, "cave-plan", []int{4, 0}, false},
	{207, 3471, 213, "", []int{4, 0}, false},
	{213, 3586, 202, "", []int{4, 0}, false},
	{202, 3354, 202, "temple-instruction", []int{4, 0}, true},
}

func TestOriginalDaliMeetingKirinSacredRewardsAndSave(t *testing.T) {
	if os.Getenv("PAL_DALI_KIRIN") != "1" {
		t.Skip("explicit actual Dali and Kirin full-story acceptance")
	}
	root, err := os.ReadFile(campaignEvidencePath("campaign-sacred-tree-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-sacred-tree-state.json", root, "830bc1cd77291a24db93a142da10f98ca2fffdf292344702d55084421a59a96d"); err != nil {
		t.Fatal(err)
	}
	name := "campaign-sacred-tree-state.json"
	checkpoint := os.Getenv("PAL_DALI_KIRIN_CHECKPOINT")
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := daliKirinChecks{PartyOrders: map[string][]int{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 0 || n > len(daliKirinSteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported Dali debug checkpoint", checkpoint)
			}
		}
		name = "dali-kirin-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("dali-kirin-"+checkpoint+"-"+kind+".json", 1))
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
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "dali-kirin", proof: p, tables: tables, milestones: daliKirinReadings,
		allowedRoles: map[int]bool{0: true, 4: true}, reserveScenes: []int{180, 203, 207, 206, 213, 202}}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("dali-kirin-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
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
		for _, expected := range [][]int{{0, 4}, {4}, {4, 0}, {0}} {
			valid = valid || bytes.Equal(raw(roles), raw(expected))
		}
		if !valid {
			t.Fatal("original Dali temporary party changed", r.proof.Step, v.Scene, roles)
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
		text := hiddenNoticeText(v) + originalVisibleText(v, false)
		if v.Dialogue != nil {
			text += v.Dialogue.Text
		}
		for _, reward := range []string{"获得火灵珠", "获得火眼麒麟角"} {
			if strings.Contains(text, reward) {
				seen := false
				for _, prior := range checks.RewardOrder {
					seen = seen || prior == reward
				}
				if !seen {
					checks.RewardOrder = append(checks.RewardOrder, reward)
					save(fmt.Sprint("sacred-reward-", len(checks.RewardOrder)))
				}
			}
		}
		if b := v.Battle; b != nil {
			if !bytes.Equal(raw(roles), raw([]int{4, 0})) {
				t.Fatal("original Kirin cave battle party changed", v.Scene, roles)
			}
			alive := false
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive {
				t.Fatal("original Kirin cave battle defeated the whole party", v.Party)
			}
			for _, enemy := range b.Enemies {
				if enemy.Object == 463 {
					if v.Scene != 202 {
						t.Fatal("original Kirin moved to another scene", v.Scene)
					}
					checks.KirinObserved = true
					for _, member := range b.Players {
						if member.State == 2 && !checks.KirinAcceptedSaved {
							checks.KirinAcceptedSaved = true
							save("kirin-accepted")
						}
					}
				}
			}
			if observations%500 == 0 {
				t.Log("Dali public battle", observations, r.proof.Step, v.Party, b)
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("dali-kirin-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	if checkpoint == "" {
		sacredTreeField(r)
		checks.InventoryBefore = r.inventoryCounts("火灵珠", "火眼麒麟角", "金凤凰蛋壳", "水灵珠", "风灵珠")
		if checks.InventoryBefore["火灵珠"] != 0 || checks.InventoryBefore["火眼麒麟角"] != 0 || checks.InventoryBefore["水灵珠"] != 0 || checks.InventoryBefore["金凤凰蛋壳"] != 0 || checks.InventoryBefore["风灵珠"] != 1 {
			t.Fatal("original Dali entry sacred inventory changed", checks.InventoryBefore)
		}
		save("step-0")
	}
	for r.proof.Step < len(daliKirinSteps) {
		q := daliKirinSteps[r.proof.Step]
		t.Log("original Dali waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 5000; i++ {
			v := sacredTreeField(r)
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("original Dali route entered an unexpected field", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 50 {
				t.Fatal("original Dali approach blocked", q, at)
			}
			x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
			if q.Search && fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				r.search(q.Event, x, y)
			} else {
				reach := 0
				if q.Search {
					reach = 96
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
			t.Fatal("original Dali waypoint or party incomplete", q, v.Scene, roles)
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	checks.InventoryAfter = r.inventoryCounts("火灵珠", "火眼麒麟角", "金凤凰蛋壳", "水灵珠", "风灵珠")
	if checks.InventoryAfter["火灵珠"] != 1 || checks.InventoryAfter["火眼麒麟角"] != 1 || checks.InventoryAfter["水灵珠"] != 0 || checks.InventoryAfter["金凤凰蛋壳"] != 0 || checks.InventoryAfter["风灵珠"] != 1 {
		t.Fatal("original Kirin sacred rewards incorrect", checks.InventoryAfter)
	}
	save("sacred-items-collected")
	stance := sacredTreeField(r)
	delta := [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}}[stance.Field.Direction]
	frontX, frontY := stance.X+delta[0], stance.Y+delta[1]
	beast := tables.Events[3353]
	if frontX/32 != int(beast.X)/32 || frontY/16 != int(beast.Y)/16 || frontX%32 != int(beast.X)%32 {
		t.Fatal("repeat confirmation no longer faces the original Kirin interaction", stance.X, stance.Y, stance.Field.Direction)
	}
	battles, readings := r.proof.Battles, r.proof.Readings
	for checks.RepeatInputs < 3 {
		r.keys(2, 4)
		r.progress()
		sacredTreeField(r)
		checks.RepeatInputs++
		save(fmt.Sprint("ended-kirin-confirm-", checks.RepeatInputs))
	}
	if battles != r.proof.Battles || readings != r.proof.Readings || !bytes.Equal(raw(checks.InventoryAfter), raw(r.inventoryCounts("火灵珠", "火眼麒麟角", "金凤凰蛋壳", "水灵珠", "风灵珠"))) {
		t.Fatal("ended original Kirin interaction repeated its story, battle or rewards")
	}
	for _, q := range []struct{ Scene, Event, Dest int }{{202, 3353, 213}, {213, 3587, 207}} {
		r.reserve(q.Event)
		for i := 0; i < 3000 && sacredTreeField(r).Scene != q.Dest; i++ {
			v := sacredTreeField(r)
			if v.Scene != q.Scene {
				t.Fatal("original Kirin return entered an unexpected scene", v.Scene, q)
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, xFrom(tables, q.Event), yFrom(tables, q.Event), 0)
			r.progress()
		}
		save(fmt.Sprint("returned-", q.Dest))
	}
	v := sacredTreeField(r)
	finalRoles := []int{}
	for _, member := range v.Party {
		finalRoles = append(finalRoles, member.Role)
	}
	if !bytes.Equal(raw(finalRoles), raw([]int{4, 0})) || !bytes.Equal(raw(checks.InventoryAfter), raw(r.inventoryCounts("火灵珠", "火眼麒麟角", "金凤凰蛋壳", "水灵珠", "风灵珠"))) {
		t.Fatal("original Dali return changed earned sacred rewards or party", finalRoles)
	}
	for name := range daliKirinReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original Dali milestone missing", name, r.proof.Seen)
		}
	}
	if v.Scene != 207 || v.Map != 163 || !checks.KirinObserved || !checks.KirinAcceptedSaved || r.proof.Battles == 0 || r.proof.Battles != r.proof.Wins || !bytes.Equal(raw(checks.RewardOrder), raw([]string{"获得火灵珠", "获得火眼麒麟角"})) || len(checks.PartyOrders) != 4 {
		t.Fatal("original Dali and Kirin ending incomplete", v.Scene, v.Map, r.proof, checks)
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "original Dali guard, Anu-only war council, reunion, Kirin cave and ordinary victory, fire orb/horn rewards, temple instruction, original party changes and fresh-runtime saves; no native edits or premature sacred egg/water orb grant", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "rootStage18SHA256": hex.EncodeToString(rootHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-dali-kirin.json": report, "campaign-dali-kirin-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
