package snooker

// Failure inventory before implementation: gaps between pocket capture circles
// and cushion endpoints let rolling balls leave the table; either middle lip,
// mirrored lower pockets, grazing approaches, hard shots and side spin leak;
// closing the gaps accidentally seals centered pots or swallows solid-rail hits.
// Isolated physics is required to exercise exact mouth geometry deterministically.
import (
	"fmt"
	"math"
	"testing"
)

func TestPhysicsMiddlePocketLipsContainRollingBalls(t *testing.T) {
	for _, bottom := range []bool{false, true} {
		for _, dx := range []float64{-.09, -.06, -.056, -.052, -.049, -.04, 0, .04, .049, .052, .056, .06, .09} {
			for _, yaw := range []float64{-.12, 0, .12} {
				for _, power := range []float64{.2, .6, 1} {
					y, angle := .23, -math.Pi/2+yaw
					if bottom {
						y, angle = TableHeight-y, -angle
					}
					m, e := Simulate([]Ball{testBall(0, TableWidth/2+dx, y)}, Strike{Angle: angle, Power: power})
					if e != nil {
						t.Fatalf("bottom=%v dx=%g yaw=%g power=%g: %v", bottom, dx, yaw, power, e)
					}
					if len(m.Off) > 0 {
						saveMotion(t, m)
						t.Fatalf("rolling ball escaped pocket lip: bottom=%v dx=%g yaw=%g power=%g off=%v", bottom, dx, yaw, power, m.Off)
					}
					for _, b := range m.Balls {
						if !b.Down && (b.Pos.X < 0 || b.Pos.X > TableWidth || b.Pos.Y < 0 || b.Pos.Y > TableHeight) {
							t.Fatalf("ball stopped outside table: %+v", b)
						}
					}
				}
			}
		}
	}
}
func TestPhysicsMiddlePocketSpinAndCentre(t *testing.T) {
	for _, bottom := range []bool{false, true} {
		for _, dx := range []float64{-.052, 0, .052} {
			for _, side := range []float64{-.8, 0, .8} {
				y, angle := .23, -math.Pi/2
				if bottom {
					y, angle = TableHeight-y, -angle
				}
				m, e := Simulate([]Ball{testBall(0, TableWidth/2+dx, y)}, Strike{Angle: angle, Power: .7, Side: side})
				if e != nil || len(m.Off) > 0 {
					t.Fatalf("pocket spin bottom=%v dx=%g side=%g: off=%v err=%v", bottom, dx, side, m.Off, e)
				}
				if dx == 0 && len(m.Potted) != 1 {
					t.Fatal("centered middle-pocket shot no longer pots")
				}
			}
		}
	}
	// A square hit well away from the pocket must remain a cushion rebound.
	m, e := Simulate([]Ball{testBall(0, 1, .12)}, Strike{Angle: -math.Pi / 2, Power: .25})
	if e != nil || len(m.Potted) > 0 || len(m.Off) > 0 {
		t.Fatalf("solid cushion became a pocket: %v %+v", e, m)
	}
}
func TestPhysicsCornerPocketJawsContainRollingBalls(t *testing.T) {
	for corner := 0; corner < 4; corner++ {
		for _, offset := range []float64{.03, .05, .07, .09} {
			for _, yaw := range []float64{-.15, 0, .15} {
				p, angle := Vec{.23, offset}, math.Pi+yaw
				if corner&1 != 0 {
					p.X, angle = TableWidth-p.X, math.Pi-angle
				}
				if corner&2 != 0 {
					p.Y, angle = TableHeight-p.Y, -angle
				}
				t.Run(fmt.Sprintf("%d-%g-%g", corner, offset, yaw), func(t *testing.T) {
					m, e := Simulate([]Ball{{ID: 0, Pos: p}}, Strike{Angle: angle, Power: .7})
					if e != nil || len(m.Off) > 0 {
						t.Fatalf("corner jaw leak off=%v err=%v", m.Off, e)
					}
				})
			}
		}
	}
}
