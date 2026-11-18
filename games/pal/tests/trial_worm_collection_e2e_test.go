package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type trialWormChecks struct {
	Phase, RouteStep, ReturnStep, Escapes, RepeatInputs int
	Before, Collected, After                            map[string]int
	Portals, VisitedCandidates                          []int
	ParentSHA256                                        string
}

type trialWormRoute struct {
	Kind               string
	Scene, Event, X, Y int
	DestinationScene   int
	Destination        [2]int
	Search             bool
}

func trialWormInventory(r *caiyiForestDriver, selection string, names ...string) map[string]int {
	r.t.Helper()
	for attempt := 0; attempt < 16; attempt++ {
		sacredTreeField(r)
		r.keys(2048, 4)
		ready := false
		for frame := 0; frame < 128; frame++ {
			v := r.visible()
			if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
				ready = true
				break
			}
			if v.Battle != nil || v.Dialogue != nil || v.Phase == "script" {
				sacredTreeField(r)
				break
			}
			r.progress()
		}
		if !ready {
			continue
		}
		r.keys(65536, 4)
		r.progress()
		r.progress()
		amounts, previous := map[string]int{}, ""
		for page := 0; page < 128; page++ {
			v := r.visible()
			for _, name := range names {
				if n, ok := campaignInventoryAmount(v, name); ok {
					amounts[name] = n
				}
			}
			if selection != "" && strings.Contains(originalVisibleText(v, true), selection) {
				if amounts[selection] != 1 {
					r.t.Fatal("original selected trial item missing or duplicated", selection, amounts)
				}
				return amounts
			}
			text := originalVisibleText(v, false)
			if selection == "" && text == previous {
				break
			}
			previous = text
			keys := uint32(128)
			if selection != "" {
				keys = 32
			}
			r.keys(keys, 4)
			r.progress()
			r.progress()
		}
		if selection != "" {
			r.t.Fatal("original trial item was not selectable", selection)
		}
		r.keys(1, 4)
		sacredTreeField(r)
		return amounts
	}
	r.t.Fatal("trial inventory did not open after original encounter interruptions")
	return nil
}

func TestOriginalTrialCaveThirtySixWormsDeliveryAndRecoverySave(t *testing.T) {
	if os.Getenv("PAL_TRIAL_WORMS") != "1" {
		t.Skip("explicit original trial-cave worm collection and actual delivery")
	}
	parentBytes, err := os.ReadFile(campaignEvidencePath("sacred-medicine-trial-entry.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	var parent struct {
		Passed, ResumedTestRoute bool
		RootStage21SHA256        string
		Party                    []pal.PartyMember
		Cash                     uint32
	}
	rootStage21, err := os.ReadFile(campaignEvidencePath("campaign-old-yuhang-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptedCampaignRoot("campaign-old-yuhang-state.json", rootStage21, "b191bb4254cfd8bbb9a51459d5b9da8c3a78b7aee67dca5669f0c0268f0e9ce3"); err != nil {
		t.Fatal(err)
	}
	rootStage21Hash := sha256.Sum256(rootStage21)
	if err := json.Unmarshal(parentBytes, &parent); err != nil || !parent.Passed || parent.ResumedTestRoute || parent.RootStage21SHA256 != hex.EncodeToString(rootStage21Hash[:]) {
		t.Fatal("worm route requires the unresumed actual medicine and birth route", err, parent)
	}
	name := "sacred-medicine-trial-entry-state.json"
	parentState, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptedCampaignRoot("sacred-medicine-trial-entry-state.json", parentState, "21fb731fd3128ac89dbf8b3525bde41c74e3fb594afac01e041b6f574f79a5c8"); err != nil {
		t.Fatal(err)
	}
	parentHash := sha256.Sum256(parentState)
	checks := trialWormChecks{ParentSHA256: hex.EncodeToString(parentHash[:])}
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checkpoint := os.Getenv("PAL_TRIAL_WORMS_CHECKPOINT")
	if checkpoint != "" {
		if checkpoint != "failure" {
			t.Fatal("unsupported worm route checkpoint", checkpoint)
		}
		name = "trial-worm-failure-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("trial-worm-failure-"+kind+".json", 1))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatal(err)
			}
		}
		if checks.ParentSHA256 != hex.EncodeToString(parentHash[:]) {
			t.Fatal("worm debug checkpoint belongs to another trial entry")
		}
	}
	planBytes, err := os.ReadFile("testdata/trial-worm-route.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		SSSSHA256, DataSHA256 string
		Steps                 []trialWormRoute
	}
	if err := json.Unmarshal(planBytes, &plan); err != nil || len(plan.Steps) < 30 {
		t.Fatal("original worm route plan is incomplete", err)
	}
	for name, expected := range map[string]string{"sss.mkf": plan.SSSSHA256, "data.mkf": plan.DataSHA256} {
		data, err := os.ReadFile("../../../.local/pal/original-98/" + name)
		hash := sha256.Sum256(data)
		if err != nil || hex.EncodeToString(hash[:]) != expected {
			t.Fatal("worm plan original source changed", name, err)
		}
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "trial-worm", proof: p, tables: tables, allowedRoles: map[int]bool{0: true, 1: true, 4: true},
		reserveScenes: []int{216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 215, 203, 180, 187, 177, 173, 175},
		milestones:    map[string]string{"worm-loot": "获得一只傀儡虫", "worms-delivered": "干的不错", "ling-rest": "你不多休养几天再走吗", "ling-mother": "我想去祭拜我娘亲", "sage-hope": "同心协力帮助赵姑娘", "child-stays": "孩子寄放在我这", "ling-ready": "走吧，我们到神殿去"}}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("trial-worm-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.notice = func(v pal.View) {
		if b := v.Battle; b != nil {
			r.battlePartySize = len(v.Party)
			alive := false
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive {
				t.Fatal("ordinary trial cave battle defeated the party", v.Party)
			}
			label := fmt.Sprint("accepted-", r.proof.Battles)
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
				if err := os.WriteFile(campaignEvidencePath("trial-worm-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	items := []string{"傀儡虫", "土灵珠", "水灵珠", "风灵珠", "火灵珠", "火眼麒麟角", "金凤凰蛋壳"}
	if checkpoint == "" {
		v := sacredTreeField(r)
		if v.Scene != 216 || !bytes.Equal(raw(v.Party), raw(parent.Party)) || v.Cash != parent.Cash {
			t.Fatal("worm route source does not match actual parent ending", v.Scene, v.Party)
		}
		checks.Before = trialWormInventory(r, "", items...)
		if checks.Before["傀儡虫"] >= 36 || checks.Before["土灵珠"] != 1 {
			t.Fatal("worm collection entry inventory changed", checks.Before)
		}
		save("before-collection")
	}
	approach := func(q trialWormRoute) {
		t.Helper()
		r.reserve(q.Event)
		visits, seen := map[[2]int]int{}, false
		for i := 0; i < 6000; i++ {
			v := sacredTreeField(r)
			if q.Kind == "portal" && v.Scene == q.DestinationScene && (q.Scene != q.DestinationScene || fieldTestDistance(v.X, v.Y, q.Destination[0], q.Destination[1]) < 48) {
				if q.Destination != [2]int{} && fieldTestDistance(v.X, v.Y, q.Destination[0], q.Destination[1]) >= 48 {
					t.Fatal("original trial portal landing changed", q, v.X, v.Y)
				}
				return
			}
			if v.Scene != q.Scene {
				t.Fatal("trial cave route entered unexpected field", q, v.Scene, v.X, v.Y)
			}
			x, y, visible := q.X, q.Y, false
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					x, y, visible, seen = actor[3], actor[4], true, true
				}
			}
			if q.Kind == "encounter" && !visible && (seen || fieldTestDistance(v.X, v.Y, q.X, q.Y) <= 96) {
				return
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 50 {
				t.Fatal("trial worm route approach blocked", q, at)
			}
			if q.Search && fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				r.search(q.Event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			}
			r.progress()
		}
		t.Fatal("trial worm approach did not complete", q)
	}
	for checks.Phase == 0 {
		amount := trialWormInventory(r, "", items...)["傀儡虫"]
		if amount >= 36 {
			checks.Collected = trialWormInventory(r, "", items...)
			checks.Phase = 1
			save("thirty-six-collected")
			break
		}
		if checks.RouteStep >= len(plan.Steps) {
			t.Fatal("ordinary trial candidate route did not collect enough worms", amount, checks)
		}
		q := plan.Steps[checks.RouteStep]
		t.Log("actual trial worm route", checks.RouteStep, amount, q)
		approach(q)
		if q.Kind == "portal" {
			checks.Portals = append(checks.Portals, q.Event)
		} else {
			checks.VisitedCandidates = append(checks.VisitedCandidates, q.Event)
		}
		checks.RouteStep++
		r.proof.Step = checks.RouteStep
		save(fmt.Sprint("route-", checks.RouteStep))
	}
	for checks.Phase == 1 {
		v := sacredTreeField(r)
		if v.Scene == 215 {
			checks.Phase = 2
			save("escaped-cave")
			break
		}
		if checks.Escapes >= 3 {
			t.Fatal("original earth orb did not escape trial cave", v.Scene)
		}
		trialWormInventory(r, "土灵珠", "土灵珠")
		save(fmt.Sprint("escape-selected-", checks.Escapes))
		r.keys(2, 4)
		r.progress()
		sacredTreeField(r)
		checks.Escapes++
	}
	returns := []trialWormRoute{{Kind: "portal", Scene: 215, Event: 3647, DestinationScene: 203}, {Kind: "portal", Scene: 203, Event: 3383, DestinationScene: 180}, {Kind: "portal", Scene: 180, Event: 2954, DestinationScene: 187}, {Kind: "portal", Scene: 187, Event: 3131, DestinationScene: 177}, {Kind: "portal", Scene: 177, Event: 2917, DestinationScene: 173}, {Kind: "portal", Scene: 173, Event: 2863, DestinationScene: 175, Search: true}}
	for checks.Phase == 2 && checks.ReturnStep < len(returns) {
		q := returns[checks.ReturnStep]
		q.X, q.Y = xFrom(tables, q.Event), yFrom(tables, q.Event)
		t.Log("actual worm delivery return", checks.ReturnStep, q)
		approach(q)
		checks.ReturnStep++
		save(fmt.Sprint("return-", checks.ReturnStep))
	}
	v := sacredTreeField(r)
	checks.After = trialWormInventory(r, "", items...)
	if v.Scene != 175 || v.Map != 202 || len(v.Party) != 3 || v.Party[0].Role != 1 || v.Party[1].Role != 0 || v.Party[2].Role != 4 || checks.After["傀儡虫"] != checks.Collected["傀儡虫"]-36 || r.proof.Battles != r.proof.Wins {
		t.Fatal("actual worm delivery or recovered original party incomplete", v.Scene, v.Party, checks, r.proof)
	}
	for _, name := range []string{"土灵珠", "水灵珠", "风灵珠", "火灵珠"} {
		if checks.Before[name] != 1 || checks.After[name] != 1 {
			t.Fatal("worm collection changed earned sacred orb", name, checks)
		}
	}
	for checks.RepeatInputs < 3 {
		r.keys(2, 4)
		r.progress()
		sacredTreeField(r)
		checks.RepeatInputs++
	}
	if !bytes.Equal(raw(checks.After), raw(trialWormInventory(r, "", items...))) {
		t.Fatal("worm handover repeated after confirmation")
	}
	checks.Phase = 3
	save("delivery-complete")
	for name := range r.milestones {
		if !r.proof.Seen[name] {
			t.Fatal("original post-delivery milestone missing", name)
		}
	}
	planHash := sha256.Sum256(planBytes)
	report := raw(map[string]any{"passed": true, "scope": "ordinary trial cave encounters and actual 36-worm inventory, original selected earth-orb escape, actual delivery consumes exactly 36, recovery dialogue and original Ling/Li/Anu party, fresh-runtime saved actions/readings/item selection; static research is test planning only", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "parentStage22EntryResumedTestRoute": parent.ResumedTestRoute, "parentSHA256": checks.ParentSHA256, "rootStage21SHA256": parent.RootStage21SHA256, "planSHA256": hex.EncodeToString(planHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-sacred-medicine.json": report, "campaign-sacred-medicine-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
