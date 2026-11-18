package pal_test

// E2E first: the original blocked mountain path, woman/cave investigation or
// toad battle cannot continue; the corpse's five-poison orb is lost/duplicated;
// the original temporary [4,5,4] Miao party loses single-seat ownership;
// its automatic battle cannot save/restore or return to Li/Yue; poison/inn
// cutscenes, Gaijiao's rescue and the original boat arrival are skipped. Static
// source/event coordinates below only plan ordinary test walkthrough inputs.
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

type toadMountainProof struct {
	MiaoResourcesSaved                                               bool
	Step, Battles, Wins, ScriptedFinishes, Readings, Saves, Commands int
	InBattle, MiaoBattle, MiaoParty, MiaoFinished, BossWon           bool
	BattleCash                                                       uint32
	RewardCash                                                       int
	LastPrompt                                                       uint64
	MiaoAutoFrames                                                   int
	Seen, Saved                                                      map[string]bool
}

type toadMountainDriver struct {
	*riverMedicineDriver
	proof  toadMountainProof
	tables *resources.Tables
}

var toadMountainReadings = map[string]string{
	"woman": "靠近我的男人会没命的", "refusal": "月如～算了，咱们走吧",
	"suspicion": "那女人绝非善类", "reveal": "既然被你们看到我的脸", "orb": "五毒珠",
	"sword-saint": "他是剑圣前辈", "inn": "我叫盖罗娇", "drugged": "小睡片刻吧",
	"miao-confrontation": "把公主殿下交给我们", "ling-in-sedan": "是．．逍遥哥哥吗",
	"blood-poison": "赤血毒", "flying-sword": "饶你不得", "ling-cry": "是灵儿的声音",
	"gaijiao-rescued": "李逍遥救醒盖罗娇", "ling-taken": "那位老道士自称是独孤剑圣",
	"capital-plan": "我有位姨妈住在那", "boat": "坐船一块回去吧", "arrival": "是尚书府！？",
}

var toadMountainSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{106, 1965, 103, ""}, {103, 1881, 102, ""}, {102, 1847, 102, "woman"},
	{102, 1844, 104, ""}, {104, 1909, 104, "refusal"}, {104, 1905, 102, ""},
	{102, 1844, 104, ""}, {104, 1909, 104, "reveal"}, {104, 1911, 104, "orb"},
	{104, 1906, 105, ""}, {105, 1928, 107, ""}, {107, 1971, 111, ""},
	{111, 2062, 111, "ling-cry"}, {111, 2056, 110, ""}, {110, 2040, 110, "capital-plan"},
	{110, 2036, 108, ""}, {108, 1992, 113, ""}, {113, 2087, 112, ""},
	{112, 2072, 101, "arrival"},
}

func TestOriginalToadMountainPoisonOrbMiaoPartyAndSave(t *testing.T) {
	name := "campaign-yangzhou-state.json"
	p := toadMountainProof{Seen: map[string]bool{}, Saved: map[string]bool{}}
	if checkpoint := os.Getenv("PAL_TOAD_MOUNTAIN_CHECKPOINT"); checkpoint != "" {
		if checkpoint != "failure" {
			kind := "step-"
			if strings.HasPrefix(checkpoint, "battle-") {
				kind = "battle-"
			}
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, kind))
			if err != nil || n <= 0 || checkpoint != fmt.Sprint(kind, n) {
				t.Fatal("unsupported toad-mountain test checkpoint", checkpoint)
			}
		}
		name = "toad-mountain-" + checkpoint + "-state.json"
		data, err := os.ReadFile(campaignEvidencePath("toad-mountain-"+checkpoint+"-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(source)
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &toadMountainDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p, tables: tables}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("toad-mountain-failure-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	for r.proof.Step < len(toadMountainSteps) {
		q := toadMountainSteps[r.proof.Step]
		t.Log("toad mountain waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		if q.Scene == 112 && q.Event == 2072 && !r.proof.Saved["dock-stair-pose"] {
			// The actual pier is approached around the railing from the
			// north, rather than by walking into its nearest southern wall.
			r.walk(112, 1232, 840)
			r.walk(112, 1312, 880)
			r.walk(112, 1248, 912)
			r.keys(32, 3)
			r.keys(0, 4)
			r.progress()
			r.field()
			r.save("dock-stair-pose")
		}
		visits := map[[2]int]int{}
		for i := 0; i < 2200; i++ {
			v := r.field()
			if q.Token != "" && r.proof.Seen[q.Token] || q.Token == "" && v.Scene == q.Dest {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("original toad-mountain route entered unexpected scene", q, v.Scene, v.X, v.Y)
			}
			place := [2]int{v.X, v.Y}
			visits[place]++
			if visits[place] > 32 {
				t.Fatal("toad-mountain walkthrough stance blocked", q, place)
			}
			e := tables.Events[q.Event-1]
			x, y := int(e.X), int(e.Y)
			for _, a := range v.Field.Actors {
				if a[0] == 1 && a[1] == q.Event {
					x, y = a[3], a[4]
				}
			}
			if q.Scene == 112 && q.Event == 2072 {
				r.keys(2, 4) // Face the boat from the original shore-side stair.
			} else if e.TriggerMode > 0 && e.TriggerMode < 4 && fieldTestDistance(v.X, v.Y, x, y) <= 160 {
				r.search(q.Event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			}
			r.progress()
			if i%100 == 0 {
				t.Log("toad mountain walk", i, v.Scene, v.X, v.Y, x, y)
			}
		}
		v := r.field()
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("original toad-mountain waypoint incomplete", q, v.Scene, v.X, v.Y)
		}
		if q.Token == "orb" {
			r.riverMedicineDriver.inventory("五毒珠", false)
			r.save("orb-inventory")
			beforeCash := r.visible().Cash
			for retry := 0; retry < 3; retry++ {
				r.search(1911, 816, 872)
				r.progress()
				r.field()
			}
			r.riverMedicineDriver.inventory("五毒珠", false)
			if r.visible().Cash != beforeCash {
				t.Fatal("repeated original toad corpse search changed cash")
			}
			r.save("orb-repeat-search")
		}
		r.proof.Step++
		r.save(fmt.Sprint("step-", r.proof.Step))
	}
	v := r.field()
	for token := range toadMountainReadings {
		if !r.proof.Seen[token] {
			t.Fatal("original toad-mountain milestone missing", token)
		}
	}
	if v.Scene != 101 || v.Map != 105 || !r.proof.BossWon || !r.proof.MiaoParty || !r.proof.MiaoFinished || r.proof.Wins+r.proof.ScriptedFinishes != r.proof.Battles || r.proof.ScriptedFinishes != 1 || len(v.Party) != 2 || v.Party[0].Role != 2 || v.Party[1].Role != 0 || !r.proof.Saved["miao-auto"] || r.proof.MiaoAutoFrames < 4 || !r.proof.MiaoResourcesSaved {
		t.Fatal("original toad-mountain ending/temporary party incomplete", r.proof, v.Scene, v.Map, v.Party)
	}
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(sourceHash[:]), "resumedTestRoute": os.Getenv("PAL_TOAD_MOUNTAIN_CHECKPOINT") != "", "scope": "original mountain/cave woman investigation, actual toad boss, original corpse search and five-poison orb once, inn drugging, original temporary duplicated-role Miao party and scripted battle, blood poison, Gaijiao rescue and original boat/capital arrival; exact fresh-runtime saves and stale-input rejection; test walkthrough only, no native state/scripts/stats edits", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "fivePoisonOrbAmount": 1, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-toad-mountain-state.json": r.g.Data, "campaign-toad-mountain.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *toadMountainDriver) save(name string) {
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
		r.t.Fatal("toad-mountain save changed original continuation", name)
	}
	if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("toad-mountain save accepted old input", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err = os.WriteFile(campaignEvidencePath("toad-mountain-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *toadMountainDriver) progress() {
	r.t.Helper()
	v, p := r.visible(), &r.proof
	miao := len(v.Party) == 3 && v.Party[0].Role == 4 && v.Party[1].Role == 5 && v.Party[2].Role == 4
	for _, member := range v.Party {
		if member.Controller != 0 || !miao && member.Role != 0 && member.Role != 2 {
			r.t.Fatal("toad mountain lost original party/solo ownership", v.Party)
		}
	}
	if miao && !p.MiaoParty {
		p.MiaoParty = true
		r.save("miao-party")
	}
	if p.InBattle && v.Battle == nil {
		p.InBattle = false
		if p.MiaoBattle {
			p.MiaoFinished = true
			p.ScriptedFinishes++
			r.save("miao-battle-finished")
		} else {
			if p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
				r.t.Fatal("toad-mountain ordinary battle lost or reward changed", p.BattleCash, p.RewardCash, v.Cash)
			}
			p.Wins++
			p.BossWon = p.BossWon || v.Scene == 104 && p.Seen["reveal"]
		}
	}
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	for name, token := range toadMountainReadings {
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
			r.t.Log("toad mountain reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
	} else if b := v.Battle; b != nil {
		if !p.InBattle {
			p.InBattle, p.MiaoBattle, p.BattleCash, p.RewardCash = true, miao, v.Cash, -1
			p.Battles++
			if p.Battles > 100 {
				r.t.Fatal("toad-mountain route repeated too many battles")
			}
			r.save(fmt.Sprint("battle-", p.Battles))
			r.t.Log("toad mountain battle", p.Battles, "miao", miao, b.Enemies)
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			p.RewardCash = cash
		}
		if p.MiaoBattle {
			// Original script 16680 (0x008A) runs this NPC fight entirely
			// automatically. Advance only its native clock, without injecting
			// bot/human commands. Preserve an active mid-battle continuation.
			if b.State != 0 {
				r.t.Fatal("original automatic Miao fight opened a player selector", b)
			}
			p.MiaoAutoFrames++
			resourcesUsed := false
			for _, member := range v.Party {
				resourcesUsed = resourcesUsed || member.MP < member.MaxMP || member.HP < member.MaxHP
			}
			if p.MiaoAutoFrames >= 4 && resourcesUsed && !p.Saved["miao-auto"] {
				p.MiaoResourcesSaved = true
				r.save("miao-auto")
			}
		}
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive && !p.MiaoBattle {
			r.t.Fatal("ordinary toad-mountain battle defeated party")
		}
		var a []byte
		var err error
		if !p.MiaoBattle {
			a, err = r.m.BotAction(r.c, r.g)
		}
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

func (r *toadMountainDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 12000; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("toad mountain did not return to original field", r.visible().Phase)
	return pal.View{}
}

func (r *toadMountainDriver) search(event, x, y int) {
	r.t.Helper()
	v := r.visible()
	mode := int(r.tables.Events[event-1].TriggerMode)
	same := func(px, py int) bool { return px/32 == x/32 && py/16 == y/16 && (px%32 != 0) == (x%32 != 0) }
	best, direction, stance := int(^uint(0)>>1), -1, [2]int{}
	for i, flags := range v.Field.Cells[:289] {
		if flags&3 != 0 {
			continue
		}
		u, z := i%17-8, i/17-8
		px, py := v.X+(u-z)*16, v.Y+(u+z)*8
		for d, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
			points := [][2]int{{px, py}}
			a, b := px, py
			for j := 0; j < 4; j++ {
				points = append(points, [2]int{a + delta[0], b + delta[1]}, [2]int{a, b + delta[1]*2}, [2]int{a + delta[0]*2, b})
				a, b = a+delta[0], b+delta[1]
			}
			for j, p := range points {
				if j < mode*6-4 && same(p[0], p[1]) {
					if score := fieldTestDistance(v.X, v.Y, px, py); score < best {
						best, direction, stance = score, d, [2]int{px, py}
					}
					break
				}
			}
		}
	}
	if direction < 0 {
		r.g = campaignWalkTo(r.t, r.m, &r.c, r.g, v, x, y, event)
		return
	}
	if stance != [2]int{v.X, v.Y} {
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, stance[0], stance[1], 0)
		return
	}
	r.keys([]uint32{4, 8, 16, 32}[direction], direction)
	r.keys(0, 4)
	r.progress()
	r.field()
	r.keys(2, 4)
}

func (r *toadMountainDriver) walk(scene, x, y int) {
	r.t.Helper()
	visits := map[[2]int]int{}
	for i := 0; i < 1200; i++ {
		v := r.field()
		if v.Scene != scene || v.X == x && v.Y == y {
			return
		}
		p := [2]int{v.X, v.Y}
		visits[p]++
		if visits[p] > 32 {
			r.t.Fatal("original toad-mountain walking waypoint blocked", scene, x, y, p)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 0)
		r.progress()
	}
	r.t.Fatal("original toad-mountain walking waypoint exhausted", scene, x, y)
}

func (r *toadMountainDriver) reserve(target int) {
	for scene := 101; scene <= 114; scene++ {
		r.w.avoidTouches[scene] = nil
		for i := r.tables.Scenes[scene-1].FirstEvent; i < r.tables.Scenes[scene].FirstEvent; i++ {
			e := r.tables.Events[i]
			portal := false
			for pc := int(e.TriggerScript); pc > 0 && pc < int(e.TriggerScript)+6 && pc < len(r.tables.Scripts); pc++ {
				s := r.tables.Scripts[pc]
				portal = portal || s.Opcode == 0x59
				if s.Opcode <= 3 {
					break
				}
			}
			if int(i)+1 != target && e.State > 0 && e.Sprite == 0 && e.TriggerMode >= 4 && portal {
				radius := (int(e.TriggerMode)-4)*32 + 16
				r.w.avoidTouches[scene] = append(r.w.avoidTouches[scene], [3]int{int(e.X), int(e.Y), radius})
			}
		}
	}
}
