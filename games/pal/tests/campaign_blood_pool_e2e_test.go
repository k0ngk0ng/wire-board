package pal_test

// E2E first: blood-pool movement stalls when local prediction is unavailable;
// plate/bridge state resets on resume; ordinary enemy or king battles fail or
// pay twice; the original bead confrontation/lore is skipped; the earth orb is
// absent/duplicated; three-role manual ownership or suspended readings are
// lost; a restored runtime accepts pre-save controls. Static map coordinates
// and switch identities are walkthrough data only, never production bot data.
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

type bloodPoolProof struct {
	Route, Switches, Battles, Wins, Commands, Readings, Saves int
	Named, Bead, Agreement, Challenge, KingWon, Orb           bool
	Legend, Escape, Fate, Worry, Leave, Item                  bool
	InBattle, KingBattle                                      bool
	RewardXP, RewardCash                                      int
	BattleCash                                                uint32
	LastPrompt                                                uint64
	Saved                                                     map[string]bool
	Open                                                      map[int]bool
	Rewards                                                   [][2]int
}

type bloodPoolDriver struct {
	*riverMedicineDriver
	proof bloodPoolProof
}

type bloodPoolSwitch struct {
	Event, X, Y int
	Opens       []int
}

func TestOriginalBloodPoolRedGhostKingEarthOrbAndSave(t *testing.T) {
	name := "campaign-ghost-tomb-state.json"
	p := bloodPoolProof{Saved: map[string]bool{}, Open: map[int]bool{}}
	if os.Getenv("PAL_BLOOD_CHECKPOINT") == "failure" {
		name = "blood-pool-failure-state.json"
		b, err := os.ReadFile(campaignEvidencePath("blood-pool-failure-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
	}
	r := &bloodPoolDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"blood-pool-failure-state.json": r.g.Data, "blood-pool-failure-proof.json": raw(r.proof), "blood-pool-failure-view.json": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	r.w = campaignMapWalker{allowTouch: true, staticOnly: true, avoidTouches: map[int][][3]int{}}
	for r.proof.Route < len(bloodPoolRoute) {
		q := bloodPoolRoute[r.proof.Route]
		r.reserve()
		t.Log("blood-pool switch", r.proof.Route, q)
		r.walk(q.X, q.Y, 0)
		pressed := false
		for i := 0; i < 64; i++ {
			v := r.field()
			for _, draw := range v.Draws {
				if draw.Kind == 0 && draw.C == q.Event && draw.D == 1 && draw.B == 1 {
					pressed = true
				}
			}
			if pressed {
				break
			}
			r.progress()
		}
		if !pressed {
			t.Fatal("original blood-pool plate was not pressed", q, r.visible().Draws)
		}
		for _, event := range q.Opens {
			r.proof.Open[event] = true
		}
		r.proof.Switches++
		r.proof.Route++
		r.save(fmt.Sprint("switch-", r.proof.Route))
	}
	r.reserve()
	r.walk(752, 104, 96)
	for i := 0; i < 1024 && !r.proof.Leave; i++ {
		r.progress()
	}
	v := r.field()
	if v.Scene != 60 || r.proof.Switches != len(bloodPoolRoute) || !r.proof.Named || !r.proof.Bead || !r.proof.Agreement || !r.proof.Challenge || !r.proof.KingWon || !r.proof.Orb || !r.proof.Legend || !r.proof.Escape || !r.proof.Fate || !r.proof.Worry || !r.proof.Leave || r.proof.Battles != r.proof.Wins {
		t.Fatal("original blood-pool story incomplete", r.proof, v.Scene)
	}
	r.keys(2048, 4)
	for scans := 0; scans < 128; scans++ {
		v = r.visible()
		if v.Phase != "menu" || v.Menu.Kind != 5 || v.Seq != v.AppliedSeq {
			r.progress()
			continue
		}
		if amount, found := campaignInventoryAmount(v, "土灵珠"); found {
			if amount != 1 {
				t.Fatal("original earth orb duplicated", amount)
			}
			r.proof.Item = true
			r.save("earth-orb-inventory")
			r.keys(1, 4)
			break
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	v = r.field()
	if !r.proof.Item || v.Scene != 60 {
		t.Fatal("original earth orb unavailable", r.proof, v.Scene)
	}
	r.save("complete")
	source, err := os.ReadFile(campaignEvidencePath("campaign-ghost-tomb-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "ordinary original blood-pool plates and movement from genuine tomb save; original king confrontation/battle, exact displayed cash reward, earth orb acquired once, original five-orb lore and Ling dialogue, one owned three-role party, fresh-runtime saves with exact public continuity and old input rejected; wave rendering and autonomous exploration are not accepted by this test", "scene": v.Scene, "map": v.Map, "party": v.Party, "proof": r.proof}, "", "  ")
	for name, data := range map[string][]byte{"campaign-blood-pool-state.json": r.g.Data, "campaign-blood-pool.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *bloodPoolDriver) save(name string) {
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
		r.t.Fatal("blood-pool save changed original continuation", name)
	}
	if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("blood-pool accepted old controls", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err := os.WriteFile(campaignEvidencePath("blood-pool-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *bloodPoolDriver) progress() {
	r.t.Helper()
	v := r.visible()
	p := &r.proof
	if len(v.Party) != 3 || v.Party[0].Role != 0 || v.Party[1].Role != 1 || v.Party[2].Role != 2 {
		r.t.Fatal("blood-pool lost original party", v.Party)
	}
	for _, member := range v.Party {
		if member.Controller != 0 {
			r.t.Fatal("blood-pool lost solo ownership", v.Party)
		}
	}
	if p.InBattle && v.Battle == nil {
		if p.RewardXP < 0 || p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
			r.t.Fatal("blood-pool reward missing/repeated or defeat", p, v.Cash)
		}
		p.InBattle = false
		p.Wins++
		p.Rewards = append(p.Rewards, [2]int{p.RewardXP, p.RewardCash})
		if p.KingBattle {
			p.KingWon = true
			r.save("king-victory")
			v = r.visible()
		}
	}
	if d := v.Dialogue; d != nil {
		if d.ID != p.LastPrompt {
			p.LastPrompt = d.ID
			p.Readings++
			p.Named = p.Named || strings.Contains(d.Text, "主人～他就是赤鬼王")
			p.Bead = p.Bead || strings.Contains(d.Text, "是玉佛珠在帮你们")
			p.Agreement = p.Agreement || strings.Contains(d.Text, "井水不犯河水")
			p.Challenge = p.Challenge || strings.Contains(d.Text, "今日我等便要为世间除害")
			p.Orb = p.Orb || strings.Contains(d.Text, "土灵珠在这妖怪身上")
			p.Legend = p.Legend || strings.Contains(d.Text, "女娲大神聚天地灵气")
			p.Escape = p.Escape || strings.Contains(d.Text, "我们逃脱这洞窟就靠这灵珠了")
			p.Fate = p.Fate || strings.Contains(d.Text, "不该再躲藏了")
			p.Worry = p.Worry || strings.Contains(d.Text, "都好深奥")
			p.Leave = p.Leave || strings.Contains(d.Text, "村民们在等我们的好消息")
			r.t.Log("blood-pool reading", p.Readings, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		for _, mark := range []struct {
			name string
			done bool
		}{{"named-reading", p.Named}, {"bead-reading", p.Bead}, {"agreement-reading", p.Agreement}, {"challenge-reading", p.Challenge}, {"orb-reading", p.Orb}, {"legend-reading", p.Legend}, {"escape-reading", p.Escape}, {"fate-reading", p.Fate}, {"worry-reading", p.Worry}, {"leave-reading", p.Leave}} {
			if mark.done && !p.Saved[mark.name] {
				r.save(mark.name)
			}
		}
		v = r.visible()
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
	} else if v.Battle != nil {
		if !p.InBattle {
			p.InBattle = true
			p.Battles++
			p.BattleCash = v.Cash
			p.RewardXP, p.RewardCash = -1, -1
			p.KingBattle = false
			for _, enemy := range v.Battle.Enemies {
				if enemy.Object == 473 {
					p.KingBattle = true
				}
			}
			if p.Battles > 60 {
				r.t.Fatal("blood-pool repeated too many battles", p)
			}
			if p.KingBattle {
				if !p.Challenge || p.KingWon {
					r.t.Fatal("king battle out of original order")
				}
				r.save("king-battle")
				v = r.visible()
			}
			r.t.Log("blood-pool battle", p.Battles, p.KingBattle, v.Battle.Enemies)
		}
		if xp, cash, ok := campaignBattleReward(v); ok {
			p.RewardXP, p.RewardCash = xp, cash
		}
		if p.KingBattle && !p.Saved["king-summons"] && len(v.Battle.Enemies) > 1 {
			occupied := 0
			for _, e := range v.Battle.Enemies {
				if e.Object != 0 {
					occupied++
				}
			}
			if occupied > 1 {
				r.save("king-summons")
				v = r.visible()
			}
		}
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive {
			r.t.Fatal("original blood-pool party defeated")
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

func (r *bloodPoolDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 4096; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("original blood-pool flow never returned to field", r.visible().Phase)
	return pal.View{}
}

func (r *bloodPoolDriver) walk(x, y, reach int) {
	r.t.Helper()
	visits := map[[2]int]int{}
	for i := 0; i < 1800; i++ {
		v := r.field()
		if r.proof.KingWon {
			return
		}
		if v.Scene != 60 {
			r.t.Fatal("blood-pool route left original scene", v.Scene)
		}
		if fieldTestDistance(v.X, v.Y, x, y) <= reach {
			return
		}
		place := [2]int{v.X, v.Y}
		visits[place]++
		if visits[place] > 24 {
			r.t.Fatal("blood-pool waypoint blocked", x, y, v.X, v.Y)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, reach)
		r.progress()
		if i%100 == 0 {
			r.t.Log("blood-pool walking", i, v.X, v.Y, "toward", x, y)
		}
	}
	r.t.Fatal("original blood-pool waypoint not reached", x, y)
}

func (r *bloodPoolDriver) reserve() {
	r.w.avoidTouches[60] = nil
	for event, p := range bloodPoolBlockers {
		if !r.proof.Open[event] {
			r.w.avoidTouches[60] = append(r.w.avoidTouches[60], [3]int{p[0], p[1], 16})
		}
	}
}

var bloodPoolRoute = []bloodPoolSwitch{
	{Event: 996, X: 1296, Y: 1432, Opens: []int{995}},
	{Event: 993, X: 1296, Y: 1736, Opens: []int{987, 992}},
	{Event: 1014, X: 928, Y: 1872, Opens: []int{1013}},
	{Event: 1011, X: 848, Y: 1656, Opens: []int{1010}},
	{Event: 1008, X: 624, Y: 1896, Opens: []int{1007}},
	{Event: 1005, X: 800, Y: 1568, Opens: []int{998, 1004}},
	{Event: 1024, X: 976, Y: 1224, Opens: []int{1022, 1023}},
	{Event: 1019, X: 1344, Y: 1280, Opens: []int{1017, 1018}},
	{Event: 1029, X: 992, Y: 1136, Opens: []int{1027, 1028}},
	{Event: 1032, X: 1040, Y: 1288, Opens: []int{1031}},
}
var bloodPoolBlockers = map[int][2]int{
	987:  {1328, 1688},
	992:  {1392, 1656},
	995:  {1296, 1512},
	998:  {880, 1528},
	1004: {800, 1488},
	1007: {864, 1872},
	1010: {704, 1872},
	1013: {720, 1864},
	1017: {1024, 1152},
	1018: {1008, 1240},
	1022: {1040, 1160},
	1023: {1024, 1248},
	1027: {1056, 1168},
	1028: {1040, 1256},
	1031: {1072, 1176},
	1034: {1504, 1312},
	1035: {1568, 1280},
	1037: {1536, 464},
	1041: {1584, 440},
	1044: {1376, 240},
	1073: {1424, 1640},
	1074: {304, 1608},
	1075: {592, 1320},
	1076: {832, 1840},
	1077: {1488, 1128},
	1078: {1680, 856},
	1079: {672, 544},
}
