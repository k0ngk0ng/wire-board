package racing

import (
	"errors"
	"fmt"
	"math"
)

// A signed route winding, not a collection of mandatory road-sample gates.
// This is persisted privately and never sent as dynamic client telemetry.
type classicLapTracker struct {
	Version int     `json:"v"`
	Arc     float64 `json:"arc"`
	Turns   int     `json:"turns"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

func classicAlong(t Track, c *Car) (float64, float64, int) {
	info := classicTrackInfoFor(t)
	i := c.RouteIndex
	if i < 0 || i >= len(t.Points) {
		i = 0
	}
	j := (i + 1) % len(t.Points)
	a, b := t.Points[i], t.Points[j]
	dx, dy := b.X-a.X, b.Y-a.Y
	f := 0.
	if d := dx*dx + dy*dy; d > 1e-12 {
		f = clamp(((c.X-a.X)*dx+(c.Y-a.Y)*dy)/d, 0, 1)
	}
	span := info.length - info.arcs[i]
	if j > i {
		span = info.arcs[j] - info.arcs[i]
	}
	arc := info.arcs[i] + span*f
	if arc >= info.length {
		arc = 0
	}
	r, rr := t.Road[i], t.Road[j]
	nx, ny, nz := r.NX+(rr.NX-r.NX)*f, r.NY+(rr.NY-r.NY)*f, r.NZ+(rr.NZ-r.NZ)*f
	if math.Abs(ny) < .1 {
		ny = 1
	}
	height := r.Height + (rr.Height-r.Height)*f - (nx*(c.X-a.X-dx*f)+nz*(c.Y-a.Y-dy*f))/ny
	return arc, height, i
}

func initClassicLapTracker(c *Car, t Track, newRace bool) {
	arc, _, _ := classicAlong(t, c)
	turns := c.Lap
	if newRace || c.Gate < 0 && arc > classicTrackInfoFor(t).length/2 {
		turns--
	}
	c.LapTrack = &classicLapTracker{Version: 1, Arc: arc, Turns: turns, X: c.X, Y: c.Y}
}

func validateClassicLapTracker(c *Car, t Track) error {
	if c.LapTrack == nil {
		initClassicLapTracker(c, t, false)
		return nil
	}
	p := c.LapTrack
	if p.Version != 1 || !finite(p.Arc) || !finite(p.X) || !finite(p.Y) || p.Arc < 0 || p.Arc >= classicTrackInfoFor(t).length || p.Turns < c.Lap-1 || p.Turns > c.Lap {
		return errors.New("赛车沿线进度状态无效")
	}
	return nil
}

func updateClassicProgress(s *State, c *Car, before Point, at int64) {
	if c.FinishAt > 0 {
		return
	}
	t := s.Track
	info := classicTrackInfoFor(t)
	if c.LapTrack == nil {
		initClassicLapTracker(c, t, false)
	}
	p := c.LapTrack
	arc, height, index := classicAlong(t, c)
	distance := math.Hypot(c.X-p.X, c.Y-p.Y)
	delta := arc - p.Arc
	// The engine supplies the physical-step origin. Genuine integration and
	// multi-car collision corrections are trusted at any displacement; a saved
	// position changed outside that pipeline no longer matches this origin.
	continuous := math.Hypot(before.X-p.X, before.Y-p.Y) <= 1e-6
	if math.Abs(c.Height-height) >= 20 {
		// A real field may differ from the extrapolated VROAD plane. Compare
		// actual surfaces at this position; overlapping bridge layers remain
		// distinct, without any lateral-distance restriction.
		if ground := ClassicTerrainAt(t, c.X, c.Y, height); ground.Found {
			height = ground.Height
		}
	}
	valid := continuous && math.Abs(c.Height-height) < 20
	oldArc := p.Arc
	p.Arc, p.X, p.Y = arc, c.X, c.Y
	c.Gate, c.Checkpoint = index, (index+1)%len(t.Points)
	if !valid {
		p.Turns = c.Lap - 1
		return
	}
	if distance > 1e-6 {
		if delta > info.length/2 {
			p.Turns = max(c.Lap-1, p.Turns-1)
		}
		if delta < -info.length/2 {
			p.Turns++
			if p.Turns > c.Lap {
				c.Lap++
				s.emit("lap", c.Seat, at, 1)
				if c.Lap >= s.Options.Laps {
					cross := info.length - oldArc + arc
					f := clamp((info.length-oldArc)/math.Max(cross, 1e-8), 0, 1)
					c.FinishAt = at - stepMS + int64(math.Round(f*float64(stepMS)))
					c.Progress = float64(c.Lap) * info.length
					c.VX, c.VY, c.Speed, c.Throttle = 0, 0, 0, 0
					c.Brake, c.BrakeAmount, c.Horn = true, nil, false
					if c.Motion != nil {
						c.Motion.VX, c.Motion.VZ, c.Motion.Throttle, c.Motion.YawRate = 0, 0, 0, 0
						c.Motion.Brake = 1
					}
					s.emit("finish", c.Seat, at, 1)
					s.Log = append(s.Log, fmt.Sprintf("玩家 %d 完成 %d 圈", c.Seat+1, c.Lap))
					if s.FinishDeadline == 0 {
						s.FinishDeadline = c.FinishAt + 30000
						s.Phase = "finishing"
					}
					return
				}
			}
		}
	}
	c.Progress = float64(p.Turns)*info.length + arc
	if c.Lap > 0 {
		c.Progress = math.Max(float64(c.Lap)*info.length, c.Progress)
	}
}

func resetClassicPosition(s *State, c *Car) {
	// Project onto the CURRENT same-layer road segment, never a stale gate.
	road := ClassicRoadAt(s.Track, c.X, c.Y, c.Height, c.RouteIndex)
	c.RouteIndex = road.Index
	oldArc, _, _ := classicAlong(s.Track, c)
	oldTurns := c.Lap
	if c.LapTrack != nil {
		oldTurns = c.LapTrack.Turns
	}
	p := Point{road.X, road.Y}
	v := tangent(s.Track, road.Index)
	// Existing legal-lane clearance handles original walls on the VROAD line.
	lane, _ := classicDrivingLane(s.Track, road, ClassicVehicleBody(c.Vehicle), s.Props)
	l := math.Hypot(road.RX, road.RZ)
	if l > 1e-8 {
		p.X += road.RX / l * lane
		p.Y += road.RZ / l * lane
	}
	c.X, c.Y, c.Angle = p.X, p.Y, math.Atan2(v.Y, v.X)
	classicPose(c, s.Track)
	arc, _, index := classicAlong(s.Track, c)
	turns := oldTurns
	total := classicTrackInfoFor(s.Track).length
	if arc-oldArc > total/2 {
		turns = max(c.Lap-1, turns-1)
	}
	if arc-oldArc < -total/2 {
		turns = c.Lap
	}
	c.LapTrack = &classicLapTracker{Version: 1, Arc: arc, Turns: turns, X: c.X, Y: c.Y}
	c.Gate, c.Checkpoint = index, (index+1)%len(s.Track.Points)
}
