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

type towerSwordChecks struct {
	BeforeWeapon, AfterWeapon string
	BeforeStats, AfterStats   [5]int
	ExpectedDeltas            [5]int
	InventoryBefore           map[string]int
	InventoryAfter            map[string]int
}

type towerBottomChecks struct {
	Equipment                           towerSwordChecks
	InventoryBefore, InventoryAfterKing map[string]int
	Bosses                              map[int]bool
	Reinforcements                      map[int]bool
	PartyOrders                         map[string][]int
	Pillars                             []int
}

func TestOriginalTowerKingSavedBattleObservation(t *testing.T) {
	if os.Getenv("PAL_TOWER_KING_OBSERVE") != "1" {
		t.Skip("explicit original king public battle observation")
	}
	r := loadRiverMedicine(t, "tower-bottom-failure-state.json")
	trace := []any{}
	defer func() {
		for kind, data := range map[string][]byte{"trace": raw(trace), "view": raw(r.visible()), "state": r.g.Data} {
			if err := os.WriteFile(campaignEvidencePath("tower-king-observation-"+kind+".json", 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 700; step++ {
		v := r.visible()
		if v.Battle == nil {
			t.Log("saved king battle left original battle", step, v.Scene, v.Cash)
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
				trace = append(trace, map[string]any{"step": step, "action": action, "battle": v.Battle, "party": v.Party, "text": originalVisibleText(v, false)})
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
	t.Log("bounded observation captured; this does not accept a battle victory or story stage")
}

var towerBottomReadings = map[string]string{
	"ling-reveal": "我只是丑陋的蛇女", "memory-restored": "我想起来了",
	"marriage-vow": "一夜夫妻、一世恩情", "sisters": "从今以后你我就是好姐妹啦",
	"three-together": "我们三人要永远在一起", "king-threat": "竟敢私纵犯妖",
	"ling-defense": "我身虽为妖，心却非魔", "bodhi": "得到菩提袈裟",
	"demons-plead": "请您帮助我们离开这地方吧", "pool-tolerance": "对化妖池水我可以忍受",
	"party-reunited": "我们三个人又", "seven-pillars": "七星磐龙柱",
	"pillars-destroyed": "七根龙柱全部毁去", "demons-freed": "我们终于重见天日了",
	"forgot-benefactors": "居然忘了三位恩公", "saint-reproach": "你怎么不阻止他们呢",
}

func towerBottomField(r *caiyiForestDriver) pal.View {
	r.t.Helper()
	for i := 0; i < 40000; i++ {
		v := r.visible()
		if v.Scene == 174 || v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			if r.notice != nil {
				r.notice(v)
			}
			return r.visible()
		}
		r.progress()
	}
	r.t.Fatal("tower bottom did not reach an original field boundary", r.visible().Phase)
	return pal.View{}
}

func TestOriginalTowerKingSevenDragonsCollapseAndSave(t *testing.T) {
	checkpoint := os.Getenv("PAL_TOWER_BOTTOM_CHECKPOINT")
	name := "campaign-tower-upper-state.json"
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := towerBottomChecks{Bosses: map[int]bool{}, Reinforcements: map[int]bool{}, PartyOrders: map[string][]int{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 1 || n > 10 || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported tower bottom test checkpoint", checkpoint)
			}
		}
		name = "tower-bottom-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("tower-bottom-"+checkpoint+"-"+kind+".json", 1))
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
		prefix: "tower-bottom", milestones: towerBottomReadings, reserveScenes: []int{145, 146, 153},
		allowedRoles: map[int]bool{0: true, 1: true, 2: true}, battlePartySize: 3,
		allowedMenus: map[int]bool{6: true}}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("tower-bottom-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
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
		for _, scene := range []int{170, 171, 172, 150, 151, 152} {
			if v.Scene == scene && !r.proof.Saved[fmt.Sprint("scene-", scene)] {
				save(fmt.Sprint("scene-", scene))
			}
		}
		if v.Phase == "field" && (v.Scene == 153 || v.Scene == 145) {
			if !bytes.Equal(raw(roles), raw([]int{1, 0, 2})) {
				t.Fatal("original Ling-led tower party order changed", v.Scene, roles)
			}
			label := fmt.Sprint("field-party-", v.Scene)
			if _, ok := checks.PartyOrders[label]; !ok {
				checks.PartyOrders[label] = roles
				save(label)
			}
		}
		if b := v.Battle; b != nil {
			if observations%250 == 0 {
				t.Log("tower bottom public battle", observations, r.proof.Step, v.Party, b)
			}
			alive := false
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive {
				t.Fatal("original tower battle defeated the whole party", observations, r.proof.Step, v.Party)
			}
			primary := 519
			expected := []int{0, 1, 2}
			if v.Scene == 145 {
				primary = 539 + r.proof.Step - 3
				expected = []int{1, 0, 2}
			} else if v.Scene != 146 {
				t.Fatal("unexpected original tower battle scene", v.Scene)
			}
			for _, enemy := range b.Enemies {
				if enemy.Object == 0 {
					continue
				}
				if enemy.Object == primary {
					checks.Bosses[primary] = true
				} else {
					checks.Reinforcements[enemy.Object] = true
				}
				if primary == 519 {
					label := "king-party"
					if _, ok := checks.PartyOrders[label]; !ok {
						checks.PartyOrders[label] = roles
						save(label)
					}
				}
				if !bytes.Equal(raw(roles), raw(expected)) {
					t.Fatal("original tower battle party order changed", enemy.Object, roles)
				}
				for _, member := range b.Players {
					label := fmt.Sprint("accepted-", primary)
					if member.State == 2 && !r.proof.Saved[label] {
						save(label)
					}
				}
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("tower-bottom-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	if r.proof.Step == 0 {
		v := r.field()
		if v.Scene != 146 || v.Map != 74 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 {
			t.Fatal("tower bottom did not start from actual earned upper-tower ending", v.Scene, v.Party)
		}
		checks.InventoryBefore = r.inventoryCounts("菩提袈裟", "天书", "七星剑")
		checks.Equipment = towerEquipEarnedSword(r)
		r.proof.Step = 1
		save("step-1")
	}
	if r.proof.Step == 1 {
		r.reserve(2400)
		for i := 0; i < 2400; i++ {
			v := towerBottomField(r)
			if v.Scene == 153 {
				break
			}
			if v.Scene != 146 {
				t.Fatal("unexpected field during original Ling reunion", v.Scene, v.Phase)
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, 1536, 432, 0)
			r.progress()
		}
		v := towerBottomField(r)
		if v.Scene != 153 || r.proof.Battles != 1 || r.proof.Wins != 1 || !checks.Bosses[519] {
			t.Fatal("original tower king/reunion did not complete", v.Scene, r.proof)
		}
		checks.InventoryAfterKing = r.inventoryCounts("菩提袈裟", "天书", "七星剑")
		if checks.InventoryAfterKing["菩提袈裟"] != checks.InventoryBefore["菩提袈裟"]+1 || checks.InventoryAfterKing["天书"] != 1 || checks.InventoryAfterKing["七星剑"] != 0 {
			t.Fatal("original tower king reward or owned items changed", checks.InventoryBefore, checks.InventoryAfterKing)
		}
		r.proof.Step = 2
		save("step-2")
	}
	if r.proof.Step == 2 {
		r.reserve(2475)
		for i := 0; i < 2400; i++ {
			v := towerBottomField(r)
			if v.Scene == 145 {
				break
			}
			if v.Scene != 153 {
				t.Fatal("unexpected original chemical pool approach", v.Scene)
			}
			if fieldTestDistance(v.X, v.Y, 960, 1056) <= 96 {
				r.search(2475, 960, 1056)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, 960, 1056, 96)
			}
			r.progress()
		}
		if v := towerBottomField(r); v.Scene != 145 || v.Map != 185 {
			t.Fatal("original seven-dragon chamber was not reached", v.Scene, v.Map)
		}
		r.proof.Step = 3
		save("step-3")
	}
	for r.proof.Step >= 3 && r.proof.Step < 10 {
		index := r.proof.Step - 3
		event, object := 2391+index, 539+index
		x, y := xFrom(tables, event), yFrom(tables, event)
		r.reserve(event)
		save(fmt.Sprint("pillar-", event, "-before"))
		for i := 0; i < 2400; i++ {
			v := towerBottomField(r)
			if r.proof.Wins >= index+2 {
				break
			}
			if v.Scene != 145 {
				t.Fatal("original dragon pillar left before normal victory", event, v.Scene, r.proof)
			}
			if fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				r.search(event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 96)
			}
			r.progress()
		}
		v := towerBottomField(r)
		if r.proof.Wins != index+2 || r.proof.Battles != index+2 || !checks.Bosses[object] {
			t.Fatal("original dragon victory did not belong to its pillar", event, object, r.proof)
		}
		checks.Pillars = append(checks.Pillars, event)
		if index < 6 {
			if v.Scene != 145 {
				t.Fatal("original tower collapsed before all seven pillars", index, v.Scene)
			}
			save(fmt.Sprint("pillar-", event, "-won"))
			cash, battles := v.Cash, r.proof.Battles
			for repeat := 0; repeat < 3; repeat++ {
				r.search(event, x, y)
				r.progress()
				towerBottomField(r)
			}
			if r.proof.Battles != battles || r.visible().Cash != cash {
				t.Fatal("original destroyed pillar repeated battle or reward", event)
			}
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := towerBottomField(r)
	if v.Scene != 174 || v.Map != 20 || len(v.Party) != 1 || v.Party[0].Role != 0 || r.proof.Wins != 8 || r.proof.Battles != 8 || r.proof.ScriptedFinishes != 0 || len(checks.Bosses) != 8 || len(checks.Pillars) != 7 {
		t.Fatal("original tower collapse ending incomplete", v.Scene, v.Map, v.Party, r.proof, checks)
	}
	for name := range towerBottomReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original tower bottom narrative milestone missing", name, r.proof.Seen)
		}
	}
	for _, scene := range []int{146, 170, 171, 172, 154, 153, 145, 150, 151, 152} {
		if r.proof.Scenes[scene] == 0 {
			t.Fatal("original tower memory/collapse scene not observed", scene)
		}
	}
	r.proof.Scenes[174]++
	save("next-stage-entry")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "original Ling reunion/memory and three-role tower king battle, actual Bodhi robe reward, original demons/sage and chemical pool, Ling-led three-role party, seven independent normal dragon pillar victories with saved repeat searches, original tower collapse/cutaways and next-scene entry; normal menu/input only, no native state edits", "scene": v.Scene, "map": v.Map, "phase": v.Phase, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-tower-bottom.json": report, "campaign-tower-bottom-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func towerWeaponEffects(t *testing.T, tables *resources.Tables, item int) [5]int {
	t.Helper()
	values := [5]int{}
	start := int(tables.Objects[item][3])
	for pc := start; pc < start+32; pc++ {
		s := tables.Scripts[pc]
		switch s.Opcode {
		case 0, 1:
			return values
		case 0x18:
			if s.Operands[0] != 14 || int(s.Operands[1]) != item {
				t.Fatal("unexpected original weapon slot", item, pc, s)
			}
		case 0x17:
			if s.Operands[0] != 14 || s.Operands[1] < 17 || s.Operands[1] > 21 {
				t.Fatal("unexpected original weapon attribute", item, pc, s)
			}
			values[s.Operands[1]-17] = int(int16(s.Operands[2]))
		default:
			t.Fatal("unreviewed original weapon equipment effect", item, pc, s)
		}
	}
	t.Fatal("original weapon script did not end", item)
	return values
}

func towerEquipEarnedSword(r *caiyiForestDriver) towerSwordChecks {
	r.t.Helper()
	initial := r.field()
	names := []string{"七星剑", "木剑", "铁剑", "酒", "止血草", "金创药", "水果", "蜂王蜜", "蜂蜜", "灵山仙芝", "灵葫仙丹", "雪莲子", "还神丹"}
	checks := towerSwordChecks{InventoryBefore: r.inventoryCounts(names...)}
	if checks.InventoryBefore["七星剑"] != 1 {
		r.t.Fatal("earned original seven-star sword is missing", checks.InventoryBefore)
	}
	r.keys(4096, 4)
	settle := func(ready func(pal.View) bool) {
		r.t.Helper()
		for i := 0; i < 128; i++ {
			v := r.visible()
			if v.Seq == v.AppliedSeq && ready(v) {
				return
			}
			r.progress()
		}
		r.t.Fatal("original sword equipment menu did not settle", r.visible().Menu)
	}
	settle(func(v pal.View) bool { return v.Menu.Kind == 5 })
	r.keys(65536, 4)
	r.progress()
	r.progress()
	for i := 0; ; i++ {
		if strings.Contains(originalVisibleText(r.visible(), true), "七星剑") {
			break
		}
		if i > 96 {
			r.t.Fatal("earned sword not found in original equipment list")
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	r.save("sword-item-selected")
	r.keys(2, 4)
	settle(func(v pal.View) bool { return v.Menu.Kind == 6 })
	if !strings.Contains(originalVisibleText(r.visible(), true), "李逍遥") {
		r.t.Fatal("original equipment selector did not start with Li")
	}
	checks.BeforeWeapon = templeEquipmentRow(caveEquipmentNodes(r.visible()), 77)
	previous, reviewed := map[string]int{"木剑": 166, "铁剑": 168}[checks.BeforeWeapon]
	if !reviewed {
		r.t.Fatal("unreviewed actual weapon baseline", checks.BeforeWeapon)
	}
	previousEffects, sword := towerWeaponEffects(r.t, r.tables, previous), towerWeaponEffects(r.t, r.tables, 186)
	for i, y := range []int{14, 36, 58, 80, 102} {
		checks.BeforeStats[i] = templeEquipmentValue(caveEquipmentNodes(r.visible()), y)
		checks.ExpectedDeltas[i] = sword[i] - previousEffects[i]
		if checks.BeforeStats[i] < 0 {
			r.t.Fatal("original weapon baseline stat missing", y)
		}
	}
	r.save("sword-role-selected")
	r.keys(2, 4)
	settle(func(v pal.View) bool { return templeEquipmentRow(caveEquipmentNodes(v), 77) == "七星剑" })
	checks.AfterWeapon = templeEquipmentRow(caveEquipmentNodes(r.visible()), 77)
	for i, y := range []int{14, 36, 58, 80, 102} {
		checks.AfterStats[i] = templeEquipmentValue(caveEquipmentNodes(r.visible()), y)
		if checks.AfterStats[i]-checks.BeforeStats[i] != checks.ExpectedDeltas[i] {
			r.t.Fatal("original seven-star sword net attribute changed", y, checks)
		}
	}
	r.save("sword-equipped")
	for i := 0; i < 8; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			break
		}
		r.keys(1, 4)
		r.progress()
		r.progress()
	}
	final := r.field()
	checks.InventoryAfter = r.inventoryCounts(names...)
	for _, name := range names {
		expected := checks.InventoryBefore[name]
		if name == "七星剑" {
			expected = 0
		} else if name == checks.BeforeWeapon {
			expected++
		}
		if checks.InventoryAfter[name] != expected {
			r.t.Fatal("original sword swap lost or duplicated items", name, checks)
		}
	}
	if final.Scene != initial.Scene || final.X != initial.X || final.Y != initial.Y || final.Cash != initial.Cash || !bytes.Equal(raw(final.Party), raw(initial.Party)) {
		r.t.Fatal("original sword equipment changed non-equipment state", initial.Party, final.Party, initial.Cash, final.Cash)
	}
	r.save("sword-equipped-field")
	return checks
}

func TestOriginalTowerEarnedSwordEquipmentAndSave(t *testing.T) {
	name := "campaign-tower-upper-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables,
		prefix: "tower-equipment", milestones: map[string]string{}, allowedMenus: map[int]bool{6: true},
		proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("tower-equipment-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	checks := towerEquipEarnedSword(r)
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "earned original sword selected and equipped on Li through original menus; all five net attributes compared with original equipment scripts, inventory swap and fresh-runtime saves, no native-state edits", "checks": checks, "saves": r.proof.Saves})
	for name, data := range map[string][]byte{"tower-equipment.json": report, "campaign-tower-equipped-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("original sword equipment verified", fmt.Sprint(checks.ExpectedDeltas), r.proof.Saves)
}
