package pal_test

// Test first: a real map transition/ride traps; using the visible hammer fails
// to activate the facing statue; a teammate consumes it; partial puzzle or ride
// progress resets on save; the sixth statue does not consume the hammer/open
// the way; shared reading or old input survives incorrectly. Full-map routing
// and coordinates here are walkthrough data, never runtime bot knowledge.
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

func TestOriginalCampaignSixStatuesAndSavedMaze(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			originalOpenedIslandMaze(t, players)
		})
	}
}

func originalOpenedIslandMaze(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalIslandLanding(t, players)
	return runOriginalIslandMaze(t, m, c, g, players, 0)
}

func runOriginalIslandMaze(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players, initialDone int, initialLotus ...[][4]int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	statues := [][3]int{{242, 576, 976}, {238, 432, 488}, {239, 496, 232}, {240, 1472, 272}, {241, 1776, 824}, {243, 1472, 1280}}
	done, scans, saves := initialDone, 0, 0
	attempted, selectedSaved, partialSaved, finalSaved := false, false, false, false
	ridePending, rideSeen, rideSaved, rideComplete := false, false, false, initialDone > 0
	finalText, hammerChecked := false, false
	battles, battleWins, battleCommands := 0, 0, 0
	battleInFlight, battleSaved := false, false
	battleCash := uint32(0)
	rewardXP, rewardCash := -1, -1
	battleRewards := [][2]int{}
	pages := []string{}
	last := uint64(0)
	lastField := [2]int{}
	walker := campaignMapWalker{}
	if len(initialLotus) > 0 {
		walker.lotus = map[[2]int][2]int{}
		for _, r := range initialLotus[0] {
			walker.lotus[[2]int{r[0], r[1]}] = [2]int{r[2], r[3]}
		}
	}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			lotus := [][4]int{}
			for source, dest := range walker.lotus {
				lotus = append(lotus, [4]int{source[0], source[1], dest[0], dest[1]})
			}
			if err := os.WriteFile(campaignEvidencePath("maze-failure-state.json", players), g.Data, 0600); err != nil {
				t.Log(err)
			}
			if err := os.WriteFile(campaignEvidencePath("maze-failure-plan.json", players), raw(map[string]any{"done": done, "attempted": attempted, "rideComplete": rideComplete, "lotus": lotus}), 0600); err != nil {
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
					t.Fatal("maze partial reading changed", actor, read)
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
		if before.Scene != after.Scene || before.Map != after.Map || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || !bytes.Equal(raw(before.Menu), raw(after.Menu)) || !bytes.Equal(raw(before.Overlay), raw(after.Overlay)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
			t.Fatal("maze save changed original progress or drawing")
		}
		c.Actor = after.Leader
		if _, err = m.Apply(c, g, raw(old)); err == nil {
			t.Fatal("maze accepted pre-save input")
		}
		saves++
	}
	for step := 0; step < 7000; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != last {
				last = d.ID
				pages = append(pages, d.Text)
				t.Log("maze story", v.Scene, v.X, v.Y, d.Text)
				finalText = finalText || strings.Contains(d.Text, "第六具石像") && strings.Contains(d.Text, "破天锤")
			}
			if finalText && !finalSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				finalSaved = true
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			c.Actor = v.Leader
			keys := uint32(32)
			if done == 6 {
				if strings.Contains(originalVisibleText(v, false), "破天锤") {
					t.Fatal("sixth statue did not consume hammer")
				}
				hammerChecked, keys = true, 1
			} else if attempted {
				keys = 1
			} else if strings.Contains(originalVisibleText(v, true), "破天锤") {
				if !selectedSaved {
					save(false)
					v = view(t, m, g, 0)
					selectedSaved = true
				}
				if players > 1 {
					wrong := c
					wrong.Actor = (v.Leader + 1) % players
					if _, err := m.Apply(wrong, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, ControlAt: v.ControlAt, Seq: v.Seq + 1, Direction: 4, Keys: 2})); err == nil {
						t.Fatal("nonleader used shared maze item")
					}
				}
				keys, attempted = 2, true
			} else {
				scans++
				if scans > 64 {
					t.Fatal("maze inventory did not select hammer", originalVisibleText(v, false))
				}
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			place := [4]int{v.Scene, v.X, v.Y, done}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("maze walkthrough repeats a position", place)
			}
			if battleInFlight {
				if rewardXP < 0 || rewardCash < 0 || v.Cash != battleCash+uint32(rewardCash) {
					t.Fatal("campaign reward disagrees with original result display", rewardXP, rewardCash, battleCash, v.Cash)
				}
				battleWins++
				battleRewards = append(battleRewards, [2]int{rewardXP, rewardCash})
				battleInFlight = false
				t.Log("campaign battle won", battleWins, v.Cash, v.HP, v.Party)
			}
			if rideSeen {
				rideComplete, ridePending = true, false
			}
			if v.Scene == 19 {
				if done != 6 || !finalText || !hammerChecked || !partialSaved || !rideSaved || !finalSaved || !selectedSaved || v.Map != 25 || len(v.Party) != 1 || v.Cash < 550 || battles == 0 || battleWins != battles || !battleSaved {
					t.Fatal("original maze incomplete", done, finalText, hammerChecked, partialSaved, rideSaved, finalSaved, v.Map, v.Cash)
				}
				save(false)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original shore/forest/maze with campaign battles and original rewards, six facing hammer uses, saved battle/selected item/partial puzzle/moving lotus/final shared reading, hammer consumption and original opened-way map; not full campaign", "players": players, "pages": pages, "destroyedStatues": done, "selectedHammerSaved": selectedSaved, "partialPuzzleSaved": partialSaved, "movingLotusSaved": rideSaved, "finalReadingSaved": finalSaved, "hammerConsumed": hammerChecked, "scene": v.Scene, "map": v.Map, "cash": v.Cash, "saves": saves, "campaignBattles": battles, "campaignBattleWins": battleWins, "battleCommands": battleCommands, "battleSaved": battleSaved, "battleRewards": battleRewards}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-six-statues.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			if v.Scene == 18 && attempted && done < 6 && fieldTestDistance(v.X, v.Y, statues[done][1], statues[done][2]) <= 96 {
				visible := false
				for _, actor := range v.Field.Actors {
					visible = visible || actor[0] == 1 && actor[1] == statues[done][0]
				}
				if !visible {
					done++
					attempted, scans = false, 0
					t.Log("destroyed statue", done, v.X, v.Y)
				}
			}
			if done == 3 && !partialSaved {
				save(false)
				v = view(t, m, g, 0)
				partialSaved = true
			}
			c.Actor = v.Leader
			if done == 6 && !hammerChecked {
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
			} else {
				x, y, reach := 1472, 448, 0
				switch v.Scene {
				case 15:
				case 16:
					x, y = 1824, 944
				case 18:
					ridePending = true
					if done == 1 && !rideComplete {
						x, y, ridePending = 944, 888, true
					} else if done == 6 {
						x, y = 1024, 912
					} else {
						x, y, reach = statues[done][1], statues[done][2], 32
					}
				default:
					t.Fatal("maze walkthrough left route", v.Scene, v.X, v.Y)
				}
				if reach == 32 && fieldTestDistance(v.X, v.Y, x, y) <= 32 {
					direction := -1
					for d, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
						if v.X+delta[0] == x && v.Y+delta[1] == y {
							direction = d
						}
					}
					if direction < 0 {
						t.Fatal("maze search pose not aligned", v.X, v.Y, x, y)
					}
					keys := []uint32{4, 8, 16, 32}[direction]
					if v.Field.Direction == direction {
						direction, keys = 4, 2048
					}
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
				} else {
					g = walker.walk(t, m, &c, g, v, x, y, reach)
				}
			}
			lastField = [2]int{v.X, v.Y}
		} else if v.Scene == 18 && v.Phase == "script" && ridePending && fieldTestDistance(v.X, v.Y, lastField[0], lastField[1]) > 32 {
			rideSeen = true
			if !rideSaved {
				save(false)
				rideSaved = true
			}
		} else if v.Battle != nil {
			if !battleInFlight {
				battles++
				battleInFlight, battleCash = true, v.Cash
				rewardXP, rewardCash = -1, -1
				t.Log("campaign battle", battles, v.Battle.Enemies, v.Party)
			}
			if xp, cash, ok := campaignBattleReward(v); ok {
				rewardXP, rewardCash = xp, cash
			}
			if v.Battle.Phase == 0 && v.Battle.State == 1 && !battleSaved {
				save(false)
				v = view(t, m, g, 0)
				battleSaved = true
			}
			alive := false
			for _, player := range v.Battle.Players {
				alive = alive || player.HP > 0
			}
			if !alive {
				t.Fatal("campaign battle party defeated", v.Battle.Players)
			}
			for _, actor := range g.Actors {
				c.Actor = actor
				before := bytes.Clone(g.Data)
				action, err := m.BotAction(c, g)
				if err != nil || !bytes.Equal(before, g.Data) {
					t.Fatal("campaign battle bot failed or mutated source", err)
				}
				if len(action) > 0 {
					g, err = m.Apply(campaignBotContext(c), g, action)
					if err != nil {
						t.Fatal(err)
					}
					battleCommands++
				}
			}
		}
		offset := int64(200)
		if v.Scene == 18 && v.Phase != "menu" {
			offset = 0
		}
		c.Now = time.UnixMilli(g.SystemDeadline + offset)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("maze walk", step, v.Scene, v.X, v.Y, v.Phase, done)
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("original maze did not finish", v.Scene, v.X, v.Y, done, attempted)
	return m, c, g
}
