package basketball

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func defaultOptions() Options {
	return Options{Court: "new-york", Teams: [2]uint8{0, 1}, Characters: [4]uint8{0, 1, 0, 1}}
}

// Assets exist for all eight courts and 32 characters. Selection is gated by
// recovered gameplay, independently of whether a preview image can be loaded.
func preparedCourt(name string) (uint8, bool) {
	switch name {
	case "court-0":
		return 0, true
	case "new-york":
		return 1, true
	case "court-2":
		return 2, true
	case "court-3":
		return 3, true
	case "court-4":
		return 4, true
	case "court-5":
		return 5, true
	case "court-6":
		return 6, true
	case "court-7":
		return 7, true
	default:
		return 0, false
	}
}
func validateRoster(o Options) error {
	if o.Teams[0] >= 8 || o.Teams[1] >= 8 || o.Teams[0] == o.Teams[1] {
		return fmt.Errorf("请选择两支不同的球队")
	}
	for _, v := range o.Characters {
		if v > 3 {
			return fmt.Errorf("无效球员")
		}
	}
	if o.Characters[0] == o.Characters[1] || o.Characters[2] == o.Characters[3] {
		return fmt.Errorf("同队请选择不同球员")
	}
	return nil
}

type preparationAction struct {
	Side           *int           `json:"side,omitempty"`
	Team           *uint8         `json:"team,omitempty"`
	Opponent       *uint8         `json:"opponent,omitempty"`
	Tactic         *uint8         `json:"tactic,omitempty"`
	InventoryIndex *uint8         `json:"inventoryIndex,omitempty"`
	Slot           *campaign.Slot `json:"slot,omitempty"`
	Type           string         `json:"type"`
	Court          string         `json:"court,omitempty"`
	Actor          *int           `json:"actor,omitempty"`
	Character      *uint8         `json:"character,omitempty"`
}

func prepare(ctx gameapi.PreparationContext, current, raw json.RawMessage) (json.RawMessage, error) {
	normalized, err := configure(ctx.Capacity, current)
	if err != nil {
		return nil, err
	}
	var o Options
	if err = json.Unmarshal(normalized, &o); err != nil {
		return nil, err
	}
	actors := o.seatActors()
	if ctx.Actor < 0 || ctx.Actor >= len(ctx.Bots) || ctx.Actor >= len(actors) || ctx.Bots[ctx.Actor] || len(ctx.Bots) > ctx.Capacity {
		return nil, fmt.Errorf("只有入座玩家可以选择")
	}
	var a preparationAction
	if err = strict(raw, &a); err != nil {
		return nil, fmt.Errorf("无效准备选择")
	}
	if o.Campaign != nil {
		if a.Side != nil || a.Team != nil {
			return nil, fmt.Errorf("战役不能修改所属球队")
		}
		return prepareCampaign(ctx, o, a)
	}
	if a.Opponent != nil || a.Tactic != nil || a.InventoryIndex != nil || a.Slot != nil {
		return nil, fmt.Errorf("单场模式不接受战役准备选项")
	}
	if a.Type != "team" && (a.Side != nil || a.Team != nil) {
		return nil, fmt.Errorf("准备操作包含无效球队字段")
	}
	switch a.Type {
	case "team":
		if !ctx.Host || a.Side == nil || a.Team == nil || *a.Side < 0 || *a.Side > 1 || *a.Team >= 8 || a.Actor != nil || a.Character != nil || a.Court != "" {
			return nil, fmt.Errorf("只有房主能选择有效球队")
		}
		if o.Teams[*a.Side^1] == *a.Team {
			return nil, fmt.Errorf("两队不能选择相同球队")
		}
		o.Teams[*a.Side] = *a.Team
	case "court":
		if !ctx.Host || a.Actor != nil || a.Character != nil {
			return nil, fmt.Errorf("只有房主能选择场地")
		}
		if _, ok := preparedCourt(a.Court); !ok {
			return nil, fmt.Errorf("该场地尚未开放")
		}
		o.Court = a.Court
	case "character":
		if a.Actor == nil || a.Character == nil || a.Court != "" || *a.Actor < 0 || *a.Actor >= 4 || *a.Character > 3 {
			return nil, fmt.Errorf("无效球员选择")
		}
		actor := *a.Actor
		if !ctx.Host && actor != actors[ctx.Actor] {
			return nil, fmt.Errorf("只能选择自己的球员")
		}
		partner := actor ^ 1
		if o.Characters[actor] != *a.Character && o.Characters[partner] == *a.Character {
			for seat, owned := range actors {
				if owned == partner && seat < len(ctx.Bots) && !ctx.Bots[seat] && !ctx.Host {
					return nil, fmt.Errorf("该球员已由队友选择")
				}
			}
			o.Characters[partner] = o.Characters[actor]
		}
		o.Characters[actor] = *a.Character
	default:
		return nil, fmt.Errorf("未知准备选择")
	}
	next, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return configure(ctx.Capacity, next)
}
