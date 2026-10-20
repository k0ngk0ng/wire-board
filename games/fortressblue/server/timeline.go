package fortressblue

import (
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
	"sort"
	"time"
)

const tankTrackHalfWidth = 10.0

// A narrow hole under the centre does not remove support while either track
// still rests on solid terrain. This is a platform footprint, not an inferred
// original sprite collision mask.
func tankGroundY(m fbmaps.Map, mask []byte, x, y float64) float64 {
	ground := float64(m.Height) + 200
	for _, dx := range []float64{-10, -6, -2, 0, 2, 6, 10} {
		if x+dx < 0 || x+dx >= float64(m.Width) {
			continue
		}
		g := groundY(m, mask, x+dx, y)
		if g >= 0 {
			ground = math.Min(ground, g)
		}
	}
	return ground
}

// Check the swept body's width, not just the centre column. Small rises remain
// walkable; tall walls and overhead platforms must not intersect the body.
func walkGround(m fbmaps.Map, mask []byte, x, y, nx float64) (float64, bool) {
	if nx < tankTrackHalfWidth || nx > float64(m.Width)-tankTrackHalfWidth {
		return 0, false
	}
	ground := tankGroundY(m, mask, nx, y-8)
	if ground-y < -8 {
		return 0, false
	}
	bodyY := math.Min(y, ground)
	for dx := -tankTrackHalfWidth; dx <= tankTrackHalfWidth; dx += 4 {
		for _, dy := range []float64{12, 20, 28} {
			if solid(m, mask, nx+dx, bodyY-dy) {
				return 0, false
			}
		}
	}
	return ground, true
}

// beginFalls publishes movement without publishing future fall damage. The
// same plan is used after walking off a ledge and after a shell removes support.
func beginFalls(s *State, at, shotID, started int64) {
	m, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(m, s.Craters)
	r := &Resolution{Stage: "settling", ShotID: shotID, At: started, ImpactAt: at, SettleAt: at + 450}
	if shotID > 0 && s.LastShot != nil {
		r.LaunchAt = s.LastShot.LaunchAt
	}
	for _, tank := range s.Tanks {
		if !tank.Alive {
			continue
		}
		ground := tankGroundY(m, mask, tank.X, tank.Y-10)
		if ground < 0 {
			ground = float64(m.Height) + 200
		}
		if ground <= tank.Y {
			continue
		}
		duration := int64(math.Ceil(math.Sqrt(2*(ground-tank.Y)/Gravity) * 1000))
		duration = max(180, min(2400, duration))
		f := Fall{Seat: tank.Seat, FromX: tank.X, FromY: tank.Y, ToX: tank.X, ToY: ground, At: at, EndAt: at + duration}
		r.Falls = append(r.Falls, f)
		r.SettleAt = max(r.SettleAt, f.EndAt)
	}
	sort.SliceStable(r.Falls, func(i, j int) bool { return r.Falls[i].EndAt < r.Falls[j].EndAt })
	s.Resolution = r
	s.Deadline = 0
	if shotID > 0 && s.LastShot != nil {
		s.LastShot.SettleAt = r.SettleAt
	}
}

func (s *State) resolutionDeadline() int64 {
	r := s.Resolution
	if r == nil {
		return 0
	}
	if r.Stage == "flying" {
		if s.PendingShot != nil && !s.PendingShot.Launched && r.LaunchAt > 0 {
			return r.LaunchAt
		}
		return r.ImpactAt
	}
	next := r.SettleAt
	for _, f := range r.Falls {
		if !f.Landed {
			next = min(next, f.EndAt)
		}
	}
	return next
}

func impactShot(s *State) {
	r, shot, pending := s.Resolution, s.LastShot, s.PendingShot
	if r == nil || shot == nil || pending == nil {
		return
	}
	at := r.ImpactAt
	for _, arc := range shot.Arcs {
		if arc.OutOfBounds {
			continue
		}
		if pending.Kind == "heal" {
			shot.Blasts = append(shot.Blasts, applyHealingBlast(s, shot.Seat, pending.Weapon, arc.Impact, at))
			continue
		}
		blast := applyBlast(s, shot.Seat, pending.Weapon, arc.Impact, pending.Kind, at, arc.Hit)
		shot.Blasts = append(shot.Blasts, blast)
		shot.Craters = append(shot.Craters, Crater{X: blast.X, Y: blast.Y, R: blast.R, Seat: shot.Seat})
		s.emit("explode", shot.Seat, -1, blast.X, blast.Y, 0, pending.Kind, at)
		extinguishUnsupportedHazards(s)
		if pending.Kind == "fire" || pending.Kind == "poison" {
			addGroundHazard(s, shot.Seat, pending.Kind, arc.Impact, at)
		}
		if pending.Kind == "lightning" && arc.Hit >= 0 {
			if chain := nearestEnemy(s, shot.Seat, arc.Hit, arc.Impact, 90); chain >= 0 {
				before := s.Tanks[chain].HP
				applyDirectDamage(s, shot.Seat, chain, 55, "lightning", at)
				last := &shot.Blasts[len(shot.Blasts)-1]
				last.Hits = append(last.Hits, Hit{Victim: chain, Damage: before - s.Tanks[chain].HP, Kind: "lightning"})
			}
		}
	}
	beginFalls(s, at, shot.ID, shot.At)
	s.PendingShot = nil
}

func landTank(s *State, f *Fall) {
	if f.Landed {
		return
	}
	f.Landed = true
	t := &s.Tanks[f.Seat]
	if !t.Alive {
		return
	}
	t.X, t.Y = f.ToX, f.ToY
	m, _ := fbmaps.ByID(s.MapID)
	if f.ToY > float64(m.Height) {
		t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
		s.emit("death", f.Seat, f.Seat, t.X, t.Y, 0, "坠入深渊", f.EndAt)
		return
	}
	hurt := max(0, int((f.ToY-f.FromY-36)*.9))
	hurt = min(hurt, t.HP)
	t.HP -= hurt
	s.emit("fall", f.Seat, f.Seat, t.X, t.Y, hurt, "", f.EndAt)
	if t.HP <= 0 {
		t.Alive = false
		t.Deaths++
		s.emit("death", f.Seat, f.Seat, t.X, t.Y, 0, "坠落", f.EndAt)
	}
}

func processResolution(s *State, now time.Time) {
	if s.Resolution == nil {
		return
	}
	if s.Resolution.Stage == "flying" {
		if s.PendingShot != nil && !s.PendingShot.Launched && now.UnixMilli() >= s.Resolution.LaunchAt {
			if !s.Tanks[s.LastShot.Seat].Alive {
				cancelUnlaunchedShot(s, now.UnixMilli())
				return
			}
			s.PendingShot.Launched = true
			shot := s.LastShot
			t := s.Tanks[shot.Seat]
			s.emit("shot", shot.Seat, -1, t.X, t.Y, 0, shot.Weapon, s.Resolution.LaunchAt)
		}
		if now.UnixMilli() < s.Resolution.ImpactAt {
			return
		}
		impactShot(s)
	}
	r := s.Resolution
	if r == nil || r.Stage != "settling" {
		return
	}
	for i := range r.Falls {
		if !r.Falls[i].Landed && now.UnixMilli() >= r.Falls[i].EndAt {
			landTank(s, &r.Falls[i])
		}
	}
	if now.UnixMilli() < r.SettleAt {
		return
	}
	shotID := r.ShotID
	s.Resolution = nil
	s.PendingShot = nil
	// The world scheduler checks the result after every effect at this same
	// timestamp, including ground hazards, has been applied.
	s.needsAdvance = true
	s.resumeMovement = shotID == 0 && s.Tanks[s.Current].Alive

}

func applyHealingBlast(s *State, seat int, weapon Weapon, center Point, at int64) Blast {
	blast := Blast{X: center.X, Y: center.Y, R: blastRadius(weapon), Seat: seat, Victim: -1, Kind: "heal", Terrain: false}
	for i := range s.Tanks {
		target := &s.Tanks[i]
		c := tankCenter(*target)
		if !target.Alive || math.Hypot(c.X-center.X, c.Y-center.Y) > blast.R {
			continue
		}
		healed := min(target.MaxHP-target.HP, target.MaxHP*30/100)
		target.HP += healed
		blast.Hits = append(blast.Hits, Hit{Victim: i, Healing: healed, Kind: "heal"})
		if healed > 0 {
			s.emit("heal", seat, i, target.X, target.Y, healed, "heal", at)
		}
	}
	return blast
}

// A dead tank cannot complete its preparation animation. An airborne shell is
// already independent of the shooter and is deliberately not cancelled here.
func cancelUnlaunchedShot(s *State, at int64) bool {
	if s.Resolution == nil || s.Resolution.Stage != "flying" || s.PendingShot == nil || s.PendingShot.Launched || s.LastShot == nil {
		return false
	}
	shot := s.LastShot
	shot.CancelledAt = at
	s.Resolution = nil
	s.PendingShot = nil
	s.needsAdvance = true
	s.resumeMovement = false
	s.emit("shot_cancelled", shot.Seat, -1, s.Tanks[shot.Seat].X, s.Tanks[shot.Seat].Y, 0, "准备阶段被击毁", at)
	return true
}
