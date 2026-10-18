package basketball

import (
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

type CampaignEquipmentPreview struct {
	Index uint8                     `json:"index"`
	Item  campaign.Item             `json:"item"`
	After [4]engine.CharacterGrades `json:"after"`
}
type CampaignAttributes struct {
	Current   [4]engine.CharacterGrades  `json:"current"`
	Inventory []CampaignEquipmentPreview `json:"inventory"`
}

func campaignCharacterGrades(p campaign.Progress, character uint8) (engine.CharacterGrades, error) {
	base, err := engine.OriginalCharacterGrades(0, character)
	if err != nil {
		return base, err
	}
	slots := p.Equipment[character]
	return engine.ApplyOriginalEquipmentGrades(base, [3]int8{int8(slots[0]), int8(slots[1]), int8(slots[2])})
}

// Pure presentation: current stats reproduce original Scene2's equipped grades.
// Prospective stats compose the owned source Equip operation with the same Go
// grade program on a copy. Neither preview changes source preparation or RNG.
func campaignAttributes(p campaign.Progress) (CampaignAttributes, error) {
	view := CampaignAttributes{Inventory: []CampaignEquipmentPreview{}}
	if err := p.Validate(); err != nil {
		return view, err
	}
	for character := uint8(0); character < 4; character++ {
		grade, err := campaignCharacterGrades(p, character)
		if err != nil {
			return view, err
		}
		view.Current[character] = grade
	}
	for index, item := range p.Inventory {
		if item == campaign.Empty {
			continue
		}
		preview := CampaignEquipmentPreview{Index: uint8(index), Item: item}
		for character := uint8(0); character < 4; character++ {
			next, err := campaign.Equip(p, character, uint8(index))
			if err != nil {
				return view, err
			}
			grade, err := campaignCharacterGrades(next, character)
			if err != nil {
				return view, err
			}
			preview.After[character] = grade
		}
		view.Inventory = append(view.Inventory, preview)
	}
	return view, nil
}
