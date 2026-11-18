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

type caiyiForestProof struct {
	Step, Readings, Battles, Wins, ScriptedFinishes, Saves, Commands int
	BattleKind, RewardCash, ThunderBefore                            int
	BattleCash                                                       uint32
	BattleSpent                                                      uint32
	InBattle                                                         bool
	LastPrompt                                                       uint64
	Seen, Saved                                                      map[string]bool
	Scenes                                                           map[int]int
}

type caiyiForestDriver struct {
	*riverMedicineDriver
	proof           caiyiForestProof
	tables          *resources.Tables
	notice          func(pal.View)
	physicalAttacks bool
	prefix          string
	milestones      map[string]string
	reserveScenes   []int
	allowedMenus    map[int]bool
	scriptedEnemies map[int]bool
	allowedRoles    map[int]bool
	battlePartySize int
	manualBattle    func(pal.View) ([]byte, bool)
}

var caiyiForestReadings = map[string]string{
	"garden": "用百花之精气为他驱毒", "true-form": "大嫂．．竟然是只蝶精",
	"caiyi-pleads": "相公中了缠魂丝", "caiyi-escapes": "奴家奴家只好得罪了",
	"black-spider": "擅入者死", "poison-lady": "呵呵～你终于低头来找我了",
	"life-for-life": "用我的命．．换他一命", "defend-caiyi": "不许动我大嫂一根寒毛",
	"master-rescues": "孽畜！～休得伤人", "thunder-revealed": "这畜牲体内居然有",
	"sacrifice": "也只能换得他十年的寿命", "last-wish": "请诸位永远不要让相公知道",
	"jinyuan-rescues": "别怕！我来救你了", "marriage-memory": "娶到你，是我们刘家的福气",
	"reflection": "世间有无情人，妖却有深情", "yue-promise": "若换作是你，我也会这么做",
	"mount-shu": "回蜀山闭关修炼去也", "sword-saint": "他是我大师兄",
	"flight": "闭上双眼，我要施法", "arrives": "这是本门的飞仙术",
}

var caiyiForestSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{116, 2135, 117, ""}, {117, 2149, 109, ""}, {109, 2010, 109, "garden"},
	{109, 1996, 118, ""}, {118, 2152, 140, ""},
	{140, 2346, 139, "black-spider"}, {139, 2342, 156, "arrives"},
}

func TestOriginalCaiyiPoisonForestSacrificeFlightAndSave(t *testing.T) {
	name := "campaign-capital-manor-state.json"
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	if checkpoint := os.Getenv("PAL_CAIYI_FOREST_CHECKPOINT"); checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n <= 0 || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported Caiyi forest test checkpoint", checkpoint)
			}
		}
		name = "caiyi-forest-" + checkpoint + "-state.json"
		data, err := os.ReadFile(campaignEvidencePath("caiyi-forest-"+checkpoint+"-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
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
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p, tables: tables}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("caiyi-forest-failure-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	if os.Getenv("PAL_CAIYI_FOREST_CHECKPOINT") == "" {
		v := r.field()
		if v.Scene != 116 || len(v.Party) != 1 || v.Party[0].Role != 0 {
			t.Fatal("Caiyi route did not start at the actual manor ending", v.Scene, v.Party)
		}
		r.proof.ThunderBefore = r.inventoryCounts("雷灵珠")["雷灵珠"]
		if r.proof.ThunderBefore != 0 {
			t.Fatal("thunder orb already present before the original reveal")
		}
		r.save("before")
	}
	for r.proof.Step < len(caiyiForestSteps) {
		q := caiyiForestSteps[r.proof.Step]
		t.Log("Caiyi forest waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 2400; i++ {
			v := r.field()
			if q.Token != "" && r.proof.Seen[q.Token] && v.Scene == q.Dest || q.Token == "" && v.Scene == q.Dest {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("Caiyi original walkthrough entered an unexpected scene", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 40 {
				t.Fatal("Caiyi ordinary walkthrough stance blocked", q, at)
			}
			e := tables.Events[q.Event-1]
			x, y := int(e.X), int(e.Y)
			for _, a := range v.Field.Actors {
				if a[0] == 1 && a[1] == q.Event {
					x, y = a[3], a[4]
				}
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			r.progress()
			if i%100 == 0 {
				t.Log("Caiyi forest walk", i, v.Scene, v.X, v.Y, x, y)
			}
		}
		v := r.field()
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("Caiyi original walkthrough waypoint incomplete", q, v.Scene, v.X, v.Y)
		}
		r.proof.Step++
		r.save(fmt.Sprint("step-", r.proof.Step))
	}
	v := r.field()
	for name := range caiyiForestReadings {
		if !r.proof.Seen[name] {
			t.Fatal("Caiyi original milestone absent", name, r.proof.Seen)
		}
	}
	for _, scene := range []int{109, 118, 140, 139, 141, 142, 143, 144, 156} {
		if r.proof.Scenes[scene] == 0 {
			t.Fatal("Caiyi original scene/flashback was never observed", scene)
		}
	}
	if v.Scene != 156 || v.Map != 132 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 || r.proof.ScriptedFinishes != 2 || r.proof.Wins+r.proof.ScriptedFinishes != r.proof.Battles || !r.proof.Saved["selected-468"] || !r.proof.Saved["selected-435"] {
		t.Fatal("Caiyi original ending/battles/party incomplete", v.Scene, v.Party, r.proof)
	}
	after := r.inventoryCounts("雷灵珠")["雷灵珠"]
	if after != r.proof.ThunderBefore {
		t.Fatal("thunder orb granted before its original later Shu handover", r.proof.ThunderBefore, after)
	}
	r.save("thunder-not-yet-handed-over")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": os.Getenv("PAL_CAIYI_FOREST_CHECKPOINT") != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "original garden revelation and Caiyi escape, poison forest/black spider, original poison-lady scripted result and master rescue, thunder-orb reveal without premature grant, sacrifice and all three flashbacks, original flight and arrival dialogue; ordinary walkthrough inputs and fresh-runtime saved accepted actions/readings/scenes, no native scene/script/stats edits", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "thunderOrbAmount": after, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-caiyi-forest-state.json": r.g.Data, "campaign-caiyi-forest.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *caiyiForestDriver) save(name string) {
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
		r.t.Fatal("Caiyi forest save changed original continuation", name)
	}
	if _, err := r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("Caiyi forest resume accepted old input", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	prefix := r.prefix
	if prefix == "" {
		prefix = "caiyi-forest"
	}
	for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+name+"-"+kind+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *caiyiForestDriver) progress() {
	r.t.Helper()
	v, p := r.visible(), &r.proof
	if r.notice != nil {
		r.notice(v)
		v = r.visible()
	}
	p.Scenes[v.Scene]++
	for _, member := range v.Party {
		allowed := member.Role == 0 || member.Role == 2
		if r.allowedRoles != nil {
			allowed = r.allowedRoles[member.Role]
		}
		if member.Controller != 0 || !allowed {
			r.t.Fatal("Caiyi original party ownership changed", v.Party)
		}
	}
	if p.InBattle && v.Battle == nil {
		p.InBattle = false
		if p.BattleKind == 468 || p.BattleKind == 435 || r.scriptedEnemies[p.BattleKind] {
			if v.Cash != p.BattleCash {
				r.t.Fatal("Caiyi scripted battle unexpectedly issued a cash reward", p.BattleKind, p.BattleCash, v.Cash)
			}
			p.ScriptedFinishes++
			r.save(fmt.Sprint("scripted-finish-", p.BattleKind))
		} else {
			if p.RewardCash < 0 || int64(v.Cash) != int64(p.BattleCash)-int64(p.BattleSpent)+int64(p.RewardCash) {
				r.t.Fatal("Caiyi ordinary battle reward/loss mismatch", p.BattleCash, p.BattleSpent, p.RewardCash, v.Cash)
			}
			p.Wins++
		}
	}
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	milestones := r.milestones
	if milestones == nil {
		milestones = caiyiForestReadings
	}
	for name, token := range milestones {
		if !p.Seen[name] && strings.Contains(text, token) {
			p.Seen[name] = true
			r.save(name + "-reading")
		}
	}
	if (v.Scene == 142 || v.Scene == 143 || v.Scene == 144) && !p.Saved[fmt.Sprint("flashback-", v.Scene)] {
		r.save(fmt.Sprint("flashback-", v.Scene))
	}
	v = r.visible()
	if d := v.Dialogue; d != nil {
		if d.ID != p.LastPrompt {
			p.LastPrompt = d.ID
			p.Readings++
			r.t.Log("Caiyi forest reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
	} else if b := v.Battle; b != nil {
		if !p.InBattle {
			p.InBattle, p.BattleKind, p.BattleCash, p.RewardCash = true, 0, v.Cash, -1
			p.BattleSpent = 0
			p.Battles++
			for _, enemy := range b.Enemies {
				if enemy.Object == 468 || enemy.Object == 435 || r.scriptedEnemies[enemy.Object] {
					p.BattleKind = enemy.Object
				}
			}
			partySize := r.battlePartySize
			if partySize == 0 {
				partySize = 2
			}
			if len(v.Party) != partySize {
				r.t.Fatal("Caiyi battle did not preserve original Li/Yue party", v.Party)
			}
			r.save(fmt.Sprint("battle-", p.Battles))
			r.t.Log("Caiyi forest battle", p.Battles, b.Enemies)
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			p.RewardCash = cash
		}
		if p.BattleKind != 0 {
			for _, member := range b.Players {
				if member.State == 2 && !p.Saved[fmt.Sprint("selected-", p.BattleKind)] {
					r.save(fmt.Sprint("selected-", p.BattleKind))
					break
				}
			}
		}
		var a []byte
		var err error
		manual := false
		if r.manualBattle != nil {
			a, manual = r.manualBattle(v)
		}
		if !manual && r.physicalAttacks {
			if next, ok := caiyiForestPhysicalAttack(v); ok {
				a = raw(next)
			}
		} else if !manual {
			a, err = r.m.BotAction(r.c, r.g)
			if err != nil {
				r.t.Fatal(err)
			}
		}
		if len(a) > 0 {
			context := campaignBotContext(r.c)
			if manual {
				context = r.c
			}
			r.g, err = r.m.Apply(context, r.g, a)
			if err != nil {
				r.t.Fatal(err)
			}
			p.Commands++
		}
	} else if v.Phase == "menu" && v.Menu.Kind != 5 && !r.allowedMenus[v.Menu.Kind] && v.Seq == v.AppliedSeq {
		r.t.Fatal("unexpected original Caiyi menu", v.Menu)
	}
	r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
	var err error
	r.g, err = r.m.OnTimer(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
}

func caiyiForestPhysicalAttack(v pal.View) (pal.Action, bool) {
	b := v.Battle
	if v.Phase != "battle" || b == nil || b.Phase != 0 || b.State == 0 || v.Seq != v.AppliedSeq {
		return pal.Action{}, false
	}
	a := pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4, Keys: 2}
	switch b.State {
	case 1:
		if b.Menu != 0 {
			a.Keys = 1
		} else if b.Selected != 0 {
			a.Direction, a.Keys = 2, 16
		}
	case 2, 4:
		if b.Action != 2 {
			a.Keys = 1
		} else if b.State == 2 {
			target := -1
			for i, e := range b.Enemies {
				if e.Object != 0 {
					target = i
				}
			}
			if target >= 0 && b.Target != target {
				if target > b.Target {
					a.Direction, a.Keys = 3, 32
				} else {
					a.Direction, a.Keys = 1, 8
				}
			}
		}
	case 3, 5:
		a.Keys = 1
	default:
		return pal.Action{}, false
	}
	return a, true
}

func (r *caiyiForestDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 40000; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("Caiyi forest failed to return to original field", r.visible().Phase)
	return pal.View{}
}

func (r *caiyiForestDriver) inventoryCounts(names ...string) map[string]int {
	r.t.Helper()
	r.field()
	for i := 0; i < 40000; i++ {
		v := r.visible()
		if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			break
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			r.keys(2048, 4)
		}
		if v.Phase == "menu" && v.Menu.Kind != 5 {
			r.t.Fatal("Caiyi inventory request entered an unexpected original menu", v.Menu.Kind)
		}
		r.progress()
	}
	if r.visible().Menu.Kind != 5 {
		r.t.Fatal("Caiyi original inventory did not open")
	}
	r.keys(65536, 4)
	r.progress()
	r.progress()
	amounts, previous := map[string]int{}, ""
	for i := 0; i < 64; i++ {
		v := r.visible()
		for _, name := range names {
			if n, ok := campaignInventoryAmount(v, name); ok {
				amounts[name] = n
			}
		}
		page := originalVisibleText(v, false)
		if page == previous {
			break
		}
		previous = page
		r.keys(128, 4)
		r.progress()
		r.progress()
	}
	r.keys(1, 4)
	r.field()
	return amounts
}

func (r *caiyiForestDriver) reserve(target int) {
	scenes := r.reserveScenes
	if scenes == nil {
		scenes = []int{109, 116, 117, 118, 139, 140}
	}
	for _, scene := range scenes {
		r.w.avoidTouches[scene] = nil
		for i := r.tables.Scenes[scene-1].FirstEvent; i < r.tables.Scenes[scene].FirstEvent; i++ {
			e, portal := r.tables.Events[i], false
			for pc := int(e.TriggerScript); pc > 0 && pc < int(e.TriggerScript)+6 && pc < len(r.tables.Scripts); pc++ {
				s := r.tables.Scripts[pc]
				portal = portal || s.Opcode == 0x59
				if s.Opcode <= 3 {
					break
				}
			}
			if int(i)+1 != target && e.State > 0 && e.Sprite == 0 && e.TriggerMode >= 4 && portal {
				r.w.avoidTouches[scene] = append(r.w.avoidTouches[scene], [3]int{int(e.X), int(e.Y), (int(e.TriggerMode)-4)*32 + 16})
			}
		}
	}
}

func (r *caiyiForestDriver) search(event, x, y int) {
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

func TestOriginalPoisonForestThirteenRewardsAndSavedNoDuplication(t *testing.T) {
	name := "caiyi-forest-step-5-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables, physicalAttacks: true,
		proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	r.reserve(-1)
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("poison-rewards-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	initial := r.field()
	if initial.Scene != 140 || initial.Map != 129 || len(initial.Party) != 2 {
		t.Fatal("poison rewards source is not the actual forest entry", initial.Scene, initial.Map, initial.Party)
	}
	spots := []struct {
		Event, Sprite, Amount int
		Item, Notice          string
	}{
		{2360, 68, 1, "试炼果", "获得试炼果"}, {2369, 68, 1, "龙涎草", "摘得龙涎草"},
		{2359, 68, 1, "鬼枯藤", "摘得鬼枯藤"}, {2361, 68, 1, "忘魂花", "摘得忘魂花"},
		{2371, 68, 1, "止血草", "摘得止血草"}, {2368, 68, 1, "九节菖蒲", "获得九节菖蒲"},
		{2365, 10, 1, "灵山仙芝", "获得灵山仙芝"}, {2370, 10, 9, "蜘蛛卵", "获得９个蜘蛛卵"},
		{2367, 10, 1, "十里香", "获得十里香"}, {2362, 10, 1, "缠魂丝", "获得缠魂丝"},
		{2366, 10, 1, "醍醐香", "获得醍醐香"}, {2363, 10, 9, "蜘蛛卵", "获得９个蜘蛛卵"},
		{2364, 10, 1, "蜂巢", "获得蜂巢"},
	}
	names := []string{}
	for _, q := range spots {
		names = append(names, q.Item)
	}
	expected := r.inventoryCounts(names...)
	seen, frames := map[int]bool{}, map[int]int{}
	claimed := []map[string]any{}
	for _, q := range spots {
		x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
		label := fmt.Sprint("reward-", q.Event)
		r.notice = func(v pal.View) {
			if v.Field != nil {
				for _, a := range v.Field.Actors {
					if a[0] == 1 && a[1] == q.Event && a[5] > frames[q.Event] {
						frames[q.Event] = a[5]
					}
				}
			}
			if !seen[q.Event] && strings.Contains(hiddenNoticeText(v), q.Notice) {
				seen[q.Event] = true
				r.save(label + "-notice")
			}
		}
		t.Log("poison forest original reward", q.Event, q.Item, q.Amount)
		for i := 0; ; i++ {
			v := r.field()
			if v.Scene != 140 || i > 1600 {
				t.Fatal("poison forest ordinary collection approach failed", q, v.Scene, v.X, v.Y)
			}
			if fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				break
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 96)
			r.progress()
		}
		v := r.field()
		present := false
		for _, actor := range v.Field.Actors {
			if actor[0] == 1 && actor[1] == q.Event && actor[2] == q.Sprite {
				present = true
			}
		}
		if !present {
			t.Fatal("original poison herb/chest sprite missing before pickup", q)
		}
		r.save(label + "-unpicked")
		visits := map[[2]int]int{}
		for i := 0; i < 300 && !seen[q.Event]; i++ {
			v := r.field()
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 40 {
				t.Fatal("poison forest search stance blocked", q, at)
			}
			r.search(q.Event, x, y)
			r.progress()
		}
		if !seen[q.Event] {
			t.Fatal("original poison reward notice missing", q)
		}
		expected[q.Item] += q.Amount
		after := r.inventoryCounts(names...)
		if !bytes.Equal(raw(expected), raw(after)) {
			t.Fatal("original poison reward inventory quantity changed", q, expected, after)
		}
		if q.Sprite == 10 && frames[q.Event] != 1 {
			t.Fatal("original poison chest opening not observed", q.Event, frames[q.Event])
		}
		if q.Sprite == 68 {
			for _, actor := range r.field().Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					t.Fatal("original picked poison herb did not disappear", q)
				}
			}
		}
		r.save(label + "-collected")
		for retry := 0; retry < 3; retry++ {
			r.search(q.Event, x, y)
			r.progress()
			r.field()
		}
		if !bytes.Equal(raw(after), raw(r.inventoryCounts(names...))) {
			t.Fatal("original poison reward duplicated after saved searches", q)
		}
		r.save(label + "-repeated")
		claimed = append(claimed, map[string]any{"scene": 140, "event": q.Event, "position": [2]int{x, y}, "item": q.Item, "amount": q.Amount, "sprite": q.Sprite, "notice": q.Notice, "openFrame": frames[q.Event], "repeatSearches": 3})
	}
	v := r.field()
	if v.Scene != 140 || len(claimed) != 13 || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 || len(v.Party) != 2 || v.Party[0].Role != initial.Party[0].Role || v.Party[1].Role != initial.Party[1].Role {
		t.Fatal("optional poison rewards changed narrative/party", v.Scene, v.Party, r.proof)
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "optional ordinary exploration of original forest seven chests and six herbs; exact menu inventory, native appearance/opening/disappearance, timed acquisition saves and repeated searches; both nine-egg chests remain independent despite their shared script; test physical menu attacks never consume collected items; no native edits and no campaign-count increment", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "claimed": claimed, "inventory": expected, "party": v.Party, "cash": v.Cash, "proof": r.proof})
	for name, data := range map[string][]byte{"poison-forest-rewards.json": report, "poison-forest-rewards-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
