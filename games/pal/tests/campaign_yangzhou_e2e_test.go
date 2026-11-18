package pal_test

// E2E first: Yangzhou loses the original curfew, inn/night pursuit, returned
// bundle, widow/well trap, trial choice, solo rescue or restored Li/Yue party;
// selected trial decisions and script continuations change after fresh saves;
// the optional original bounty is audited separately. Static coordinates are
// walkthrough inputs only and never reach production bot observations/policy.
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

type yangzhouProof struct {
	ScriptedFinishes                                          int
	Pursuit, EscapeObserved                                   bool
	Step, Battles, Wins, Readings, Saves, Commands, Decisions int
	InBattle, SoloRescue, PartyRestored                       bool
	BattleCash, BountyBefore                                  uint32
	RewardCash                                                int
	LastPrompt                                                uint64
	Seen, Saved                                               map[string]bool
}

type yangzhouDriver struct {
	*riverMedicineDriver
	proof  yangzhouProof
	tables *resources.Tables
}

var yangzhouReadings = map[string]string{
	"curfew": "概不准出城", "room": "只剩下楼上最右边", "night": "门外有人喊叫",
	"first-pursuit": "咱们出去逮那黑衣贼", "daybreak": "东西还给你们", "bundle": "拾起布包",
	"first-escape":    "可惜奴家没空陪您玩了",
	"returned-bundle": "真是好心没好报", "widow": "对不起，告辞", "well": "井里头一定有古怪",
	"gourd": "找到紫金葫芦", "accused": "我们又不是贼", "bargain": "先留下来",
	"secret-door": "屏风后面出现一道秘门", "final-challenge": "这么快就追来啦",
	"innocent": "无罪释放", "freedom": "你们可以自由离去了",
	"exit-permission": "可让两位出城",
}

// Event-driven walkthrough. Search steps finish on an observed original text;
// other steps must actually traverse the door/stair to the expected scene.
var yangzhouSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{78, 1487, 79, ""}, {79, 1497, 81, ""}, {81, 1507, 84, ""}, {84, 1572, 80, ""},
	{80, 1501, 83, "curfew"}, {83, 1548, 94, ""}, {94, 1741, 94, "room"}, {94, 1731, 94, ""},
	{94, 1750, 94, "door-1750"}, {94, 1728, 98, ""}, {98, 1786, 98, "night"}, {98, 1780, 94, ""},
	{94, 1760, 94, "first-pursuit"}, {94, 1756, 94, "door-1756"}, {94, 1725, 86, ""}, {86, 1620, 99, ""},
	{99, 1789, 88, ""}, {88, 1637, 99, ""}, {99, 1792, 86, ""}, {86, 1622, 99, ""},
	{99, 1794, 87, ""}, {87, 1630, 87, "daybreak"}, {87, 1631, 87, "bundle"},
	{87, 1626, 85, ""}, {85, 1574, 83, ""}, {83, 1548, 94, ""}, {94, 1731, 94, ""}, {94, 1752, 94, "door-1752"}, {94, 1729, 98, ""},
	{98, 1783, 98, "returned-bundle"}, {98, 1781, 94, ""}, {94, 1732, 94, ""}, {94, 1724, 83, ""},
	{83, 1546, 85, ""}, {85, 1579, 90, ""}, {90, 1651, 90, "widow"},
	{90, 1649, 85, ""}, {85, 1580, 87, ""}, {87, 1632, 87, "well"}, {87, 1626, 85, ""},
	{85, 1610, 93, ""}, {93, 1689, 93, ""}, {93, 1691, 85, "bargain"}, {85, 1579, 90, ""},
	{90, 1661, 91, "secret-door"}, {91, 1664, 93, ""}, {93, 1692, 93, ""}, {93, 1690, 93, ""},
	{93, 1693, 82, "freedom"}, {82, 1518, 85, ""},
	{85, 1600, 85, "exit-permission"}, {85, 1583, 106, ""},
}

func TestOriginalYangzhouThiefTrialRescueAndSave(t *testing.T) {
	name := "campaign-ghost-mountain-state.json"
	p := yangzhouProof{Seen: map[string]bool{}, Saved: map[string]bool{}}
	if checkpoint := os.Getenv("PAL_YANGZHOU_CHECKPOINT"); checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n <= 0 || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported yangzhou test checkpoint", checkpoint)
			}
		}
		name = "yangzhou-" + checkpoint + "-state.json"
		data, err := os.ReadFile(campaignEvidencePath("yangzhou-"+checkpoint+"-proof.json", 1))
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
	r := &yangzhouDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p, tables: tables}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"yangzhou-failure-state.json": r.g.Data, "yangzhou-failure-proof.json": raw(r.proof), "yangzhou-failure-view.json": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	for r.proof.Step < len(yangzhouSteps) {
		q := yangzhouSteps[r.proof.Step]
		t.Log("yangzhou waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		v := r.field()
		visits := map[[2]int]int{}
		sameSceneLand := map[int][2]int{1689: {544, 1696}, 1690: {1488, 1192}, 1692: {752, 1512}, 1731: {400, 640}, 1732: {512, 752}}
		land, localPortal := sameSceneLand[q.Event]
		for i := 0; i < 2200; i++ {
			v = r.field()
			if strings.HasPrefix(q.Token, "door-") {
				for _, a := range v.Field.Actors {
					if a[0] == 1 && a[1] == q.Event && a[5] == 1 {
						r.proof.Seen[q.Token] = true
					}
				}
			}
			if q.Token != "" && r.proof.Seen[q.Token] || q.Token == "" && v.Scene == q.Dest && (q.Scene != q.Dest || localPortal && fieldTestDistance(v.X, v.Y, land[0], land[1]) < 64) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("original yangzhou route entered unexpected scene", q, v.Scene, v.X, v.Y)
			}
			place := [2]int{v.X, v.Y}
			visits[place]++
			if visits[place] > 32 {
				t.Fatal("yangzhou walkthrough stance blocked", q, place)
			}
			e := tables.Events[q.Event-1]
			x, y := int(e.X), int(e.Y)
			for _, a := range v.Field.Actors {
				if a[0] == 1 && a[1] == q.Event {
					x, y = a[3], a[4]
				}
			}
			if e.TriggerMode > 0 && e.TriggerMode < 4 && fieldTestDistance(v.X, v.Y, x, y) <= 192 {
				r.search(q.Event, x, y, q.Token == "returned-bundle" && !r.proof.Saved["bundle-selected"])
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			}
			r.progress()
			if i%100 == 0 {
				t.Log("yangzhou walk", i, v.Scene, v.X, v.Y, x, y)
			}
		}
		v = r.field()
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("original yangzhou waypoint incomplete", q, v.Scene, v.X, v.Y)
		}
		if q.Token == "freedom" {
			r.save("freedom-complete")
		}
		if q.Token == "returned-bundle" {
			r.riverMedicineDriver.absent("布包")
			r.save("bundle-returned-inventory")
		}
		r.proof.Step++
		r.save(fmt.Sprint("step-", r.proof.Step))
	}
	v := r.field()
	for token := range yangzhouReadings {
		if !r.proof.Seen[token] {
			t.Fatal("original yangzhou milestone missing", token)
		}
	}
	if v.Scene != 106 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 || !r.proof.SoloRescue || !r.proof.PartyRestored || r.proof.Wins+r.proof.ScriptedFinishes != r.proof.Battles || r.proof.ScriptedFinishes == 0 || r.proof.Decisions == 0 {
		t.Fatal("original yangzhou ending/party/trial incomplete", r.proof, v.Scene, v.Party)
	}
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(sourceHash[:]), "resumedTestRoute": os.Getenv("PAL_YANGZHOU_CHECKPOINT") != "", "optionalBountyAccepted": false, "scope": "original Yangzhou curfew, inn/night pursuit, bundle, widow/well trap, trial, solo rescue, restored manual Li/Yue party and actual city departure through ordinary inputs; fresh-runtime saves and stale-input rejection; optional bounty and hidden loot are separate, not accepted by main-story completion; test walkthrough, not autonomous exploration", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-yangzhou-state.json": r.g.Data, "campaign-yangzhou.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Explicit audit entry: the original 5500-cash script is preserved, but its
// actual ordinary standing/search route has not passed. Keeping this separate
// makes the unverified optional reward visible without calling it main story.
func TestOriginalYangzhouOptionalBountyAndSavedNoDuplication(t *testing.T) {
	if os.Getenv("PAL_YANGZHOU_BOUNTY") != "1" {
		t.Skip("explicit unresolved optional bounty audit; not acceptance")
	}
	name := "yangzhou-freedom-complete-state.json"
	if _, err := os.Stat(campaignEvidencePath(name, 1)); os.IsNotExist(err) {
		name = "yangzhou-bounty-source-state.json"
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &yangzhouDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: yangzhouProof{Seen: map[string]bool{}, Saved: map[string]bool{}}, tables: tables}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("yangzhou-bounty-audit-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	before := r.field()
	if before.Scene != 82 || len(before.Party) != 2 {
		t.Fatal("bounty audit requires actual post-acquittal court", before.Scene, before.Party)
	}
	r.reserve(1519)
	visits := map[[2]int]int{}
	for i := 0; i < 512 && !r.proof.Seen["bounty"]; i++ {
		v := r.field()
		place := [2]int{v.X, v.Y}
		visits[place]++
		if v.Scene != 82 || visits[place] > 32 {
			t.Fatal("optional magistrate approach remains unverified", v.Scene, place)
		}
		r.search(1519, 1616, 968, false)
		r.progress()
	}
	after := r.field()
	if !r.proof.Seen["bounty"] || after.Cash != before.Cash+5500 {
		t.Fatal("optional bounty not collected exactly once", before.Cash, after.Cash)
	}
	r.save("bounty-collected")
	for retry := 0; retry < 3; retry++ {
		readings := r.proof.Readings
		r.keys(2, 4)
		r.progress()
		if v := r.field(); v.Cash != after.Cash || r.proof.Readings <= readings {
			t.Fatal("saved bounty repeated or magistrate interaction disappeared", retry, after.Cash, v.Cash)
		}
	}
	if err := os.WriteFile(campaignEvidencePath("yangzhou-bounty-audit.json", 1), raw(map[string]any{"passed": true, "cashBefore": before.Cash, "cashAfter": after.Cash, "sourceCheckpoint": name, "freshRuntimeSaves": r.proof.Saves, "repeatSearches": 3}), 0600); err != nil {
		t.Fatal(err)
	}
}

// Approach a real standing/search position, including wide-range NPCs behind
// counters. Sending search while walking toward the closest wall is not enough.
// The original 13-point search footprint is used only to plan test inputs.
func (r *yangzhouDriver) search(event, x, y int, useBundle bool) {
	r.t.Helper()
	v := r.visible()
	mode := int(r.tables.Events[event-1].TriggerMode)
	if useBundle {
		mode = 1 // Face the merchant closely before using the actual plot item.
	}
	same := func(px, py int) bool { return px/32 == x/32 && py/16 == y/16 && (px%32 != 0) == (x%32 != 0) }
	reaches := func(px, py, d int) bool {
		delta := [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}}[d]
		points := [][2]int{{px, py}}
		for i := 0; i < 4; i++ {
			points = append(points, [2]int{px + delta[0], py + delta[1]}, [2]int{px, py + delta[1]*2}, [2]int{px + delta[0]*2, py})
			px, py = px+delta[0], py+delta[1]
		}
		for i, p := range points {
			if i < mode*6-4 && same(p[0], p[1]) {
				return true
			}
		}
		return false
	}
	best, direction, stance := int(^uint(0)>>1), -1, [2]int{}
	distances, queue := map[int]int{144: 0}, []int{144}
	for at := 0; at < len(queue); at++ {
		i := queue[at]
		for _, delta := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
			u, z := i%17+delta[0], i/17+delta[1]
			if u < 0 || u > 16 || z < 0 || z > 16 {
				continue
			}
			next := z*17 + u
			if _, seen := distances[next]; !seen && v.Field.Cells[next]&3 == 0 {
				distances[next] = distances[i] + 1
				queue = append(queue, next)
			}
		}
	}
	for i, flags := range v.Field.Cells[:289] {
		distance, reachable := distances[i]
		if flags&3 != 0 || !reachable {
			continue
		}
		u, z := i%17-8, i/17-8
		px, py := v.X+(u-z)*16, v.Y+(u+z)*8
		for d := 0; d < 4; d++ {
			if reaches(px, py, d) {
				if score := distance; score < best {
					best, direction, stance = score, d, [2]int{px, py}
				}
			}
		}
	}
	if direction < 0 {
		// A counter or raised dais can require going around an obstacle
		// outside the current local collision window before facing the NPC.
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 0)
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
	if useBundle {
		r.riverMedicineDriver.inventory("布包", true)
		r.save("bundle-selected")
	}
	r.keys(2, 4)
}

func (r *yangzhouDriver) save(name string) {
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
		r.t.Fatal("yangzhou save changed original continuation", name)
	}
	if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("yangzhou save accepted old controls", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err = os.WriteFile(campaignEvidencePath("yangzhou-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *yangzhouDriver) progress() {
	r.t.Helper()
	v, p := r.visible(), &r.proof
	for _, member := range v.Party {
		if member.Controller != 0 || member.Role != 0 && member.Role != 2 {
			r.t.Fatal("yangzhou lost original solo ownership", v.Party)
		}
	}
	p.SoloRescue = p.SoloRescue || p.Seen["bargain"] && len(v.Party) == 1 && v.Party[0].Role == 0
	p.PartyRestored = p.PartyRestored || p.Seen["freedom"] && len(v.Party) == 2
	if v.Dialogue != nil && strings.Contains(strings.ReplaceAll(v.Dialogue.Text, "\n", ""), yangzhouReadings["first-escape"]) {
		p.EscapeObserved = true
	}
	if p.InBattle && v.Battle == nil {
		if p.Pursuit && p.EscapeObserved && p.RewardCash < 0 && v.Cash == p.BattleCash {
			// Preserve the thief's original visible escape and unchanged cash;
			// this scripted continuation has no ordinary victory reward.
			p.ScriptedFinishes++
		} else if p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
			r.t.Fatal("yangzhou battle reward changed or battle lost", p.BattleCash, p.RewardCash, v.Cash)
		} else {
			p.Wins++
		}
		p.InBattle = false
	}
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	if strings.Contains(text, "获得五千五百文钱") && !p.Seen["bounty"] {
		p.Seen["bounty"] = true
		r.save("bounty-reading")
	}
	for name, token := range yangzhouReadings {
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
			r.t.Log("yangzhou reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
	} else if v.Battle != nil {
		if !p.InBattle {
			p.InBattle, p.BattleCash, p.RewardCash = true, v.Cash, -1
			p.Pursuit, p.EscapeObserved = false, false
			for _, enemy := range v.Battle.Enemies {
				p.Pursuit = p.Pursuit || enemy.Object == 478
			}
			p.Battles++
			if p.Battles > 60 {
				r.t.Fatal("yangzhou repeated too many battles")
			}
			r.save(fmt.Sprint("battle-", p.Battles))
			r.t.Log("yangzhou battle", p.Battles, v.Battle.Enemies)
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			p.RewardCash = cash
		}
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive {
			r.t.Fatal("yangzhou ordinary battle defeated party")
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
	} else if v.Phase == "menu" && v.Seq == v.AppliedSeq {
		// The original trial asks whether to confess. Choose Yes through its
		// actual No/Yes menu, saving the accepted selection before confirming.
		yes := -1
		for i, item := range v.Menu.Items {
			if item.Label == "是" {
				yes = i
			}
		}
		if yes < 0 {
			r.t.Fatal("unexpected original yangzhou menu", v.Menu)
		}
		if v.Menu.Selected != yes {
			r.keys(32, 4)
		} else {
			if !p.Saved["trial-selected"] {
				r.save("trial-selected")
			}
			r.keys(2, 4)
			p.Decisions++
		}
	}
	r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
	var err error
	r.g, err = r.m.OnTimer(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
}

func (r *yangzhouDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 5000; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("yangzhou did not return to original field", r.visible().Phase)
	return pal.View{}
}

func (r *yangzhouDriver) reserve(target int) {
	for scene := 78; scene <= 100; scene++ {
		r.w.avoidTouches[scene] = nil
		for i := r.tables.Scenes[scene-1].FirstEvent; i < r.tables.Scenes[scene].FirstEvent; i++ {
			e := r.tables.Events[i]
			if int(i)+1 != target && e.State > 0 && e.Sprite == 0 && e.TriggerMode >= 4 && i+1 != 1731 && i+1 != 1732 {
				r.w.avoidTouches[scene] = append(r.w.avoidTouches[scene], [3]int{int(e.X), int(e.Y), 48})
			}
		}
	}
	if target == 1519 {
		// Court guards extend beyond the local collision window. Plan around
		// all currently public actors so the walkthrough cannot oscillate
		// through a distant guard it has not yet reached with local probes.
		for _, a := range r.visible().Field.Actors {
			if a[0] == 1 && a[1] != target {
				r.w.avoidTouches[82] = append(r.w.avoidTouches[82], [3]int{a[3], a[4], 16})
			}
		}
	}
}
