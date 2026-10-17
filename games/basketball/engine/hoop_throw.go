package engine

import "errors"

type HoopThrowPlan struct {
	Target    Position
	ArcHeight Fixed
}

func positionCarriedHoop(hoop *Hoop, owner Player) error {
	var dx, height Fixed
	switch owner.Action {
	case 0x30, 0x31, 0x32, 0x33:
		dx, height = 6*Pixel, 4*Pixel
	case 0x34, 0x35:
		dx, height = 6*Pixel, 8*Pixel
	case 0x36:
		dx, height = 0, 0
	case 0x37, 0x38:
		dx, height = 0, 30*Pixel
	case 0x39, 0x3a, 0x3b, 0x3c:
		dx, height = 2*Pixel, 30*Pixel
	case 0x57:
		dx, height = 0, 8*Pixel
	default:
		return errors.New("holder's hoop-carry pose not reconstructed")
	}
	if owner.Pose.Mirrored {
		dx = -dx
	}
	hoop.Position.X = (owner.Body.Position.X &^ 255) + dx + (hoop.Position.X & 255)
	hoop.Position.Depth = (owner.Body.Position.Depth &^ 255) - Pixel + (hoop.Position.Depth & 255)
	hoop.Position.Height = (owner.Body.Position.Height &^ 255) + height + (hoop.Position.Height & 255)
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
