package engine

import "errors"

type HoopThrowPlan struct {
	Target    Position
	ArcHeight Fixed
}

func positionCarriedHoop(hoop *Hoop, owner Player) error {
	// EC7B selects the same ECF5/ED65 offsets used by other held objects.
	// In particular a carrier can fall or become a rider before the later hoop
	// dispatch. Keep following that current action instead of rejecting it.
	if int(owner.Action) >= len(originalHeldBallActionOffsets) {
		return errors.New("holder's action outside original pose table")
	}
	offset := originalHeldBallOffsets[originalHeldBallActionOffsets[owner.Action]]
	dx, height := Fixed(offset.X)*Pixel, Fixed(offset.Height)*Pixel
	if owner.Pose.Mirrored {
		dx = -dx
	}
	hoop.Position.X = signed24((owner.Body.Position.X &^ 255) + dx + (hoop.Position.X & 255))
	hoop.Position.Depth = Fixed(uint8((owner.Body.Position.Depth>>8)-Fixed(offset.SubtractDepth)))*Pixel + (hoop.Position.Depth & 255)
	hoop.Position.Height = signed24((owner.Body.Position.Height &^ 255) + height + (hoop.Position.Height & 255))
	return nil
}

// Original computer hoop throws use stack-placement heights determined by
// existing mounts, rather than a ball's scoring target. Human special throws
// have their separate high target; arc selection comes from world state.
func chooseHoopThrow(owner Player, computer, special bool, mounts [6]uint8, highArc bool) (HoopThrowPlan, bool) {
	x := Fixed(487) * Pixel
	if owner.FacingLeft {
		x = 41 * Pixel
	}
	height := Fixed(88) * Pixel
	if owner.Body.Position.Height >= 56*Pixel {
		height = 184 * Pixel
	}
	if owner.Body.Position.Height >= 152*Pixel {
		height = 280 * Pixel
	}
	clear := false
	if computer {
		height = 160 * Pixel
		if mounts[0] == 3 {
			height = 104 * Pixel
			if mounts[2] == 3 {
				height = 176 * Pixel
			}
		}
	} else if special {
		height = 272 * Pixel
		clear = true
	}
	arc := Fixed(24) * Pixel
	if highArc {
		arc = 48 * Pixel
	}
	return HoopThrowPlan{Target: Position{X: x, Depth: 47 * Pixel, Height: height}, ArcHeight: arc}, clear
}
