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
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type towerUpperChecks struct {
	InventoryBefore, InventoryAfterSword, InventoryAfterBook map[string]int
	RepeatSwordSearches, RepeatBookSearches, RescueChoices   int
	GateFrames                                               map[int]int
	OpenedBridges                                            map[int]bool
}

func TestOriginalTowerSavedEncounterProductionBot(t *testing.T) {
	if os.Getenv("PAL_TOWER_BATTLE_DIAGNOSTIC") != "1" {
		t.Skip("explicit real tower encounter production bot regression")
	}
	source, err := os.ReadFile(campaignEvidencePath("tower-encounter-bot-source-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(source)
	if hex.EncodeToString(sourceHash[:]) != "12b7239709755d5fd03a8a1267a40cf6e81b6dcfd71ea4950767dc68f0510bbd" {
		t.Fatal("retained original tower encounter source changed")
	}
	r := loadRiverMedicine(t, "tower-encounter-bot-source-state.json")
	initial := r.visible()
	if initial.Scene != 148 || initial.Battle == nil || len(initial.Party) != 2 {
		t.Fatal("tower regression source is not the actual original encounter", initial.Scene, initial.Battle)
	}
	trace := []any{}
	defer func() {
		for suffix, data := range map[string][]byte{"trace": raw(trace), "view": raw(r.visible()), "state": r.g.Data} {
			if err := os.WriteFile(campaignEvidencePath("tower-encounter-bot-"+suffix+".json", 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 6000; step++ {
		v := r.visible()
		if v.Battle == nil {
			if v.Cash <= initial.Cash {
				t.Fatal("tower original encounter finished without normal reward", initial.Cash, v.Cash)
			}
			t.Log("tower saved production battle won", step, initial.Cash, v.Cash)
			hash := sha256.Sum256(source)
			report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceSHA256": hex.EncodeToString(hash[:]), "timerObservations": step, "cashBefore": initial.Cash, "cashAfter": v.Cash, "party": v.Party, "scope": "production public-observation bot resumes the actual tower encounter with mutually summoning enemies and reaches the original normal reward; no native-state edits"})
			if err := os.WriteFile(campaignEvidencePath("tower-encounter-bot.json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		a, err := r.m.BotAction(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) > 0 {
			var action pal.Action
			if err := json.Unmarshal(a, &action); err != nil {
				t.Fatal(err)
			}
			if action.Keys != 0 || step%40 == 0 {
				trace = append(trace, map[string]any{"step": step, "keys": action.Keys, "battle": v.Battle, "party": v.Party, "text": originalVisibleText(v, false)})
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("production bot failed to finish the original saved tower encounter")
}

var towerUpperReadings = map[string]string{
	"projection": "吾乃此塔之守护神", "allowed": "汝要进入，我不必阻止",
	"sword": "七星剑", "disciple": "我来代替你师父惩罚你",
	"disciple-freed": "是我害死你们的", "waner": "我是他女儿",
	"waner-origin": "我有一半是妖族的血统", "waner-leaves": "我自有我的生存之道",
	"closed-gate": "一扇厚重的铜门", "jar": "放．．我．．出．．去",
	"ghost-threat": "我的报答，就是吃了你们", "ghost-reason": "这是我们天鬼族的规矩",
	"ghost-help": "到一扇铜门旁边等我", "gate-opened": "后会有期",
	"riddle": "还是先踏出右脚呢", "riddle-frustration": "我在这里想了五百年",
	"sage-reading": "俺在看书，别吵", "sage-question": "问题转述一遍",
	"sage-joins": "你们带我走",
	"sage-bag":   "书中仙自动跳入李逍遥行囊中", "book": "得到天书",
	"answer": "你把两只脚一齐砍下来", "riddle-solved": "然也～然也",
}

func TestOriginalTowerBookNoticeFromSavedBag(t *testing.T) {
	if os.Getenv("PAL_TOWER_BOOK_NOTICE") != "1" {
		t.Skip("explicit original book acquisition notice observation")
	}
	source, err := os.ReadFile(campaignEvidencePath("tower-upper-sage-bag-reading-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	r := loadRiverMedicine(t, "tower-upper-sage-bag-reading-state.json")
	observations := []any{}
	seen := false
	for step := 0; step < 40; step++ {
		v := r.visible()
		text := hiddenNoticeText(v)
		observations = append(observations, map[string]any{"step": step, "phase": v.Phase, "text": text})
		seen = seen || strings.Contains(text, "天书")
		r.progress()
	}
	if err := os.WriteFile(campaignEvidencePath("tower-book-notice-observations.json", 1), raw(observations), 0600); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("original book acquisition notice was not observed after saved bag entry")
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceSHA256": hex.EncodeToString(hash[:]), "timerObservations": len(observations), "originalChineseNotice": "得到天书"})
	if err := os.WriteFile(campaignEvidencePath("tower-book-notice.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}

var towerUpperSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{147, 2405, 147, "allowed"}, {147, 2404, 166, ""}, {166, 2710, 167, ""},
	{167, 2740, 148, ""}, {148, 2408, 148, "waner-leaves"}, {148, 2407, 168, ""},
	{168, 2773, 149, ""}, {149, 2430, 149, "closed-gate"}, {149, 2434, 149, "ghost-help"},
	{149, 2430, 149, "gate-opened"}, {149, 2428, 155, ""}, {155, 2494, 155, "sage-reading"},
	{155, 2496, 155, "riddle-frustration"}, {155, 2494, 155, "book"},
	{155, 2496, 155, "riddle-solved"}, {155, 2492, 157, ""},
	{157, 2534, 157, "switch-2534"}, {157, 2527, 157, "switch-2527"}, {157, 2541, 157, "switch-2541"},
	{157, 2522, 169, ""}, {169, 2805, 146, ""},
}

func TestOriginalTowerGhostRescueDeclineAndSavedChoice(t *testing.T) {
	if os.Getenv("PAL_TOWER_DECLINE") != "1" {
		t.Skip("explicit actual tower rescue choice branch")
	}
	name := "tower-upper-rescue-choice-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables,
		prefix: "tower-decline", milestones: map[string]string{}, allowedMenus: map[int]bool{12: true},
		proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	initial := r.visible()
	if initial.Scene != 149 || initial.Menu.Kind != 12 || initial.Menu.Selected != 1 {
		t.Fatal("tower decline source is not the actual selected original Yes", initial.Scene, initial.Menu)
	}
	r.keys(8, 1)
	for i := 0; i < 128 && (r.visible().Seq != r.visible().AppliedSeq || r.visible().Menu.Selected != 0); i++ {
		r.progress()
	}
	if v := r.visible(); v.Menu.Kind != 12 || v.Menu.Selected != 0 {
		t.Fatal("original rescue No could not be selected", v.Menu)
	}
	r.save("no-selected")
	r.keys(2, 4)
	v := r.field()
	closed := false
	for _, actor := range v.Field.Actors {
		closed = closed || actor[0] == 1 && actor[1] == 2434 && actor[2] == 506 && actor[5] == 0
	}
	if !closed || v.Cash != initial.Cash || !bytes.Equal(raw(v.Party), raw(initial.Party)) || r.proof.Battles != 0 {
		t.Fatal("declining rescue opened the jar or changed resources", closed, v.Cash, v.Party, r.proof)
	}
	r.save("no-returned")
	r.search(2434, 864, 1440)
	for i := 0; i < 128 && (r.visible().Menu.Kind != 12 || r.visible().Seq != r.visible().AppliedSeq); i++ {
		r.progress()
	}
	if v := r.visible(); v.Menu.Kind != 12 || v.Menu.Selected != 0 {
		t.Fatal("original rescue question could not be asked again after saved refusal", v.Menu)
	}
	r.save("asked-again")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceSHA256": hex.EncodeToString(hash[:]), "freshRuntimeSaves": r.proof.Saves, "closedJarAfterRefusal": closed, "cashUnchanged": v.Cash == initial.Cash, "scope": "actual original ghost rescue choice; saved No selection, refusal leaves the jar closed and no battle/reward, saved repeated ordinary interaction asks again with default No"})
	if err := os.WriteFile(campaignEvidencePath("tower-decline.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestOriginalTowerSwordWanerGhostSageRiddleAndSave(t *testing.T) {
	checkpoint := os.Getenv("PAL_TOWER_UPPER_CHECKPOINT")
	name := "campaign-shu-tower-state.json"
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := towerUpperChecks{GateFrames: map[int]int{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n <= 0 || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported upper tower test checkpoint", checkpoint)
			}
		}
		name = "tower-upper-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("tower-upper-"+checkpoint+"-"+kind+".json", 1))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatal(err)
			}
		}
	}
	if checks.OpenedBridges == nil {
		checks.OpenedBridges = map[int]bool{}
	}
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables, proof: p,
		prefix: "tower-upper", milestones: towerUpperReadings, reserveScenes: []int{147, 166, 167, 148, 168, 149, 155, 157, 169, 146},
		scriptedEnemies: map[int]bool{529: true},
		allowedMenus:    map[int]bool{3: true, 5: true, 10: true, 12: true, 2: true}}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("tower-upper-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.notice = func(v pal.View) {
		if v.Field != nil {
			for _, a := range v.Field.Actors {
				if a[0] == 1 && (a[1] == 2429 || a[2] == 271) && a[5] > checks.GateFrames[a[1]] {
					checks.GateFrames[a[1]] = a[5]
				}
				if a[0] == 1 && a[2] == 271 && a[5] == 1 {
					label := fmt.Sprint("switch-", a[1])
					if !r.proof.Seen[label] {
						r.proof.Seen[label] = true
						save(label)
					}
				}
				if a[0] == 1 && a[2] == 298 && a[5] == 0 && !checks.OpenedBridges[a[1]] {
					checks.OpenedBridges[a[1]] = true
					save(fmt.Sprint("bridge-open-", a[1]))
				}
			}
		}
		if v.Phase == "menu" && v.Menu.Kind == 12 && v.Seq == v.AppliedSeq {
			yes := -1
			for i, item := range v.Menu.Items {
				if item.Label == "是" {
					yes = i
				}
			}
			if v.Scene != 149 || yes < 0 {
				t.Fatal("unexpected upper tower menu", v.Scene, v.Menu)
			}
			if v.Menu.Selected != yes {
				r.keys(8, 1)
			} else {
				if !r.proof.Saved["rescue-choice"] {
					save("rescue-choice")
				}
				checks.RescueChoices++
				r.keys(2, 4)
			}
		}
		if b := v.Battle; b != nil {
			for _, member := range b.Players {
				if member.State == 2 && !r.proof.Saved[fmt.Sprint("accepted-battle-", r.proof.Battles)] {
					save(fmt.Sprint("accepted-battle-", r.proof.Battles))
				}
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("tower-upper-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	items := []string{"七星剑", "天书"}
	if checkpoint == "" {
		v := r.field()
		if v.Scene != 147 || v.Map != 141 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 {
			t.Fatal("upper tower source is not the actual earned tower entry", v.Scene, v.Party)
		}
		checks.InventoryBefore = r.inventoryCounts(items...)
		if checks.InventoryBefore["七星剑"] != 0 || checks.InventoryBefore["天书"] != 0 {
			t.Fatal("tower quest items were already present before their original events", checks.InventoryBefore)
		}
		save("before")
	}
	for r.proof.Step < len(towerUpperSteps) {
		q := towerUpperSteps[r.proof.Step]
		t.Log("upper tower waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 3200; i++ {
			v := r.field()
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("upper tower ordinary route entered an unexpected scene", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 40 {
				t.Fatal("upper tower ordinary stance blocked", q, at)
			}
			e := tables.Events[q.Event-1]
			x, y := int(e.X), int(e.Y)
			for _, a := range v.Field.Actors {
				if a[0] == 1 && a[1] == q.Event {
					x, y = a[3], a[4]
				}
			}
			if e.TriggerMode > 0 && e.TriggerMode < 4 && fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				r.search(q.Event, x, y)
			} else {
				reach := 0
				if e.TriggerMode > 0 && e.TriggerMode < 4 {
					reach = 96
				}
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, reach)
			}
			r.progress()
			if i%200 == 0 {
				t.Log("upper tower walk", i, v.Scene, v.X, v.Y, x, y)
			}
		}
		v := r.field()
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("upper tower ordinary waypoint incomplete", q, v.Scene, v.X, v.Y)
		}
		if q.Token == "waner-leaves" || q.Token == "book" {
			after := r.inventoryCounts(items...)
			item, retries := "七星剑", &checks.RepeatSwordSearches
			if q.Token == "book" {
				checks.InventoryAfterBook = after
				item, retries = "天书", &checks.RepeatBookSearches
			} else {
				checks.InventoryAfterSword = after
			}
			if after[item] != checks.InventoryBefore[item]+1 {
				t.Fatal("original upper tower item quantity changed", item, checks.InventoryBefore, after)
			}
			save(item + "-verified")
			for repeat := 0; repeat < 3; repeat++ {
				r.search(q.Event, int(tables.Events[q.Event-1].X), int(tables.Events[q.Event-1].Y))
				r.progress()
				r.field()
				*retries++
			}
			if !bytes.Equal(raw(after), raw(r.inventoryCounts(items...))) {
				t.Fatal("original upper tower item duplicated after saved repeated searches", item)
			}
			save(item + "-repeated")
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := r.field()
	for name := range towerUpperReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original upper tower milestone absent", name, r.proof.Seen)
		}
	}
	for _, scene := range []int{147, 166, 167, 148, 168, 149, 155, 157, 169, 146} {
		if r.proof.Scenes[scene] == 0 {
			t.Fatal("original upper tower scene never observed", scene)
		}
	}
	final := r.inventoryCounts(items...)
	for _, event := range []int{2523, 2524, 2528, 2529, 2530, 2531, 2535, 2536, 2537, 2538} {
		if !checks.OpenedBridges[event] {
			t.Fatal("original tower bridge did not reach its open frame", event, checks.OpenedBridges)
		}
	}
	if v.Scene != 146 || v.Map != 74 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 || r.proof.Battles != r.proof.Wins+1 || r.proof.ScriptedFinishes != 1 || !r.proof.Saved["selected-529"] || !r.proof.Saved["scripted-finish-529"] || checks.RescueChoices != 1 || checks.RepeatSwordSearches != 3 || checks.RepeatBookSearches != 3 || checks.GateFrames[2429] != 6 || final["七星剑"] != 1 || final["天书"] != 1 {
		t.Fatal("original upper tower ending incomplete", v.Scene, v.Map, v.Party, r.proof, checks, final)
	}
	save("bottom-approach")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "original upper demon tower route, projection, seven-star sword, dead Shu disciple and Jiang Waner, ghost king rescue/battle/copper gate, sage and pensive ghost riddle, original map exits and bottom approach; ordinary inputs and fresh-runtime saves, no native scene/script/stats edits", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "finalItems": final, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-tower-upper-state.json": r.g.Data, "campaign-tower-upper.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
