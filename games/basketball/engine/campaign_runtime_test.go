package engine

// Failure inventory before implementation: derive the opponent from the home
// court; reset earned period/win masks; use inventory as free equipment; bind
// equipment to a seat instead of a character; invent RNG/reward draws; expose
// internal tactic0; accept completed/malformed progress or defeated opponents;
// drop progress/difficulty/equipment on JSON restore or intermission; break
// source AI when the platform takes over either cooperative seat.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

type campaignHomeOracle struct {
	Config       OriginalMatchConfiguration
	HumanTactics [2]uint8
	Period       uint8
	WinBits      [2]uint8
	Grades       [4]CharacterGrades
	Controls     [4]uint8
}

func campaignHomeFixture(t *testing.T) (campaign.Progress, CampaignMatchSelection, campaignHomeOracle) {
	t.Helper()
	var f campaignHomeOracle
	raw, err := os.ReadFile("testdata/campaign-home-entry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	p, err := campaign.DecodePassword("020010")
	if err != nil {
		t.Fatal(err)
	}
	p, err = campaign.Equip(p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	selection := CampaignMatchSelection{Opponent: f.Config.Teams[1], Characters: f.Config.Characters, HumanTactics: f.HumanTactics, Seed: f.Config.Seed}
	return p, selection, f
}

func TestCampaignPasswordEquipmentRuntimeFlow(t *testing.T) {
	p, selection, f := campaignHomeFixture(t)
	for _, mode := range []string{"solo", "solo-automated", "coop-automated"} {
		t.Run(mode, func(t *testing.T) {
			progress := p
			if mode == "coop-automated" {
				progress.Mode = 1
			}
			var firstHash string
			for repeat := 0; repeat < 2; repeat++ {
				s, err := NewCampaignRuntime(progress, selection)
				if err != nil {
					t.Fatal(err)
				}
				if s.Config.Campaign == nil || *s.Config.Campaign != progress {
					t.Fatal("starting progress is missing from the resumable runtime configuration")
				}
				want := f.Config
				if progress.Mode == 1 {
					want.Mode = 1
					want.ControllerAssignments[1] = 1
				}
				if s.Config.Match != want || s.Config.HumanTactics != f.HumanTactics || s.Opening == nil || s.Opening.Grades != f.Grades || s.Opening.Transition.Match.Period != f.Period || s.Opening.Transition.TeamWinBits != f.WinBits || s.Opening.Transition.Random != selection.Seed {
					t.Fatal("campaign constructor differs from source menu selections/attributes", s.Config, s.Opening)
				}
				if mode != "solo" {
					s, err = SetRuntimeAutomation(s, [4]bool{true, true, true, true})
					if err != nil {
						t.Fatal(err)
					}
				}
				seen := [2]bool{}
				intermission := false
				var halfScores [2]uint16
				restores := 0
				digest := sha256.New()
				for frame := 0; frame < 48000 && (s.Outcome == nil || !s.Outcome.Confirmed); frame++ {
					var held [4]Buttons
					if s.AwaitingNextPeriod && s.WaitVideoFrames == 0 {
						held[0] = ButtonStart
					}
					step, e := StepVideo(s, held)
					if e != nil {
						t.Fatalf("video%d: %v", s.VideoFrames, e)
					}
					if frame%300 == 0 || s.AwaitingNextPeriod {
						bundle := struct {
							Progress campaign.Progress
							Runtime  RuntimeState
						}{progress, s}
						raw, e := json.Marshal(bundle)
						if e != nil {
							t.Fatal(e)
						}
						var restored struct {
							Progress campaign.Progress
							Runtime  RuntimeState
						}
						if e = json.Unmarshal(raw, &restored); e != nil {
							t.Fatal(e)
						}
						if restored.Progress != progress {
							t.Fatal("campaign inventory/equipment changed on restore")
						}
						if e = ValidateRuntimeState(restored.Runtime); e != nil {
							t.Fatal(e)
						}
						again, e := StepVideo(restored.Runtime, held)
						if e != nil || !reflect.DeepEqual(again, step) {
							t.Fatal("restored video, display or audio differs", e)
						}
						restores++
					}
					s = step.State
					if step.LogicAdvanced {
						raw, _ := json.Marshal(step)
						digest.Write(raw)
					}
					if s.Playing != nil {
						w := s.Playing
						if w.Match.Period < 2 || w.Match.Period > 3 || w.Context.Difficulty != w.Match.Period || w.TeamWinBits != f.WinBits {
							t.Fatal("lost campaign half-period/difficulty/wins")
						}
						wantCharacters := f.Config.Characters
						if w.Match.Period == 3 {
							// Original member confirmation selects a new opponent
							// roster; its RNG/commit is checked by intermission oracles.
							wantCharacters = s.Config.Match.Characters
						}
						if w.Context.Actions.CourtID != 0 || w.Context.Actions.Teams != f.Config.Teams || w.Context.Actions.Characters != wantCharacters || w.Context.Actions.Equipment != 1 {
							t.Fatal("home match lost team/character/equipment selection")
						}
						if mode == "solo" && w.Context.Actions.Controls != f.Controls {
							t.Fatal("source controller assignment mismatch")
						}
						if mode != "solo" && w.Context.Actions.Controls != ([4]uint8{128, 128, 128, 128}) {
							t.Fatal("platform takeover failed")
						}
						if w.Match.Period == 3 && !seen[1] && (!intermission || w.World.Preparation.Scores != halfScores) {
							t.Fatal("intermission lost score")
						}
						seen[w.Match.Period-2] = true
					}
					if s.AwaitingNextPeriod && !intermission {
						if !seen[0] || s.VideoFrames < 10800 || s.Transition.Scene.Transition.Match.Period != 3 {
							t.Fatal("skipped natural first half")
						}
						halfScores = s.Transition.Scene.Transition.Scores
						intermission = true
					}
				}
				if !seen[0] || !seen[1] || restores < 72 || s.VideoFrames < 21600 || s.Outcome == nil || !s.Outcome.Confirmed {
					raw, _ := json.Marshal(s)
					t.Logf("before=%s", raw)
					t.Fatal("campaign match did not finish normally", s.VideoFrames, restores, s.Outcome)
				}
				if err = ValidateRuntimeState(s); err != nil {
					t.Fatal(err)
				}
				out, transition := s.Outcome, s.Transition.Scene.Transition
				wantPeriod, wantWins := uint8(2), f.WinBits
				if out.WinningTeam == 0 {
					wantPeriod = 4
					wantWins[1] |= 1
				}
				if transition.Match.Period != wantPeriod || transition.TeamWinBits != wantWins || out.Scores[out.WinningTeam] <= out.Scores[out.WinningTeam^1] {
					t.Fatal("source result progression differs", transition.Match.Period, transition.TeamWinBits, out)
				}
				hash := fmt.Sprintf("%x", digest.Sum(nil))
				if repeat == 0 {
					firstHash = hash
				} else if hash != firstHash {
					t.Fatal("repeat changed trajectory")
				}
				t.Logf("mode=%s repeat=%d videos=%d logic=%d restores=%d scores=%v trajectory_sha256=%s; normal typed campaign preparation and runtime, not full original trajectory equality", mode, repeat, s.VideoFrames, s.LogicSteps, restores, out.Scores, hash)
			}
		})
	}
}

func TestCampaignRuntimeRejectsInvalidPreparation(t *testing.T) {
	p, selection, _ := campaignHomeFixture(t)
	for _, name := range []string{"mode", "period", "inventory", "equipment", "opponent0", "opponent8", "defeated", "completed", "duplicate-human", "duplicate-opponent", "character", "tactic0", "tactic6"} {
		t.Run(name, func(t *testing.T) {
			progress, choice := p, selection
			switch name {
			case "mode":
				progress.Mode = 128
			case "period":
				progress.Period = 4
			case "inventory":
				progress.Inventory[1] = 1
			case "equipment":
				progress.Equipment[0][1] = 1
			case "opponent0":
				choice.Opponent = 0
			case "opponent8":
				choice.Opponent = 8
			case "defeated":
				progress.WinBits = [2]uint8{1, 1}
				progress.Period = 4
			case "completed":
				progress.WinBits = [2]uint8{127, 127}
				progress.Period = 28
			case "duplicate-human":
				choice.Characters[1] = choice.Characters[0]
			case "duplicate-opponent":
				choice.Characters[3] = choice.Characters[2]
			case "character":
				choice.Characters[3] = 4
			case "tactic0":
				choice.HumanTactics[0] = 0
			case "tactic6":
				choice.HumanTactics[1] = 6
			}
			got, err := NewCampaignRuntime(progress, choice)
			if err == nil || !reflect.DeepEqual(got, RuntimeState{}) {
				t.Fatal("invalid preparation produced a runtime", err)
			}
		})
	}
}

func TestCampaignRuntimeEquipmentBelongsToCharacter(t *testing.T) {
	p, selection, f := campaignHomeFixture(t)
	selection.Characters[0], selection.Characters[1] = 1, 0
	s, err := NewCampaignRuntime(p, selection)
	if err != nil {
		t.Fatal(err)
	}
	if s.Config.Match.Equipment != f.Config.Equipment || s.Opening.Grades[0] != f.Grades[1] || s.Opening.Grades[1] != f.Grades[0] {
		t.Fatal("equipment followed seat instead of character")
	}
	if p.Equipment[0][0] != 1 || p.Inventory[0] != campaign.Empty {
		t.Fatal("constructor changed owned equipment")
	}
}

func TestCampaignRuntimeInventoryAndShotRestoreValidation(t *testing.T) {
	progress, err := campaign.New(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	progress.Period, progress.WinBits = 16, [2]uint8{15, 15}
	password, err := campaign.EncodePassword(progress)
	if err != nil {
		t.Fatal(err)
	}
	progress, err = campaign.DecodePassword(password)
	if err != nil || progress.Inventory[7] == campaign.Empty {
		t.Fatal("source password must restore eight earned items", err)
	}
	_, selection, _ := campaignHomeFixture(t)
	selection.Opponent = 5
	s, err := NewCampaignRuntime(progress, selection)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2000 && s.Playing == nil; i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Playing == nil {
		t.Fatal("no live campaign world")
	}
	reserved := s.Playing.Context.PursuitReserved
	if reserved == nil || reserved.XHigh88 == nil || *reserved.XHigh88 != uint8(progress.Inventory[7]) {
		t.Fatal("inventory alias reset to FF")
	}
	if err = ValidateRuntimeState(s); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shot", "inventory", "config-equipment", "config-progress"} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(s)
			var bad RuntimeState
			json.Unmarshal(raw, &bad)
			switch name {
			case "shot":
				bad.Playing.Context.Actions.ShotEquipment[0] = 1
			case "inventory":
				*bad.Playing.Context.PursuitReserved.XHigh88 = 255
			case "config-equipment":
				bad.Config.Match.Equipment[0][0] = 1
			case "config-progress":
				bad.Config.Campaign.Period = 0
			}
			if ValidateRuntimeState(bad) == nil {
				t.Fatal("accepted inconsistent restored campaign", name)
			}
		})
	}
	next, err := StepVideo(s, [4]Buttons{})
	if err != nil {
		t.Fatal(err)
	}
	s.Config.Campaign.Inventory[7] = campaign.Empty
	if next.State.Config.Campaign.Inventory[7] != progress.Inventory[7] {
		t.Fatal("runtime snapshots share mutable campaign configuration")
	}
}
