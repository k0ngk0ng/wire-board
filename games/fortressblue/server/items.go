package fortressblue

import (
	"errors"
	"fmt"

	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
)

// startingItems grants each seat two distinct original items. The choice is
// deterministic from the match epoch so restores and bots stay reproducible.
func startingItems(epoch string, seat int) []int {
	ids := fbdata.ItemIDs()
	if len(ids) == 0 {
		return nil
	}
	out := make([]int, 0, StartingItems)
	for n := 0; n < StartingItems && n < len(ids); n++ {
		index := int(hashString(epoch, "item", fmt.Sprint(seat), fmt.Sprint(n)) % uint64(len(ids)))
		for containsInt(out, ids[index]) {
			index = (index + 1) % len(ids)
		}
		out = append(out, ids[index])
	}
	return out
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func useItem(s *State, seat, slot int, at int64) error {
	if !s.canAct(seat) {
		return errors.New("还没轮到你使用道具")
	}
	t := &s.Tanks[seat]
	if slot < 0 || slot >= len(t.Items) {
		return errors.New("道具不存在")
	}
	item, ok := fbdata.ItemByID(t.Items[slot])
	if !ok {
		return errors.New("道具有效性错误")
	}
	t.Items = append(t.Items[:slot], t.Items[slot+1:]...)
	t.ItemTurn = s.Turn
	switch item.Effect {
	case "heal_self":
		t.HP = min(t.MaxHP, t.HP+item.Amount)
	case "heal_team":
		for i := range s.Tanks {
			ally := &s.Tanks[i]
			if ally.Team == t.Team && ally.Alive {
				ally.HP = min(ally.MaxHP, ally.HP+item.Amount)
			}
		}
	case "fuel":
		t.Fuel = min(200.0, t.Fuel+float64(item.Amount))
	case "shield_self":
		addStatus(t, Status{Kind: "shield", Turns: max(1, item.Turns), Source: seat})
	case "power_self":
		addStatus(t, Status{Kind: "power", Turns: max(1, item.Turns), Source: seat})
	case "no_wind":
		addStatus(t, Status{Kind: "nowind", Turns: max(1, item.Turns), Source: seat})
	case "reverse_wind":
		s.Wind = -s.Wind
	default:
		return errors.New("道具效果尚未实现")
	}
	s.emit("item", seat, -1, t.X, t.Y, item.ID, item.Name, at)
	s.note(fmt.Sprintf("%d 号位使用道具：%s", seat+1, item.Name))
	return nil
}

func hasStatus(t *Tank, kind string) bool {
	for _, status := range t.Statuses {
		if status.Kind == kind && status.Turns > 0 {
			return true
		}
	}
	return false
}

func consumeStatus(t *Tank, kind string) bool {
	for i, status := range t.Statuses {
		if status.Kind == kind && status.Turns > 0 {
			t.Statuses = append(t.Statuses[:i], t.Statuses[i+1:]...)
			return true
		}
	}
	return false
}

// botItem picks a useful item once per turn. Returning false lets the same
// BotAction call continue into its normal move/fire decision.
func botItem(s *State, seat int) (int, bool) {
	tank := &s.Tanks[seat]
	if len(tank.Items) == 0 || tank.ItemTurn == s.Turn {
		return 0, false
	}
	hpRatio := float64(tank.HP) / float64(max(1, tank.MaxHP))
	for slot, id := range tank.Items {
		item, ok := fbdata.ItemByID(id)
		if !ok {
			continue
		}
		switch item.Effect {
		case "heal_self", "heal_team":
			if hpRatio < 0.62 {
				return slot, true
			}
		case "shield_self":
			if hpRatio < 0.85 && !hasStatus(tank, "shield") {
				return slot, true
			}
		case "power_self":
			if !hasStatus(tank, "power") && hashString(s.Epoch, "power", fmt.Sprint(seat), fmt.Sprint(s.Turn))%3 == 0 {
				return slot, true
			}
		case "no_wind":
			if s.Wind != 0 && !hasStatus(tank, "nowind") {
				return slot, true
			}
		case "reverse_wind":
			if absInt(s.Wind) >= 4 {
				return slot, true
			}
		case "fuel":
			if tank.Fuel < 12 {
				return slot, true
			}
		}
	}
	return 0, false
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
