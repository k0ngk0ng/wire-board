package engine

import "fmt"

// CampaignRosterSelection is the roster ready for the original B2EF commit.
// Grade calculation and match initialization consume this committed selection;
// they must not draw a replacement opponent roster themselves.
type CampaignRosterSelection struct {
	Characters [4]uint8    `json:"characters"`
	Random     RandomState `json:"random"`
}

// SelectOriginalCampaignRoster restores bank3 896D's automatic opponent
// selection for the platform's campaign modes 0/1. Invoke it once at an actual
// member-selection confirmation, using the current shared RNG. NewRuntime,
// snapshot restoration and grade calculation must not implicitly call it.
// Invalid input returns no roster and preserves the supplied random state.
func SelectOriginalCampaignRoster(mode uint8, teammates [2]uint8, random RandomState) (CampaignRosterSelection, error) {
	initial := CampaignRosterSelection{Random: random}
	if mode > 1 {
		return initial, fmt.Errorf("automatic opponent roster requires campaign mode 0 or 1, got %d", mode)
	}
	if teammates[0] >= 4 || teammates[1] >= 4 || teammates[0] == teammates[1] {
		return initial, fmt.Errorf("campaign teammates must be distinct original character slots 0..3")
	}
	next, value := NextRandom(random)
	// Original 8986 contains 1,2,3,1: first opponent is always0; there is no
	// uniform three-way draw or retry to avoid duplicate characters.
	partner := [4]uint8{1, 2, 3, 1}[value&3]
	return CampaignRosterSelection{Characters: [4]uint8{teammates[0], teammates[1], 0, partner}, Random: next}, nil
}
