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

type shuTowerChecks struct {
	InventoryBefore, InventoryAfterGifts map[string]int
	LearnedBefore, LearnedAfter          map[string]bool
	RepeatMasterSearches                 int
}

var shuTowerReadings = map[string]string{
	"warning": "在我师兄面前不要叫我师父", "saint": "晚辈李逍遥，拜见前辈",
	"offer": "你做我仙剑派的弟子", "decline": "晚辈乃世俗之人", "denial": "我何时救过年轻的女孩",
	"jade": "李逍遥收起玉佛珠", "ling-captured": "主人关进锁妖塔", "confrontation": "妖邪魔物，自该打入锁妖塔",
	"rescue-vow": "你们不放人，我自己去救", "tower-history": "严禁门下弟子进入锁妖塔",
	"gifts": "酒剑仙传授给李逍遥", "thunder-handover": "这颗雷灵珠呢，是从蜘蛛精身",
	"wine-god-limit": "一生之中只能使用九次", "yue-joins": "妹子的事，怎能少我一份",
	"tower-warning": "你们真的要去闯锁妖塔啊",
}

var shuTowerSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{156, 2519, 156, "warning"}, {156, 2521, 158, ""}, {158, 2642, 160, "offer"},
	{160, 2651, 160, "denial"}, {160, 2655, 161, ""}, {161, 2677, 161, "ling-captured"},
	{161, 2676, 160, ""}, {160, 2651, 159, "yue-joins"},
	{159, 2646, 162, "tower-warning"}, {162, 2687, 163, ""}, {163, 2699, 164, ""}, {164, 2706, 147, ""},
}

func TestOriginalShuSaintJadeBeadTeachingGiftsAndTowerSave(t *testing.T) {
	name := "campaign-caiyi-forest-state.json"
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := shuTowerChecks{}
	if checkpoint := os.Getenv("PAL_SHU_TOWER_CHECKPOINT"); checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n <= 0 || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported Shu/tower test checkpoint", checkpoint)
			}
		}
		name = "shu-tower-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("shu-tower-"+checkpoint+"-"+kind+".json", 1))
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
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables, proof: p,
		prefix: "shu-tower", milestones: shuTowerReadings, reserveScenes: []int{147, 156, 158, 159, 160, 161, 162, 163, 164},
		allowedMenus: map[int]bool{3: true, 5: true, 10: true, 12: true}}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	r.notice = func(v pal.View) {
		if v.Scene == 165 && !r.proof.Saved["tower-lift"] {
			r.save("tower-lift")
		}
	}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("shu-tower-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("shu-tower-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	names := []string{"玉佛珠", "雷灵珠", "圣灵符", "天师符", "金刚符", "酒"}
	if os.Getenv("PAL_SHU_TOWER_CHECKPOINT") == "" {
		v := r.field()
		if v.Scene != 156 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 {
			t.Fatal("Shu test source is not the earned actual arrival", v.Scene, v.Party)
		}
		checks.InventoryBefore = r.inventoryCounts(names...)
		checks.LearnedBefore = shuOriginalSpellMenu(r)
		for _, name := range []string{"酒神", "醉仙望月步", "仙风云体术"} {
			if checks.LearnedBefore[name] {
				t.Fatal("Shu teaching spell already present before its original event", name)
			}
		}
		save("before")
	}
	for r.proof.Step < len(shuTowerSteps) {
		q := shuTowerSteps[r.proof.Step]
		t.Log("Shu/tower waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 2400; i++ {
			v := r.field()
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("Shu/tower ordinary route entered an unexpected scene", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 40 {
				t.Fatal("Shu/tower ordinary stance blocked", q, at)
			}
			e := tables.Events[q.Event-1]
			x, y := int(e.X), int(e.Y)
			for _, a := range v.Field.Actors {
				if a[0] == 1 && a[1] == q.Event {
					x, y = a[3], a[4]
				}
			}
			if q.Event == 2677 {
				if v.X != 640 || v.Y != 1424 {
					r.g = r.w.walk(t, r.m, &r.c, r.g, v, 640, 1424, 0)
				} else {
					r.keys(32, 3)
					r.keys(0, 4)
					r.progress()
					r.field()
					r.keys(2, 4)
				}
			} else if e.TriggerMode > 0 && e.TriggerMode < 4 && fieldTestDistance(v.X, v.Y, x, y) <= 192 {
				r.search(q.Event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			}
			r.progress()
			if i%100 == 0 {
				t.Log("Shu/tower walk", i, v.Scene, v.X, v.Y, x, y)
			}
		}
		v := r.field()
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("Shu/tower ordinary waypoint incomplete", q, v.Scene, v.X, v.Y)
		}
		if q.Token == "ling-captured" && r.inventoryCounts("玉佛珠")["玉佛珠"] != checks.InventoryBefore["玉佛珠"]+1 {
			t.Fatal("original sealed jade bead return changed across save")
		}
		if q.Token == "yue-joins" {
			checks.InventoryAfterGifts = r.inventoryCounts(names...)
			for item, amount := range map[string]int{"玉佛珠": 1, "雷灵珠": 1, "圣灵符": 3, "天师符": 3, "金刚符": 3, "酒": 2} {
				if checks.InventoryAfterGifts[item] != checks.InventoryBefore[item]+amount {
					t.Fatal("original Shu gift quantity changed", item, checks.InventoryBefore, checks.InventoryAfterGifts)
				}
			}
			checks.LearnedAfter = shuOriginalSpellMenu(r)
			for _, name := range []string{"酒神", "醉仙望月步", "仙风云体术"} {
				if !checks.LearnedAfter[name] {
					t.Fatal("original Shu teaching absent from real Li spell menu", name, checks.LearnedAfter)
				}
			}
			save("gifts-verified")
			for retry := 0; retry < 3; retry++ {
				v := r.field()
				x, y := xFrom(tables, 2649), yFrom(tables, 2649)
				for _, a := range v.Field.Actors {
					if a[0] == 1 && a[1] == 2649 {
						x, y = a[3], a[4]
					}
				}
				r.search(2649, x, y)
				r.progress()
				r.field()
				checks.RepeatMasterSearches++
			}
			if !bytes.Equal(raw(checks.InventoryAfterGifts), raw(r.inventoryCounts(names...))) {
				t.Fatal("original Shu gifts duplicated after saved repeated conversation")
			}
			save("gifts-repeated")
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := r.field()
	for name := range shuTowerReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original Shu milestone absent", name, r.proof.Seen)
		}
	}
	if v.Scene != 147 || v.Map != 141 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 || !r.proof.Saved["tower-lift"] || checks.RepeatMasterSearches != 3 {
		t.Fatal("original Shu/tower ending incomplete", v.Scene, v.Party, r.proof, checks)
	}
	for _, scene := range []int{156, 158, 159, 160, 161, 162, 163, 164, 165, 147} {
		if r.proof.Scenes[scene] == 0 {
			t.Fatal("original Shu/tower scene was not observed", scene)
		}
	}
	finalItems := r.inventoryCounts("玉佛珠", "雷灵珠")
	if finalItems["玉佛珠"] != checks.InventoryBefore["玉佛珠"]+1 || finalItems["雷灵珠"] != checks.InventoryBefore["雷灵珠"]+1 {
		t.Fatal("original Shu key items lost on tower entry", finalItems)
	}
	save("tower-entered")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": os.Getenv("PAL_SHU_TOWER_CHECKPOINT") != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "original Shu master warning, sword saint meeting/denial, sealed jade bead return, confrontation, actual three-spell teaching and six inventory deltas, saved repeated master interaction without duplicate gifts, Yue choice, sky bridges and native tower-lift entry; ordinary input only, no native scene/script/stats edits", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "finalKeyItems": finalItems, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-shu-tower-state.json": r.g.Data, "campaign-shu-tower.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func shuOriginalSpellMenu(r *caiyiForestDriver) map[string]bool {
	r.t.Helper()
	r.field()
	r.keys(32768, 4)
	for i := 0; i < 256 && (r.visible().Menu.Kind != 3 || r.visible().Seq != r.visible().AppliedSeq); i++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 3 {
		r.t.Fatal("original Shu spell role selector did not open", r.visible().Menu)
	}
	for i := 0; i < 32; i++ {
		v := r.visible()
		selected := -1
		for i, item := range v.Menu.Items {
			if item.Label == "李逍遥" {
				selected = i
			}
		}
		if selected < 0 {
			r.t.Fatal("original Shu spell role selector has no Li", v.Menu)
		}
		if v.Menu.Selected == selected {
			r.keys(2, 4)
			break
		}
		r.keys(8, 1)
		r.progress()
		r.progress()
	}
	for i := 0; i < 128 && (r.visible().Menu.Kind != 10 || r.visible().Seq != r.visible().AppliedSeq); i++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 10 || r.visible().Menu.Role != 0 {
		r.t.Fatal("original Shu real Li spell list did not open", r.visible().Menu)
	}
	r.keys(65536, 4)
	r.progress()
	r.progress()
	found, previous := map[string]bool{}, ""
	for i := 0; i < 32; i++ {
		v := r.visible()
		text := originalVisibleText(v, false)
		for _, name := range []string{"酒神", "醉仙望月步", "仙风云体术"} {
			found[name] = found[name] || strings.Contains(text, name)
		}
		if text == previous {
			break
		}
		previous = text
		r.keys(128, 4)
		r.progress()
		r.progress()
	}
	r.save("spell-menu-" + fmt.Sprint(found["酒神"]))
	r.keys(1, 4)
	r.field()
	return found
}
