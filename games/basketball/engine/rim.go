package engine

import "errors"

type HoopAction uint8

const (
	HoopInactive HoopAction = iota
	HoopReady
	HoopScored
	HoopJostled
	HoopFalling
	HoopCarried
	HoopThrown
	HoopLobbed
	HoopKnocked
)

type Hoop struct {
	Position    Position       `json:"position"`
	Action      HoopAction     `json:"action"`
	Entered     bool           `json:"entered"`
	CanReceive  bool           `json:"canReceive"`
	ScoreLock   uint8          `json:"scoreLock"`
	Velocity    Velocity       `json:"velocity"`
	Gravity     Fixed          `json:"gravity"`
	Pose        Pose           `json:"pose"`
	FacingLeft  bool           `json:"facingLeft"`
	Animation   AnimationState `json:"animation"`
	Airborne    bool           `json:"airborne"`
	Durability  int8           `json:"durability"`
	Carried     bool           `json:"carried"`
	CarrierSeat uint8          `json:"carrierSeat"`
	Request     HoopRequest    `json:"request"`
}

type RimState struct {
	Ball            Ball        `json:"ball"`
	Hoops           [6]Hoop     `json:"hoops"`
	Random          RandomState `json:"random"`
	Scores          [2]uint16   `json:"scores"`
	ScoreSoundDelay uint8       `json:"scoreSoundDelay"`
}

type RimContext struct {
	PreviousBallAction BallAction
	SelectedHoop       int
	ShotOriginX        Fixed
	ShooterAction      Action
	ShotSkill          uint8
	ScoringBonus       bool
}

type RimOutcome uint8

const (
	RimNoContact RimOutcome = iota
	RimUnderside
	RimGoal
	RimMiss
	RimStuck
)

type RimResolution struct {
	State       RimState
	Outcome     RimOutcome
	Hoop        int
	ScoringSide uint8
	Points      uint8
}

func rimContains(ball Position, hoop Hoop) bool {
	if hoop.ScoreLock != 0 || !hoop.CanReceive {
		return false
	}
	switch hoop.Action {
	case HoopReady, HoopJostled, HoopCarried:
	default:
		return false
	}
	x, z, h := ball.X>>8, ball.Depth>>8, ball.Height>>8
	hx, hz, hh := hoop.Position.X>>8, hoop.Position.Depth>>8, hoop.Position.Height>>8
	// The original checks the high coordinate byte before low-byte bounds.
	if x>>8 != hx>>8 || h>>8 != hh>>8 {
		return false
	}
	return x > hx-8 && x <= hx+8 && z > hz+1 && z <= hz+8 && h > hh && h <= hh+8
}

func rimDistanceChance(origin Fixed, selected int) uint8 {
	target := int32(463)
	if selected >= 0 && selected%2 == 0 {
		target = 65
	}
	delta := uint16(target - int32(origin>>8))
	if delta&0x8000 != 0 {
		// Original distance approximation complements the high byte without
		// propagating the low-byte negation carry. Keep its boundary behavior.
		delta = uint16(^uint8(delta>>8))<<8 | uint16(uint8(-uint8(delta)))
	}
	chance := [8]uint8{127, 120, 112, 104, 96, 88, 80, 72}
	return chance[(delta>>6)&7]
}

// ResolveRimContact restores the original rim search and its outcomes. The
// separate backboard phase runs before it; hoop animation, score-lock reset
// and court-specific score events are owned by the rest of the world update.
func ResolveRimContact(original RimState, ctx RimContext) (RimResolution, error) {
	result := RimResolution{State: original, Hoop: -1}
	if ctx.SelectedHoop < -1 || ctx.SelectedHoop >= len(original.Hoops) {
		return result, errors.New("invalid selected hoop")
	}
	s := &result.State
	for i := range s.Hoops {
		hoop := &s.Hoops[i]
		if !rimContains(s.Ball.Body.Position, *hoop) {
			continue
		}
		result.Hoop = i
		b := &s.Ball
		if b.Body.Velocity.Height >= 0 {
			b.Deflection = DeflectHeight
			b.Request = BallRequest{Action: BallDeflected, Pending: true}
			result.Outcome = RimUnderside
			return result, nil
		}
		goal := b.Action == 10 || b.Action >= 12 || ctx.PreviousBallAction >= 12 || (ctx.ShooterAction >= 0x58 && ctx.ShooterAction <= 0x5c)
		if !goal {
			var value byte
			s.Random, value = NextRandom(s.Random)
			if value&7 < ctx.ShotSkill {
				s.Random, value = NextRandom(s.Random)
				goal = value&127 < rimDistanceChance(ctx.ShotOriginX, ctx.SelectedHoop)
			}
		}
		if !goal {
			var value byte
			s.Random, value = NextRandom(s.Random)
			if value&7 == 0 && b.Action != 10 && i == ctx.SelectedHoop {
				b.Action, b.Entered = BallStuck, false
				result.Outcome = RimStuck
				return result, nil
			}
			// Rim rejection negates only the original integer velocity byte.
			// Its fractional byte is retained, unlike ordinary axis reflection.
			vz := uint16(b.Body.Velocity.Height)
			b.Body.Velocity.Height = Fixed(int16(uint16(uint8(-uint8(vz>>8)))<<8 | vz&255))
			if b.Body.Velocity.X>>8 == 0 {
				integer := Fixed(2) * Pixel
				if b.Body.Position.X>>16 != 0 {
					integer = -2 * Pixel
				}
				b.Body.Velocity.X = integer + (b.Body.Velocity.X & 255)
			}
			result.Outcome = RimMiss
			return result, nil
		}
		hoop.Action, hoop.Entered = HoopScored, false
		b.Body.Position.X = (hoop.Position.X &^ 255) + (b.Body.Position.X & 255)
		b.Body.Position.Depth = (hoop.Position.Depth &^ 255) + 2*Pixel + (b.Body.Position.Depth & 255)
		b.Body.Velocity.X >>= 2
		b.Body.Velocity.Depth >>= 2
		b.Body.Velocity.Height = 0
		hoop.ScoreLock = 16
		s.ScoreSoundDelay = 8
		points := [4]uint8{4, 3, 3, 3}
		result.Points = points[((hoop.Position.Height>>8)&255)>>6]
		if ctx.ScoringBonus {
			result.Points += 2
		}
		result.ScoringSide = uint8((hoop.Position.X>>16)+1) & 1
		s.Scores[result.ScoringSide] += uint16(result.Points)
		result.Outcome = RimGoal
		return result, nil
	}
	return result, nil
}
