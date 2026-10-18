package engine

import "errors"

type playerTerrainContext struct{ HasBall, Computer bool }

func clampPlayerTravel(body *Body, court FlatCourt) {
	if body.Position.X < court.MinX {
		body.Position.X = court.LeftReset + (body.Position.X & 255)
		body.Velocity.X, body.Velocity.Depth = 0, 0
	}
	if body.Position.X >= court.MaxX {
		body.Position.X = court.RightReset + (body.Position.X & 255)
		body.Velocity.X, body.Velocity.Depth = 0, 0
	}
	if body.Position.Depth < court.MinDepth {
		body.Position.Depth = court.MinDepth + (body.Position.Depth & 255)
	}
	if body.Position.Depth >= court.MaxDepth {
		body.Position.Depth = court.MaxDepth - Pixel + (body.Position.Depth & 255)
	}
}

// advancePlayerOnCourt adds the original New York back-court passage to normal
// motion. Passenger detachment and other object classes remain world concerns.
func advancePlayerOnCourt(p *Player, ctx playerTerrainContext, court FlatCourt) (BallCommand, error) {
	if court.originalMotion != nil {
		return court.originalMotion.player(p, &ctx)
	}
	var ball BallCommand
	// The passage branch integrates motion but bypasses the terrain lookup.
	if p.NewYorkPassage == 0 {
		court.recordMotionSurface(p.Body.Position)
	}
	previous := p.Body.Position
	next := IntegrateMotion(p.Body)
	clampPlayerTravel(&next, court)
	heldCommand := func() {
		ball = BallCommand{Change: true, Action: BallHeld, ChangeRequest: true, Request: BallRequest{Action: BallHeld, Pending: true}}
	}
	if p.NewYorkPassage != 0 {
		if !court.NewYorkPassages {
			return ball, errors.New("passage state requires the New York court")
		}
		p.Body = next
		p.Body.Velocity = Velocity{}
		p.Body.Gravity = 0
		p.Body.Position.Depth = previous.Depth
		p.Action, p.Entered = ActionIdle, false
		if ctx.HasBall {
			p.Action = ActionDribbleIdle
		}
		if !ctx.Computer {
			p.Request = ActionRequest{Action: p.Action, Pending: true}
		}
		delta := 10 * Pixel
		exitPage := uint16(1)
		if p.NewYorkPassage == 2 {
			delta = -delta
			exitPage = 0
		}
		p.Body.Position.X = signed24((previous.X &^ 255) + delta + (next.Position.X & 255))
		x := uint16(p.Body.Position.X >> 8)
		if x>>8 == exitPage && uint8(x)&0xf0 == 0x80 {
			p.NewYorkPassage = 0
			p.Body.Position.Height = 0
			p.Body.Velocity.X = 6 * Pixel
			if exitPage == 1 {
				p.Body.Velocity.X = -6 * Pixel
			}
			p.Body.Velocity.Height = 8 * Pixel
			p.Body.Gravity = Pixel
			p.Pose = Pose{Frame: PoseAirborne, Mirrored: exitPage == 1}
			p.Action, p.Entered = ActionJump, true
			if ctx.HasBall {
				p.Action = ActionCarrierJump
				heldCommand()
			}
		}
		return ball, nil
	}
	// The entrance is reached by a side collision behind the raised area.
	// A fall from above instead lands on the platform and is not an entrance.
	raised := court.NewYorkPassages && previous.Depth >= 73*Pixel && next.Position.Depth >= court.RaisedDepth && (next.Position.X < court.RaisedLeftEnd || next.Position.X >= court.RaisedRightStart)
	if raised && next.Position.Height>>8 < court.RaisedFloor>>8 && !(next.Velocity.Height < 0 && previous.Height>>8 >= court.RaisedFloor>>8) {
		p.Body = next
		// The original side resolver applies the previous floor's travel
		// attribute before entering the passage and clearing the airborne flag.
		if !p.Airborne && courtSurface(court, previous.X, previous.Depth) > court.Floor {
			p.Body.Position.Depth = surfaceDepth(next.Position.Depth, previous.Depth)
		}
		if p.Body.Velocity.Height < 0 {
			p.Body.Velocity.X = 0
		}
		if previous.X>>8 > next.Position.X>>8 {
			p.Body.Position.X = (previous.X &^ 2047) + (previous.X & 255)
		} else if previous.X>>8 < next.Position.X>>8 {
			p.Body.Position.X = (previous.X &^ 2047) + 7*Pixel + (previous.X & 255)
		}
		x := uint16(p.Body.Position.X >> 8)
		if x>>8 == 0 && uint8(x) < 137 {
			p.NewYorkPassage = 1
		} else if x>>8 != 0 && uint8(x) >= 135 {
			p.NewYorkPassage = 2
		} else {
			return ball, errors.New("raised side contact outside restored passage")
		}
		p.Action, p.Entered = ActionIdle, false
		if ctx.HasBall {
			p.Action = ActionCatchBall
			heldCommand()
		}
		p.Request = ActionRequest{Action: p.Action, Pending: true}
		p.Body.Velocity.Height, p.Body.Gravity = 0, 0
		p.Airborne = false
		floor := court.Floor
		if next.Position.Depth >= court.RaisedDepth && (previous.X < court.RaisedLeftEnd || previous.X >= court.RaisedRightStart) {
			floor = court.RaisedFloor
		}
		p.Body.Position.Height = -256*Pixel + floor + (next.Position.Height & 255)
		return ball, nil
	}
	return ball, advancePlayerSurface(p, next, previous, court)
}

// courtSurface decodes the original New York terrain height. The two raised
// depth rows also have travel flags, handled separately by surfaceDepth.
func courtSurface(court FlatCourt, x, depth Fixed) Fixed {
	if court.RaisedDepth > 0 && depth >= court.RaisedDepth && (x < court.RaisedLeftEnd || x >= court.RaisedRightStart) {
		return court.RaisedFloor
	}
	return court.Floor
}
func surfaceDepth(depth, sourceDepth Fixed) Fixed {
	if sourceDepth >= 80*Pixel {
		return ((depth>>8)&240)*Pixel - 8*Pixel + (depth & 255)
	}
	return (depth &^ 2047) + (depth & 255)
}
func advancePlayerSurface(p *Player, next Body, previous Position, court FlatCourt) error {
	p.Body = next
	b := &p.Body
	floor := courtSurface(court, next.Position.X, next.Position.Depth)
	oldFloor := courtSurface(court, previous.X, previous.Depth)
	ahead := func() {
		if !p.Airborne && oldFloor > court.Floor {
			b.Position.Depth = surfaceDepth(b.Position.Depth, previous.Depth)
		}
	}
	if b.Position.Height>>8 == floor>>8 {
		if b.Velocity.Height < Pixel {
			b.Position.Height = floor
			if b.Velocity.Height == 0 && b.Gravity == 0 {
				p.Airborne = false
			}
		}
		if !p.Airborne && floor > court.Floor {
			b.Position.Depth = surfaceDepth(b.Position.Depth, next.Position.Depth)
		}
		return nil
	}
	if b.Position.Height > floor {
		p.Airborne = true
		if p.Action == Action(14) {
			p.Action, p.Entered = ActionFalling, false
		}
		return nil
	}
	if b.Velocity.Height < 0 && previous.Height >= 0 {
		if previous.Height>>8 >= floor>>8 {
			b.Position.Height = floor
			b.Velocity.Height = 0
			p.Airborne = false
			// The terrain side routine restores the former floor point when no
			// side/depth obstacle was selected, even on a normal platform landing.
			b.Position.X, b.Position.Depth = previous.X, previous.Depth
			b.Gravity = 0
			ahead()
			return nil
		}
		if b.Position.Height>>8 < oldFloor>>8 {
			b.Position.X, b.Position.Depth = previous.X, previous.Depth
			b.Position.Height = oldFloor
			b.Velocity.Height, b.Gravity = 0, 0
			p.Airborne = false
			ahead()
			return nil
		}
	}
	p.Airborne = b.Velocity.Height != 0 || b.Gravity != 0
	blocked := false
	xFloor := courtSurface(court, next.Position.X, previous.Depth)
	if xFloor>>8 > Fixed(uint8(b.Position.Height>>8)) {
		if b.Velocity.Height < 0 {
			b.Velocity.X = 0
		}
		if previous.X>>8 < next.Position.X>>8 {
			b.Position.X = (previous.X &^ 2047) + 7*Pixel + (previous.X & 255)
		} else if previous.X>>8 > next.Position.X>>8 {
			b.Position.X = (previous.X &^ 2047) + (previous.X & 255)
		}
		ahead()
		blocked = true
	} else if b.Velocity.Height == 0 && b.Gravity == 0 {
		b.Position.Height = boardReplaceLow(b.Position.Height, uint8(xFloor>>8))
	}
	yFloor := courtSurface(court, previous.X, next.Position.Depth)
	if yFloor>>8 > Fixed(uint8(b.Position.Height>>8)) {
		if b.Velocity.Height < 0 {
			b.Velocity.Depth = 0
		}
		b.Position.Depth = (previous.Depth &^ 2047) + 7*Pixel + (previous.Depth & 255)
		ahead()
		blocked = true
	} else if b.Velocity.Height == 0 && b.Gravity == 0 {
		b.Position.Height = boardReplaceLow(b.Position.Height, uint8(yFloor>>8))
	}
	if !blocked {
		b.Position.X, b.Position.Depth = previous.X, previous.Depth
	}
	return nil
}
