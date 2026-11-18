package pal_test

// Test first: Liu's rescue/overnight prerequisites are skipped; the original
// battle is lost or replayed; sleeping Ling'er cannot rejoin; Li's solo duel
// gains extra roles; the manor illness/garden/alarm sequence cannot advance;
// Ling'er remains after disappearing or Yueru joins prematurely; the snake
// encounter fails; save/resume loses script position, visible graphics,
// resources, party order or command selection; old controls survive a save.
// Coordinates and original story markers are test walkthrough data only.
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

type tournamentProof struct {
	Overnight                                                                                                                             bool
	InitialCash                                                                                                                           uint32
	Summoned                                                                                                                              bool
	Rescue, LiuThanks, LiuFarewell, Wake, Tournament, DuelWon, Manor, Illness, Garden, Horoscope, Alarm, Missing, Search, Companion, Cave bool
	Saved                                                                                                                                 map[string]bool
	BattleSaved                                                                                                                           [3]bool
	BattleScenes                                                                                                                          []int
	InBattle                                                                                                                              bool
	InnDownstairs, InnCounter                                                                                                             bool
	Readings, Saves, Battles                                                                                                              int
	LastPrompt                                                                                                                            uint64
}

func TestOriginalCampaignSuzhouTournamentAndManorSave(t *testing.T) {
	m, c, g := originalYueruMeeting(t, 1)
	runOriginalTournament(t, m, c, g, tournamentProof{})
}

// Explicit diagnostic entry: never substitutes a checkpoint for the canonical
// new-game chain above, and never edits native memory or original resources.
func TestOriginalCampaignSuzhouTournamentFromCheckpoint(t *testing.T) {
	entry := os.Getenv("PAL_TOURNAMENT_CHECKPOINT")
	if entry != "1" && entry != "failure" {
		t.Skip("set PAL_TOURNAMENT_CHECKPOINT=1 after the stage-3 E2E")
	}
	name := "campaign-yueru-meeting-state.json"
	proof := tournamentProof{}
	if entry == "failure" {
		name = "tournament-failure-state.json"
		b, err := os.ReadFile(campaignEvidencePath("tournament-failure-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &proof); err != nil {
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
	if entry == "1" && (v.Scene != 23 || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 1) {
		t.Fatal("not the actual stage-3 checkpoint", v.Scene, v.Party)
	}
	runOriginalTournament(t, m, c, g, proof)
}

func runOriginalTournament(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, proof tournamentProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	if proof.Saved == nil {
		proof.Saved = map[string]bool{}
	}
	startCash := view(t, m, g, 0).Cash
	if proof.InitialCash == 0 {
		proof.InitialCash = startCash
	}
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"tournament-failure-state.json": g.Data, "tournament-failure-proof.json": raw(proof), "tournament-failure-view.json": raw(view(t, m, g, 0))} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), b, 0600); err != nil {
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
		after := view(t, m, g, 0)
		normalize := func(v pal.View) pal.View {
			v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
			return v
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(after))) {
			t.Fatal("tournament save changed original visible state", name)
		}
		c.Actor = 0
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("tournament accepted pre-save controls", name)
		}
		proof.Saved[name], proof.Saves = true, proof.Saves+1
		if err := os.WriteFile(campaignEvidencePath("tournament-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	walkNPC := func(v pal.View, x, y, npc int) {
		for _, a := range v.Field.Actors {
			if a[0] == 1 && a[1] == npc {
				x, y = a[3], a[4]
			}
		}
		if fieldTestDistance(v.X, v.Y, x, y) > 160 {
			g = walker.walk(t, m, &c, g, v, x, y, 96)
		} else {
			g = campaignWalkTo(t, m, &c, g, v, x, y, npc)
		}
	}
	for step := 0; step < 10000; step++ {
		v := view(t, m, g, 0)
		seen := map[int]bool{}
		for _, p := range v.Party {
			if p.Controller != 0 || p.Role < 0 || p.Role > 2 || seen[p.Role] {
				t.Fatal("tournament changed original party ownership", v.Party)
			}
			seen[p.Role] = true
		}
		if !seen[0] || len(v.Party) > 2 || seen[1] && seen[2] || seen[2] && !proof.Missing {
			t.Fatal("tournament invented or lost original party", v.Party, proof)
		}
		if v.Cash < startCash {
			t.Fatal("original tournament route lost/restarted", v.Cash, v.Party)
		}
		if v.Battle == nil {
			proof.InBattle = false
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Rescue = proof.Rescue || strings.Contains(d.Text, "癞蛤蟆想吃天鹅肉")
				proof.LiuThanks = proof.LiuThanks || strings.Contains(d.Text, "刘某不胜感激")
				proof.Overnight = proof.Overnight || strings.Contains(d.Text, "赵灵儿不胜酒力")
				proof.LiuFarewell = proof.LiuFarewell || strings.Contains(d.Text, "我有要事先行告辞")
				proof.Wake = proof.Wake || strings.Contains(d.Text, "逍遥哥哥说好要带你逛苏州城")
				proof.Tournament = proof.Tournament || v.Scene == 33 && strings.Contains(d.Text, "这不是刘兄吗")
				proof.DuelWon = proof.DuelWon || v.Scene == 33 && strings.Contains(d.Text, "我认输了")
				proof.Manor = proof.Manor || v.Scene == 39 && strings.Contains(d.Text, "好漂亮的房子")
				proof.Illness = proof.Illness || v.Scene == 37 && strings.Contains(d.Text, "胸口很闷")
				proof.Garden = proof.Garden || v.Scene == 36 && strings.Contains(d.Text, "我才不是随随便便的女人")
				proof.Summoned = proof.Summoned || v.Scene == 36 && strings.Contains(d.Text, "老爷在前厅请您过去")
				proof.Horoscope = proof.Horoscope || v.Scene == 35 && strings.Contains(d.Text, "合八字")
				proof.Alarm = proof.Alarm || v.Scene == 35 && strings.Contains(d.Text, "西厢房里有")
				proof.Missing = proof.Missing || strings.Contains(d.Text, "赵姑娘不见了")
				proof.Search = proof.Search || v.Scene == 38 && strings.Contains(d.Text, "找到灵儿我是不会回来的")
				proof.Companion = proof.Companion || v.Scene == 40 && strings.Contains(d.Text, "我们都已经．．已经是伙伴了")
				proof.Cave = proof.Cave || v.Scene == 40 && strings.Contains(d.Text, "你带我到隐龙窟")
				t.Log("tournament reading", proof.Readings, v.Scene, d.ID, len(v.Party), strings.ReplaceAll(d.Text, "\n", " / "))
			}
			for _, mark := range []struct {
				name    string
				reached bool
			}{
				{"liu-rescue", proof.Rescue}, {"liu-overnight", proof.LiuThanks}, {"overnight-caption", proof.Overnight}, {"liu-farewell", proof.LiuFarewell}, {"ling-wake", proof.Wake},
				{"tournament", proof.Tournament}, {"duel-victory", proof.DuelWon}, {"manor", proof.Manor}, {"ling-illness", proof.Illness},
				{"garden", proof.Garden}, {"summoned", proof.Summoned}, {"horoscope", proof.Horoscope}, {"alarm", proof.Alarm}, {"ling-missing", proof.Missing},
				{"search", proof.Search}, {"companion", proof.Companion}, {"cave", proof.Cave},
			} {
				if mark.reached && !proof.Saved[mark.name] {
					save(mark.name)
				}
			}
			if v.Scene == 33 && strings.Contains(d.Text, "亮招吧") && !proof.Saved["duel-entry"] {
				save("duel-entry")
			}
			v = view(t, m, g, 0)
			d = v.Dialogue
			if len(d.Read) != 1 || d.Read[0] {
				t.Fatal("solo tournament reading changed", d)
			}
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
		} else if v.Battle != nil {
			if !proof.InBattle {
				proof.Battles++
				proof.InBattle = true
				proof.BattleScenes = append(proof.BattleScenes, v.Scene)
				if proof.Battles > 3 {
					t.Fatal("original tournament replayed a battle", proof)
				}
			}
			if v.Scene == 33 && (len(v.Party) != 1 || v.Party[0].Role != 0) {
				t.Fatal("original tournament duel is not Li alone", v.Party)
			}
			if !proof.BattleSaved[proof.Battles-1] {
				name := []string{"rescue-battle", "duel-battle", "snake-battle"}[proof.Battles-1]
				save(name)
				proof.BattleSaved[proof.Battles-1] = true
			}
			c.Actor = 0
			a, err := m.BotAction(c, g)
			if err != nil {
				t.Fatal(err)
			}
			if len(a) > 0 {
				g, err = m.Apply(campaignBotContext(c), g, a)
				if err != nil {
					t.Fatal(err)
				}
			}
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if v.Scene == 40 && proof.Cave && len(v.Party) == 2 {
				// Original DATA team 23 grants 41+42+30 cash; teams 24/25
				// grant none. No rewards may repeat across the saved events.
				if !proof.Overnight || !proof.Summoned || v.Cash != proof.InitialCash+113 || !bytes.Equal(raw(proof.BattleScenes), raw([]int{29, 33, 37})) {
					t.Fatal("original tournament prerequisites/rewards changed", proof, v.Cash)
				}
				if proof.Battles != 3 || !proof.Rescue || !proof.LiuThanks || !proof.LiuFarewell || !proof.Wake || !proof.Tournament || !proof.DuelWon || !proof.Manor || !proof.Illness || !proof.Garden || !proof.Horoscope || !proof.Alarm || !proof.Missing || !proof.Search || !proof.Companion || v.Party[0].Role != 2 || v.Party[1].Role != 0 {
					t.Fatal("original tournament/manor incomplete", proof, v.Party)
				}
				save("complete")
				if err := os.WriteFile(campaignEvidencePath("campaign-tournament-state.json", 1), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": 1, "scope": "original Liu rescue and overnight, Ling'er waking/rejoining, Li-only tournament duel, manor illness/garden/horoscope/alarm, snake encounter, disappearance and Yueru formally leading the search; ordinary inputs and production battle bot; fresh-runtime personal saves", "scene": v.Scene, "map": v.Map, "cashBefore": startCash, "cash": v.Cash, "party": v.Party, "proof": proof, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-tournament.json", 1), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			stage := 0
			for _, done := range []bool{proof.LiuFarewell, proof.Wake, proof.Manor, proof.Illness, proof.Garden, proof.Alarm, proof.Missing, proof.Search, proof.Cave} {
				if done {
					stage++
				}
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 24 {
				t.Fatal("tournament walkthrough repeats position", place, proof)
			}
			x, y := 0, 0
			switch v.Scene {
			case 23:
				x, y = 880, 1080
			case 24:
				if !proof.LiuFarewell {
					x, y = 1088, 800
				} else {
					x, y = 704, 448
				}
			case 26:
				if !proof.LiuFarewell {
					x, y = 464, 696
				} else {
					// The original staircase moves the party off the raised
					// gallery. Walking straight at the exit hits its railing.
					if v.X == 912 && v.Y == 656 || v.Y >= 648 && v.X < 1000 {
						proof.InnDownstairs = true
					}
					if v.X == 912 && v.Y == 792 {
						proof.InnCounter = true
					}
					if proof.InnCounter {
						x, y = 1008, 840
					} else if proof.InnDownstairs {
						x, y = 912, 792
					} else {
						x, y = 800, 544
					}
				}
			case 29:
				walkNPC(v, 944, 792, 525)
			case 27:
				if len(v.Party) == 1 {
					walkNPC(v, 640, 320, 508)
				} else {
					x, y = 576, 496
				}
			case 33:
				walkNPC(v, 1200, 760, 551)
			case 35:
				if proof.Alarm {
					x, y = 688, 1128
				} else {
					walkNPC(v, 1184, 1264, 574)
				}
			case 39:
				x, y = 1392, 1176
			case 36:
				switch {
				case proof.Alarm && !proof.Missing:
					walkNPC(v, 704, 752, 597)
				case proof.Alarm:
					x, y = 464, 872
				case !proof.Illness:
					x, y = 464, 872
				case !proof.Garden:
					walkNPC(v, 1440, 464, 591)
				case !proof.Summoned:
					walkNPC(v, 1488, 776, 595)
				default:
					x, y = 1584, 808
				}
			case 37:
				if proof.Alarm {
					walkNPC(v, 1072, 696, 603)
				} else if !proof.Illness {
					walkNPC(v, 1024, 624, 602)
				} else {
					x, y = 1136, 472
				}
			case 40:
				walkNPC(v, 784, 888, 634)
			default:
				t.Fatal("tournament left original route", v.Scene, v.X, v.Y, proof)
			}
			if x != 0 {
				if v.Scene == 26 && proof.InnDownstairs {
					g = campaignWalkTo(t, m, &c, g, v, x, y, 0)
				} else {
					g = walker.walk(t, m, &c, g, v, x, y, 0)
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
			t.Log("tournament position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party))
		}
	}
	t.Fatal("original tournament/manor never finished", proof)
	return m, c, g
}
