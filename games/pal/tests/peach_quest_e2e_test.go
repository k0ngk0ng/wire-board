package pal_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type peachQuestDriver struct {
	*capitalQuestDriver
	signs            map[string]bool
	fixtures         []string
	bossBattles      int
	observedBoss     bool
	soloVillageSaved bool
	forceInputs      int
}

func newPeachQuest(t *testing.T, prefix string) *peachQuestDriver {
	t.Helper()
	r := &peachQuestDriver{capitalQuestDriver: newCapitalQuest(t, "campaign-sacred-tree-state.json", prefix), signs: map[string]bool{}, fixtures: []string{}}
	r.reserveScenes = []int{175, 176, 177, 179, 180, 181, 182, 183, 184, 187, 190, 191, 192}
	r.reserve(-1)
	r.notice = r.observePeach
	r.manualBattle = func(v pal.View) ([]byte, bool) {
		if v.Scene != 182 || v.Battle == nil {
			return nil, false
		}
		a, ok := caiyiForestPhysicalAttack(v)
		if !ok {
			return nil, true
		}
		if v.Battle.State == 1 && v.Battle.Menu == 0 {
			a.Direction, a.Keys = 4, 32768
			r.forceInputs++
		}
		return raw(a), true
	}
	return r
}

func (r *peachQuestDriver) observePeach(v pal.View) {
	r.observeQuest(v)
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	for label, token := range map[string]string{"fog-cleared": "阿奴祭起风灵珠", "mission": "把村长救回来", "song-story": "我只想和阿桃在一起", "metal-orb": "获得金罡珠", "village-gift": "此物乃是寿葫芦", "gourd-accepted": "晚辈就恭敬不如从命了", "tree-reveal": "原来你们是桃树精", "anu-reunion": "水灵珠没找着"} {
		if !r.signs[label] && strings.Contains(text, token) {
			r.signs[label] = true
			r.save(label + "-reading")
		}
	}
	if v.Scene == 184 && !r.soloVillageSaved {
		if len(v.Party) == 1 && v.Party[0].Role == 0 {
			r.soloVillageSaved = true
			r.save("village-solo-in-progress")
		}
	}
	if v.Scene == 183 && len(v.Party) == 1 && v.Party[0].Role == 0 && !r.proof.Saved["gourd-return-solo"] {
		r.save("gourd-return-solo")
	}
	if v.Battle == nil {
		r.observedBoss = false
	}
	if v.Scene == 182 && v.Battle != nil && !r.observedBoss {
		r.observedBoss = true
		r.bossBattles++
		r.save(fmt.Sprint("wooden-taoist-battle-", r.bossBattles))
	}
}

func (r *peachQuestDriver) fixture(scene int) {
	r.t.Helper()
	v := r.field()
	if v.Scene != scene {
		r.t.Fatal("original Peach fixture scene changed", scene, v.Scene)
	}
	label := fmt.Sprint("field-", scene)
	r.save(label)
	r.fixtures = append(r.fixtures, "peach-quest-v1-"+label+"-state.json")
}

func (r *peachQuestDriver) clearFog() {
	r.t.Helper()
	r.begin("clear-fog", "", "", "")
	before := r.field()
	if before.Scene != 179 {
		r.t.Fatal("original Peach mist source changed", before.Scene)
	}
	trialWormInventory(r.caiyiForestDriver, "风灵珠", "风灵珠")
	r.save("wind-orb-selected-in-fog")
	r.keys(2, 4)
	for step := 0; step < 2400; step++ {
		r.progress()
		v := r.visible()
		if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			r.keys(1, 4)
		}
		if v.Phase == "field" && v.Scene == 181 && v.Seq == v.AppliedSeq {
			if v.Cash != before.Cash || !r.signs["fog-cleared"] {
				r.t.Fatal("original mist clearing lost its real reading or cash")
			}
			r.inventory("wind-orb-retained", map[string]int{"风灵珠": 1, "金罡珠": 0, "寿葫芦": 0})
			return
		}
	}
	r.t.Fatal("original wind orb did not clear Peach mist", r.visible().Phase, r.visible().Scene)
}

func TestOriginalPeachVillageWindWoodenTaoistLongevityGourdAndSavedReturn(t *testing.T) {
	if os.Getenv("PAL_PEACH_QUEST") != "1" {
		t.Skip("explicit original Peach village optional quest")
	}
	r := newPeachQuest(t, "peach-quest-v1")
	initial := r.field()
	if initial.Scene != 180 || len(initial.Party) != 2 || initial.Party[0].Role != 0 || initial.Party[1].Role != 4 {
		t.Fatal("original Peach quest root changed", initial.Scene, initial.Party)
	}
	r.inventory("initial", map[string]int{"风灵珠": 1, "金罡珠": 0, "寿葫芦": 0})
	r.use("风灵珠", "wind-orb-wrong-use", "")
	r.inventory("wrong-wind-orb-retained", map[string]int{"风灵珠": 1, "金罡珠": 0, "寿葫芦": 0})
	r.door(2954, 187)
	r.door(3134, 179)
	r.save("mist-before")
	r.clearFog()
	r.fixture(181)
	r.door(2971, 190)
	r.fixture(190)
	r.door(3162, 181)
	r.door(2961, 176)
	r.fixture(176)
	r.talk(2909, "peach-mission", "", "")
	if !r.signs["mission"] {
		t.Fatal("original A-Tao mission absent")
	}
	r.door(2894, 191)
	r.fixture(191)
	r.talk(3198, "song-history", "", "")
	if !r.signs["song-story"] {
		t.Fatal("original Song confession absent")
	}
	r.door(3196, 182)
	r.save("wooden-taoist-before")
	trigger := r.tables.Events[2979]
	visits := map[[2]int]int{}
	for step := 0; step < 1800; step++ {
		v := r.field()
		if v.Scene == 183 {
			break
		}
		if v.Scene == 184 {
			r.progress()
			continue
		}
		if v.Scene != 182 {
			t.Fatal("original Peach rescue entered unexpected scene", v.Scene)
		}
		at := [2]int{v.X, v.Y}
		visits[at]++
		if visits[at] > 40 {
			t.Fatal("original wooden Taoist approach blocked", at)
		}
		if fieldTestDistance(v.X, v.Y, int(trigger.X), int(trigger.Y)) < 192 {
			r.keys(8, 1)
			r.keys(0, 4)
		} else {
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(trigger.X), int(trigger.Y), 0)
		}
		r.progress()
	}
	v := r.field()
	partyMatches := len(v.Party) == 1 && v.Party[0].Role == 0 || len(v.Party) == 2 && v.Party[0].Role == 4 && v.Party[1].Role == 0
	if v.Scene != 183 || !partyMatches || !r.soloVillageSaved || !r.proof.Saved["gourd-return-solo"] || r.bossBattles != 1 {
		t.Fatal("original Peach rescue or temporary solo party changed", v.Scene, v.Party, r.bossBattles)
	}
	r.inventory("gifts-received", map[string]int{"风灵珠": 1, "金罡珠": 1, "寿葫芦": 1})
	if len(r.field().Party) == 1 {
		r.talk(2985, "anu-return", "", "")
	}
	v = r.field()
	if len(v.Party) != 2 || v.Party[0].Role != 4 || v.Party[1].Role != 0 {
		t.Fatal("original Anu reunion order changed", v.Party)
	}
	r.save("anu-rejoined")
	for repeat := 1; repeat <= 3; repeat++ {
		r.door(2984, 181)
		r.door(2961, 183)
		r.inventory(fmt.Sprint("rescue-repeat-", repeat), map[string]int{"风灵珠": 1, "金罡珠": 1, "寿葫芦": 1})
		r.save(fmt.Sprint("rescue-repeat-", repeat))
	}
	if r.bossBattles != 1 || r.forceInputs == 0 {
		t.Fatal("original wooden Taoist battle repeated after restore")
	}
	r.door(2984, 181)
	r.door(2972, 190)
	r.door(3161, 181)
	r.door(2960, 187)
	final := r.field()
	finalItems := r.inventory("final", map[string]int{"风灵珠": 1, "金罡珠": 1, "寿葫芦": 1})
	for _, sign := range []string{"fog-cleared", "mission", "song-story", "metal-orb", "village-gift", "gourd-accepted", "tree-reveal"} {
		if !r.signs[sign] {
			t.Fatal("original Peach optional quest reading absent", sign)
		}
	}
	cash := 0
	for _, battle := range r.battles {
		cash += battle["rewardCash"].(int)
	}
	if r.proof.Battles != r.proof.Wins || len(r.battles) != r.proof.Battles || int64(final.Cash)-int64(initial.Cash) != int64(cash) || r.proof.ScriptedFinishes != 0 {
		t.Fatal("original Peach battle and wallet ledger changed", r.proof, r.battles)
	}
	r.finish("peach-quest-v1-result.json", map[string]any{"passed": true, "kind": "original-peach-village-quest", "event": 2980, "sourceCheckpoint": r.source, "resumedTestRoute": false, "cashInitial": initial.Cash, "cashFinal": final.Cash, "partyInitial": initial.Party, "partyFinal": final.Party, "finalInventory": finalItems, "signs": r.signs, "fixtureCheckpoints": r.fixtures, "bossBattles": r.bossBattles, "forceInputs": r.forceInputs, "temporarySoloSaved": r.soloVillageSaved, "returnAttempts": 3, "battleCashChanges": r.battles, "scene": final.Scene})
}

func TestOriginalPeachWoodenTaoistNativeForceDiagnostic(t *testing.T) {
	if os.Getenv("PAL_PEACH_BOSS_DIAGNOSTIC") != "1" {
		t.Skip("explicit actual pre-boss saved native force diagnostic")
	}
	r := newPeachQuest(t, "peach-boss-force-diagnostic-v1")
	r.source = "peach-quest-v1-wooden-taoist-before-state.json"
	r.riverMedicineDriver = loadRiverMedicine(t, r.source)
	r.prefix = "peach-boss-force-diagnostic-v1"
	initial := r.field()
	trigger := r.tables.Events[2979]
	for step := 0; step < 1800; step++ {
		v := r.field()
		if v.Scene == 183 {
			break
		}
		if v.Scene == 182 {
			if fieldTestDistance(v.X, v.Y, int(trigger.X), int(trigger.Y)) < 192 {
				r.keys(8, 1)
				r.keys(0, 4)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(trigger.X), int(trigger.Y), 0)
			}
		} else if v.Scene != 184 {
			t.Fatal("original force diagnostic entered unexpected scene", v.Scene)
		}
		r.progress()
	}
	final := r.field()
	if final.Scene != 183 || r.bossBattles != 1 || r.proof.Battles != r.proof.Wins {
		t.Fatal("original force diagnostic did not complete the real boss", final.Scene, r.proof)
	}
	r.inventory("gifts", map[string]int{"风灵珠": 1, "金罡珠": 1, "寿葫芦": 1})
	r.finish("peach-boss-force-diagnostic-v1-result.json", map[string]any{"passed": true, "diagnosticOnly": true, "resumedTestRoute": true, "cashBefore": initial.Cash, "cashAfter": final.Cash, "bossBattles": r.bossBattles, "party": final.Party})
}
