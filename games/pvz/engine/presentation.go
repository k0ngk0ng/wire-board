package engine

import "errors"

// Presentation contains only values consumed by a displayed frame. Local
// integer stepping exports this, never a complete save or a fresh checksum.
// Audio events remain exclusively on the authenticated authority timeline.
type Presentation struct {
	Epoch           string       `json:"epoch"`
	Tick            int64        `json:"tick"`
	Phase           string       `json:"phase"`
	PhaseEndTick    int64        `json:"phaseEndTick"`
	Players         []Player     `json:"players"`
	Plants          []Plant      `json:"plants"`
	Zombies         []Zombie     `json:"zombies"`
	Projectiles     []Projectile `json:"projectiles"`
	Mowers          []Mower      `json:"mowers"`
	Pickups         []Pickup     `json:"pickups"`
	Graves          []Grave      `json:"graves"`
	Craters         []Crater     `json:"craters"`
	Terrain         []Terrain    `json:"terrain"`
	Ladders         []Ladder     `json:"ladders"`
	FogClearedUntil int64        `json:"fogClearedUntil"`
}

func Present(s State) Presentation {
	return Presentation{Epoch: s.Epoch, Tick: s.Tick, Phase: s.Phase, PhaseEndTick: s.PhaseEndTick, Players: s.Players, Plants: s.Plants, Zombies: s.Zombies, Projectiles: s.Projectiles, Mowers: s.Mowers, Pickups: s.Pickups, Graves: s.Graves, Craters: s.Craters, Terrain: s.Terrain, Ladders: s.Ladders, FogClearedUntil: s.FogClearedUntil}
}

// StepProjection advances only already disclosed movement. A redacted state
// cannot decide collisions, damage, deaths, income, fog, terrain expiration,
// attacks, transformations or phase transitions. Its entities live until the
// next authenticated projection or the player's own explicit operation.
func StepProjection(s *State, ticks int) error {
	if !s.Projection || ticks < 0 || ticks > 5 {
		return errors.New("无效投影推进")
	}
	if s.Phase != "battle" && s.Phase != "prepare" {
		return nil
	}
	ticks = min(ticks, max(0, int(s.PhaseEndTick-s.Tick-1)))
	for i := range s.Zombies {
		s.Zombies[i].X += s.Zombies[i].MotionX * ticks
	}
	for i := range s.Projectiles {
		q := &s.Projectiles[i]
		q.X += q.MotionX * ticks
		q.Y += q.MotionY * ticks
		q.Height = max(0, q.Height+q.MotionHeight*ticks)
	}
	for i := range s.Mowers {
		s.Mowers[i].X += s.Mowers[i].MotionX * ticks
	}
	s.Tick += int64(ticks)
	return nil
}

type motionPoint struct {
	x, y, height int
	row          int
	state        string
}
type motionRecord struct {
	zombies     map[int]motionPoint
	projectiles map[int]motionPoint
	mowers      map[int]motionPoint
	plants      map[int]string
}

// Record actual displacement after the whole rule tick; velocity inferred
// from a target would disclose hidden targeting and predict unobserved hits.
func captureMotion(s *State) motionRecord {
	m := motionRecord{zombies: make(map[int]motionPoint, len(s.Zombies)), projectiles: make(map[int]motionPoint, len(s.Projectiles)), mowers: make(map[int]motionPoint, len(s.Mowers)), plants: make(map[int]string, len(s.Plants))}
	for _, v := range s.Zombies {
		m.zombies[v.ID] = motionPoint{x: v.X, row: v.Row}
	}
	for _, v := range s.Projectiles {
		m.projectiles[v.ID] = motionPoint{x: v.X, y: v.Y, height: v.Height}
	}
	for _, v := range s.Mowers {
		m.mowers[v.Row] = motionPoint{x: v.X, state: v.State}
	}
	for _, v := range s.Plants {
		m.plants[v.ID] = v.State
	}
	return m
}
func (m motionRecord) finish(s *State) {
	for i := range s.Zombies {
		v := &s.Zombies[i]
		v.MotionX = 0
		if p, ok := m.zombies[v.ID]; ok && p.row == v.Row && abs(v.X-p.x) <= 1000 {
			v.MotionX = v.X - p.x
		}
	}
	for i := range s.Projectiles {
		v := &s.Projectiles[i]
		v.MotionX, v.MotionY, v.MotionHeight = 0, 0, 0
		if p, ok := m.projectiles[v.ID]; ok {
			v.MotionX, v.MotionY, v.MotionHeight = v.X-p.x, v.Y-p.y, v.Height-p.height
		}
	}
	for i := range s.Mowers {
		v := &s.Mowers[i]
		v.MotionX = 0
		if p, ok := m.mowers[v.Row]; ok && p.state == "moving" && v.State == "moving" {
			v.MotionX = v.X - p.x
		}
	}
	for i := range s.Plants {
		v := &s.Plants[i]
		if previous, ok := m.plants[v.ID]; v.State != "" && (!ok || previous != v.State) {
			if v.Flags == nil {
				v.Flags = map[string]int64{}
			}
			v.Flags["phaseAt"] = s.Tick
		}
	}
}
