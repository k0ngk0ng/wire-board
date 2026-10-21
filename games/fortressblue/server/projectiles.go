package fortressblue

import (
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
)

type weaponBehavior struct {
	Count        int
	Kind, Motion string
}

func nativeWeapon(tank int, mode string) weaponBehavior {
	p := weaponBehavior{Count: 1}
	if mode != "special" {
		if tank == 7 || tank == 13 {
			p.Count = 3
		}
		return p
	}
	switch tank {
	case 1:
		p.Kind = "fire"
	case 2:
		p.Kind = "poison_status"
	case 4:
		p.Count = 3
	case 5:
		p.Kind = "poison"
	case 6:
		p.Motion = "homing"
	case 7:
		p.Count = 9
	case 8:
		p.Count = 3
		p.Motion = "wave"
	case 9:
		p.Count = 2
		p.Kind = "mine"
	case 10:
		p.Kind = "ion"
	case 11:
		p.Kind = "immobile"
	case 12:
		p.Count = 2
		p.Motion = "wave"
	case 13:
		p.Count = 9
		p.Motion = "homing"
	case 14:
		p.Kind = "team_heal"
	}
	return p
}

type projectileMotion struct {
	VX, VY float64
	Motion string
	Wave   float64
}

// The original nearest-object routine has no team or shooter exclusion. The
// retained snapshot registry includes selected wrecks, so they remain eligible.
func homingTarget(point Point, tanks []Tank, radius float64) (Point, bool) {
	best := radius * radius
	target := Point{}
	found := false
	for _, tank := range tanks {
		if !tank.Selected {
			continue
		}
		p := tankCenter(tank)
		d := (p.X-point.X)*(p.X-point.X) + (p.Y-point.Y)*(p.Y-point.Y)
		if d < best {
			best, target, found = d, p, true
		}
	}
	return target, found
}
func weaponArcs(s *State, seat int, mode string, angle, power float64, facing int, kind string) []Arc {
	t := s.Tanks[seat]
	spec, _ := tankSpec(t.TankID)
	m, _ := fbmaps.ByID(s.MapID)
	mask := bulletMaskFor(m, s.Craters)
	profile := nativeWeapon(t.TankID, mode)
	if kind == "heal" {
		profile = weaponBehavior{Count: 1}
	}
	arcs := make([]Arc, 0, profile.Count+1)
	for i := 0; i < profile.Count; i++ {
		motion := projectileMotion{Motion: profile.Motion}
		// Counts/3x3 arrangement come from f3303. Velocity deltas below are platform
		// units; native update frequency and all original ballistic constants are not
		// recovered. No invented per-projectile damage scaling is applied.
		if profile.Count == 9 {
			motion.VX = float64(i/3-1) * 18
			motion.VY = float64(i%3-1) * 15
		} else if profile.Count > 1 {
			center := float64(i) - float64(profile.Count-1)/2
			motion.VX = center * 20
			motion.VY = center * 15
		}
		if profile.Motion == "wave" {
			motion.VX = 0
			motion.VY = 0
			motion.Wave = float64(i) - float64(profile.Count-1)/2
		}
		arc := simulateProjectile(m, mask, s.Tanks, seat, t.X, t.Y, angle, power, facing, float64(spec.Range)/50, s.Wind, MaxPath, motion)
		arc.Effect = kind
		if kind == "ion" {
			arc.Effect = "ion_marker"
		}
		arcs = append(arcs, arc)
	}
	return arcs
}
func applyTeamHealingBlast(s *State, seat int, weapon Weapon, center Point, at int64) Blast {
	r := blastRadius(weapon)
	blast := Blast{At: at, X: center.X, Y: center.Y, R: r, Seat: seat, Victim: -1, Kind: "heal"}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive || t.Team != s.Tanks[seat].Team {
			continue
		}
		p := tankCenter(*t)
		distance := math.Hypot(p.X-center.X, p.Y-center.Y)
		if distance > r {
			continue
		}
		gain := min(t.MaxHP-t.HP, max(0, int(math.Round(weapon.Damage*(1-distance/r)))))
		if gain <= 0 {
			continue
		}
		t.HP += gain
		blast.Hits = append(blast.Hits, Hit{Victim: i, Healing: gain, Kind: "heal"})
		s.emit("heal", seat, i, t.X, t.Y, gain, "heal", at)
	}
	return blast
}
