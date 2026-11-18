package pal_test

// E2E first: the actual capital arrival loses the manor/illness/dinner chain;
// Lin's private duel rejects the original solo party or loses accepted moves
// after saving; help/funding, the paid Taoist and sleeping household cannot
// continue; original ritual chests or wine duplicate/disappear; the river rescue
// or accepted four-item ritual restarts after restore; Yue's original departure
// is lost. Coordinates/scripts here only plan ordinary test walkthrough input.
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

type capitalManorProof struct {
	Step, Battles, Wins, ScriptedFinishes, Readings, Saves, Commands, Decisions int
	InBattle, FatherBattle, FatherFinished, FatherSolo                          bool
	BattleCash, BeforeCash                                                      uint32
	RewardCash                                                                  int
	LastPrompt                                                                  uint64
	RitualBefore                                                                map[string]int
	CashChanges                                                                 map[string][2]uint32
	ChestFrames                                                                 map[int]int
	Seen, Saved                                                                 map[string]bool
}

type capitalManorDriver struct {
	*riverMedicineDriver
	proof  capitalManorProof
	tables *resources.Tables
	notice func(pal.View)
}

var capitalManorReadings = map[string]string{
	"welcome": "也叫我云姨", "illness": "最近一个月以来", "patient": "每次发病时却是一次比一次难受",
	"meal": "陪晋元吃", "beggar": "您真大方", "father-arrives": "您怎么会到这来",
	"father-challenge": "林家的七诀剑气", "father-accepts": "希望你能好好照顾她",
	"caiyi-herbs": "这树林里", "medicine-conflict": "你每天给我喝的药根本不是",
	"patient-wakes": "她一定是妖怪", "help": "可否请你们替我拿个主意",
	"funding": "得三万文钱", "taoist-price": "一万五、不二价", "taoist-paid": "成交！",
	"taoist-fails": "请来这个笨蛋", "house-asleep": "而刘晋元却不见了",
	"incense-candle": "得到檀香、蜡烛", "paper": "得到符纸", "manor-chest": "得到三千文钱", "wine": "谢谢您的惠顾",
	"river-rescue": "这一觉睡得好饱", "ritual-request": "顺便给我带一壶上好的",
	"ritual-accepted": "都弄齐了", "ritual-start": "千万不要离开我七步以外",
	"master-asleep": "他．．睡着了", "yue-leaves": "去找出妖怪呀",
}

var capitalManorSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{101, 1810, 115, ""}, {115, 2123, 119, ""}, {119, 2161, 116, ""}, {116, 2135, 117, ""},
	{117, 2149, 109, ""}, {109, 1997, 123, ""}, {123, 2203, 124, ""}, {124, 2205, 125, ""},
	{125, 2209, 125, "patient"}, {125, 2206, 124, ""}, {124, 2204, 123, ""}, {123, 2200, 122, "meal"},
	{122, 2196, 119, ""}, {119, 2163, 115, ""}, {115, 2124, 101, ""},
	{101, 1826, 101, "beggar"}, {101, 1810, 115, ""}, {115, 2123, 118, "father-accepts"},
	{118, 2153, 109, ""}, {109, 1997, 123, ""}, {123, 2203, 124, ""}, {124, 2205, 125, ""},
	{125, 2210, 126, "patient-wakes"}, {126, 2214, 124, ""}, {124, 2204, 123, ""}, {123, 2200, 109, ""},
	{109, 2006, 109, "help"}, {109, 2003, 109, "funding"}, {109, 1998, 117, ""},
	{117, 2150, 116, ""}, {116, 2136, 119, ""}, {119, 2163, 115, ""}, {115, 2124, 101, ""},
	{101, 1811, 128, ""}, {128, 2223, 129, ""}, {129, 2246, 127, "taoist-fails"},
	{127, 2216, 124, ""}, {124, 2204, 123, ""}, {123, 2200, 109, ""}, {109, 1998, 117, ""},
	{117, 2150, 116, ""}, {116, 2136, 119, ""}, {119, 2162, 120, ""},
	{120, 2186, 120, "house-asleep"}, {120, 2187, 120, "incense-candle"}, {120, 2188, 120, "paper"},
	{120, 2189, 120, "manor-chest"},
	{120, 2174, 119, ""}, {119, 2163, 115, ""}, {115, 2124, 101, ""},
	{101, 1838, 101, "ritual-request"}, {101, 1811, 128, ""}, {128, 2225, 128, "wine"},
	{128, 2222, 101, ""}, {101, 1839, 116, "yue-leaves"},
}

func TestOriginalCapitalManorIllnessDuelRitualAndSave(t *testing.T) {
	name := "campaign-toad-mountain-state.json"
	p := capitalManorProof{Seen: map[string]bool{}, Saved: map[string]bool{}, RitualBefore: map[string]int{}}
	if checkpoint := os.Getenv("PAL_CAPITAL_MANOR_CHECKPOINT"); checkpoint != "" {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n <= 0 || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported capital-manor test checkpoint", checkpoint)
			}
		}
		name = "capital-manor-" + checkpoint + "-state.json"
		data, err := os.ReadFile(campaignEvidencePath("capital-manor-"+checkpoint+"-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
	}
	if p.CashChanges == nil {
		p.CashChanges = map[string][2]uint32{}
	}
	if p.ChestFrames == nil {
		p.ChestFrames = map[int]int{}
	}
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &capitalManorDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p, tables: tables}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("capital-manor-failure-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	for r.proof.Step < len(capitalManorSteps) {
		q := capitalManorSteps[r.proof.Step]
		t.Log("capital manor waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		if q.Token == "beggar" || q.Token == "funding" || q.Token == "taoist-fails" || q.Token == "wine" || q.Token == "manor-chest" {
			r.proof.BeforeCash = r.field().Cash
		}
		if q.Token == "manor-chest" {
			r.save("manor-chest-before")
		}
		if q.Token == "incense-candle" && !r.proof.Saved["ritual-inventory-before"] {
			r.proof.RitualBefore = r.inventoryCounts("檀香", "蜡烛", "符纸", "酒")
			r.save("ritual-inventory-before")
		}
		visits := map[[2]int]int{}
		for i := 0; i < 2400; i++ {
			v := r.field()
			if q.Token != "" && r.proof.Seen[q.Token] || q.Token == "" && v.Scene == q.Dest {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("original capital-manor route entered unexpected scene", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 40 {
				t.Fatal("capital-manor walkthrough stance blocked", q, at)
			}
			e := tables.Events[q.Event-1]
			x, y := int(e.X), int(e.Y)
			for _, a := range v.Field.Actors {
				if a[0] == 1 && a[1] == q.Event {
					x, y = a[3], a[4]
				}
			}
			if e.TriggerMode > 0 && e.TriggerMode < 4 && fieldTestDistance(v.X, v.Y, x, y) <= 192 {
				r.search(q.Event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			}
			r.progress()
			if i%100 == 0 {
				t.Log("capital manor walk", i, v.Scene, v.X, v.Y, x, y)
			}
		}
		v := r.field()
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] {
			t.Fatal("original capital-manor waypoint incomplete", q, v.Scene, v.X, v.Y)
		}
		if q.Token == "beggar" && v.Cash != r.proof.BeforeCash/2 || q.Token == "funding" && v.Cash != r.proof.BeforeCash+30000 || q.Token == "taoist-fails" && v.Cash+15000 != r.proof.BeforeCash || q.Token == "wine" && v.Cash+100 != r.proof.BeforeCash || q.Token == "manor-chest" && v.Cash != r.proof.BeforeCash+3000 {
			t.Fatal("original manor funding/payment changed across its script", q.Token, r.proof.BeforeCash, v.Cash)
		}
		if q.Token == "beggar" || q.Token == "funding" || q.Token == "taoist-fails" || q.Token == "wine" || q.Token == "manor-chest" {
			r.proof.CashChanges[q.Token] = [2]uint32{r.proof.BeforeCash, v.Cash}
		}
		if q.Token == "incense-candle" || q.Token == "paper" {
			items := []string{"檀香", "蜡烛"}
			if q.Token == "paper" {
				items = []string{"符纸"}
			}
			after := r.inventoryCounts(items...)
			for _, name := range items {
				if after[name] != r.proof.RitualBefore[name]+1 {
					t.Fatal("original ritual chest quantity changed", name, r.proof.RitualBefore[name], after[name])
				}
			}
			r.save(q.Token + "-inventory")
			for retry := 0; retry < 3; retry++ {
				r.search(q.Event, xFrom(tables, q.Event), yFrom(tables, q.Event))
				r.progress()
				r.field()
			}
			if !bytes.Equal(raw(after), raw(r.inventoryCounts(items...))) {
				t.Fatal("ritual chest duplicated after saved repeat search", q.Event)
			}
		}
		if q.Token == "manor-chest" {
			r.save("manor-chest-collected")
			for retry := 0; retry < 3; retry++ {
				r.search(q.Event, xFrom(tables, q.Event), yFrom(tables, q.Event))
				r.progress()
				if r.field().Cash != v.Cash {
					t.Fatal("original optional manor chest duplicated after resume")
				}
			}
		}
		if q.Token == "wine" {
			p.RitualBefore = r.inventoryCounts("檀香", "蜡烛", "符纸", "酒")
			for _, item := range []string{"檀香", "蜡烛", "符纸", "酒"} {
				if p.RitualBefore[item] < 1 {
					t.Fatal("original ritual item absent before handover", item, p.RitualBefore)
				}
			}
			r.proof.RitualBefore = p.RitualBefore
			r.save("ritual-ready")
		}
		r.proof.Step++
		r.save(fmt.Sprint("step-", r.proof.Step))
	}
	v := r.field()
	for token := range capitalManorReadings {
		if !r.proof.Seen[token] {
			t.Fatal("original capital-manor milestone missing", token)
		}
	}
	for _, event := range []int{2187, 2188, 2189} {
		if r.proof.ChestFrames[event] < 1 {
			t.Fatal("original manor chest opening was not observed", event, r.proof.ChestFrames)
		}
	}
	if v.Scene != 116 || len(v.Party) != 1 || v.Party[0].Role != 0 || !r.proof.FatherFinished || !r.proof.FatherSolo || !r.proof.Saved["father-selected"] || r.proof.Wins+r.proof.ScriptedFinishes != r.proof.Battles {
		t.Fatal("capital-manor ending/duel/departure incomplete", r.proof, v.Scene, v.Party)
	}
	after := r.inventoryCounts("檀香", "蜡烛", "符纸", "酒")
	for name, n := range r.proof.RitualBefore {
		if after[name] != n-1 {
			t.Fatal("original ritual item not consumed once", name, n, after[name])
		}
	}
	r.save("ritual-consumed")
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "resumedTestRoute": os.Getenv("PAL_CAPITAL_MANOR_CHECKPOINT") != "", "scope": "original manor illness/dinner, city clue and half-cash payment, solo Lin duel, medicine dispute, actual funding and paid Taoist, sleeping household, original ritual chests/wine and optional cash chest, river master rescue and four-item handover/ritual/Yue departure; fresh-runtime continuation and stale-input rejection, no native scene/script/stats edits; walkthrough only", "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "ritualAfter": after, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-capital-manor-state.json": r.g.Data, "campaign-capital-manor.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func xFrom(t *resources.Tables, event int) int { return int(t.Events[event-1].X) }
func yFrom(t *resources.Tables, event int) int { return int(t.Events[event-1].Y) }

func (r *capitalManorDriver) save(name string) {
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
		r.t.Fatal("capital-manor save changed original continuation", name)
	}
	if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("capital-manor save accepted old input", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err = os.WriteFile(campaignEvidencePath("capital-manor-"+name+"-"+kind+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *capitalManorDriver) progress() {
	r.t.Helper()
	v, p := r.visible(), &r.proof
	if r.notice != nil {
		r.notice(v)
		v = r.visible()
	}
	if v.Field != nil {
		for _, actor := range v.Field.Actors {
			if actor[0] == 1 && (actor[1] == 2187 || actor[1] == 2188 || actor[1] == 2189) && actor[5] > p.ChestFrames[actor[1]] {
				p.ChestFrames[actor[1]] = actor[5]
			}
		}
	}
	for _, member := range v.Party {
		if member.Controller != 0 || member.Role != 0 && member.Role != 2 {
			r.t.Fatal("capital manor lost original party ownership", v.Party)
		}
	}
	if p.InBattle && v.Battle == nil {
		p.InBattle = false
		if p.FatherBattle {
			p.FatherFinished = true
			p.ScriptedFinishes++
			r.save("father-finished")
		} else {
			if p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
				r.t.Fatal("ordinary capital battle lost or cash reward changed", p.BattleCash, p.RewardCash, v.Cash)
			}
			p.Wins++
		}
	}
	text := hiddenNoticeText(v) + originalVisibleText(v, false)
	if v.Dialogue != nil {
		text += v.Dialogue.Text
	}
	text = strings.NewReplacer("\n", "", "　", "", " ", "").Replace(text)
	for name, token := range capitalManorReadings {
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
			r.t.Log("capital manor reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
	} else if b := v.Battle; b != nil {
		if !p.InBattle {
			p.InBattle, p.FatherBattle, p.BattleCash, p.RewardCash = true, false, v.Cash, -1
			p.Battles++
			for _, enemy := range b.Enemies {
				p.FatherBattle = p.FatherBattle || enemy.Object == 525
			}
			if p.FatherBattle {
				if len(v.Party) != 1 || v.Party[0].Role != 0 {
					r.t.Fatal("original Lin duel did not use Li alone", v.Party)
				}
				p.FatherSolo = true
			}
			r.save(fmt.Sprint("battle-", p.Battles))
			r.t.Log("capital manor battle", p.Battles, b.Enemies)
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			p.RewardCash = cash
		}
		if p.FatherBattle && len(b.Players) == 1 && b.Players[0].State == 2 && !p.Saved["father-selected"] {
			r.save("father-selected")
		}
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive {
			r.t.Fatal("capital-manor battle defeated party")
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
	} else if v.Phase == "menu" && v.Menu.Kind != 5 && v.Seq == v.AppliedSeq {
		yes := -1
		for i, item := range v.Menu.Items {
			if item.Label == "是" {
				yes = i
			}
		}
		if yes < 0 {
			r.t.Fatal("unexpected original capital-manor menu", v.Menu)
		}
		if v.Menu.Selected != yes {
			r.keys(32, 4)
		} else {
			name := fmt.Sprint("choice-", p.Decisions)
			if !p.Saved[name] {
				r.save(name)
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

func (r *capitalManorDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 16000; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("capital manor did not return to original field", r.visible().Phase)
	return pal.View{}
}

func (r *capitalManorDriver) inventoryCounts(names ...string) map[string]int {
	r.t.Helper()
	r.field()
	r.keys(2048, 4)
	for i := 0; i < 128 && (r.visible().Menu.Kind != 5 || r.visible().Seq != r.visible().AppliedSeq); i++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 5 {
		r.t.Fatal("original manor inventory did not open")
	}
	r.keys(65536, 4)
	r.progress()
	r.progress()
	amounts := map[string]int{}
	previous := ""
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

func (r *capitalManorDriver) search(event, x, y int) {
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

func (r *capitalManorDriver) reserve(target int) {
	for _, scene := range []int{101, 109, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129} {
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

// The guide's manor-chair/wall searches must be real invisible original
// rewards, preserve their own claim state and timed notice, and add exactly
// one item across fresh-runtime saves. This is an optional exploration branch
// from the earned manor arrival; it does not increment the campaign count.
func TestOriginalCapitalManorHiddenShoesAndSavedNoDuplication(t *testing.T) {
	name := "capital-manor-step-2-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &capitalManorDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables,
		proof: capitalManorProof{Seen: map[string]bool{}, Saved: map[string]bool{}, RitualBefore: map[string]int{}, CashChanges: map[string][2]uint32{}, ChestFrames: map[int]int{}}}
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("capital-hidden-failure-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	initial := r.field()
	if initial.Scene != 119 || len(initial.Party) != 2 || initial.Party[0].Role != 2 || initial.Party[1].Role != 0 {
		t.Fatal("hidden manor source is not the earned original arrival", initial.Scene, initial.Party)
	}
	baseline := r.inventoryCounts("绣花鞋", "十里香")
	claimed := []map[string]any{}
	spots := []struct {
		Event int
		Item  string
	}{{2172, "绣花鞋"}}
	if os.Getenv("PAL_CAPITAL_INCENSE_AUDIT") == "1" {
		// This original record is retained as unverified. The explicit audit
		// repeats the failed ordinary approach; it never relocates the item.
		spots = append(spots, struct {
			Event int
			Item  string
		}{2173, "十里香"})
	}
	for _, q := range spots {
		x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
		key := fmt.Sprint("hidden-", q.Event)
		r.notice = func(v pal.View) {
			if !r.proof.Seen[key] && strings.Contains(hiddenNoticeText(v), "发现"+q.Item) {
				r.proof.Seen[key] = true
				r.save(key + "-notice")
			}
		}
		r.reserve(q.Event)
		r.save(key + "-unpicked")
		visits := map[[2]int]int{}
		for i := 0; i < 600 && !r.proof.Seen[key]; i++ {
			v := r.field()
			if v.Scene != 119 {
				t.Fatal("hidden manor search left the original hall", q, v.Scene)
			}
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					t.Fatal("hidden manor reward gained an invented actor", q)
				}
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 40 {
				t.Fatal("hidden manor ordinary search stance blocked", q, at)
			}
			if fieldTestDistance(v.X, v.Y, x, y) <= 192 {
				r.search(q.Event, x, y)
			} else {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			}
			r.progress()
		}
		if !r.proof.Seen[key] {
			t.Fatal("original hidden manor notice absent", q)
		}
		baseline[q.Item]++
		after := r.inventoryCounts("绣花鞋", "十里香")
		if !bytes.Equal(raw(baseline), raw(after)) {
			t.Fatal("hidden manor inventory changed original quantities", baseline, after)
		}
		r.save(key + "-collected")
		for retry := 0; retry < 3; retry++ {
			r.search(q.Event, x, y)
			r.progress()
			r.field()
		}
		if !bytes.Equal(raw(after), raw(r.inventoryCounts("绣花鞋", "十里香"))) {
			t.Fatal("hidden manor reward duplicated after saved searches", q)
		}
		r.save(key + "-repeated")
		claimed = append(claimed, map[string]any{"scene": 119, "event": q.Event, "position": [2]int{x, y}, "item": q.Item, "amount": 1, "notice": "发现" + q.Item, "invisible": true, "repeatSearches": 3})
	}
	final := r.field()
	if initial.Cash != final.Cash || !bytes.Equal(raw(initial.Party), raw(final.Party)) {
		t.Fatal("optional manor searches changed cash/party", initial.Cash, final.Cash, initial.Party, final.Party)
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "actual invisible original manor shoes search, timed acquisition save, exact menu inventory and repeated searches; optional branch only, incense is separately unverified unless the explicit audit passes; walkthrough data never used by production bots", "incenseAuditEnabled": os.Getenv("PAL_CAPITAL_INCENSE_AUDIT") == "1", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "claimed": claimed, "inventory": baseline, "cash": final.Cash, "party": final.Party, "proof": r.proof})
	if err := os.WriteFile(campaignEvidencePath("capital-hidden-rewards.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}
