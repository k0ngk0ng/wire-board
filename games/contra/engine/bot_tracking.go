package contra

import (
	"math"
	"sort"
)

const botTrackLimit = 64

// publicMemory records only observations previously visible to this seat. It
// neither resolves unknown HUD data from rules nor changes the caller's slices.
func (b *Bot) publicMemory(o BotObservation) BotObservation {
	o.Enemies = append([]BotEnemy(nil), o.Enemies...)
	o.PlayerCandidates = append([]BotPlayerCandidate(nil), o.PlayerCandidates...)
	// A decoded cell necessarily came with a visible grid alignment. This also
	// accepts older public observations that predate the explicit GridKnown bit.
	if !o.GridKnown {
		for _, row := range o.TerrainKnown {
			for _, known := range row {
				o.GridKnown = o.GridKnown || known
			}
		}
	}
	if o.StageKnown {
		if b.StageKnown && o.Stage != b.Stage {
			b.SceneKnown = false
		}
		b.StageKnown = true
	} else if b.StageKnown {
		o.Stage = b.Stage
	}
	if o.SceneKnown {
		b.SceneKnown = true
	} else if b.SceneKnown {
		o.Scene = b.Scene
	}
	if o.Phase == "" {
		if b.BlankTicks < 255 {
			b.BlankTicks++
		}
		if b.BlankTicks <= 12 && b.Phase == "battle" {
			o.Phase = "battle"
		}
		// Missing recognizable sprites do not disclose a scene change. Stop
		// inferring battle after the grace window, but retain the last visible
		// scene until a new stage or scene actually provides contrary evidence.
	} else {
		b.BlankTicks = 0
	}
	// These are recognizable visual families. Shared closed Base art stays a
	// shared class; its source level or private owner never enters this inference.
	for _, e := range o.Enemies {
		if o.Phase != "battle" {
			break
		}
		switch {
		case e.Family == "base-emplacement":
			o.Scene = 1
			b.SceneKnown = true
		case (e.Kind == 8 || e.Kind == 10) && e.Sprite == 0:
			o.Scene = 0x80
			b.SceneKnown = true
		case e.Kind == 16 && e.Sprite >= 93 && e.Sprite <= 98:
			o.Stage, o.Scene = 1, 0x80
			b.StageKnown, b.SceneKnown = true, true
		case e.Kind == 28 && e.Sprite >= 104 && e.Sprite <= 108:
			o.Stage, o.Scene = 3, 0x80
			b.StageKnown, b.SceneKnown = true, true
		case e.Kind == 16 && e.Sprite == 72 || e.Kind == 17 && e.Sprite == 73 || e.Kind == 19 && e.Sprite == 74 || e.Kind == 21 && (e.Sprite == 122 || e.Sprite == 123):
			o.Stage, o.Scene, o.Vertical = 2, 0, true
			b.StageKnown, b.SceneKnown = true, true
		case e.Family == "jungle-hatch":
			o.Stage, o.Scene = 0, 0
			b.StageKnown, b.SceneKnown = true, true
		}
	}
	if b.StageKnown && o.Stage == 2 {
		o.Vertical = true
	}
	if o.GridKnown && b.GridKnown && o.Tick == b.LastTick+1 {
		b.CameraX += botViewDelta(o.ViewX, b.LastViewX, 16)
		b.CameraY += botViewDelta(o.ViewY, b.LastViewY, 16)
	}
	if !o.GridKnown {
		o.ViewX, o.ViewY = b.LastViewX, b.LastViewY
	}
	if o.Phase != "battle" {
		if o.Phase == "intro" || o.Phase == "continue" || o.Phase == "game-over" || o.Phase == "finished" {
			b.PlayerSeenAt = [2]uint64{}
		}
		return o
	}
	b.associateVisiblePlayers(&o)
	return o
}

func visualTrackFamily(e BotEnemy) uint16 {
	if e.Sprite == 0 && (e.Family == "" || e.Family == "base-emplacement") && (e.Kind == 19 || e.Kind == 20 || e.Kind == 254) {
		return 300
	}
	return uint16(e.Kind)
}

// Current-frame ordinal IDs have no persistence. Match public geometry to
// recent public tracks, then assign controller-local identities deterministically.
func (b *Bot) trackVisualEntities(o *BotObservation) {
	sort.Slice(o.Enemies, func(i, j int) bool {
		a, c := o.Enemies[i], o.Enemies[j]
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		if a.Y != c.Y {
			return a.Y < c.Y
		}
		if a.X != c.X {
			return a.X < c.X
		}
		if a.Sprite != c.Sprite {
			return a.Sprite < c.Sprite
		}
		return a.Appearance < c.Appearance
	})
	used := [botTrackLimit]bool{}
	for i := range o.Enemies {
		e := &o.Enemies[i]
		best := -1
		cost := math.MaxFloat64
		for id, old := range b.PreviousEnemies {
			if old.Family != "" && e.Family != "" && old.Family != e.Family {
				continue
			}
			if used[id] || b.TrackSeenAt[id] == 0 || o.Tick < b.TrackSeenAt[id] || o.Tick-b.TrackSeenAt[id] > 12 || visualTrackFamily(old) != visualTrackFamily(*e) {
				continue
			}
			age := float64(o.Tick - b.TrackSeenAt[id])
			px := float64(old.X) + b.EnemyVX[id]*age - float64(b.CameraX-b.TrackCameraX[id])
			py := float64(old.Y) + b.EnemyVY[id]*age - float64(b.CameraY-b.TrackCameraY[id])
			distance := math.Abs(float64(e.X)-px) + math.Abs(float64(e.Y)-py)
			if distance <= 16+age*6 && distance < cost {
				best, cost = id, distance
			}
		}
		if best < 0 {
			for id := 0; id < botTrackLimit; id++ {
				if !used[id] && (b.TrackSeenAt[id] == 0 || o.Tick > b.TrackSeenAt[id]+12) {
					best = id
					b.TrackKind[id] = 0
					b.EnemyVX[id], b.EnemyVY[id] = 0, 0
					break
				}
			}
		}
		e.ID = best
		if best < 0 {
			continue
		}
		used[best] = true
		if e.Sprite == 0 && (e.Kind == 19 || e.Kind == 20) {
			b.TrackKind[best] = e.Kind
		}
		if e.Kind == 254 && b.TrackKind[best] != 0 {
			e.Kind = b.TrackKind[best]
		}
	}
}
