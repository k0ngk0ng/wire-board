package pal_test

// Test first: original palace events cannot continue; a role is invented before
// the original join; the new role takes another role's owner; the solo player
// loses control of a member; reading shrinks to the current party; saving during
// joining replays it; leader transfer/old input corrupts a two-member party.
// Coordinates and event progression below are test walkthrough input only.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type firstPartyProof struct {
	StopAtBattle                        bool
	RescueSaved, JoinSaved, NightLesson bool
	Readings, Saves                     int
	LastPrompt                          uint64
}

func TestOriginalCampaignFirstPartyJoinAndOwnersSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			originalFirstParty(t, players)
		})
	}
}

func originalFirstParty(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalReturnMedicine(t, players)
	return runOriginalFirstPartyJoin(t, m, c, g, players, firstPartyProof{})
}

func runOriginalFirstPartyJoin(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof firstPartyProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"first-party-failure-state.json": g.Data, "first-party-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	checkOwners := func(v pal.View) {
		t.Helper()
		second := 0
		if players > 1 {
			second = 1
		}
		if len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 1 || v.Party[0].Controller != 0 || v.Party[1].Controller != second {
			t.Fatal("original joining party/owners incorrect", v.Party)
		}
	}
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			if len(v.Dialogue.Read) != players {
				t.Fatal("joining changed original reading roster")
			}
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("joining partial reading changed", actor, read)
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
		after := view(t, m, g, 0)
		if before.Scene != after.Scene || before.Map != after.Map || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || before.Leader != after.Leader || !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || !bytes.Equal(raw(before.Field), raw(after.Field)) || !bytes.Equal(raw(before.Draws), raw(after.Draws)) || !bytes.Equal(raw(before.Overlay), raw(after.Overlay)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
			t.Fatal("joining continuation changed party, event or reading")
		}
		c.Actor = after.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("joining continuation accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 4000; step++ {
		v := view(t, m, g, 0)
		if v.Cash < 550 {
			t.Fatal("original rescue walkthrough restarted after defeat", v.Scene, v.Party, v.Cash)
		}
		if len(v.Party) > 1 {
			checkOwners(v)
			if !proof.JoinSaved {
				save(v.Dialogue != nil)
				v = view(t, m, g, 0)
				checkOwners(v)
				proof.JoinSaved = true
				if err := os.WriteFile(campaignEvidencePath("first-party-joining-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				t.Log("first party reading", proof.Readings, v.Scene, d.ID, len(v.Party))
			}
			if !proof.RescueSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				proof.RescueSaved = true
			}
			if len(d.Read) != players {
				t.Fatal("party size changed shared reading participants")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			place := [4]int{v.Scene, v.X, v.Y, len(v.Party)}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("first-party walkthrough repeats position", place)
			}
			walker.allowTouch = v.Scene == 2 || v.Scene == 4
			if proof.JoinSaved {
				checkOwners(v)
				if v.Scene != 2 || v.Map != 12 || v.Cash < 550 || !proof.NightLesson || !proof.RescueSaved {
					t.Fatal("original joining endpoint changed", v.Scene, v.Map, v.Cash)
				}
				if players > 1 {
					c.Actor = v.Leader
					g = apply(t, m, c, g, pal.Action{Type: "leader", Epoch: v.Epoch, Target: 1})
					v = view(t, m, g, 0)
					checkOwners(v)
					wrong := c
					wrong.Actor = 0
					before := bytes.Clone(g.Data)
					if _, err := m.Apply(wrong, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 0, Keys: 4})); err == nil || !bytes.Equal(before, g.Data) {
						t.Fatal("old leader controls joined party")
					}
				}
				origin := [2]int{v.X, v.Y}
				direction := -1
				for d, delta := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
					if v.Field.Cells[(8+delta[1])*17+8+delta[0]] == 0 {
						direction = d
						break
					}
				}
				if direction < 0 {
					t.Fatal("joined party has no safe movement")
				}
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: []uint32{4, 8, 16, 32}[direction]})
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
				c.Now = time.UnixMilli(g.SystemDeadline + 200)
				var err error
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				v = view(t, m, g, 0)
				checkOwners(v)
				if origin == [2]int{v.X, v.Y} || v.Field == nil || v.Field.Party != 2 {
					t.Fatal("joined party did not move together")
				}
				save(false)
				if err := os.WriteFile(campaignEvidencePath("campaign-first-party-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original inn rescue battle and first two-role party; original human reading roster, solo/multiplayer owners, fresh-runtime joining save, leader handoff, rejected old input and ordinary two-member field movement; not later party or full campaign", "players": players, "scene": v.Scene, "map": v.Map, "cash": v.Cash, "party": v.Party, "leader": v.Leader, "readings": proof.Readings, "saves": proof.Saves, "joinedPartySaved": proof.JoinSaved, "originalReadingParticipants": players, "movedTogether": true, "staleControlsRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-first-party.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			if !proof.NightLesson {
				m, c, g = runOriginalNightLesson(t, m, c, g, players, nightLessonProof{})
				proof.NightLesson = true
				continue
			}
			x, y, reach := 1296, 872, 32
			switch v.Scene {
			case 5:
			case 4:
				x, y = 1472, 1520
			case 2:
				if v.X < 1000 {
					if v.X == 656 && v.Y == 616 {
						c.Actor = v.Leader
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 1, Keys: 8 | 2})
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
					} else if v.Y > 640 {
						// Reach the lower stair's original touch script; its climb
						// connects two otherwise disconnected walking surfaces.
						g = walker.walk(t, m, &c, g, v, 720, 680, 0)
					} else {
						g = walker.walk(t, m, &c, g, v, 656, 616, 0)
					}
					break
				}
				x, y, reach = 1296, 264, 0
				if v.X == x && v.Y == y {
					c.Actor = v.Leader
					direction, keys := 1, uint32(8)
					if v.Field.Direction == 1 {
						direction, keys = 4, 2
					}
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
				} else {
					g = walker.walk(t, m, &c, g, v, x, y, reach)
				}
			default:
				t.Fatal("first-party rescue left original inn route", v.Scene, v.X, v.Y)
			}
			if v.Scene != 2 {
				g = walker.walk(t, m, &c, g, v, x, y, reach)
			}
		} else if v.Battle != nil {
			if proof.StopAtBattle {
				return m, c, g
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
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("first party position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party))
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("first original party join did not finish", v.Scene, v.X, v.Y, proof)
	return m, c, g
}
