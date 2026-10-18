package engine

// Failure inventory for settlement, before implementation: award before the
// original result is confirmed; reuse the pre-result period/mask; draw random
// loot; equip a reward automatically; lose ordered inventory/equipment; award
// twice on retries or JSON restore; reward a defeat or final Period28; silently
// write beyond inventory; allow altered result/start snapshots; reset RNG or
// retain old scores/defeated opponents when preparing the next match.
import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

func runCampaignHomeForSettlement(t *testing.T) RuntimeState {
	t.Helper()
	p, choice, _ := campaignHomeFixture(t)
	s, err := NewCampaignRuntime(p, choice)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 48000 && (s.Outcome == nil || !s.Outcome.Confirmed); i++ {
		var held [4]Buttons
		if s.AwaitingNextPeriod && s.WaitVideoFrames == 0 {
			held[0] = ButtonStart
		}
		next, e := StepVideo(s, held)
		if e != nil {
			t.Fatal(s.VideoFrames, e)
		}
		s = next.State
	}
	if s.Outcome == nil || !s.Outcome.Confirmed || s.VideoFrames < 21600 || s.Outcome.WinningTeam != 0 {
		t.Fatal("normal equipped home match did not reach its actual winning result", s.VideoFrames, s.Outcome)
	}
	return s
}

func TestCampaignSettlementAndNextMatch(t *testing.T) {
	var first CampaignMatchResult
	for repeat := 0; repeat < 2; repeat++ {
		s := runCampaignHomeForSettlement(t)
		before, _ := json.Marshal(s)
		result, err := SettleCampaignRuntime(s)
		if err != nil {
			t.Fatal(err)
		}
		// Original reward-calls fixture: rewardSet0/opponent1/postPeriod4 = item0.
		if result.Reward != 0 || !result.RewardStored || result.NextStage != CampaignPrepare || result.Progress.Period != 4 || result.Progress.WinBits != ([2]uint8{1, 1}) || result.Progress.Inventory[0] != 0 || result.Progress.Inventory[1] != campaign.Empty || result.Progress.Equipment != s.Config.Campaign.Equipment || result.Random != s.Transition.Scene.Transition.Random {
			t.Fatal("settlement differs from original earned reward/progress", result)
		}
		again, err := SettleCampaignRuntime(s)
		after, _ := json.Marshal(s)
		if err != nil || !reflect.DeepEqual(again, result) || string(before) != string(after) {
			t.Fatal("retry mutated input or awarded twice", err)
		}
		var restored RuntimeState
		if err = json.Unmarshal(before, &restored); err != nil {
			t.Fatal(err)
		}
		replayed, err := SettleCampaignRuntime(restored)
		if err != nil || !reflect.DeepEqual(replayed, result) {
			t.Fatal("restored settlement differs", err)
		}
		if repeat == 0 {
			first = result
		} else if !reflect.DeepEqual(first, result) {
			t.Fatal("repeated real match changed settlement")
		}
		for _, name := range []string{"unconfirmed", "no-campaign", "changed-score", "already-advanced"} {
			t.Run(name, func(t *testing.T) {
				var bad RuntimeState
				json.Unmarshal(before, &bad)
				switch name {
				case "unconfirmed":
					bad.Outcome.Confirmed = false
				case "no-campaign":
					bad.Config.Campaign = nil
				case "changed-score":
					bad.Outcome.Scores[0]++
				case "already-advanced":
					p := result.Progress
					bad.Config.Campaign = &p
				}
				if _, e := SettleCampaignRuntime(bad); e == nil {
					t.Fatal("accepted invalid settlement", name)
				}
			})
		}
		_, choice, _ := campaignHomeFixture(t)
		choice.Seed = result.Random
		if _, err = NewCampaignRuntime(result.Progress, choice); err == nil {
			t.Fatal("defeated opponent selectable in the next away round")
		}
		opponents, err := campaign.AvailableOpponents(result.Progress)
		if err != nil || len(opponents) != 6 || opponents[0] != 4 {
			t.Fatal("original remaining map order differs", opponents, err)
		}
		choice.Opponent = opponents[0]
		roster, err := SelectOriginalCampaignRoster(result.Progress.Mode, [2]uint8{0, 1}, result.Random)
		if err != nil {
			t.Fatal(err)
		}
		choice.Characters, choice.Seed = roster.Characters, roster.Random
		next, err := NewCampaignRuntime(result.Progress, choice)
		if err != nil {
			t.Fatal(err)
		}
		if next.Opening.Transition.Random != roster.Random || next.Config.Match.Characters != roster.Characters || next.Config.Campaign == nil || *next.Config.Campaign != result.Progress {
			t.Fatal("next match reset random/progress")
		}
		for i := 0; i < 2000 && next.Playing == nil; i++ {
			step, e := StepVideo(next, [4]Buttons{})
			if e != nil {
				t.Fatal(e)
			}
			next = step.State
		}
		if next.Playing == nil || next.Playing.Match.Period != 4 || next.Playing.Context.Difficulty != 4 || next.Playing.Context.Actions.CourtID != 4 || next.Playing.Context.Actions.Teams != ([2]uint8{0, 4}) || next.Playing.World.Preparation.Scores != ([2]uint16{}) {
			t.Fatal("next campaign match did not enter correctly")
		}
		if err = ValidateRuntimeState(next); err != nil {
			t.Fatal(err)
		}
		t.Logf("repeat=%d completed_video=%d scores=%v reward=%d progress_period=%d next_court=%d next_period=%d next_video=%d", repeat, s.VideoFrames, s.Outcome.Scores, result.Reward, result.Progress.Period, next.Config.Match.Court, next.Playing.Match.Period, next.VideoFrames)
	}
}

type campaignSettlementSourceState struct {
	Progress   campaign.Progress
	Random     RandomState
	Characters [4]uint8
}

func TestCampaignSettlementOriginalCases(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Label                    string
			StartingProgress         campaign.Progress
			ConfirmedProgress        campaign.Progress
			Opponent                 uint8
			Won                      bool
			NextStage                CampaignStage
			RewardItem               campaign.Item
			RewardStored             bool
			Before, Confirmed, After campaignSettlementSourceState
			Phases                   []struct {
				Kind     string
				RawAfter []byte
			}
		}
	}
	readPropAttackFixture(t, "testdata/campaign-settlement-cases.json.gz", &fixture)
	if len(fixture.Cases) != 720 {
		t.Fatal("incomplete original result/reward matrix", len(fixture.Cases))
	}
	counts := map[CampaignStage]int{}
	full := 0
	for i, row := range fixture.Cases {
		choice := CampaignMatchSelection{Opponent: row.Opponent, Characters: row.Before.Characters, HumanTactics: [2]uint8{4, 4}, Seed: row.Before.Random}
		s, err := NewCampaignRuntime(row.StartingProgress, choice)
		if err != nil {
			t.Fatal(i, row.Label, err)
		}
		var confirmed []byte
		for _, phase := range row.Phases {
			if phase.Kind == "result-confirm" {
				confirmed = phase.RawAfter
			}
		}
		if len(confirmed) != 2048 {
			t.Fatal("missing original confirmed-result RAM", row.Label)
		}
		// This is an isolated settlement oracle. The confirmed result is from
		// original instructions; the surrounding runtime shell is constructed,
		// not claimed to be an entire naturally played Go/ROM match.
		o := *s.Opening
		o.Transition = transitionRAM(confirmed)
		s.Opening = nil
		s.Transition = &RuntimeTransitionState{Scene: o}
		winner := uint8(0)
		if !row.Won {
			winner = 1
		}
		s.Outcome = &RuntimeOutcome{WinningTeam: winner, Scores: o.Transition.Scores, Confirmed: true, NextScene: o.Transition.Scene, NextPhase: o.Transition.Match.Phase}
		if err = ValidateRuntimeState(s); err != nil {
			t.Fatal("invalid source result shell", i, row.Label, err)
		}
		result, err := SettleCampaignRuntime(s)
		if err != nil {
			t.Fatal(i, row.Label, err)
		}
		if result.Progress != row.After.Progress || result.NextStage != row.NextStage || result.Reward != row.RewardItem || result.RewardStored != row.RewardStored || result.Random != row.Confirmed.Random {
			t.Fatal("original settlement differs", i, row.Label, result, row.After)
		}
		counts[row.NextStage]++
		if row.NextStage == CampaignPrepare && !row.RewardStored {
			full++
		}
	}
	if counts[CampaignPrepare] != 560 || counts[CampaignDefeat] != 128 || counts[CampaignComplete] != 32 || full != 112 {
		t.Fatal("missing original branches", counts, full)
	}
	t.Logf("matched %d original settled results: branches=%v full-inventory-displays=%d", len(fixture.Cases), counts, full)
}

func TestCampaignSettlementRejectsRunningMatch(t *testing.T) {
	p, choice, _ := campaignHomeFixture(t)
	s, err := NewCampaignRuntime(p, choice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SettleCampaignRuntime(s); err == nil {
		t.Fatal("awarded before match result")
	}
}
