package pal_test

// E2E first: returning from the earned earth-orb escape loses the kidnapping
// quest; original guards or mountain stairs cannot be passed; the scripted
// elder battle is incorrectly required to be won; Ling/the bead remain after
// departure; healing/farewell or two-role manual ownership is skipped; a fresh
// save replays rewards/readings or admits old input. Route/event IDs below are
// test walkthrough inputs only, never production bot observations or policy.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type ghostMountainProof struct {
	Route, Battles, Wins, ScriptedFinishes, Readings, Saves, Commands int
	InBattle, ElderBattle, ElderFinished, LiOnly                      bool
	BattleCash                                                        uint32
	RewardCash                                                        int
	LastPrompt                                                        uint64
	Seen, Saved                                                       map[string]bool
}

type ghostMountainDriver struct {
	*riverMedicineDriver
	proof ghostMountainProof
}

var ghostMountainReadings = map[string]string{
	"kidnapped": "梦慈被抓走了", "ultimatum": "自己一个人到", "rescue-plan": "我一定要去救她",
	"gate-identity": "我就是赵灵儿", "gate-refusal": "擅闯者、杀无赦", "ambush": "有埋伏",
	"elder": "老臣得罪了", "threat": "立刻自尽", "father": "巫王陛下得了重病",
	"promise": "老臣愿以人格担保", "ling-leaves": "我就跟你们走", "bead-leaves": "玉佛珠随赵灵儿离去",
	"doctor": "有我在死不了的", "determination": "我不会让她离开我", "farewell": "韩医仙一行人走了",
}

func TestOriginalGhostMountainRescueLingDepartureAndSave(t *testing.T) {
	name := "campaign-earth-orb-escaped-state.json"
	p := ghostMountainProof{Seen: map[string]bool{}, Saved: map[string]bool{}}
	if os.Getenv("PAL_GHOST_MOUNTAIN_CHECKPOINT") == "failure" {
		name = "ghost-mountain-failure-state.json"
		b, err := os.ReadFile(campaignEvidencePath("ghost-mountain-failure-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
	}
	r := &ghostMountainDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"ghost-mountain-failure-state.json": r.g.Data, "ghost-mountain-failure-proof.json": raw(r.proof), "ghost-mountain-failure-view.json": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	for r.proof.Route < len(ghostMountainRoute) {
		q := ghostMountainRoute[r.proof.Route]
		if r.field().Scene != q.Scene {
			t.Fatal("wrong original ghost-mountain route", r.proof.Route, q, r.visible().Scene)
		}
		r.reserve(q.Event)
		t.Log("ghost mountain waypoint", r.proof.Route, q)
		r.walk(q.Scene, q.X, q.Y, 0)
		v := r.field()
		if v.Scene != q.Dest || fieldTestDistance(v.X, v.Y, q.Land[0], q.Land[1]) > 128 {
			t.Fatal("original ghost-mountain stair/door failed", q, v.Scene, v.X, v.Y)
		}
		r.proof.Route++
		r.save(fmt.Sprint("route-", r.proof.Route))
		if r.proof.Route == 6 && !r.proof.Seen["rescue-plan"] {
			// The original physician's substitute actor becomes searchable
			// after the king. Walk/search normally; do not execute its script.
			r.reserve(0)
			for i := 0; i < 400 && !r.proof.Seen["rescue-plan"]; i++ {
				v = r.field()
				if fieldTestDistance(v.X, v.Y, 1248, 656) > 160 {
					r.walk(54, 1248, 656, 96)
				} else {
					r.g = campaignWalkTo(t, r.m, &r.c, r.g, v, 1248, 656, 907)
					r.progress()
				}
			}
			if !r.proof.Seen["kidnapped"] || !r.proof.Seen["ultimatum"] || !r.proof.Seen["rescue-plan"] {
				t.Fatal("original kidnapping/rescue request was not read", r.proof.Seen)
			}
			r.field()
			r.save("rescue-request-complete")
		}
	}
	r.reserve(0)
	r.walk(68, 1344, 1040, 96)
	for i := 0; i < 3000 && !r.proof.Seen["farewell"]; i++ {
		r.progress()
	}
	v := r.field()
	for key := range ghostMountainReadings {
		if !r.proof.Seen[key] {
			t.Fatal("original ghost-mountain story milestone missing", key, r.proof)
		}
	}
	if v.Scene != 78 || v.Map != 24 || !r.proof.ElderFinished || !r.proof.LiOnly || r.proof.ScriptedFinishes != 1 || r.proof.Wins+r.proof.ScriptedFinishes != r.proof.Battles || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 || v.Party[0].HP <= 0 {
		t.Fatal("original ghost-mountain ending did not restore its surviving party", r.proof, v.Scene, v.Map, v.Party)
	}
	r.inventoryAfterDeparture()
	r.save("complete")
	source, err := os.ReadFile(campaignEvidencePath("campaign-earth-orb-escaped-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "ordinary original return from earned earth-orb escape, kidnapping request, mountain gate and actual maze stairs, scripted stone elder battle/ambush, Ling and jade-bead departure, physician rescue/farewell, original manually owned Li/Yue party; exact fresh-runtime saves and stale-input rejection; no native scene/script/stats edits", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "earthOrbAmount": 1, "jadeBeadAbsent": true, "proof": r.proof}, "", "  ")
	for name, data := range map[string][]byte{"campaign-ghost-mountain-state.json": r.g.Data, "campaign-ghost-mountain.json": report} {
		if err = os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *ghostMountainDriver) save(name string) {
	r.t.Helper()
	before := r.visible()
	var err error
	r.g, err = r.m.SaveControl(r.c, r.g, true)
	if err != nil {
		r.t.Fatal(err)
	}
	r.c.Now = r.c.Now.Add(24 * time.Hour)
	r.m, _ = nativeModule(r.t)
	r.g, err = r.m.ResumeSaved(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
	normalize := func(v pal.View) pal.View {
		v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
		return v
	}
	if !bytes.Equal(raw(normalize(before)), raw(normalize(r.visible()))) {
		r.t.Fatal("ghost-mountain save changed original continuation", name)
	}
	if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("ghost-mountain resume admitted old controls", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err = os.WriteFile(campaignEvidencePath("ghost-mountain-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *ghostMountainDriver) progress() {
	r.t.Helper()
	v, p := r.visible(), &r.proof
	for _, member := range v.Party {
		if member.Controller != 0 || member.Role < 0 || member.Role > 2 {
			r.t.Fatal("ghost mountain changed original solo ownership", v.Party)
		}
	}
	if p.InBattle && v.Battle == nil {
		p.InBattle = false
		if p.ElderBattle {
			p.ElderFinished = true
			p.ScriptedFinishes++
			r.save("elder-battle-finished")
		} else {
			if p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
				r.t.Fatal("ordinary ghost-mountain battle lost or repeated reward", p.BattleCash, p.RewardCash, v.Cash)
			}
			p.Wins++
		}
	}
	if p.ElderFinished && len(v.Party) == 1 && v.Party[0].Role == 0 {
		p.LiOnly = true
		if !p.Saved["li-only"] {
			r.save("li-only")
		}
	}
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	for name, token := range ghostMountainReadings {
		if !p.Seen[name] && strings.Contains(text, token) {
			p.Seen[name] = true
			r.save(name + "-reading")
		}
	}
	v = r.visible()
	if d := v.Dialogue; d != nil {
		if d.ID != p.LastPrompt {
			p.LastPrompt = d.ID
			p.Readings++
			r.t.Log("ghost mountain reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
	} else if b := v.Battle; b != nil {
		if !p.InBattle {
			p.InBattle = true
			p.Battles++
			p.BattleCash, p.RewardCash = v.Cash, -1
			p.ElderBattle = false
			for _, e := range b.Enemies {
				p.ElderBattle = p.ElderBattle || e.Object == 496
			}
			if p.Battles > 70 {
				r.t.Fatal("ghost-mountain route repeated too many battles")
			}
			if p.ElderBattle {
				r.save("elder-battle")
			}
			r.t.Log("ghost mountain battle", p.Battles, p.ElderBattle, b.Enemies)
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			p.RewardCash = cash
		}
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive && !p.ElderBattle {
			r.t.Fatal("ordinary ghost-mountain battle defeated the party")
		}
		a, err := r.m.BotAction(r.c, r.g)
		if err != nil {
			r.t.Fatal(err)
		}
		if len(a) > 0 {
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				r.t.Fatal(err)
			}
			p.Commands++
		}
	}
	r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
	var err error
	r.g, err = r.m.OnTimer(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
}

func (r *ghostMountainDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 5000; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("original ghost mountain did not return to the field", r.visible().Phase)
	return pal.View{}
}

func (r *ghostMountainDriver) walk(scene, x, y, reach int) {
	r.t.Helper()
	visits := map[[2]int]int{}
	for i := 0; i < 1600; i++ {
		v := r.field()
		if v.Scene != scene || fieldTestDistance(v.X, v.Y, x, y) <= reach {
			return
		}
		place := [2]int{v.X, v.Y}
		visits[place]++
		if visits[place] > 24 {
			r.t.Fatal("original ghost-mountain waypoint blocked", scene, x, y, place)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, reach)
		r.progress()
		if i%100 == 0 {
			r.t.Log("ghost mountain walking", i, scene, v.X, v.Y, x, y)
		}
	}
	r.t.Fatal("ghost-mountain waypoint exhausted", scene, x, y)
}

func (r *ghostMountainDriver) reserve(target int) {
	for _, q := range ghostMountainPortals {
		r.w.avoidTouches[q[0]] = nil
	}
	for _, q := range ghostMountainPortals {
		if q[1] != target {
			r.w.avoidTouches[q[0]] = append(r.w.avoidTouches[q[0]], [3]int{q[2], q[3], 48})
		}
	}
}

func (r *ghostMountainDriver) inventoryAfterDeparture() {
	r.t.Helper()
	r.keys(2048, 4)
	for i := 0; i < 128 && (r.visible().Menu.Kind != 5 || r.visible().Seq != r.visible().AppliedSeq); i++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 5 {
		r.t.Fatal("ghost-mountain ending inventory unavailable")
	}
	r.keys(65536, 4)
	r.progress()
	r.progress()
	foundOrb := false
	for i := 0; i < 80; i++ {
		v := r.visible()
		if n, ok := campaignInventoryAmount(v, "玉佛珠"); ok && n != 0 {
			r.t.Fatal("jade bead remained after Ling's original departure", n)
		}
		if n, ok := campaignInventoryAmount(v, "土灵珠"); ok {
			if n != 1 {
				r.t.Fatal("earned earth orb lost/duplicated in ghost mountain", n)
			}
			foundOrb = true
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	if !foundOrb {
		r.t.Fatal("earned earth orb missing after ghost mountain")
	}
	r.save("departure-inventory")
	r.keys(1, 4)
	r.field()
}

var ghostMountainRoute = []tombWaypoint{
	{Scene: 65, Event: 1219, X: 352, Y: 1872, Dest: 64, Land: [2]int{1568, 288}},
	{Scene: 64, Event: 1205, X: 1872, Y: 1912, Dest: 62, Land: [2]int{192, 464}},
	{Scene: 62, Event: 1175, X: 1280, Y: 1392, Dest: 55, Land: [2]int{1376, 320}},
	{Scene: 55, Event: 924, X: 992, Y: 1760, Dest: 50, Land: [2]int{960, 624}},
	{Scene: 50, Event: 803, X: 1632, Y: 704, Dest: 53, Land: [2]int{416, 1264}},
	{Scene: 53, Event: 886, X: 864, Y: 816, Dest: 54, Land: [2]int{1280, 880}},
	{Scene: 54, Event: 904, X: 1312, Y: 896, Dest: 53, Land: [2]int{896, 832}},
	{Scene: 53, Event: 885, X: 384, Y: 1280, Dest: 50, Land: [2]int{1600, 720}},
	{Scene: 50, Event: 802, X: 800, Y: 544, Dest: 55, Land: [2]int{864, 1696}},
	{Scene: 55, Event: 926, X: 192, Y: 1296, Dest: 56, Land: [2]int{1728, 1488}},
	{Scene: 56, Event: 934, X: 192, Y: 1504, Dest: 71, Land: [2]int{1632, 720}},
	{Scene: 71, Event: 1380, X: 1584, Y: 1032, Dest: 69, Land: [2]int{1696, 1728}},
	{Scene: 69, Event: 1365, X: 1328, Y: 904, Dest: 75, Land: [2]int{624, 1592}},
	{Scene: 75, Event: 1415, X: 464, Y: 1192, Dest: 69, Land: [2]int{896, 896}},
	{Scene: 69, Event: 1363, X: 576, Y: 1056, Dest: 74, Land: [2]int{1168, 1192}},
	{Scene: 74, Event: 1408, X: 1264, Y: 808, Dest: 69, Land: [2]int{368, 824}},
	{Scene: 69, Event: 1368, X: 560, Y: 520, Dest: 70, Land: [2]int{560, 1848}},
	{Scene: 70, Event: 1373, X: 1232, Y: 712, Dest: 77, Land: [2]int{768, 1776}},
	{Scene: 77, Event: 1428, X: 416, Y: 496, Dest: 68, Land: [2]int{816, 1704}},
}

var ghostMountainPortals = [][4]int{
	{50, 801, 528, 1656}, {50, 802, 800, 544}, {50, 803, 1632, 704}, {50, 804, 896, 1168},
	{50, 805, 624, 1304}, {50, 806, 1072, 1256}, {50, 807, 800, 1392}, {50, 808, 1280, 1408}, {50, 809, 1024, 1536}, {50, 830, 928, 832},
	{53, 885, 384, 1280}, {53, 886, 864, 816}, {54, 904, 1312, 896},
	{55, 924, 992, 1760}, {55, 925, 1488, 248}, {55, 926, 192, 1296}, {55, 927, 352, 512},
	{56, 933, 1808, 1464}, {56, 934, 192, 1504}, {62, 1175, 1280, 1392}, {62, 1176, 160, 448},
	{64, 1205, 1872, 1912}, {64, 1206, 1600, 272}, {65, 1219, 352, 1872}, {65, 1220, 560, 264}, {68, 1340, 880, 1736},
	{69, 1358, 1648, 1704}, {69, 1359, 1360, 1144}, {69, 1360, 1392, 568}, {69, 1361, 1664, 1216}, {69, 1362, 944, 648},
	{69, 1363, 576, 1056}, {69, 1364, 320, 800}, {69, 1365, 1328, 904}, {69, 1366, 848, 872}, {69, 1367, 640, 1280}, {69, 1368, 560, 520},
	{70, 1371, 496, 1880}, {70, 1372, 1424, 808}, {70, 1373, 1232, 712}, {71, 1380, 1584, 1032}, {71, 1381, 1680, 744},
	{74, 1407, 1216, 1216}, {74, 1408, 1264, 808}, {75, 1414, 672, 1616}, {75, 1415, 464, 1192}, {77, 1426, 912, 1912}, {77, 1427, 720, 1800}, {77, 1428, 416, 496},
}
