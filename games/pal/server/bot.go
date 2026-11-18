package pal

func battleBotAction(v View, actor int, memory *battleBotMemory) (Action, bool) {
	b := v.Battle
	if v.Phase != "battle" || b == nil || b.Phase != 0 || b.State == 0 || b.Current < 0 || b.Current >= len(v.Party) || v.Party[b.Current].Controller != actor || v.Seq != v.AppliedSeq {
		return Action{}, false
	}
	action := Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4}
	if direction, keys, ok := revivalBotKeys(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	if direction, keys, ok := battleItemBotKeys(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	if direction, keys, ok := battleAttackScopeKeys(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	if direction, keys, ok := soloCooperationBotKeys(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	if direction, keys, ok := recoveryBotKeys(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	if battleRoundAutoReady(v, actor, memory) || battleLastRoleAutoReady(v, actor, memory) {
		action.Keys = 32768
		return action, true
	}
	if direction, keys, ok := offensiveBotKeys(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	switch b.State {
	case 1:
		if b.Menu != 0 {
			action.Keys = 1
		} else if p := v.Party[b.Current]; p.MaxMP > 0 && p.MP*2 <= p.MaxMP {
			if b.Selected != 0 {
				action.Direction, action.Keys = 2, 16
			} else {
				action.Keys = 2
			}
		} else {
			if b.Selected != 0 {
				action.Direction, action.Keys = 2, 16
			} else {
				action.Keys = 2
			}
		}
	case 2, 4:
		if b.Action == 2 {
			action.Direction, action.Keys, _ = battleEnemyTargetKeys(v, memory)
		} else {
			action.Keys = 1
		}
	case 3, 5:
		action.Keys = 1
	default:
		return Action{}, false
	}
	return action, true
}
