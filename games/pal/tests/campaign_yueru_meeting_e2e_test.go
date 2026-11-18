package pal_test

// Test first: original encounter/duel cannot finish with ordinary bot input;
// the two distinct cries for help lose their branch on save; rescuers lose the
// second battle; scripted injured/recovery poses are skipped or cannot resume;
// the temporary solo party loses owners/readers; the sword is missing/duplicated;
// saved menus, combat or arrival alter original state or accept stale controls.
// Script markers and route coordinates are walkthrough data only.
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

type yueruMeetingProof struct {
	Meeting, Tied, FirstCall, Bravado, SecondCall, Stabbed, Revival, Rejoined, SwordVisible, SwordPicked, SwordChecked bool
	MeetingSaved, TiedSaved, CallSaved, SecondCallSaved, StabSaved, RejoinSaved, SwordSaved                            bool
	BattleSaved                                                                                                        [2]bool
	InBattle                                                                                                           bool
	Readings, Saves, Battles, Scans                                                                                    int
	LastPrompt                                                                                                         uint64
}

func TestOriginalCampaignYueruMeetingAndRescueSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			originalYueruMeeting(t, players)
		})
	}
}

func originalYueruMeeting(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalDeparture(t, players)
	return runOriginalYueruMeeting(t, m, c, g, players, yueruMeetingProof{})
}

func runOriginalYueruMeeting(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof yueruMeetingProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	startCash := view(t, m, g, 0).Cash
	walker := campaignMapWalker{allowTouch: true}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"yueru-meeting-failure-state.json": g.Data, "yueru-meeting-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), b, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	owners := func(v pal.View) {
		t.Helper()
		seen := map[int]bool{}
		for _, p := range v.Party {
			want := 0
			if p.Role == 1 && players > 1 {
				want = 1
			}
			if p.Role < 0 || p.Role > 1 || seen[p.Role] || p.Controller != want {
				t.Fatal("Yueru meeting changed original owners or added her prematurely", v.Party)
			}
			seen[p.Role] = true
		}
		if !seen[0] || len(v.Party) > 2 {
			t.Fatal("Yueru meeting invented/lost original party", v.Party)
		}
	}
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("Yueru meeting partial reading changed")
				}
			}
		}
		before := v
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
		v = view(t, m, g, 0)
		if before.Scene != v.Scene || before.Map != v.Map || before.X != v.X || before.Y != v.Y || before.Cash != v.Cash || before.Leader != v.Leader || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Battle), raw(v.Battle)) || !bytes.Equal(raw(before.Menu), raw(v.Menu)) || !bytes.Equal(raw(before.Field), raw(v.Field)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) || !bytes.Equal(raw(before.Draws), raw(v.Draws)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) {
			t.Fatal("Yueru meeting save changed original combat/script/menu")
		}
		c.Actor = v.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("Yueru meeting accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 6000; step++ {
		v := view(t, m, g, 0)
		v = view(t, m, g, v.Leader)
		owners(v)
		if v.Cash < startCash {
			t.Fatal("original Yueru encounter battle lost/restarted", v.Cash, v.Party)
		}
		if v.Battle == nil {
			proof.InBattle = false
		}
		if proof.Revival && v.Phase == "field" && v.Field != nil {
			visible := false
			for _, actor := range v.Field.Actors {
				visible = visible || actor[0] == 1 && actor[1] == 399
			}
			if visible {
				proof.SwordVisible = true
			} else if proof.SwordVisible {
				// The original pickup caption is timed, so it need not create
				// a reading stop. Confirm disappearance, then verify inventory.
				proof.SwordPicked = true
			}
		}
		if proof.Revival && len(v.Party) == 2 && !proof.RejoinSaved {
			proof.Rejoined = true
			save(v.Dialogue != nil)
			proof.RejoinSaved = true
			v = view(t, m, g, v.Leader)
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Meeting = proof.Meeting || strings.Contains(d.Text, "林月如")
				proof.Tied = proof.Tied || strings.Contains(d.Text, "被吊在树上的滋味")
				proof.FirstCall = proof.FirstCall || strings.Contains(d.Text, "远远传来一声尖叫") || strings.Contains(d.Text, "刚才那位姑娘在喊救命")
				proof.Bravado = proof.Bravado || strings.Contains(d.Text, "我随便喊喊")
				proof.SecondCall = proof.SecondCall || strings.Contains(d.Text, "远远又传来一阵尖叫") || strings.Contains(d.Text, "她好像真的出事了")
				proof.Stabbed = proof.Stabbed || strings.Contains(d.Text, "自当受此一剑")
				proof.Revival = proof.Revival || strings.Contains(d.Text, "我胸口的伤")
				proof.SwordPicked = proof.SwordPicked || strings.Contains(d.Text, "拾起地上的宝剑")
				t.Log("Yueru reading", proof.Readings, v.Scene, d.ID, len(v.Party))
			}
			switch {
			case proof.Meeting && !proof.MeetingSaved:
				save(true)
				proof.MeetingSaved = true
			case proof.Tied && !proof.TiedSaved:
				save(true)
				proof.TiedSaved = true
			case proof.FirstCall && !proof.CallSaved:
				save(true)
				proof.CallSaved = true
			case proof.SecondCall && !proof.SecondCallSaved:
				save(true)
				proof.SecondCallSaved = true
			case proof.Stabbed && !proof.StabSaved:
				injured := false
				for _, draw := range v.Draws {
					injured = injured || draw.Kind == 0 && draw.A == 228
				}
				if len(v.Party) != 1 || !injured {
					t.Fatal("original injured pose or solo party missing", v.Party, v.Draws)
				}
				save(true)
				proof.StabSaved = true
				if err := os.WriteFile(campaignEvidencePath("yueru-injury-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			v = view(t, m, g, v.Leader)
			d = v.Dialogue
			if len(d.Read) != players {
				t.Fatal("Yueru temporary party dropped original human readers")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Battle != nil {
			if !proof.InBattle {
				proof.Battles++
				proof.InBattle = true
				if proof.Battles > 2 {
					t.Fatal("original Yueru route replayed a story battle")
				}
			}
			if !proof.BattleSaved[proof.Battles-1] {
				save(false)
				proof.BattleSaved[proof.Battles-1] = true
				name := "yueru-duel-state.json"
				if proof.Battles == 2 {
					name = "yueru-rescue-state.json"
				}
				if err := os.WriteFile(campaignEvidencePath(name, players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, actor := range g.Actors {
				c.Actor = actor
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
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			proof.Scans++
			if proof.Scans > 64 {
				t.Fatal("original Yue sword inventory did not settle", originalVisibleText(v, false))
			}
			keys := uint32(32)
			if n, ok := campaignInventoryAmount(v, "越女剑"); ok {
				if n != 1 {
					t.Fatal("original Yue sword missing or duplicated", n)
				}
				if !proof.SwordSaved {
					save(false)
					proof.SwordSaved = true
					v = view(t, m, g, v.Leader)
				}
				proof.SwordChecked, keys = true, 1
			}
			c.Actor = v.Leader
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if proof.Battles == 1 && !proof.Tied {
				t.Fatal("production bot did not win original Yueru duel", v.Party)
			}
			if proof.SwordPicked && !proof.SwordChecked {
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
			} else if v.Scene == 23 {
				if !proof.MeetingSaved || !proof.TiedSaved || !proof.CallSaved || !proof.SecondCallSaved || !proof.StabSaved || !proof.RejoinSaved || !proof.SwordSaved || !proof.SwordChecked || proof.Battles != 2 || !proof.BattleSaved[0] || !proof.BattleSaved[1] || len(v.Party) != 2 {
					t.Fatal("original Yueru meeting/rescue incomplete", proof, v.Party)
				}
				save(false)
				if err := os.WriteFile(campaignEvidencePath("campaign-yueru-meeting-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": players, "scope": "actual original Yueru encounter/duel, two cries and original rescue battle, injury/recovery script and Yue sword pickup, original temporary party/rejoin and Suzhou entry; production battle bot and fresh-runtime saves; not full campaign/autoplay", "scene": v.Scene, "map": v.Map, "cashBefore": startCash, "cash": v.Cash, "party": v.Party, "readings": proof.Readings, "saves": proof.Saves, "battles": proof.Battles, "injuryScriptSaved": proof.StabSaved, "swordInventorySaved": proof.SwordSaved, "originalReaders": players, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-yueru-meeting.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			} else if v.Scene == 22 {
				stage := 0
				for _, done := range []bool{proof.Tied, proof.FirstCall, proof.Bravado, proof.SecondCall, proof.Revival, proof.SwordChecked} {
					if done {
						stage++
					}
				}
				place := [4]int{v.Scene, v.X, v.Y, stage}
				visits[place]++
				if visits[place] > 48 {
					t.Fatal("Yueru walkthrough repeats position", place, proof)
				}
				switch {
				case !proof.Tied:
					g = campaignWalkTo(t, m, &c, g, v, 912, 1032, 398)
				case proof.Revival && !proof.SwordPicked:
					g = campaignWalkTo(t, m, &c, g, v, 912, 1032, 399)
				case proof.SwordChecked:
					g = walker.walk(t, m, &c, g, v, 1472, 336, 0)
				case proof.SecondCall || proof.FirstCall && !proof.Bravado:
					g = campaignWalkTo(t, m, &c, g, v, 960, 1008, 404)
				default:
					g = walker.walk(t, m, &c, g, v, 1376, 384, 0)
				}
			} else {
				t.Fatal("Yueru route left original scenes", v.Scene)
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("Yueru position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party))
		}
	}
	t.Fatal("original Yueru encounter did not finish", proof)
	return m, c, g
}
