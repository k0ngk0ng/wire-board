package contra

// The original has three scene types, independent of horizontal/vertical
// scrolling. Invalid combinations remain errors at the public Step boundary.
func validPlayerScene(w *OutdoorWorld) bool {
	return w.Level < 8 && w.ScrollType <= 1 && (w.SceneType == 0 || (w.SceneType == 1 || w.SceneType == 0x80) && w.ScrollType == 0)
}
func sceneKind(w *OutdoorWorld) int {
	if w.SceneType == 0 {
		return 0
	}
	if w.SceneType&0x80 != 0 {
		return 1
	}
	return 2
}

func (p *Player) indoorUp(c *PlayerStepContext, sounds *[]uint8) {
	if c.ScreenCleared == 0 {
		p.Shock = 48
		*sounds = append(*sounds, 0x34)
		return
	}
	if c.Peer.GameOver == 0 && (c.Peer.State != 1 || c.Peer.Motion.Jump != 0) {
		p.Sequence = 0
		return
	}
	players := [2]*Player{}
	players[c.Actor], players[1-c.Actor] = p, c.Peer
	// Original order is second actor then first, including inactive roles.
	for actor := 1; actor >= 0; actor-- {
		player := players[actor]
		player.Sequence = 5
		if player.IndoorAdvance != 0 {
			continue
		}
		c.World.IndoorScroll = 1
		player.IndoorAdvance = 1
		player.Motion.AnimationFrame, player.Motion.AnimationTimer = 0, 0
		player.IndoorVelocityY = -176
		player.IndoorAnchorX, player.IndoorAnchorY = player.X, player.y()
		velocity := int16(128-int16(player.X)) * 2
		if c.Work != nil {
			c.Work.RecordPerspectiveX(player.X)
		}
		player.VelocityX, player.FractionVelocityX = int8(velocity>>8), uint8(velocity)
	}
}

func (p *Player) stepIndoorMotion(w *OutdoorWorld) {
	p.Sequence = 5
	// $D771 updates the pixel/fraction pair while preserving the separate
	// offscreen byte; it does not use the outdoor 24-bit position addition.
	position := uint16(p.Motion.Position) + uint16(p.IndoorVelocityY)
	p.Motion.Position = (p.Motion.Position &^ 0xffff) | int32(position)
	if w.IndoorScroll >= 2 {
		p.endIndoorMotion()
	}
}
func (p *Player) endIndoorMotion() {
	if p.Motion.Jump != 0 {
		return
	}
	p.IndoorAdvance = 0
	p.setY(p.IndoorAnchorY)
	p.X = p.IndoorAnchorX
}
