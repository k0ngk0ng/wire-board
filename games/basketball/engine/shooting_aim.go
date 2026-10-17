package engine

// DunkSearchContext is the current ball and the original six mount flags /
// owners. It deliberately does not pick a nearest basket or change ownership.
type DunkSearchContext struct {
	BallPosition Position
	MountFlags   [6]uint8
	HoopOwners   [6]uint8
}

// selectOriginalDunkTarget restores $99CF. Even a failed airborne search writes
// the current half-court selector before checking the spatial windows.
func selectOriginalDunkTarget(result *CarrierStep, ctx DunkSearchContext) {
	p := &result.Carrier.Player
	if !p.Airborne {
		return
	}
	side := uint8(0)
	if uint16(p.Body.Position.X>>8)>>8 != 0 {
		side = 1
	}
	result.ChangeDunkTarget, result.DunkTargetObject = true, side
	x, depth := uint8(p.Body.Position.X>>8), uint8(p.Body.Position.Depth>>8)
	if x < [2]uint8{56, 185}[side] || x >= [2]uint8{88, 212}[side] || depth < 42 || depth >= 53 {
		return
	}
	low := uint8(ctx.BallPosition.Height >> 8)
	tier := uint8(0)
	baseOwner := uint8(7)
	lowerMin, lowerMax, raisedMin, raisedMax := uint8(80), uint8(92), uint8(112), uint8(124)
	if uint8(ctx.BallPosition.Height>>16) != 0 {
		tier, baseOwner = 2, 11
		lowerMin, lowerMax, raisedMin, raisedMax = 16, 29, 48, 61
	} else if low >= 128 {
		tier, baseOwner = 1, 9
		lowerMin, lowerMax, raisedMin, raisedMax = 175, 189, 207, 221
	}
	owner := baseOwner + side
	flag := uint8(1)
	if low >= raisedMin && low < raisedMax {
		owner |= 128
		flag = 2
	} else if low < lowerMin || low >= lowerMax {
		return
	}
	if ctx.MountFlags[tier*2+side]&flag == 0 {
		return
	}
	for i, candidate := range ctx.HoopOwners {
		if candidate != owner {
			continue
		}
		result.DunkTargetObject = uint8(i + 7)
		action := Action(0x58)
		if owner&1 == 0 && p.Heading == HeadingLeft || owner&1 != 0 && p.Heading == HeadingRight {
			action = 0x59
		} else if p.Heading == HeadingUp {
			action = 0x5b
		}
		p.Action, p.Entered = action, false
		return
	}
}
