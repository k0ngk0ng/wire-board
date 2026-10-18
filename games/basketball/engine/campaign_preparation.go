package engine

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

const maxCampaignPreparationRevision = 1<<53 - 1

// CampaignPreparation stores accepted menu operations, separately from the
// loadout used in the preceding half. Closing a UI submits no inverse operation.
// Progress.Period stays at the earned even baseline; Period identifies the live
// odd intermission. The host additionally binds requests to its match epoch.
type CampaignPreparation struct {
	Progress     campaign.Progress `json:"progress"`
	Teammates    [2]uint8          `json:"teammates"`
	HumanTactics [2]uint8          `json:"humanTactics"`
	Period       uint8             `json:"period"`
	Revision     uint64            `json:"revision"`
}

type CampaignPreparationView struct {
	CampaignPreparation
	CanChooseTactic bool `json:"canChooseTactic"`
}

// An equip request names an owned inventory index, never a free item ID.
// HumanTactics are per team. A campaign can edit only the Nekketsu team's
// instruction, and only while it has exactly one human and one computer.
type CampaignPreparationAction struct {
	Type           string        `json:"type"`
	Period         uint8         `json:"period"`
	Revision       uint64        `json:"revision"`
	Character      uint8         `json:"character,omitempty"`
	InventoryIndex uint8         `json:"inventoryIndex,omitempty"`
	Slot           campaign.Slot `json:"slot,omitempty"`
	Teammates      [2]uint8      `json:"teammates,omitempty"`
	Tactic         uint8         `json:"tactic,omitempty"`
}

type CampaignPreparationStep struct {
	State         RuntimeState
	DiscardedItem campaign.Item
}

func validateCampaignPreparationValues(p CampaignPreparation) error {
	if err := p.Progress.Validate(); err != nil {
		return err
	}
	if p.Period > 27 || p.Period&1 == 0 || p.Period != p.Progress.Period+1 || p.Revision > maxCampaignPreparationRevision {
		return fmt.Errorf("invalid campaign preparation window or revision")
	}
	if p.Teammates[0] >= 4 || p.Teammates[1] >= 4 || p.Teammates[0] == p.Teammates[1] {
		return fmt.Errorf("invalid campaign teammate selection")
	}
	for _, tactic := range p.HumanTactics {
		if tactic < 1 || tactic > 5 {
			return fmt.Errorf("invalid original campaign tactic")
		}
	}
	return nil
}

func campaignOwnedItems(p campaign.Progress) (counts [32]uint8, total int) {
	add := func(item campaign.Item) {
		if item != campaign.Empty {
			counts[item]++
			total++
		}
	}
	for _, item := range p.Inventory {
		add(item)
	}
	for _, slots := range p.Equipment {
		for _, item := range slots {
			add(item)
		}
	}
	return
}

func validatePendingCampaignPreparation(s RuntimeState) error {
	if s.CampaignPreparation == nil {
		return nil
	}
	if s.Config.Campaign == nil || !s.AwaitingNextPeriod || s.Transition == nil || s.Carry == nil || s.Outcome != nil {
		return fmt.Errorf("campaign preparation outside an active intermission")
	}
	p, base := *s.CampaignPreparation, *s.Config.Campaign
	if err := validateCampaignPreparationValues(p); err != nil {
		return err
	}
	if p.Period != s.Transition.Scene.Transition.Match.Period || p.Progress.Mode != base.Mode || p.Progress.Period != base.Period || p.Progress.WinBits != base.WinBits || p.Progress.RewardSet != base.RewardSet || p.HumanTactics[1] != s.Config.HumanTactics[1] {
		return fmt.Errorf("campaign preparation changed earned progress or opposing-team tactics")
	}
	before, totalBefore := campaignOwnedItems(base)
	after, totalAfter := campaignOwnedItems(p.Progress)
	for item, count := range after {
		if count > before[item] {
			return fmt.Errorf("campaign preparation contains unowned item %d", item)
		}
	}
	// Original removal can discard only with all16 inventory cells occupied.
	// Accepted operations below establish provenance; this restore invariant
	// rejects creation and losses impossible even under that capacity behavior.
	if totalAfter < totalBefore && totalAfter < 16 {
		return fmt.Errorf("campaign preparation lost owned equipment outside full-inventory removal")
	}
	return nil
}

func ViewCampaignPreparation(s RuntimeState) (CampaignPreparationView, error) {
	if err := ValidateRuntimeState(s); err != nil {
		return CampaignPreparationView{}, err
	}
	if s.Config.Campaign == nil || !s.AwaitingNextPeriod || s.Transition == nil || s.Outcome != nil {
		return CampaignPreparationView{}, fmt.Errorf("campaign preparation is not available")
	}
	p := CampaignPreparation{Progress: *s.Config.Campaign, Teammates: [2]uint8{s.Config.Match.Characters[0], s.Config.Match.Characters[1]}, HumanTactics: s.Config.HumanTactics, Period: s.Transition.Scene.Transition.Match.Period}
	if s.CampaignPreparation != nil {
		p = *s.CampaignPreparation
	}
	humans := 0
	for _, controller := range s.Config.Match.ControllerAssignments[:2] {
		if controller >= 0 {
			humans++
		}
	}
	return CampaignPreparationView{CampaignPreparation: p, CanChooseTactic: humans == 1}, nil
}

func editCampaignPreparation(original CampaignPreparation, a CampaignPreparationAction, canChooseTactic bool) (CampaignPreparation, campaign.Item, error) {
	fail := func(err error) (CampaignPreparation, campaign.Item, error) { return original, campaign.Empty, err }
	if err := validateCampaignPreparationValues(original); err != nil {
		return fail(err)
	}
	if a.Period != original.Period || a.Revision != original.Revision {
		return fail(fmt.Errorf("stale campaign preparation action"))
	}
	p := original
	discarded := campaign.Empty
	var err error
	switch a.Type {
	case "equip":
		p.Progress, err = campaign.Equip(p.Progress, a.Character, a.InventoryIndex)
	case "unequip":
		var result campaign.UnequipStep
		result, err = campaign.Unequip(p.Progress, a.Character, a.Slot)
		p.Progress, discarded = result.Progress, result.DiscardedItem
	case "teammates":
		p.Teammates = a.Teammates
	case "tactic":
		if !canChooseTactic {
			return fail(fmt.Errorf("partner tactics require one human and one computer on the team"))
		}
		p.HumanTactics[0] = a.Tactic
	default:
		return fail(fmt.Errorf("unknown campaign preparation action %q", a.Type))
	}
	if err != nil {
		return fail(err)
	}
	if err = validateCampaignPreparationValues(p); err != nil {
		return fail(err)
	}
	if p == original {
		return original, campaign.Empty, nil
	}
	if p.Revision == maxCampaignPreparationRevision {
		return fail(fmt.Errorf("campaign preparation revision exhausted"))
	}
	p.Revision++
	return p, discarded, nil
}

// ApplyCampaignPreparation changes accepted preparation only. It advances no
// video frame, RNG, physics or clock. Runtime snapshots never call this implicitly.
// Seat/host authorization and epoch binding remain the platform adapter's job.
func ApplyCampaignPreparation(original RuntimeState, a CampaignPreparationAction) (CampaignPreparationStep, error) {
	initial := CampaignPreparationStep{State: original, DiscardedItem: campaign.Empty}
	view, err := ViewCampaignPreparation(original)
	if err != nil {
		return initial, err
	}
	p, discarded, err := editCampaignPreparation(view.CampaignPreparation, a, view.CanChooseTactic)
	if err != nil {
		return initial, err
	}
	if p == view.CampaignPreparation {
		return initial, nil
	}
	s := cloneRuntime(original)
	s.CampaignPreparation = &p
	if err = ValidateRuntimeState(s); err != nil {
		return initial, err
	}
	return CampaignPreparationStep{State: s, DiscardedItem: discarded}, nil
}
