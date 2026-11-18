package pal_test

// Test first: wrong original forest/temple route; missing greeter/abbot chain;
// ordinary battles fail or rewards repeat; the original temporary Ling'er-only
// scene is replaced with an invented party; illusion/portrait/continuation
// fails; bead acquisition duplicates or its equipment menu cannot target Ling;
// fresh runtimes lose events/resources/menus or accept pre-save controls.
// Original routes and enemy identities below are E2E walkthrough data only.
import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type jadeTempleProof struct {
	Greeter, Aura, Butcher, Abbot, Shave, CaptorWon, AbbotWon              bool
	LingOnly, Revealed, Lecture, Named, Carry, Thanks, Brothers, Fulfilled bool
	Bead                                                                   bool
	InBattle                                                               bool
	BattleKind, RewardXP, RewardCash                                       int
	BattleCash                                                             uint32
	Battles, Wins, Commands, Readings, Saves, Scans                        int
	LastPrompt                                                             uint64
	Saved                                                                  map[string]bool
	Rewards                                                                [][2]int
}

func TestOriginalJadeTempleBattlesBeadAndSave(t *testing.T) {
	name := "campaign-white-river-state.json"
	p := jadeTempleProof{Saved: map[string]bool{}}
	if os.Getenv("PAL_TEMPLE_CHECKPOINT") == "failure" {
		name = "jade-temple-failure-state.json"
		b, err := os.ReadFile(campaignEvidencePath("jade-temple-failure-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	runJadeTemple(t, m, c, g, p)
}

func runJadeTemple(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, p jadeTempleProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{allowTouch: true}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"jade-temple-failure-state.json": g.Data, "jade-temple-failure-proof.json": raw(p), "jade-temple-failure-view.json": raw(view(t, m, g, 0))} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	save := func(name string) {
		t.Helper()
		before := view(t, m, g, 0)
		var err error
		g, err = m.SaveControl(c, g, true)
		if err != nil {
			t.Fatal(err)
		}
		c.Now = c.Now.Add(24 * time.Hour)
		m, _ = nativeModule(t)
		g, err = m.ResumeSaved(c, g)
		if err != nil {
			t.Fatal(err)
		}
		normalize := func(v pal.View) pal.View {
			v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
			return v
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(view(t, m, g, 0)))) {
			t.Fatal("temple save changed original continuation", name)
		}
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("temple save accepted old controls", name)
		}
		p.Saves++
		p.Saved[name] = true
		for suffix, data := range map[string][]byte{"state": g.Data, "proof": raw(p)} {
			if err := os.WriteFile(campaignEvidencePath("jade-temple-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for step := 0; step < 16000; step++ {
		v := view(t, m, g, 0)
		for _, member := range v.Party {
			if member.Controller != 0 || member.Role < 0 || member.Role > 2 {
				t.Fatal("temple changed original solo ownership", v.Party)
			}
		}
		if p.InBattle && v.Battle == nil {
			if p.RewardXP < 0 || p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
				t.Fatal("temple reward missing/repeated or defeat", v.Cash, p)
			}
			p.InBattle = false
			p.Wins++
			p.Rewards = append(p.Rewards, [2]int{p.RewardXP, p.RewardCash})
			if p.BattleKind == 1 {
				p.CaptorWon = true
				save("captor-victory")
			}
			if p.BattleKind == 2 {
				p.AbbotWon = true
				save("abbot-victory")
			}
			v = view(t, m, g, 0)
		}
		// Original battle return and party reduction may share one timer
		// batch. Verify the reward first, then the script's Ling'er-only cast.
		if len(v.Party) == 1 {
			if !p.AbbotWon || v.Party[0].Role != 1 {
				t.Fatal("temple lost the original temporary Ling'er role", v.Party, p)
			}
			p.LingOnly = true
			if !p.Saved["ling-only"] {
				save("ling-only")
				v = view(t, m, g, 0)
			}
		} else if len(v.Party) != 3 {
			t.Fatal("temple gained an invented party", v.Party)
		}
		if d := v.Dialogue; d != nil {
			if d.ID != p.LastPrompt {
				p.LastPrompt = d.ID
				p.Readings++
				p.Greeter = p.Greeter || strings.Contains(d.Text, "是来参佛的吗")
				p.Aura = p.Aura || strings.Contains(d.Text, "有股妖气")
				p.Butcher = p.Butcher || strings.Contains(d.Text, "俺本来是个杀猪的")
				p.Abbot = p.Abbot || strings.Contains(d.Text, "想请大师下山治退尸妖")
				p.Shave = p.Shave || strings.Contains(d.Text, "出家当和尚")
				p.Revealed = p.Revealed || strings.Contains(d.Text, "我乃达摩法师所持佛珠所化")
				p.Lecture = p.Lecture || strings.Contains(d.Text, "修业之本在于舍己助人")
				p.Named = p.Named || strings.Contains(d.Text, "以后你就叫做")
				p.Carry = p.Carry || strings.Contains(d.Text, "请将我配带在身上")
				p.Thanks = p.Thanks || strings.Contains(d.Text, "不必再受妖怪的控制")
				p.Brothers = p.Brothers || strings.Contains(d.Text, "韩家小姐很思念你")
				p.Fulfilled = p.Fulfilled || strings.Contains(d.Text, "功德圆满")
				t.Log("jade temple reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
			}
			for _, mark := range []struct {
				name string
				done bool
			}{{"greeter", p.Greeter}, {"aura", p.Aura}, {"butcher", p.Butcher}, {"abbot-request", p.Abbot}, {"shave", p.Shave}, {"revealed", p.Revealed}, {"lecture", p.Lecture}, {"named", p.Named}, {"carry", p.Carry}, {"thanks", p.Thanks}, {"brothers", p.Brothers}, {"fulfilled", p.Fulfilled}} {
				if mark.done && !p.Saved[mark.name] {
					save(mark.name)
				}
			}
			v = view(t, m, g, 0)
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		} else if v.Battle != nil {
			if !p.InBattle {
				p.InBattle = true
				p.Battles++
				p.BattleCash = v.Cash
				p.RewardXP, p.RewardCash = -1, -1
				p.BattleKind = 0
				for _, enemy := range v.Battle.Enemies {
					if enemy.Object == 482 {
						p.BattleKind = 1
					}
					if enemy.Object == 524 {
						p.BattleKind = 2
					}
				}
				if p.Battles > 30 {
					t.Fatal("temple repeated too many battles", p)
				}
				if p.BattleKind == 1 {
					if p.CaptorWon || !p.Shave {
						t.Fatal("captor battle out of story order")
					}
					save("captor-battle")
				}
				if p.BattleKind == 2 {
					if !p.CaptorWon || p.AbbotWon {
						t.Fatal("abbot battle out of story order")
					}
					save("abbot-battle")
				}
				t.Log("jade temple battle", p.Battles, p.BattleKind, v.Scene, v.Party, v.Battle.Enemies)
				v = view(t, m, g, 0)
			}
			if xp, cash, ok := campaignBattleReward(v); ok {
				p.RewardXP, p.RewardCash = xp, cash
			}
			alive := false
			for _, member := range v.Party {
				alive = alive || member.HP > 0
			}
			if !alive {
				t.Fatal("original temple party defeated")
			}
			a, err := m.BotAction(c, g)
			if err != nil {
				t.Fatal(err)
			}
			if len(a) > 0 {
				g, err = m.Apply(campaignBotContext(c), g, a)
				if err != nil {
					t.Fatal(err)
				}
				p.Commands++
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			p.Scans++
			if p.Scans > 128 {
				t.Fatal("earned jade bead inventory did not settle", originalVisibleText(v, false))
			}
			keys := uint32(32)
			if amount, found := campaignInventoryAmount(v, "玉佛珠"); found {
				if amount != 1 {
					t.Fatal("original jade bead duplicated", amount)
				}
				p.Bead = true
				if !p.Saved["bead-inventory"] {
					save("bead-inventory")
					v = view(t, m, g, 0)
				}
				keys = 1
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if v.Scene == 58 && p.Bead {
				if !p.Greeter || !p.Aura || !p.Butcher || !p.Abbot || !p.CaptorWon || !p.AbbotWon || !p.LingOnly || !p.Revealed || !p.Lecture || !p.Named || !p.Carry || !p.Thanks || !p.Brothers || !p.Fulfilled || p.Battles != p.Wins || len(v.Party) != 3 || v.Party[0].Role != 0 || v.Party[1].Role != 1 || v.Party[2].Role != 2 {
					t.Fatal("original jade temple story incomplete", p, v.Party)
				}
				save("complete")
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "developmentProfile": developmentHealthProfile(), "scope": "actual original forest and illusion temple, greeter/abbot, ordinary captor/abbot battles with exact displayed cash rewards, temporary Ling'er-only scene, original bead transformation and villagers released, one actual jade bead, restored manually owned party and fresh-runtime personal saves; bead equipment verified separately", "scene": v.Scene, "map": v.Map, "party": v.Party, "proof": p, "oldInputRejected": true}, "", "  ")
				for name, data := range map[string][]byte{"campaign-jade-temple-state.json": g.Data, "campaign-jade-temple.json": report} {
					if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				return m, c, g
			}
			stage := 0
			for _, done := range []bool{p.Greeter, p.Butcher, p.Shave, p.CaptorWon, p.AbbotWon, p.Named, p.Fulfilled} {
				if done {
					stage++
				}
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 32 {
				t.Fatal("temple walkthrough repeats a position", place, v.Field, p)
			}
			x, y, npc := 0, 0, 0
			switch v.Scene {
			case 54:
				x, y = 1312, 896
			case 53:
				x, y = 384, 1280
			case 50:
				x, y = 800, 544
			case 55:
				x, y = 352, 512
			case 57:
				if !p.Greeter {
					x, y, npc = 1088, 1104, 939
				} else if !p.Butcher {
					x, y = 1376, 864
				} else {
					x, y = 992, 560
				}
			case 59:
				x, y, npc = 656, 1000, 959
			case 58:
				if !p.Fulfilled {
					t.Fatal("temple illusion ended before its original story")
				}
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
			default:
				t.Fatal("temple left original story route", v.Scene, v.X, v.Y)
			}
			if x != 0 {
				if npc != 0 && fieldTestDistance(v.X, v.Y, x, y) <= 160 {
					g = campaignWalkTo(t, m, &c, g, v, x, y, npc)
				} else {
					reach := 0
					if npc != 0 {
						reach = 96
					}
					g = walker.walk(t, m, &c, g, v, x, y, reach)
				}
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("jade temple position", step, v.Scene, v.X, v.Y, v.Phase, v.Party)
		}
	}
	t.Fatal("original jade temple never completed", p)
	return m, c, g
}
