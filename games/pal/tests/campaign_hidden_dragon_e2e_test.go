package pal_test

// Test first: original cave stairs loop or skip the snake guard; ordinary
// battle input cannot win the snake/fox encounters; a lost fight is mistaken
// for progress; rewards or the stone key repeat on resume; key use away from
// the original door is accepted or the real facing pose fails; the key is not
// consumed or the door/captive rescue cannot continue; new runtimes change the
// original party, C continuation, menu, drawings, RNG/resources or accept old
// controls. All script identities and coordinates below are walkthrough data
// used only in this E2E, never production exploration/battle strategy.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type hiddenDragonProof struct {
	ItemConfirmEpoch                                             uint64
	ItemConfirms                                                 int
	CooperativeCommands                                          int
	TargetNavigation                                             int
	TargetMark                                                   [4]int
	Equipment                                                    caveEquipmentProof
	RouteStep                                                    int
	CavePartyReordered                                           bool
	Entrance, SnakeDemand, SnakeWon, SnakeCorpse                 bool
	FoxDemand, FoxWon, KeyAward, RescuePlan                      bool
	DoorLocked, KeySelected, KeyUsed, KeyConsumed, DoorOpened    bool
	Thanks, Village, NewSearch                                   bool
	Saved                                                        map[string]bool
	InitialCash, BattleCash                                      uint32
	InBattle                                                     bool
	BattleKind, RewardXP, RewardCash                             int
	Readings, Saves, Battles, Wins, Commands, Scans, PostKeyWait int
	Rewards                                                      [][2]int
	LastPrompt                                                   uint64
	KeyCheckPage                                                 string
}

func TestOriginalCampaignHiddenDragonRescueAndStoneKeySave(t *testing.T) {
	m, c, g := originalYueruMeeting(t, 1)
	m, c, g = runOriginalTournament(t, m, c, g, tournamentProof{})
	runOriginalHiddenDragon(t, m, c, g, hiddenDragonProof{})
}

func TestOriginalCampaignHiddenDragonFromCheckpoint(t *testing.T) {
	entry := os.Getenv("PAL_DRAGON_CHECKPOINT")
	if entry != "1" && entry != "failure" && entry != "snake" && entry != "fox" && entry != "prepared" && entry != "whip" {
		t.Skip("set PAL_DRAGON_CHECKPOINT=1 after the stage-4 E2E")
	}
	name := "campaign-tournament-state.json"
	proof := hiddenDragonProof{}
	if entry == "prepared" {
		name = "hidden-dragon-prepared-state.json"
		b, err := os.ReadFile(campaignEvidencePath("cave-equipment.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &proof.Equipment); err != nil {
			t.Fatal(err)
		}
	}
	if entry == "failure" || entry == "snake" || entry == "fox" || entry == "whip" {
		name = "hidden-dragon-failure-state.json"
		proofName := "hidden-dragon-failure-proof.json"
		if entry == "snake" {
			name = "hidden-dragon-snake-battle-state.json"
		}
		if entry == "fox" {
			name = "hidden-dragon-fox-battle-state.json"
			proofName = "hidden-dragon-fox-battle-proof.json"
		}
		if entry == "whip" {
			name = "hidden-dragon-snake-corpse-state.json"
			proofName = "hidden-dragon-snake-corpse-proof.json"
		}
		b, err := os.ReadFile(campaignEvidencePath(proofName, 1))
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
	if entry == "1" && (v.Scene != 40 || len(v.Party) != 2 || v.Party[0].Role != 2 || v.Party[1].Role != 0) {
		t.Fatal("not the original stage-4 checkpoint", v.Scene, v.Party)
	}
	if entry == "whip" {
		for i := 0; view(t, m, g, 0).Phase != "field"; i++ {
			if i > 128 {
				t.Fatal("snake corpse reading did not finish")
			}
			v = view(t, m, g, 0)
			if v.Dialogue != nil {
				g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			}
			c.Now = time.UnixMilli(g.SystemDeadline + 200)
			g, err = m.OnTimer(c, g)
			if err != nil {
				t.Fatal(err)
			}
		}
		var gear caveEquipmentProof
		m, c, g, gear = equipOriginalCaveWeapons(t, m, c, g, []caveWeaponChoice{{"长鞭", "林月如", "yue-whip", 2}})
		proof.Equipment.Saves += gear.Saves
		proof.Equipment.Weapons = append(proof.Equipment.Weapons, gear.Weapons...)
	}
	runOriginalHiddenDragon(t, m, c, g, proof)
}

func runOriginalHiddenDragon(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, proof hiddenDragonProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	if v := view(t, m, g, 0); v.Scene == 40 && v.Phase == "field" && !proof.Equipment.Passed {
		m, c, g, proof.Equipment = equipOriginalCaveWeapons(t, m, c, g, []caveWeaponChoice{{"铁剑", "李逍遥", "li-iron", 0}})
	}
	if proof.Saved == nil {
		proof.Saved = map[string]bool{}
	}
	if proof.InitialCash == 0 {
		proof.InitialCash = view(t, m, g, 0).Cash
	}
	// A stair is an absorbing transition, not a walkable connection across
	// the map. Route over actual landings after cutting all stair footprints.
	// source scene, destination scene, event, source x/y, landing x/y.
	route := [][7]int{
		{45, 47, 702, 1344, 592, 1504, 1376}, {47, 46, 747, 1440, 768, 272, 1448},
		{46, 47, 726, 1184, 288, 800, 1632}, {47, 41, 752, 528, 1656, 1776, 1848},
		{41, 47, 637, 528, 952, 1664, 1696}, {47, 42, 754, 1552, 1544, 448, 816},
		{42, 47, 674, 1376, 864, 640, 1024}, {47, 42, 756, 176, 1112, 1152, 944},
	}
	// scene, event, x, y, original touch radius.
	portals := [][5]int{
		{40, 633, 192, 240, 48}, {43, 687, 1792, 1808, 48}, {43, 688, 400, 696, 48}, {43, 689, 1808, 1448, 48},
		{45, 699, 1904, 1896, 80}, {45, 700, 720, 1432, 48}, {45, 701, 912, 1304, 48}, {45, 702, 1344, 592, 80},
		{46, 724, 192, 1488, 48}, {46, 725, 304, 1912, 48}, {46, 726, 1184, 288, 48},
		{47, 746, 832, 512, 48}, {47, 747, 1440, 768, 48}, {47, 748, 1440, 1408, 48}, {47, 749, 992, 1056, 48}, {47, 750, 496, 712, 48}, {47, 751, 832, 1648, 48}, {47, 752, 528, 1656, 48}, {47, 753, 1728, 1728, 48}, {47, 754, 1552, 1544, 48}, {47, 755, 704, 1056, 48}, {47, 756, 176, 1112, 48},
		{41, 636, 1856, 1888, 48}, {41, 637, 528, 952, 48},
		{42, 671, 768, 672, 48}, {42, 673, 496, 824, 48}, {42, 674, 1376, 864, 48}, {42, 675, 1200, 968, 48},
		{48, 759, 448, 976, 16}, {48, 760, 1344, 1360, 80},
	}
	walker := campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"hidden-dragon-failure-state.json": g.Data, "hidden-dragon-failure-proof.json": raw(proof), "hidden-dragon-failure-view.json": raw(view(t, m, g, 0))} {
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
			t.Fatal("hidden dragon save changed original state", name)
		}
		c.Actor = 0
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("hidden dragon accepted pre-save controls", name)
		}
		proof.Saves++
		proof.Saved[name] = true
		if err := os.WriteFile(campaignEvidencePath("hidden-dragon-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(campaignEvidencePath("hidden-dragon-"+name+"-proof.json", 1), raw(proof), 0600); err != nil {
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
	for step := 0; step < 16000; step++ {
		v := view(t, m, g, 0)
		if proof.RouteStep < len(route) {
			r := route[proof.RouteStep]
			if v.Scene == r[1] && fieldTestDistance(v.X, v.Y, r[5], r[6]) <= 128 {
				proof.RouteStep++
				save(fmt.Sprint("stair-", proof.RouteStep))
				v = view(t, m, g, 0)
			}
		}
		targetPortal := 0
		if v.Scene == 40 {
			targetPortal = 633
		}
		if v.Scene == 43 {
			targetPortal = 688
		}
		if proof.RouteStep < len(route) && v.Scene == route[proof.RouteStep][0] {
			targetPortal = route[proof.RouteStep][2]
		}
		if proof.RouteStep == 0 && v.Scene == 47 {
			targetPortal = 750
		} // diagnostic recovery from the explored dead end
		if proof.RouteStep == len(route) && v.Scene == 42 && proof.SnakeCorpse {
			targetPortal = 671
		}
		if v.Scene == 48 && proof.KeyConsumed {
			targetPortal = 759
		}
		walker.avoidTouches[v.Scene] = nil
		for _, p := range portals {
			if p[0] == v.Scene && p[1] != targetPortal {
				walker.avoidTouches[v.Scene] = append(walker.avoidTouches[v.Scene], [3]int{p[2], p[3], p[4]})
			}
		}
		if len(v.Party) != 2 || !(v.Party[0].Role == 0 && v.Party[1].Role == 2 || v.Party[0].Role == 2 && v.Party[1].Role == 0) || v.Party[0].Controller != 0 || v.Party[1].Controller != 0 {
			t.Fatal("hidden dragon changed original solo party", v.Party)
		}
		// The original entrance script puts Li in front for the cave search.
		if v.Party[0].Role == 0 {
			if !proof.Entrance {
				t.Fatal("cave party reordered before the original entrance")
			}
			proof.CavePartyReordered = true
		} else if proof.CavePartyReordered {
			t.Fatal("original cave party order was lost", v.Party)
		}
		if proof.CavePartyReordered && !proof.Saved["cave-party"] {
			save("cave-party")
			v = view(t, m, g, 0)
		}
		if v.Cash < proof.InitialCash {
			t.Fatal("original cave battle lost/restarted", v.Cash, v.Party)
		}
		if proof.InBattle && v.Battle == nil {
			if proof.RewardXP < 0 || proof.RewardCash < 0 || v.Cash != proof.BattleCash+uint32(proof.RewardCash) {
				t.Fatal("original cave reward missing/repeated", proof, v.Cash)
			}
			proof.InBattle = false
			proof.Wins++
			proof.Rewards = append(proof.Rewards, [2]int{proof.RewardXP, proof.RewardCash})
			if proof.BattleKind == 1 {
				if proof.RewardCash != 1100 {
					t.Fatal("original snake reward changed", proof.RewardCash)
				}
				proof.SnakeWon = true
				save("snake-victory")
			} else if proof.BattleKind == 2 {
				// Summoned bees add their original rewards to the fox's
				// 350 cash. The displayed total must still match the exact
				// wallet increase checked above.
				if proof.RewardCash < 350 {
					t.Fatal("original fox reward missing", proof.RewardCash)
				}
				proof.FoxWon = true
				save("fox-victory")
			}
			v = view(t, m, g, 0)
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Entrance = proof.Entrance || strings.Contains(d.Text, "这里就是蛇妖的洞窟")
				proof.SnakeDemand = proof.SnakeDemand || v.Scene == 42 && strings.Contains(d.Text, "把灵儿还我")
				proof.SnakeCorpse = proof.SnakeCorpse || strings.Contains(d.Text, "蛇妖已死")
				proof.FoxDemand = proof.FoxDemand || v.Scene == 48 && strings.Contains(d.Text, "你杀了我相公")
				proof.KeyAward = proof.KeyAward || v.Scene == 48 && strings.Contains(d.Text, "石钥匙")
				proof.RescuePlan = proof.RescuePlan || v.Scene == 48 && strings.Contains(d.Text, "女孩子们救出去")
				proof.DoorLocked = proof.DoorLocked || strings.Contains(d.Text, "此石门已上锁")
				proof.Thanks = proof.Thanks || v.Scene == 49 && strings.Contains(d.Text, "多谢公子相救之恩")
				proof.Village = proof.Village || v.Scene == 49 && strings.Contains(d.Text, "不远的白河村")
				proof.NewSearch = proof.NewSearch || v.Scene == 49 && strings.Contains(d.Text, "只得从头找起了")
				t.Log("hidden dragon reading", proof.Readings, v.Scene, d.ID, strings.ReplaceAll(d.Text, "\n", " / "))
			}
			for _, mark := range []struct {
				name    string
				reached bool
			}{
				{"entrance", proof.Entrance}, {"snake-demand", proof.SnakeDemand}, {"snake-corpse", proof.SnakeCorpse}, {"fox-demand", proof.FoxDemand},
				{"key-award", proof.KeyAward}, {"rescue-plan", proof.RescuePlan}, {"locked-door", proof.DoorLocked}, {"rescued-thanks", proof.Thanks},
				{"village", proof.Village}, {"new-search", proof.NewSearch},
			} {
				if mark.reached && !proof.Saved[mark.name] {
					save(mark.name)
				}
			}
			v = view(t, m, g, 0)
			d = v.Dialogue
			if len(d.Read) != 1 || d.Read[0] {
				t.Fatal("hidden dragon lost solo reading", d)
			}
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
		} else if v.Battle != nil {
			if !proof.InBattle {
				proof.InBattle = true
				proof.Battles++
				proof.BattleCash = v.Cash
				proof.RewardXP, proof.RewardCash = -1, -1
				proof.BattleKind = 0
				for _, e := range v.Battle.Enemies {
					if e.Object == 486 {
						proof.BattleKind = 1
					}
					if e.Object == 469 {
						proof.BattleKind = 2
					}
				}
				t.Log("hidden dragon battle", proof.Battles, proof.BattleKind, v.Scene, v.Party, v.Battle.Enemies)
				if proof.Battles > 40 {
					t.Fatal("cave route repeated too many battles", proof)
				}
				if proof.BattleKind == 1 {
					if proof.SnakeWon {
						t.Fatal("snake respawned")
					}
					save("snake-battle")
				}
				if proof.BattleKind == 2 {
					if !proof.SnakeWon || proof.FoxWon {
						t.Fatal("fox fought out of original order")
					}
					save("fox-battle")
				}
				v = view(t, m, g, 0)
			}
			if xp, cash, ok := campaignBattleReward(v); ok {
				proof.RewardXP, proof.RewardCash = xp, cash
			}
			if proof.BattleKind != 0 && v.Battle.Action == 4 && (v.Battle.State == 2 || v.Battle.State == 4) {
				name := "snake-cooperation-target"
				if proof.BattleKind == 2 {
					name = "fox-cooperation-target"
				}
				if !proof.Saved[name] {
					save(name)
					v = view(t, m, g, 0)
				}
			}
			if v.Party[0].HP <= 0 && v.Party[1].HP <= 0 {
				t.Fatal("original cave party defeated", v.Party)
			}
			c.Actor = 0
			a, err := m.BotAction(c, g)
			if err != nil {
				t.Fatal(err)
			}
			if len(a) > 0 {
				var proposed pal.Action
				if err := json.Unmarshal(a, &proposed); err != nil {
					t.Fatal(err)
				}
				if os.Getenv("PAL_BATTLE_TRACE") == "1" && proof.BattleKind == 2 && (v.Battle.State == 1 && v.Battle.Menu == 0 || proposed.Keys == 2 || proposed.Keys == 1) {
					var memory struct {
						BattleBot any `json:"battleBot"`
					}
					if err := json.Unmarshal(g.Data, &memory); err != nil {
						t.Fatal(err)
					}
					t.Log("original fox public decision", step, v.Battle.State, v.Battle.Menu, v.Battle.Current, v.Battle.Selected, v.Battle.Action, v.Battle.Target, proposed.Keys, v.Party, originalVisibleText(v, true), memory.BattleBot)
				}
				if v.Battle.State == 1 && v.Battle.Menu == 2 && proposed.Keys == 2 {
					if proof.ItemConfirmEpoch == v.Epoch {
						proof.ItemConfirms++
					} else {
						proof.ItemConfirmEpoch = v.Epoch
						proof.ItemConfirms = 1
					}
					if proof.ItemConfirms > 8 {
						t.Fatal("original supply selector did not accept repeated confirmations", v.Epoch, originalVisibleText(v, true), originalVisibleText(v, false))
					}
				} else {
					proof.ItemConfirms = 0
				}
				if v.Battle.Action == 4 && (v.Battle.State == 2 || v.Battle.State == 4) && proposed.Keys == 2 {
					proof.CooperativeCommands++
				}
				if v.Battle.State == 2 && v.Battle.Action == 2 && (proposed.Keys == 8 || proposed.Keys == 32) {
					mark := [4]int{v.Battle.Current, v.Battle.Target, len(v.Battle.Enemies), v.Battle.Object}
					if mark == proof.TargetMark {
						proof.TargetNavigation++
					} else {
						proof.TargetMark = mark
						proof.TargetNavigation = 1
					}
					if proof.TargetNavigation > 8 {
						t.Fatal("original physical target selector did not advance", mark, string(a))
					}
				} else {
					proof.TargetNavigation = 0
				}
				if step%100 == 0 {
					t.Log("hidden dragon command", string(a), v.Battle.Phase, v.Battle.State, v.Battle.Menu, v.Battle.Current, v.Battle.Selected, v.Battle.Target, v.Party)
				}
				g, err = m.Apply(campaignBotContext(c), g, a)
				if err != nil {
					t.Fatal(err)
				}
				proof.Commands++
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			keys := uint32(32)
			proof.Scans++
			if proof.Scans > 128 {
				t.Fatal("original stone key inventory did not settle", originalVisibleText(v, false))
			}
			if proof.KeyUsed {
				keys = 0
				if proof.KeyConsumed {
					keys = 1
				} else if proof.PostKeyWait == 0 {
					keys = 65536 // Home: inspect every inventory page after use.
				} else if proof.PostKeyWait%3 == 0 {
					page := originalVisibleText(v, false)
					if strings.Contains(page, "石钥匙") {
						t.Fatal("stone key was not consumed at its original door", v.X, v.Y, v.Field, originalVisibleText(v, false))
					}
					if page == proof.KeyCheckPage {
						proof.KeyConsumed = true
						save("key-consumed-menu")
						v = view(t, m, g, 0)
						keys = 1
					} else {
						proof.KeyCheckPage = page
						keys = 128 // Original PageDown clamps on the last page.
					}
				}
				proof.PostKeyWait++
			} else if strings.Contains(originalVisibleText(v, true), "石钥匙") {
				if n, ok := campaignInventoryAmount(v, "石钥匙"); !ok || n != 1 {
					t.Fatal("original stone key missing/duplicated", n, ok)
				}
				// The original centered award notice is not a character
				// reading stop. Verify the earned item in the actual inventory.
				if !proof.KeyAward {
					proof.KeyAward = true
					save("key-award")
					v = view(t, m, g, 0)
				}
				if !proof.KeySelected {
					proof.KeySelected = true
					save("key-selected")
					v = view(t, m, g, 0)
				}
				keys = 2
				proof.KeyUsed = true
			}
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if v.Scene == 49 {
				if proof.RouteStep != len(route) {
					t.Fatal("original cave stair route incomplete", proof.RouteStep)
				}
				if !proof.CavePartyReordered || v.Party[0].Role != 0 || v.Party[1].Role != 2 {
					t.Fatal("original cave leader/party missing", v.Party)
				}
				if !proof.Entrance || !proof.SnakeWon || !proof.SnakeCorpse || !proof.FoxWon || !proof.KeyAward || !proof.RescuePlan || !proof.DoorLocked || !proof.KeySelected || !proof.KeyConsumed || !proof.DoorOpened || !proof.Thanks || !proof.Village || !proof.NewSearch || proof.Battles != proof.Wins {
					t.Fatal("original hidden dragon rescue incomplete", proof)
				}
				for _, a := range v.Field.Actors {
					if a[0] == 1 && a[1] >= 791 && a[1] <= 795 {
						t.Fatal("original rescued girls did not depart", a)
					}
				}
				save("complete")
				if err := os.WriteFile(campaignEvidencePath("campaign-hidden-dragon-state.json", 1), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": 1, "scope": "actual original hidden dragon cave route, ordinary snake/fox battles and rewards, stone key acquisition/selection/consumption and original door animation, captive rescue and White River village invitation; production battle bot, normal walkthrough inputs and fresh-runtime personal saves", "scene": v.Scene, "map": v.Map, "cashBefore": proof.InitialCash, "cash": v.Cash, "party": v.Party, "proof": proof, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-hidden-dragon.json", 1), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			stage := 0
			for _, done := range []bool{proof.Entrance, proof.SnakeWon, proof.SnakeCorpse, proof.FoxWon, proof.RescuePlan, proof.DoorLocked, proof.KeyConsumed, proof.DoorOpened} {
				if done {
					stage++
				}
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 24 {
				t.Fatal("hidden dragon walkthrough repeats position", place, proof)
			}
			x, y := 0, 0
			switch v.Scene {
			case 40:
				x, y = 192, 240
			case 43:
				x, y = 400, 696
			case 45, 46, 47, 41:
				if proof.RouteStep == 0 && v.Scene == 47 {
					x, y = 496, 712
				} else if proof.RouteStep < len(route) && v.Scene == route[proof.RouteStep][0] {
					x, y = route[proof.RouteStep][3], route[proof.RouteStep][4]
				} else {
					t.Fatal("original cave stair entered an unexpected landing", v.Scene, v.X, v.Y, proof.RouteStep)
				}
			case 42:
				if proof.RouteStep < len(route) {
					x, y = route[proof.RouteStep][3], route[proof.RouteStep][4]
				} else if !proof.SnakeWon || !proof.SnakeCorpse {
					walkNPC(v, 816, 776, 672)
				} else {
					x, y = 768, 672
				}
			case 48:
				switch {
				case !proof.FoxWon:
					walkNPC(v, 1440, 304, 761)
				case !proof.RescuePlan:
					t.Fatal("original fox aftermath was skipped")
				case !proof.DoorLocked:
					// This door has original search mode 1 (adjacent),
					// unlike the longer-range NPC searches elsewhere.
					if v.X != 480 || v.Y != 992 {
						x, y = 480, 992
					} else {
						c.Actor = 0
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 1, Keys: 8 | 2})
					}
				case !proof.KeyUsed:
					if v.X != 480 || v.Y != 992 {
						x, y = 480, 992
					} else {
						c.Actor = 0
						direction, keys := 1, uint32(8)
						if v.Field.Direction == 1 {
							direction, keys = 4, 2048
						}
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
					}
				case !proof.KeyConsumed:
					// Original apply-to-all items return directly to the field.
					// Reopen the real inventory to verify removal before leaving.
					c.Actor = 0
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
				default:
					for _, a := range v.Field.Actors {
						if a[0] == 1 && a[1] == 758 && a[5] >= 3 {
							proof.DoorOpened = true
						}
					}
					if proof.DoorOpened && !proof.Saved["open-door"] {
						save("open-door")
						v = view(t, m, g, 0)
					}
					x, y = 448, 976
				}
			default:
				t.Fatal("hidden dragon left original route", v.Scene, v.X, v.Y, proof)
			}
			if x != 0 {
				g = walker.walk(t, m, &c, g, v, x, y, 0)
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("hidden dragon position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party), v.Cash)
			if v.Battle != nil {
				t.Log("hidden dragon battle observation", v.Battle.Phase, v.Battle.State, v.Battle.Menu, v.Battle.Current, v.Battle.Selected, v.Battle.Target, v.Party)
			}
		}
	}
	t.Fatal("original hidden dragon rescue never finished", proof)
	return m, c, g
}
