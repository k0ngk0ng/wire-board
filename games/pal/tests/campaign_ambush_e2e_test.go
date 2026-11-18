package pal_test

// Test first: return maps/boat fail, production bots cannot win the original
// inn ambush, reward/departure replays on save, the surviving Li role changes
// owner when Ling'er leaves, shared reading drops an original participant, or
// saved combat/script/final room state cannot continue in a fresh runtime.
// Route and story markers are test walkthrough data only.
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

type ambushProof struct {
	Boat, Ambush, Victory, Single, Wake            bool
	BoatSaved, BattleSaved, SingleSaved, WakeSaved bool
	Readings, Saves                                int
	BattleCash                                     uint32
	LastPrompt                                     uint64
}

func TestOriginalCampaignInnAmbushAndSinglePartySave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			originalAmbush(t, players)
		})
	}
}

func originalAmbush(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalHomecoming(t, players)
	return runOriginalAmbush(t, m, c, g, players, ambushProof{})
}

func runOriginalAmbush(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof ambushProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"ambush-failure-state.json": g.Data, "ambush-failure-proof.json": raw(proof)} {
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
				t.Fatal("ambush departure changed role owner", v.Party)
			}
			seen[p.Role] = true
		}
		if !seen[0] || len(v.Party) > 2 {
			t.Fatal("ambush lost surviving Li role", v.Party)
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
					t.Fatal("ambush partial reading changed")
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
		if before.Scene != v.Scene || before.Map != v.Map || before.X != v.X || before.Y != v.Y || before.Cash != v.Cash || before.Leader != v.Leader || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Battle), raw(v.Battle)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Field), raw(v.Field)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) || !bytes.Equal(raw(before.Draws), raw(v.Draws)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) {
			t.Fatal("ambush continuation changed original state")
		}
		c.Actor = v.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 32768})); err == nil {
			t.Fatal("ambush accepted pre-save controls")
		}
		proof.Saves++
	}
	for step := 0; step < 6000; step++ {
		v := view(t, m, g, 0)
		v = view(t, m, g, v.Leader)
		owners(v)
		if v.GameOver != nil {
			t.Fatal("original ambush reached a real game-over boundary", v.GameOver, v.Party)
		}
		if v.Cash < 550 {
			t.Fatal("original ambush bots lost/restarted", v.Scene, v.Cash, v.Party)
		}
		if proof.Ambush && v.Battle == nil && v.Cash > proof.BattleCash {
			proof.Victory = true
		}
		if proof.Ambush && len(v.Party) == 1 {
			proof.Single = true
			if !proof.SingleSaved {
				save(v.Dialogue != nil)
				proof.SingleSaved = true
				v = view(t, m, g, v.Leader)
			}
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Boat = proof.Boat || strings.Contains(d.Text, "那就走人啦")
				proof.Wake = proof.Wake || v.Scene == 2 && strings.Contains(d.Text, "累坏了")
				t.Log("ambush reading", proof.Readings, v.Scene, d.ID, len(v.Party))
			}
			if proof.Boat && !proof.BoatSaved {
				save(true)
				proof.BoatSaved = true
				v = view(t, m, g, v.Leader)
				d = v.Dialogue
			}
			if proof.Wake && !proof.WakeSaved {
				save(true)
				proof.WakeSaved = true
				v = view(t, m, g, v.Leader)
				d = v.Dialogue
			}
			if len(d.Read) != players {
				t.Fatal("ambush lost original shared readers")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Battle != nil {
			if v.Scene == 4 && !proof.Ambush {
				proof.Ambush = true
				proof.BattleCash = v.Cash
			}
			if proof.Ambush && !proof.BattleSaved {
				save(false)
				proof.BattleSaved = true
				for name, b := range map[string][]byte{"ambush-battle-state.json": g.Data, "ambush-battle-proof.json": raw(proof)} {
					if err := os.WriteFile(campaignEvidencePath(name, players), b, 0600); err != nil {
						t.Fatal(err)
					}
				}
				t.Log("original ambush initial party", view(t, m, g, 0).Party)
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
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			stage := 0
			if proof.Boat {
				stage++
			}
			if proof.Ambush {
				stage++
			}
			if proof.Single {
				stage++
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("ambush walkthrough repeats position", place)
			}
			if v.Scene == 2 && proof.Wake {
				if !proof.BoatSaved || !proof.BattleSaved || !proof.SingleSaved || !proof.WakeSaved || !proof.Victory || !proof.Single || len(v.Party) != 1 || v.Party[0].Role != 0 || v.Party[0].Controller != 0 {
					t.Fatal("original inn ambush incomplete", proof, v.Party)
				}
				save(false)
				if err := os.WriteFile(campaignEvidencePath("campaign-ambush-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": players, "scope": "actual original return boat, inn ambush with production battle bot, original victory and Ling'er departure, aunt intervention and next room; saved combat/departure/shared reading across fresh runtimes; not full campaign/autoplay", "scene": v.Scene, "map": v.Map, "cash": v.Cash, "battleCashBefore": proof.BattleCash, "party": v.Party, "readings": proof.Readings, "saves": proof.Saves, "singlePartySaved": proof.SingleSaved, "battleSaved": proof.BattleSaved, "originalReaders": players, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-ambush.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			walker.allowTouch = v.Scene == 16
			x, y, reach := 224, 1120, 32
			switch v.Scene {
			case 20:
			case 17:
				x, y = 240, 856
			case 19:
				x, y = 512, 1184
			case 16:
				x, y = 352, 1696
			case 15:
				g = campaignWalkTo(t, m, &c, g, v, 704, 832, 202)
			case 6:
				x, y = 192, 1680
			case 5:
				x, y = 1296, 872
			case 4:
				x, y, reach = 1128, 1624, 64
			default:
				t.Fatal("ambush return left original route", v.Scene, v.X, v.Y)
			}
			if v.Scene != 15 {
				g = walker.walk(t, m, &c, g, v, x, y, reach)
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("ambush position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party))
		}
	}
	t.Fatal("original inn ambush did not finish", proof)
	return m, c, g
}
