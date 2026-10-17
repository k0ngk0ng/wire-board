package basketball

import (
	"encoding/json"
	"fmt"
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
	default:
		return 0, false
	}
}
func validateRoster(o Options) error {
	if o.Teams != ([2]uint8{0, 1}) {
		return fmt.Errorf("该球队阵容尚未开放")
	}
	for _, v := range o.Characters {
		if v > 1 {
			return fmt.Errorf("该球员尚未开放")
		}
	}
	if o.Characters[0] == o.Characters[1] || o.Characters[2] == o.Characters[3] {
		return fmt.Errorf("同队请选择不同球员")
	}
	return nil
}

type preparationAction struct {
	Type      string `json:"type"`
	Court     string `json:"court,omitempty"`
	Actor     *int   `json:"actor,omitempty"`
	Character *uint8 `json:"character,omitempty"`
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
	switch a.Type {
	case "court":
		if !ctx.Host || a.Actor != nil || a.Character != nil {
			return nil, fmt.Errorf("只有房主能选择场地")
		}
		if _, ok := preparedCourt(a.Court); !ok {
			return nil, fmt.Errorf("该场地尚未开放")
		}
		o.Court = a.Court
	case "character":
		if a.Actor == nil || a.Character == nil || a.Court != "" || *a.Actor < 0 || *a.Actor >= 4 || *a.Character > 1 {
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
