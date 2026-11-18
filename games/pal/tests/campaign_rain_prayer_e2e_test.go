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

type prayerChecks struct {
	Phase, NextOrb, GiftRepeatInputs, EndRepeatInputs int
	Before, WithSacred, AfterWrongHoly, Opened, After map[string]int
	PillarInventory                                   map[string]map[string]int
	PillarFrames                                      map[int]int
	PartyOrders                                       map[string][]int
	CinematicScenes                                   map[int]bool
	BossObserved, BossAcceptedSaved, LingDressed      bool
}

var prayerReadings = map[string]string{
	"temple-refuge": "你们赶紧到神殿内避难", "tribe-plea": "请您保佑这些无辜的族人吧",
	"mother-greeting": "我的孩子～辛苦你了", "mother-wish": "继承为娘最后的心愿",
	"mother-blessing": "为你祝福", "prayer-intent": "我要学娘一样施法祭天祈雨",
	"tribe-recognizes": "简直是巫后再世", "altar-inscription": "西疆斩风魔东海杀雷神",
	"rain-invocation": "灵儿念出祭雨咒文", "rain-request": "请求您赐予这片土地新的生命",
	"rain-miracle": "这是神迹啊", "rain-retreat": "女娲娘娘显灵了", "baiyue-order": "通通不许走",
	"tribe-thanks": "我代全族的人向您致谢", "priestess-offer": "留下的祭司之位好吗",
	"earth-beast": "黑苗人竟然真的使用魔兽来作战", "defend-people": "不能让它伤害到无辜的人",
	"pursue-mastermind": "我们去揪出这幕后的主使者", "face-destiny": "该面对的．．总是该去面对",
}

var prayerSteps = []struct {
	Scene, Event, Dest int
	Token              string
	Search             bool
}{
	{175, 2869, 177, "", false}, {177, 2916, 187, "", false}, {187, 3132, 180, "", false},
	{180, 2955, 229, "", false}, {229, 4017, 230, "", false}, {230, 4064, 233, "", false},
	{233, 4193, 233, "temple-refuge", true}, {233, 4171, 235, "", false},
	{235, 4213, 235, "tribe-plea", true}, {235, 4223, 235, "prayer-intent", false},
	{235, 4213, 235, "tribe-recognizes", true}, {235, 4200, 242, "", false},
	{242, 4281, 242, "altar-inscription", true},
}

var prayerOrbs = []struct {
	Name         string
	Event, Frame int
}{
	{"风灵珠", 4283, 4}, {"火灵珠", 4284, 1}, {"雷灵珠", 4285, 2}, {"土灵珠", 4286, 5}, {"水灵珠", 4287, 3},
}

func prayerStairMoves(t *testing.T, tables *resources.Tables) map[int]map[[2]int][2]int {
	t.Helper()
	var relative func(int, int) ([2]int, bool)
	relative = func(pc, depth int) ([2]int, bool) {
		delta := [2]int{}
		if depth > 3 {
			return delta, false
		}
		for end := pc + 100; pc < end && pc < len(tables.Scripts); pc++ {
			s := tables.Scripts[pc]
			switch s.Opcode {
			case 0:
				return delta, true
			case 0x15, 0x9:
			case 0x6e:
				delta[0] += int(int16(s.Operands[0]))
				delta[1] += int(int16(s.Operands[1]))
			case 0x4:
				d, ok := relative(int(s.Operands[0]), depth+1)
				if !ok {
					return delta, false
				}
				delta[0] += d[0]
				delta[1] += d[1]
			default:
				return delta, false
			}
		}
		return delta, false
	}
	moves := map[int]map[[2]int][2]int{}
	rows := []map[string]any{}
	for _, scene := range []int{235, 242} {
		moves[scene] = map[[2]int][2]int{}
		count := 0
		for i := tables.Scenes[scene-1].FirstEvent; i < tables.Scenes[scene].FirstEvent; i++ {
			e := tables.Events[i]
			if e.State <= 0 || e.Sprite != 0 || e.TriggerMode < 4 || e.TriggerScript == 0 {
				continue
			}
			delta, ok := relative(int(e.TriggerScript), 0)
			if !ok || delta == [2]int{} {
				continue
			}
			areas := map[[2]int][2]int{}
			radius := (int(e.TriggerMode)-4)*32 + 16
			sacredTreeTouchArea(areas, [2]int{int(e.X), int(e.Y)}, [2]int{}, radius)
			for at := range areas {
				if _, exists := moves[scene][at]; !exists {
					moves[scene][at] = [2]int{at[0] + delta[0], at[1] + delta[1]}
				}
			}
			count++
			rows = append(rows, map[string]any{"scene": scene, "event": int(i) + 1, "script": e.TriggerScript, "radius": radius, "delta": delta})
		}
		if scene == 235 && count != 10 || scene == 242 && count != 6 {
			t.Fatal("original prayer stair references changed", scene, count)
		}
	}
	if err := os.WriteFile(campaignEvidencePath("prayer-stair-reference.json", 1), raw(rows), 0600); err != nil {
		t.Fatal(err)
	}
	return moves
}

func prayerFaceEvent(r *caiyiForestDriver, event int) bool {
	r.t.Helper()
	v := sacredTreeField(r)
	x, y := xFrom(r.tables, event), yFrom(r.tables, event)
	for _, actor := range v.Field.Actors {
		if actor[0] == 1 && actor[1] == event {
			x, y = actor[3], actor[4]
		}
	}
	distances, queue := map[int]int{144: 0}, []int{144}
	for at := 0; at < len(queue); at++ {
		i := queue[at]
		for _, d := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
			u, z := i%17+d[0], i/17+d[1]
			if u < 0 || u > 16 || z < 0 || z > 16 {
				continue
			}
			next := z*17 + u
			if _, exists := distances[next]; !exists && v.Field.Cells[next]&3 == 0 {
				distances[next] = distances[i] + 1
				queue = append(queue, next)
			}
		}
	}
	best, direction, stance := int(^uint(0)>>1), -1, [2]int{}
	for _, i := range queue {
		distance := distances[i]
		u, z := i%17-8, i/17-8
		px, py := v.X+(u-z)*16, v.Y+(u+z)*8
		for d, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
			if px+delta[0] == x && py+delta[1] == y && distance < best {
				best, direction, stance = distance, d, [2]int{px, py}
			}
		}
	}
	if direction < 0 {
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 0)
		r.progress()
		return false
	}
	if stance != [2]int{v.X, v.Y} {
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, stance[0], stance[1], 0)
		r.progress()
		return false
	}
	r.keys([]uint32{4, 8, 16, 32}[direction], direction)
	r.keys(0, 4)
	r.progress()
	v = sacredTreeField(r)
	delta := [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}}[direction]
	return v.X+delta[0] == x && v.Y+delta[1] == y && v.Field.Direction == direction
}

func prayerUseItem(r *caiyiForestDriver, item, label string) {
	r.t.Helper()
	trialWormInventory(r, item, item)
	r.save(label)
	r.keys(2, 4)
	r.progress()
	for i := 0; i < 128; i++ {
		v := r.visible()
		if v.Phase != "menu" {
			return
		}
		if v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			r.keys(1, 4)
		}
		r.progress()
	}
	r.t.Fatal("original prayer item use did not leave the inventory", item)
}

func TestOriginalRainPrayerSacredGiftsFiveOrbsAndSavedEarthBeast(t *testing.T) {
	if os.Getenv("PAL_RAIN_PRAYER") != "1" {
		t.Skip("explicit original goddess, five-orb prayer and earth beast story")
	}
	name := "campaign-sacred-medicine-state.json"
	root, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-sacred-medicine-state.json", root, "537ab6e594af984367d176e77b726e14da5f1f75e811197023ac76a0e422f582"); err != nil {
		t.Fatal(err)
	}
	checkpoint := os.Getenv("PAL_RAIN_PRAYER_CHECKPOINT")
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := prayerChecks{PillarInventory: map[string]map[string]int{}, PillarFrames: map[int]int{}, PartyOrders: map[string][]int{}, CinematicScenes: map[int]bool{}}
	if checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 0 || n > len(prayerSteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported prayer debug checkpoint", checkpoint)
			}
		}
		name = "prayer-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("prayer-"+checkpoint+"-"+kind+".json", 1))
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
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "prayer", proof: p, tables: tables, milestones: prayerReadings,
		allowedRoles: map[int]bool{0: true, 1: true, 4: true}, reserveScenes: []int{175, 177, 187, 180, 229, 230, 233, 235, 242}}
	r.w = campaignMapWalker{rememberCollision: false, allowTouch: true, avoidTouches: map[int][][3]int{}, sameSceneMoves: prayerStairMoves(t, tables)}
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("prayer-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.notice = func(v pal.View) {
		roles := []int{}
		for _, member := range v.Party {
			roles = append(roles, member.Role)
		}
		if !bytes.Equal(raw(roles), raw([]int{1, 0, 4})) && !bytes.Equal(raw(roles), raw([]int{0})) {
			t.Fatal("original prayer temporary party changed", v.Scene, roles)
		}
		label := fmt.Sprint("party-", roles)
		if !r.proof.Saved[label] {
			checks.PartyOrders[label] = roles
			save(label)
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			label = fmt.Sprint("field-", v.Scene, "-phase-", checks.Phase, "-step-", r.proof.Step, "-orb-", checks.NextOrb)
			if !r.proof.Saved[label] {
				save(label)
			}
			for _, actor := range v.Field.Actors {
				if actor[0] == 0 && actor[1] == 1 && actor[2] == 38 {
					checks.LingDressed = true
				}
				if actor[0] == 1 && actor[1] >= 4283 && actor[1] <= 4287 && actor[5] > 0 {
					checks.PillarFrames[actor[1]] = actor[5]
				}
			}
		}
		for _, scene := range []int{228, 231, 244, 245, 246, 232} {
			if v.Scene == scene && !checks.CinematicScenes[scene] {
				checks.CinematicScenes[scene] = true
				save(fmt.Sprint("cinematic-", scene))
			}
		}
		if b := v.Battle; b != nil {
			r.battlePartySize = len(roles)
			alive := false
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive {
				t.Fatal("original prayer battle defeated the party", v.Party)
			}
			for _, enemy := range b.Enemies {
				if v.Scene == 232 && enemy.Object == 466 {
					checks.BossObserved = true
				}
			}
			if v.Scene == 232 && !checks.BossAcceptedSaved {
				for _, member := range b.Players {
					if member.State == 2 {
						checks.BossAcceptedSaved = true
						save("earth-beast-accepted")
					}
				}
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("prayer-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	items := []string{"圣灵珠", "圣灵披风", "天蛇杖", "风灵珠", "火灵珠", "雷灵珠", "土灵珠", "水灵珠", "傀儡虫"}
	inventory := func() map[string]int { return trialWormInventory(r, "", items...) }
	if checkpoint == "" {
		v := sacredTreeField(r)
		checks.Before = inventory()
		parentBytes, err := os.ReadFile(campaignEvidencePath("campaign-sacred-medicine.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		var parent struct {
			Passed, ResumedTestRoute bool
			Checks                   struct{ After map[string]int }
		}
		if err := json.Unmarshal(parentBytes, &parent); err != nil || !parent.Passed || parent.ResumedTestRoute || parent.Checks.After == nil {
			t.Fatal("prayer requires the actual completed worm delivery report", err)
		}
		for item, amount := range parent.Checks.After {
			if checks.Before[item] != amount {
				t.Fatal("prayer entry inventory differs from actual parent delivery", item, checks.Before, parent.Checks.After)
			}
		}
		if checks.Before["傀儡虫"] != parent.Checks.After["傀儡虫"] {
			t.Fatal("prayer entry changed the actual remaining worm quantity", checks.Before, parent.Checks.After)
		}
		if v.Scene != 175 || checks.Before["圣灵珠"] != 0 || checks.Before["圣灵披风"] != 0 || checks.Before["天蛇杖"] != 0 {
			t.Fatal("prayer starting field or sacred gifts changed", v.Scene, checks.Before)
		}
		for _, orb := range prayerOrbs {
			if checks.Before[orb.Name] != 1 {
				t.Fatal("prayer entry is missing an earned original orb", orb.Name, checks.Before)
			}
		}
		save("step-0")
	}
	for checks.Phase == 0 && r.proof.Step < len(prayerSteps) {
		q := prayerSteps[r.proof.Step]
		t.Log("original prayer waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 8000; i++ {
			v := sacredTreeField(r)
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("prayer route entered unexpected field", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 60 {
				t.Fatal("prayer approach blocked", q, at)
			}
			if q.Search {
				if prayerFaceEvent(r, q.Event) {
					r.keys(2, 4)
					r.progress()
				}
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, xFrom(tables, q.Event), yFrom(tables, q.Event), 0)
				r.progress()
			}
		}
		v := sacredTreeField(r)
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("prayer waypoint incomplete", q, v.Scene)
		}
		if q.Event == 4223 {
			checks.WithSacred = inventory()
			for _, item := range items {
				expected := checks.Before[item]
				if item == "圣灵珠" || item == "圣灵披风" || item == "天蛇杖" {
					expected = 1
				}
				if checks.WithSacred[item] != expected {
					t.Fatal("original goddess gift or earned inventory changed", item, checks.WithSacred)
				}
			}
			save("sacred-gifts-granted")
			for checks.GiftRepeatInputs < 3 {
				r.keys(2, 4)
				r.progress()
				sacredTreeField(r)
				checks.GiftRepeatInputs++
			}
			prayerUseItem(r, "圣灵珠", "holy-selected-away")
			sacredTreeField(r)
			checks.AfterWrongHoly = inventory()
			if !bytes.Equal(raw(checks.WithSacred), raw(checks.AfterWrongHoly)) {
				t.Fatal("original sacred pearl worked outside altar or gifts duplicated", checks.AfterWrongHoly)
			}
			save("holy-wrong-place-preserved")
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	if checks.Phase == 0 {
		for i := 0; i < 2000 && !prayerFaceEvent(r, 4281); i++ {
			if i == 1999 {
				t.Fatal("original altar tablet could not be faced")
			}
		}
		prayerUseItem(r, "圣灵珠", "holy-selected-at-tablet")
		sacredTreeField(r)
		checks.Opened = inventory()
		if checks.Opened["圣灵珠"] != 0 {
			t.Fatal("original altar opening did not consume sacred pearl", checks.Opened)
		}
		for _, orb := range prayerOrbs {
			if checks.Opened[orb.Name] != 1 {
				t.Fatal("altar opening prematurely consumed an elemental orb", orb.Name)
			}
		}
		checks.Phase = 1
		save("altar-opened")
	}
	for checks.Phase == 1 && checks.NextOrb < len(prayerOrbs) {
		q := prayerOrbs[checks.NextOrb]
		t.Log("original prayer pillar", checks.NextOrb, q)
		for i := 0; i < 2000 && !prayerFaceEvent(r, q.Event); i++ {
			if i == 1999 {
				t.Fatal("original altar pillar could not be faced", q)
			}
		}
		if checks.NextOrb == 0 && !r.proof.Saved["wrong-pillar-preserved"] {
			prayerUseItem(r, "雷灵珠", "thunder-selected-at-wind-pillar")
			sacredTreeField(r)
			if !bytes.Equal(raw(checks.Opened), raw(inventory())) {
				t.Fatal("wrong original pillar accepted an orb")
			}
			save("wrong-pillar-preserved")
		}
		prayerUseItem(r, q.Name, fmt.Sprint("orb-selected-", checks.NextOrb))
		sacredTreeField(r)
		if checks.NextOrb < 4 {
			if r.visible().Scene != 242 || r.proof.Seen["rain-invocation"] || checks.PillarFrames[q.Event] != q.Frame {
				t.Fatal("prayer started early or original pillar frame was lost", q, r.visible().Scene, checks.PillarFrames)
			}
			checks.PillarInventory[q.Name] = inventory()
			for i, orb := range prayerOrbs {
				expected := 1
				if i <= checks.NextOrb {
					expected = 0
				}
				if checks.PillarInventory[q.Name][orb.Name] != expected {
					t.Fatal("original partial altar consumed wrong orb", q, orb, checks.PillarInventory[q.Name])
				}
			}
		}
		checks.NextOrb++
		save(fmt.Sprint("orb-complete-", checks.NextOrb))
	}
	v := sacredTreeField(r)
	checks.After = inventory()
	if v.Scene != 292 || v.Map != 190 || !bytes.Equal(raw(checks.After), raw(checks.WithSacred)) || !checks.LingDressed || !checks.BossObserved || !checks.BossAcceptedSaved || len(checks.PartyOrders) != 2 || checks.GiftRepeatInputs != 3 || r.proof.Battles != r.proof.Wins {
		t.Fatal("original prayer, returned orbs, boss or party ending incomplete", v.Scene, v.Map, checks, r.proof)
	}
	for _, scene := range []int{228, 231, 244, 245, 246, 232} {
		if !checks.CinematicScenes[scene] {
			t.Fatal("original rain/disaster/celebration scene missing", scene)
		}
	}
	for name := range prayerReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original prayer milestone missing", name, r.proof.Seen)
		}
	}
	checks.Phase = 2
	save("prayer-complete")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "actual post-recovery war/refuge, goddess mother and sacred gifts, original wardrobe/temporary party, wrong-place sacred pearl, tablet opening, wrong-pillar rejection, five individual orb placements, original rain/disaster/celebration and earth-beast victory, returned pearls and next-region arrival; ordinary inputs and fresh-runtime saves only", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "rootStage22SHA256": hex.EncodeToString(rootHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-rain-prayer.json": report, "campaign-rain-prayer-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
