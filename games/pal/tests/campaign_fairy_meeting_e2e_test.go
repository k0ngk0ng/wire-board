package pal_test

// Test first: the opened-way transition fails; scripted camera/sprite changes
// trap or leave the wrong hero sprite; partial reading and script locals do not
// survive resume; old input advances an event; an NPC is invented as a party
// member; the original guide/next-map transition cannot finish. All coordinates
// are walkthrough input; no route or event data reaches production bots.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type fairyMeetingProof struct {
	Grotto, Started, Reaction, Follow       bool
	GrottoSaved, ReactionSaved, FollowSaved bool
	Saves, Readings                         int
	LastPrompt                              uint64
	Sprites                                 map[int]bool
}

func TestOriginalCampaignFairyMeetingAndPalaceEntranceSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			originalFairyPalaceEntrance(t, players)
		})
	}
}

func originalFairyPalaceEntrance(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalOpenedIslandMaze(t, players)
	return runOriginalFairyMeeting(t, m, c, g, players, fairyMeetingProof{})
}

func runOriginalFairyMeeting(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof fairyMeetingProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	if proof.Sprites == nil {
		proof.Sprites = map[int]bool{}
	}
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			if err := os.WriteFile(campaignEvidencePath("fairy-meeting-failure-state.json", players), g.Data, 0600); err != nil {
				t.Log(err)
			}
			if err := os.WriteFile(campaignEvidencePath("fairy-meeting-failure-proof.json", players), raw(proof), 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("fairy meeting partial reading changed", actor, read)
				}
			}
		}
		before := v
		old := pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4, Keys: 2}
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
		if before.Scene != after.Scene || before.Map != after.Map || before.X != after.X || before.Y != after.Y || before.HP != after.HP || before.MP != after.MP || before.Cash != after.Cash || !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || !bytes.Equal(raw(before.Draws), raw(after.Draws)) || !bytes.Equal(raw(before.PaletteEffect), raw(after.PaletteEffect)) || !bytes.Equal(raw(before.Overlay), raw(after.Overlay)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
			t.Fatal("fairy meeting save changed original progress or drawing")
		}
		c.Actor = after.Leader
		if _, err = m.Apply(c, g, raw(old)); err == nil {
			t.Fatal("fairy meeting accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 3000; step++ {
		v := view(t, m, g, 0)
		for _, draw := range v.Draws {
			if draw.Kind == 0 && draw.C == 0 && draw.D == 0 {
				proof.Sprites[draw.A] = true
				proof.Started = proof.Started || draw.A == 361
				proof.Reaction = proof.Reaction || draw.A == 193
			}
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Grotto = proof.Grotto || strings.Contains(d.Text, "洞天")
				proof.Follow = proof.Follow || strings.Contains(d.Text, "好吧") && strings.Contains(d.Text, "跟我来")
				t.Log("meeting reading", proof.Readings, v.Scene, d.ID)
			}
			if proof.Grotto && !proof.GrottoSaved || proof.Follow && !proof.FollowSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				if proof.Follow {
					proof.FollowSaved = true
				} else {
					proof.GrottoSaved = true
				}
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if proof.Reaction && !proof.ReactionSaved && v.Phase == "script" {
			save(false)
			proof.ReactionSaved = true
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			stage := 0
			if proof.Started {
				stage++
			}
			if proof.Follow {
				stage++
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 40 {
				t.Fatal("meeting walkthrough repeats a position", place)
			}
			if v.Scene == 20 {
				if !proof.GrottoSaved || !proof.ReactionSaved || !proof.FollowSaved || !proof.Started || !proof.Reaction || v.Map != 8 || v.Cash != 550 || len(v.Party) != 1 || v.Party[0].Role != 0 || v.Field.Actors[0][2] != 2 {
					t.Fatal("original meeting/palace entrance incomplete", proof, v.Map, v.Cash, v.Party)
				}
				save(false)
				sprites := []int{}
				for id := range proof.Sprites {
					sprites = append(sprites, id)
				}
				sort.Ints(sprites)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original opened-way entrance, first fairy meeting, scripted hero sprite/camera changes and palace exterior transition; saved partial reading/script reaction and fresh runtime recovery; not party joining or full campaign", "players": players, "scene": v.Scene, "map": v.Map, "cash": v.Cash, "hp": v.HP, "partyRoles": []int{0}, "heroSprites": sprites, "restoredHeroSprite": 2, "readings": proof.Readings, "saves": proof.Saves, "grottoReadingSaved": proof.GrottoSaved, "scriptReactionSaved": proof.ReactionSaved, "guideReadingSaved": proof.FollowSaved, "partialReadingSaved": players > 1, "staleControlsRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-fairy-meeting.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			x, y, reach := 1872, 488, 32
			switch v.Scene {
			case 19:
			case 17:
				if proof.Follow {
					x, y, reach = 1856, 1040, 32
				} else if proof.Started {
					x, y, reach = 1088, 1424, 0
				} else {
					// This event uses the original shortest search range. Reach
					// its adjacent pose before moving/facing and confirming.
					if v.X != 1088 || v.Y != 1424 {
						g = walker.walk(t, m, &c, g, v, 1088, 1424, 0)
					} else {
						c.Actor = v.Leader
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 3, Keys: 32 | 2})
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
					}
					break
				}
			default:
				t.Fatal("fairy meeting left original route", v.Scene, v.X, v.Y)
			}
			if v.Scene != 17 || proof.Started {
				g = walker.walk(t, m, &c, g, v, x, y, reach)
			}
		} else if v.Battle != nil {
			t.Fatal("unexpected battle in scripted fairy meeting")
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("meeting position", step, v.Scene, v.X, v.Y, v.Phase)
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("original fairy meeting did not reach palace", v.Scene, v.X, v.Y, proof)
	return m, c, g
}
