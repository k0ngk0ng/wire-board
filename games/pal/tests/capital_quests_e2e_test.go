package pal_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type capitalQuestDriver struct {
	*capitalEconomyDriver
	seen    map[string]bool
	battles []map[string]any
	items   map[string]map[string]int
}

func newCapitalQuest(t *testing.T, source, prefix string) *capitalQuestDriver {
	t.Helper()
	r := &capitalQuestDriver{capitalEconomyDriver: newCapitalEconomy(t, source, prefix), seen: map[string]bool{}, battles: []map[string]any{}, items: map[string]map[string]int{}}
	r.reserveScenes = []int{101, 102, 103, 104, 105, 107, 108, 109, 112, 113, 115, 116, 117, 119, 128, 136, 138}
	r.allowedMenus[3], r.allowedMenus[10] = true, true
	r.reserve(-1)
	r.notice = r.observeQuest
	return r
}

func (r *capitalQuestDriver) observeQuest(v pal.View) {
	r.capitalEconomyDriver.observe(v)
	if r.proof.InBattle && v.Battle == nil {
		if r.proof.BattleSpent != 0 || r.proof.RewardCash < 0 || int64(v.Cash) != int64(r.proof.BattleCash)+int64(r.proof.RewardCash) {
			r.t.Fatal("original quest travel battle ledger changed", r.proof, v.Cash)
		}
		r.battles = append(r.battles, map[string]any{"cashBefore": r.proof.BattleCash, "cashAfter": v.Cash, "rewardCash": r.proof.RewardCash})
	}
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	for label, token := range map[string]string{"letter-read": "美丽温柔大方可爱善良贤淑", "jade-offer": "我这里有块玉佩", "jade-not-hui": "这玉佩不是我的呀", "jade-is-de": "是我偷", "xiang-confession": "我喜欢的是少爷", "gourd-grant": "获得紫金葫芦", "gourd-spell": "得到灵葫咒", "capital-arrival": "是尚书府"} {
		if !r.seen[label] && strings.Contains(text, token) {
			r.seen[label] = true
			r.save(label + "-reading")
		}
	}
}

func (r *capitalQuestDriver) face(event int) {
	r.t.Helper()
	visits := map[[2]int]int{}
	for step := 0; step < 1000; step++ {
		v := r.field()
		pose := [2]int{v.X, v.Y}
		visits[pose]++
		if visits[pose] > 40 {
			r.t.Fatal("original quest facing route blocked", event, pose)
		}
		if prayerFaceEvent(r.caiyiForestDriver, event) {
			return
		}
	}
	r.t.Fatal("original quest facing route timed out", event)
}

func (r *capitalQuestDriver) talk(event int, label, choice, receipt string) {
	r.t.Helper()
	r.begin(label, "", "", "")
	r.face(event)
	r.begin(label, "talk", choice, receipt)
	before := r.field()
	r.save(label + "-before")
	r.keys(2, 4)
	r.progress()
	after := r.field()
	if before.Cash != after.Cash || before.Scene != after.Scene || choice != "" && !r.choiceSent || receipt != "" && !r.readingSaved {
		r.t.Fatal("original quest conversation changed wallet, scene, choice or receipt", label, before.Cash, after.Cash, before.Scene, after.Scene)
	}
	r.begin(label, "", "", "")
	r.save(label + "-after")
}

func (r *capitalQuestDriver) use(item, label, choice string) {
	r.t.Helper()
	r.begin(label, "talk", choice, "")
	before := r.field()
	trialWormInventory(r.caiyiForestDriver, item, item)
	r.save(label + "-item-selected")
	r.keys(2, 4)
	for step := 0; step < 2000; step++ {
		r.progress()
		v := r.visible()
		if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			r.keys(1, 4)
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if v.Cash != before.Cash || v.Scene != before.Scene || choice != "" && !r.choiceSent {
				r.t.Fatal("original quest item use changed wallet, scene or decision", item, label)
			}
			r.begin(label, "", "", "")
			r.save(label + "-after")
			return
		}
	}
	r.t.Fatal("original quest item did not finish", item, label, r.visible().Phase, r.visible().Menu)
}

func (r *capitalQuestDriver) inventory(label string, want map[string]int) map[string]int {
	r.t.Helper()
	names := []string{}
	for name := range want {
		names = append(names, name)
	}
	actual := r.inventoryCounts(names...)
	for name, amount := range want {
		if actual[name] != amount {
			r.t.Fatal("original quest inventory changed", label, name, amount, actual)
		}
		actual[name] = actual[name]
	}
	r.items[label] = actual
	r.save(label + "-inventory")
	return actual
}

func (r *capitalQuestDriver) finish(name string, result map[string]any) {
	result["inventoryChecks"] = r.items
	r.capitalEconomyDriver.finish(name, result)
}

func (r *capitalQuestDriver) door(event, destination int) {
	r.t.Helper()
	r.begin("travel", "", "", "")
	r.reserve(event)
	e := r.tables.Events[event-1]
	scene := r.field().Scene
	visits := map[[2]int]int{}
	for step := 0; step < 1000; step++ {
		v := r.field()
		if v.Scene == destination {
			r.reserve(-1)
			return
		}
		pose := [2]int{v.X, v.Y}
		visits[pose]++
		if v.Scene != scene || visits[pose] > 40 {
			r.t.Fatal("original quest door changed scene or repeated a blocked pose", event, destination, v.Scene, pose)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, int(e.X), int(e.Y), 0)
		r.progress()
	}
	r.t.Fatal("original quest door timed out", event, destination)
}

func TestOriginalCapitalLetterJadeCompleteChainAndSavedChoices(t *testing.T) {
	if os.Getenv("PAL_CAPITAL_QUESTS") != "1" {
		t.Skip("explicit original letter and jade optional chain")
	}
	for _, choice := range []string{"是", "否"} {
		name := "accept"
		if choice == "否" {
			name = "decline"
		}
		t.Run(name, func(t *testing.T) {
			r := newCapitalQuest(t, "capital-manor-step-15-state.json", "capital-quests-v1-letter-"+name)
			initial := r.field()
			r.inventory("initial", map[string]int{"情书": 0, "玉佩": 0})
			r.talk(1818, "letter-request", "是", "得到一封情书")
			r.inventory("letter-received", map[string]int{"情书": 1, "玉佩": 0})
			r.use("情书", "letter-wrong-use", "")
			r.inventory("letter-wrong-use-retained", map[string]int{"情书": 1, "玉佩": 0})
			r.door(1810, 115)
			r.door(2123, 119)
			r.face(2166)
			r.save("letter-delivery-before")
			r.use("情书", "letter-delivery", choice)
			jade := 1
			if choice == "否" {
				jade = 0
			}
			r.inventory("letter-delivered", map[string]int{"情书": 0, "玉佩": jade})
			if choice == "否" {
				for repeat := 1; repeat <= 3; repeat++ {
					r.talk(2166, fmt.Sprint("xiang-declined-repeat-", repeat), "否", "")
					r.inventory(fmt.Sprint("xiang-declined-repeat-", repeat), map[string]int{"情书": 0, "玉佩": 0})
				}
				r.talk(2166, "xiang-reconsidered", "是", "")
				r.inventory("xiang-reconsidered", map[string]int{"情书": 0, "玉佩": 1})
			}
			r.use("玉佩", "jade-wrong-use", "")
			r.inventory("jade-wrong-use-retained", map[string]int{"情书": 0, "玉佩": 1})
			r.door(2163, 115)
			r.door(2124, 101)
			r.face(1818)
			r.save("jade-hui-before")
			r.use("玉佩", "jade-hui", "")
			r.inventory("jade-hui-retained", map[string]int{"情书": 0, "玉佩": 1})
			r.face(1815)
			r.save("jade-de-before")
			r.use("玉佩", "jade-de", "")
			r.inventory("jade-returned", map[string]int{"情书": 0, "玉佩": 0})
			for repeat := 1; repeat <= 3; repeat++ {
				r.talk(1818, fmt.Sprint("hui-repeat-", repeat), "", "")
				r.talk(1815, fmt.Sprint("de-repeat-", repeat), "", "")
			}
			r.door(1810, 115)
			r.door(2123, 119)
			for repeat := 1; repeat <= 3; repeat++ {
				r.talk(2166, fmt.Sprint("xiang-ending-", repeat), "", "")
			}
			for _, token := range []string{"letter-read", "jade-offer", "jade-not-hui", "jade-is-de", "xiang-confession"} {
				if !r.seen[token] {
					t.Fatal("original complete letter/jade reading absent", token)
				}
			}
			finalItems := r.inventory("final", map[string]int{"情书": 0, "玉佩": 0})
			final := r.field()
			if initial.Cash != final.Cash || !bytes.Equal(raw(initial.Party), raw(final.Party)) || r.proof.Battles != 0 {
				t.Fatal("original letter branch changed money or party")
			}
			r.finish("capital-quests-v1-letter-"+name+"-result.json", map[string]any{"passed": true, "kind": "original-letter-jade-chain", "event": 2166, "choice": choice, "completeChain": true, "reconsideredAfterDeclines": choice == "否", "cashInitial": initial.Cash, "cashFinal": final.Cash, "partyInitial": initial.Party, "partyFinal": final.Party, "finalInventory": finalItems, "readings": r.seen, "repeatTalks": 3})
		})
	}
}

func (r *capitalQuestDriver) walk(x, y int) {
	r.t.Helper()
	scene := r.field().Scene
	visits := map[[2]int]int{}
	for step := 0; step < 1000; step++ {
		v := r.field()
		if v.Scene != scene {
			r.t.Fatal("original quest waypoint changed scene", scene, v.Scene)
		}
		if v.X == x && v.Y == y {
			return
		}
		pose := [2]int{v.X, v.Y}
		visits[pose]++
		if visits[pose] > 32 {
			r.t.Fatal("original quest waypoint blocked", scene, pose, x, y)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 0)
		r.progress()
	}
	r.t.Fatal("original quest waypoint timed out", scene, x, y)
}

func (r *capitalQuestDriver) gourdSpell(label string) bool {
	r.t.Helper()
	r.begin(label, "", "", "")
	r.field()
	r.keys(32768, 4)
	for step := 0; step < 256 && (r.visible().Menu.Kind != 3 || r.visible().Seq != r.visible().AppliedSeq); step++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 3 {
		r.t.Fatal("original quest spell role selector absent")
	}
	for step := 0; step < 32; step++ {
		v := r.visible()
		wanted := -1
		for _, item := range v.Menu.Items {
			if item.Label == "李逍遥" {
				wanted = item.Value
			}
		}
		if wanted < 0 {
			r.t.Fatal("original quest spell selector has no Li")
		}
		if v.Menu.Selected == wanted {
			r.keys(2, 4)
			break
		}
		r.keys(8, 1)
		r.progress()
		r.progress()
	}
	for step := 0; step < 128 && (r.visible().Menu.Kind != 10 || r.visible().Seq != r.visible().AppliedSeq); step++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 10 || r.visible().Menu.Role != 0 {
		r.t.Fatal("original quest Li spell list absent", r.visible().Menu)
	}
	r.keys(65536, 4)
	r.progress()
	r.progress()
	found, previous := false, ""
	for step := 0; step < 64; step++ {
		v := r.visible()
		text := originalVisibleText(v, false)
		if !found && strings.Contains(text, "灵葫咒") {
			found = true
			r.save(label + "-gourd-spell-visible")
		}
		if text == previous {
			break
		}
		previous = text
		r.keys(128, 4)
		r.progress()
		r.progress()
	}
	r.save(label + "-spell-menu")
	r.keys(1, 4)
	r.field()
	return found
}

func TestOriginalCapitalCorpseBundleReturnGourdSpellAndSave(t *testing.T) {
	if os.Getenv("PAL_CAPITAL_QUESTS") != "1" {
		t.Skip("explicit original corpse bundle, gourd and spell optional chain")
	}
	r := newCapitalQuest(t, "toad-mountain-step-18-state.json", "capital-quests-v1-bundle")
	initial := r.field()
	r.inventory("initial", map[string]int{"布包": 0, "紫金葫芦": 0})
	if r.gourdSpell("initial") {
		t.Fatal("original Li already knows optional gourd spell")
	}
	travel := []map[string]any{}
	for _, q := range [][2]int{{2085, 113}, {2088, 108}, {1991, 110}, {2037, 105}, {1926, 104}, {1905, 102}} {
		r.door(q[0], q[1])
		r.save(fmt.Sprint("return-mountain-door-", q[0]))
		travel = append(travel, map[string]any{"event": q[0], "scene": r.field().Scene})
	}
	r.talk(1845, "corpse-bundle", "", "搜得布包")
	r.inventory("corpse-bundle-collected", map[string]int{"布包": 1, "紫金葫芦": 0})
	r.use("布包", "bundle-wrong-use", "")
	r.inventory("bundle-wrong-use-retained", map[string]int{"布包": 1, "紫金葫芦": 0})
	for _, q := range [][2]int{{1844, 104}, {1906, 105}, {1928, 110}, {2036, 108}, {1992, 113}, {2087, 112}} {
		r.door(q[0], q[1])
		r.save(fmt.Sprint("return-dock-door-", q[0]))
		travel = append(travel, map[string]any{"event": q[0], "scene": r.field().Scene})
	}
	for _, q := range [][2]int{{1232, 840}, {1312, 880}, {1248, 912}} {
		r.walk(q[0], q[1])
	}
	r.keys(32, 3)
	r.keys(0, 4)
	r.progress()
	r.field()
	r.save("boat-stair-before")
	for step := 0; step < 1000 && r.field().Scene != 101; step++ {
		r.keys(2, 4)
		r.progress()
	}
	if r.field().Scene != 101 || !r.seen["capital-arrival"] {
		t.Fatal("original quest boat did not reach capital")
	}
	r.save("capital-arrived-with-bundle")
	r.door(1821, 138)
	r.face(2333)
	r.save("bundle-delivery-before")
	r.use("布包", "bundle-delivery", "")
	r.inventory("gourd-received", map[string]int{"布包": 0, "紫金葫芦": 1})
	if !r.seen["gourd-grant"] || !r.seen["gourd-spell"] || !r.gourdSpell("learned") {
		t.Fatal("original gourd gift or actual learned spell absent", r.seen)
	}
	for repeat := 1; repeat <= 3; repeat++ {
		r.talk(2333, fmt.Sprint("song-repeat-", repeat), "", "")
	}
	finalItems := r.inventory("final", map[string]int{"布包": 0, "紫金葫芦": 1})
	if !r.gourdSpell("repeated") {
		t.Fatal("original learned gourd spell lost after repeated saved talk")
	}
	final := r.field()
	cashRewards := 0
	for _, battle := range r.battles {
		cashRewards += battle["rewardCash"].(int)
	}
	if int64(final.Cash)-int64(initial.Cash) != int64(cashRewards) || r.proof.Battles != r.proof.Wins || len(r.battles) != r.proof.Battles || r.proof.ScriptedFinishes != 0 {
		t.Fatal("original bundle route changed battle/cash ledger", initial.Cash, final.Cash, r.battles, r.proof)
	}
	r.finish("capital-quests-v1-bundle-result.json", map[string]any{"passed": true, "kind": "original-corpse-bundle-gourd-spell-chain", "event": 2333, "sourceCorpseEvent": 1845, "cashInitial": initial.Cash, "cashFinal": final.Cash, "partyInitial": initial.Party, "partyFinal": final.Party, "finalInventory": finalItems, "readings": r.seen, "repeatTalks": 3, "travel": travel, "battleCashChanges": r.battles, "learnedMagic": 384, "spellBefore": false, "spellAfter": true, "spellAfterRepeat": true})
}
