package engine

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

type CampaignStage string

const (
	CampaignPrepare  CampaignStage = "prepare"
	CampaignDefeat   CampaignStage = "defeat"
	CampaignComplete CampaignStage = "complete"
)

// CampaignMatchResult is the next preparation state derived from an immutable
// confirmed match. Reward is the displayed item; RewardStored distinguishes
// the original full-inventory case, where the item is shown but not inserted.
type CampaignMatchResult struct {
	Progress     campaign.Progress `json:"progress"`
	Reward       campaign.Item     `json:"reward"`
	RewardStored bool              `json:"rewardStored"`
	NextStage    CampaignStage     `json:"nextStage"`
	Random       RandomState       `json:"random"`
}

// SettleCampaignRuntime consumes the authoritative result, never a requested
// winner or score. It performs the original first-entry award transaction, not
// the reward animation or the defeat Continue/quit input. It draws no randomness
// and does not mutate the match: retries and restored results yield the same
// complete next progress, rather than appending another item to live inventory.
func SettleCampaignRuntime(s RuntimeState) (CampaignMatchResult, error) {
	if s.Config.Campaign == nil || s.Outcome == nil || !s.Outcome.Confirmed {
		return CampaignMatchResult{}, fmt.Errorf("campaign settlement requires a confirmed campaign result")
	}
	if err := ValidateRuntimeState(s); err != nil {
		return CampaignMatchResult{}, err
	}
	t := s.Transition.Scene.Transition
	result := CampaignMatchResult{Progress: *s.Config.Campaign, Reward: campaign.Empty, NextStage: CampaignPrepare, Random: t.Random}
	result.Progress.Period, result.Progress.WinBits = t.Match.Period, t.TeamWinBits
	switch {
	case s.Outcome.WinningTeam != 0:
		result.NextStage = CampaignDefeat
	case result.Progress.Period == 28:
		result.NextStage = CampaignComplete
	default:
		item, err := campaign.RewardItem(result.Progress.RewardSet, s.Config.Match.Teams[1], result.Progress.Period)
		if err != nil {
			return CampaignMatchResult{}, err
		}
		result.Reward = item
		for i, held := range result.Progress.Inventory {
			if held == campaign.Empty {
				result.Progress.Inventory[i] = item
				result.RewardStored = true
				break
			}
		}
	}
	if err := result.Progress.Validate(); err != nil {
		return CampaignMatchResult{}, err
	}
	return result, nil
}
