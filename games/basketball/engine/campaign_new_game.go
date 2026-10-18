package engine

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

// CampaignStart is newly initialized preparation and the shared random register
// after the original one-time reward-table draw. It is not a running match.
type CampaignStart struct {
	Progress campaign.Progress `json:"progress"`
	Random   RandomState       `json:"random"`
}

// NewOriginalCampaign composes original empty preparation with A205's single
// reward-table draw. Its input is the already initialized RNG immediately before
// A205: the caller owns any preceding CABC outer-loop draw or bootstrap inputs.
// Mode 0/1 selection is independent of that draw. This is an explicit new-game
// operation, never a restore, Continue, player-count edit or match constructor.
func NewOriginalCampaign(mode uint8, randomBeforeRewardDraw RandomState) (CampaignStart, error) {
	initial := CampaignStart{Random: randomBeforeRewardDraw}
	if mode > 1 {
		return initial, fmt.Errorf("new original campaign requires mode 0 or 1, got %d", mode)
	}
	next, value := NextRandom(randomBeforeRewardDraw)
	progress, err := campaign.New(mode, value&3)
	if err != nil {
		return initial, err
	}
	return CampaignStart{Progress: progress, Random: next}, nil
}
