package contra

import "math"

// Bot is deterministic controller memory, separate from Session. The platform
// calls Decide only for an explicitly added computer or voluntarily assisted
// seat. Stopping those calls and supplying the human pad cancels assistance;
// this object has no mechanism to change a match, life, position or result.
type Bot struct {
	LastTick                   uint64
	LastPad                    Buttons
	LastX, LastY               int
	LastViewX, LastViewY       uint8
	Phase                      string
	Stage                      uint8
	Scene                      uint8
	StageKnown, SceneKnown     bool
	BlankTicks                 uint8
	GridKnown                  bool
	CameraX, CameraY           int
	PhaseTicks, StillTicks     uint64
	ViewStillTicks             uint64
	JumpCooldown               uint8
	JumpTargetX                int
	JumpTargetActive           bool
	JumpPlatform               int
	PreviousEnemies            [botTrackLimit]BotEnemy
	EnemySeen                  [botTrackLimit]bool
	EnemyVX, EnemyVY           [botTrackLimit]float64
	TrackSeenAt                [botTrackLimit]uint64
	TrackCameraX, TrackCameraY [botTrackLimit]int
	TrackKind                  [botTrackLimit]uint8
	PlayerMemory               [2]BotPlayer
	PlayerSeenAt               [2]uint64
	BaseTarget                 BotEnemy
	BaseTargetSeen             uint64
	BorrowPending              [2]bool
	FacingLeft                 bool
}

func (b *Bot) Decide(o BotObservation) Buttons {
	if o.Tick != 0 && o.Tick == b.LastTick {
		return b.LastPad
	}
	if o.Actor < 0 || o.Actor > 1 || o.Seats < 1 || o.Seats > 2 || o.Actor >= o.Seats {
		return 0
	}
	keepCrouch := b.holdOccludedNozzleCrouch(o)
	o = b.publicMemory(o)
	if keepCrouch {
		if _, escape := b.lowNozzleEscape(o); escape {
			keepCrouch = false
		}
	}
	b.observeBorrowSignals(o)
	p := o.Players[o.Actor]
	if b.Phase != o.Phase || b.Stage != o.Stage || b.Scene != o.Scene || o.Tick != b.LastTick+1 {
		b.PhaseTicks, b.StillTicks = 0, 0
		b.ViewStillTicks = 0
		b.JumpCooldown = 0
		b.EnemySeen = [botTrackLimit]bool{}
		b.TrackSeenAt = [botTrackLimit]uint64{}
		b.TrackKind = [botTrackLimit]uint8{}
		b.JumpTargetActive = false
		b.BaseTargetSeen = 0
		b.FacingLeft = p.Appearance&0x40 != 0
	}
	b.trackVisualEntities(&o)
	if b.JumpTargetActive && !o.Vertical {
		b.JumpTargetX -= botViewDelta(o.ViewX, b.LastViewX, 16)
	}
	if p.Sprite >= 10 && p.Sprite <= 12 || p.Sprite >= 24 && p.Sprite <= 29 || !p.Alive || p.X == 0 {
		b.JumpTargetActive = false
	}
	b.PhaseTicks++
	if o.ViewX == b.LastViewX && o.ViewY == b.LastViewY {
		b.ViewStillTicks++
	} else {
		b.ViewStillTicks = 0
	}
	if p.X == b.LastX && p.Y == b.LastY && o.ViewX == b.LastViewX && o.ViewY == b.LastViewY {
		b.StillTicks++
	} else {
		b.StillTicks = 0
	}
	if b.JumpCooldown > 0 {
		b.JumpCooldown--
	}
	var pad Buttons
	if !o.Paused {
		switch o.Phase {
		case "intro", "interlude":
			if o.Actor == 0 && b.PhaseTicks%2 == 1 {
				pad = ButtonStart
			}
		case "continue":
			if o.ChoiceKnown && o.Actor == 0 && b.PhaseTicks%2 == 1 {
				if o.Choice == 0 {
					pad = ButtonStart
				} else {
					pad = ButtonSelect
				}
			}
		case "battle":
			peer := o.Players[o.Actor^1]
			if o.Seats == 2 && b.BorrowPending[o.Actor] && peer.LivesKnown && peer.Lives > 0 {
				// The original validates the peer's current borrowing gates.
				// Only attempt a normal A edge from the visible HUD evidence.
				if o.Tick%2 == 1 {
					pad = ButtonA
				}
			} else if keepCrouch {
				pad = ButtonDown
				if o.Tick%4 < 2 {
					pad |= ButtonB
				}
			} else if p.Alive {
				pad = b.play(o)
			} else if !p.AliveKnown && (b.PlayerSeenAt[o.Actor] == 0 || o.Tick > b.PlayerSeenAt[o.Actor]+30) {
				// Identical submerged heads do not identify a seat. A legal
				// self-input can move this seat out of the water; establish its
				// identity only when later visible artwork disambiguates it.
				for _, candidate := range o.PlayerCandidates {
					if candidate.Mask&(1<<uint(o.Actor)) != 0 && candidate.Player.Sprite >= 24 && candidate.Player.Sprite <= 29 {
						pad = ButtonRight
						if o.Tick%4 < 2 {
							pad |= ButtonB
						}
						break
					}
				}
			}
		}
	}
	b.Phase, b.Stage, b.Scene = o.Phase, o.Stage, o.Scene
	if pad&ButtonLeft != 0 {
		b.FacingLeft = true
	}
	if pad&ButtonRight != 0 {
		b.FacingLeft = false
	}
	b.LastTick, b.LastPad, b.LastX, b.LastY = o.Tick, pad, p.X, p.Y
	seen := b.EnemySeen
	b.EnemySeen = [botTrackLimit]bool{}
	for _, e := range o.Enemies {
		if e.ID >= 0 && e.ID < botTrackLimit {
			old := b.PreviousEnemies[e.ID]
			vx, vy := float64(e.X-old.X+botViewDelta(o.ViewX, b.LastViewX, 16)), float64(e.Y-old.Y+botViewDelta(o.ViewY, b.LastViewY, 16))
			if seen[e.ID] && old.Kind == e.Kind && math.Abs(vx) < 12 && math.Abs(vy) < 12 {
				b.EnemyVX[e.ID] = (b.EnemyVX[e.ID] + vx) / 2
				b.EnemyVY[e.ID] = (b.EnemyVY[e.ID] + vy) / 2
			} else {
				b.EnemyVX[e.ID], b.EnemyVY[e.ID] = 0, 0
			}
			b.PreviousEnemies[e.ID] = e
			b.EnemySeen[e.ID] = true
			b.TrackSeenAt[e.ID] = o.Tick
			b.TrackCameraX[e.ID], b.TrackCameraY[e.ID] = b.CameraX, b.CameraY
		}
	}
	b.LastViewX, b.LastViewY = o.ViewX, o.ViewY
	b.GridKnown = o.GridKnown
	return pad
}
func (b *Bot) play(o BotObservation) Buttons {
	p := o.Players[o.Actor]
	fire := Buttons(0)
	if p.Weapon&15 == 1 || p.Weapon&15 == 4 || o.Tick%4 < 2 {
		fire = ButtonB
	}
	if o.Scene == 1 {
		return b.base(o) | fire
	}
	if o.Scene == 0x80 {
		return b.baseBoss(o) | fire
	}
	// The accepted jump can precede the displayed airborne pose. Keep its
	// steering while that bounded launch interval is pending; a fresh
	// standing hazard evaluation cannot cancel an already submitted A.
	if !o.Vertical && b.JumpTargetActive && b.JumpCooldown > 0 && p.Sprite != 8 && p.Sprite != 9 {
		return botToward(p.X, b.JumpTargetX, 0) | fire
	}
	if pad, found := b.verticalFlameDropRoute(o); found {
		return pad | fire
	}
	if pad, found := b.clawGroundRoute(o); found {
		return pad | fire
	}
	// Missing support pixels do not prove that a stable, non-jumping body
	// has left the ledge. Hold position rather than defaulting to airborne
	// Right, while preserving an already held crouch. Water poses and pending
	// jump starts retain their normal input handling.
	if o.GridKnown && p.Y >= 0 && p.Y+16 < 224 && absBot(p.Y-b.LastY) <= 1 && b.JumpCooldown == 0 && p.Sprite != 8 && p.Sprite != 9 && !(p.Sprite >= 10 && p.Sprite <= 12) && !(p.Sprite >= 24 && p.Sprite <= 29) && !o.terrainKnownAt(p.X, p.Y+16) && o.supportAt(p.X, p.Y+16) == TerrainEmpty {
		if p.Sprite == 23 && b.LastPad&ButtonDown != 0 {
			if target, escape := b.lowNozzleEscape(o); escape {
				b.JumpTargetX, b.JumpTargetActive, b.JumpCooldown = target, true, 14
				return ButtonA | ButtonRight | fire
			}
			return ButtonDown | fire
		}
		// Overlapping bodies can hide support indefinitely. After sustained
		// visual stagnation, probe with a jump toward an observed landing;
		// never reinterpret the hidden supporting pixels as solid ground.
		if !o.Vertical && b.ViewStillTicks > 240 && b.StillTicks > 240 && !b.JumpTargetActive && p.Sprite >= 13 && p.Sprite <= 22 {
			hazards := b.nozzleHazardView(o)
			if target, _, _, found := b.planJump(hazards, false); found && target > p.X+12 && !botNozzleBlocksJump(hazards, target) && b.nozzleJumpOpening(hazards, target) {
				b.JumpTargetX, b.JumpTargetActive, b.JumpCooldown = target, true, 14
				return ButtonRight | ButtonA | fire
			}
		}
		return fire
	}
	if o.Vertical {
		return b.verticalAim(o, b.climb(o)) | fire
	}
	if b.followSwimmingPeer(o) {
		b.JumpTargetActive = false
		return ButtonRight | fire
	}
	if pad, found := botVisibleCarrierAim(o); found {
		return pad | fire
	}
	if o.Stage == 0 {
		for _, e := range o.Enemies {
			if e.Family == "jungle-hatch" {
				b.JumpTargetActive = false
				if p.Y < 180 && p.Sprite != 8 && p.Sprite != 9 && o.Tick%8 == 0 {
					return ButtonDown | ButtonA | fire
				}
				return ButtonRight | ButtonUp | fire
			}
		}
	}
	pad := ButtonRight
	grounded := p.Y >= 0 && p.Y < 216 && o.terrainAt(p.X, p.Y+16) != TerrainEmpty && absBot(p.Y-b.LastY) <= 1
	grounded = grounded && p.Sprite != 8 && p.Sprite != 9
	hazards := b.nozzleHazardView(o)
	if grounded && botVisibleNozzleBeamAbove(hazards) {
		b.JumpTargetActive = false
		return ButtonDown | fire
	}
	if grounded {
		if pad, ok := b.blockedWallDrop(hazards, fire); ok {
			return pad
		}
	}
	if grounded && b.JumpCooldown == 0 {
		b.JumpTargetActive = false
	}
	jump := false
	holdTarget := false
	if grounded {
		target, landingY, _, found := b.planJump(hazards, false)
		preferRise := b.ViewStillTicks > 240 && found && target > p.X+8 && landingY < p.Y-8
		// An obscured cell is not evidence of a gap. Wait for the current
		// pixels to disclose it instead of repeatedly jumping backward.
		if o.GridKnown && !o.terrainKnownAt(p.X+8, p.Y+16) {
			if !preferRise && p.Sprite != 23 && b.safeDrop(o) {
				b.JumpTargetActive = false
				return ButtonRight | fire
			}
			// Move only while the center still has a visible support margin.
			// This can reveal a cell hidden by our own feet without walking
			// across an unobserved edge or releasing an established crouch.
			next := o.terrainAt(p.X+4, p.Y+16)
			if !preferRise && p.Sprite != 23 && b.LastPad&ButtonDown == 0 && o.terrainKnownAt(p.X+4, p.Y+16) && (next == TerrainPlatform || next == TerrainSolid) && o.terrainAt(p.X+4, p.Y-4) != TerrainSolid {
				return ButtonRight | fire
			}
			// A known forward surface can justify a jump without
			// asserting whether the obscured fill below it is solid or air.
			if !found || target <= p.X+12 {
				return fire
			}
		}
		// The visible body extends beyond its center. Start while its leading
		// foot is still supported; a center-only probe misses the legal window.
		front := o.terrainAt(p.X+8, p.Y+16)
		edge := front == TerrainEmpty || front == TerrainWater || o.terrainAt(p.X+12, p.Y-4) == TerrainSolid
		// A visible shallow step can block walking even when its fill is
		// ambiguous. After stagnation, use the predicted forward landing.
		jump = found && (edge || p.Y >= 160 && landingY < p.Y-24 || preferRise || b.nearDanger(o))
		if found && landingY < p.Y-24 && botNozzleLandingRisk(hazards, p.X+16, p.Y) > 0 && botNozzleLandingRisk(hazards, target, landingY) == 0 {
			jump = true
		}
		if edge && b.safeDrop(o) && (!found || target <= p.X+12 || landingY > p.Y) {
			jump = false
			found = true
			b.JumpTargetActive = false
		}
		if jump && b.JumpCooldown == 0 {
			b.JumpTargetX, b.JumpTargetActive = target, true
		} else if edge && !found {
			pad = ButtonLeft
		}
	}
	if jump && (botNozzleBlocksJump(hazards, b.JumpTargetX) || !b.nozzleJumpOpening(hazards, b.JumpTargetX)) {
		b.JumpTargetActive = false
		return ButtonDown | fire
	}
	if jump && b.JumpCooldown == 0 {
		pad |= ButtonA
		b.JumpCooldown = 14
	}
	if !grounded && !b.JumpTargetActive && p.Y >= b.LastY && (p.Sprite == 8 || p.Sprite == 9) {
		if target, found := b.planDescent(o); found {
			b.JumpTargetX, b.JumpTargetActive = target, true
		}
	}
	if b.JumpTargetActive && !grounded {
		pad &^= ButtonLeft | ButtonRight
		pad |= botToward(p.X, b.JumpTargetX, 0)
		if p.X == b.JumpTargetX {
			if b.LastPad&ButtonLeft != 0 {
				pad |= ButtonRight
			} else {
				pad |= ButtonLeft
			}
		}
	}
	if !grounded && p.Sprite >= 8 && p.Sprite <= 9 {
		if escape := b.airEscape(o); escape != 0 {
			pad = pad&^(ButtonLeft|ButtonRight) | escape
		}
	}
	if grounded && !jump {
		for _, e := range o.Enemies {
			if (e.Kind == 4 || e.Kind == 7) && e.X-p.X > 38 && e.X-p.X < 100 && absBot(e.Y-(p.Y-8)) < 9 {
				pad = 0
				holdTarget = true
				break
			}
		}
	}
	// A stopped camera and an impassable visible wall commonly mark a boss.
	// Shooting across repeated legal jumps searches its visible weak-point height.
	if b.StillTicks > 20 && !holdTarget {
		pad = ButtonRight | (pad & ButtonA)
		if o.Tick%65 == 0 {
			pad |= ButtonA
		}
		if o.Tick%130 < 35 {
			pad |= ButtonUp
		}
	}
	for _, e := range o.Enemies {
		// Flame mouths and their beams are hazard geometry, not targets.
		// In particular, Down aiming must not turn a planned A into a drop.
		if e.Family == "machine-flame-nozzle-left" || e.Family == "machine-flame" {
			continue
		}
		if e.Kind == 0 || e.Kind == 1 || e.Kind == 2 || e.Kind == 11 || e.Kind == 15 || e.Kind == 18 && o.Stage == 0 {
			continue
		}
		dx, dy := e.X-p.X, e.Y-(p.Y-8)
		if dx > 20 && dx < 110 && absBot(dy) > 20 && absBot(dy) < dx+20 {
			if dy < 0 {
				pad &^= ButtonDown
				pad |= ButtonUp
			} else {
				pad &^= ButtonUp
				pad |= ButtonDown
			}
			break
		}
	}
	return pad | fire
}
func (b *Bot) base(o BotObservation) Buttons {
	p := o.Players[o.Actor]
	// Wall cores are public background actors. Choose a firing lane from their
	// visible position, compensating for the original perspective convergence.
	var core BotEnemy
	remembered := b.BaseTargetSeen != 0 && o.Tick >= b.BaseTargetSeen && o.Tick-b.BaseTargetSeen <= 120
	if remembered {
		core = b.BaseTarget
		for _, e := range o.Enemies {
			if absBot(e.X-core.X) > 2 || absBot(e.Y-core.Y) > 2 {
				continue
			}
			if core.Kind == 254 && e.Kind == 19 {
				remembered = false
				break
			}
			if e.Kind == 20 || e.Kind == 254 {
				core = e
				b.BaseTarget, b.BaseTargetSeen = e, o.Tick
				break
			}
		}
	}
	if !remembered {
		found, best := false, 10000
		for _, e := range o.Enemies {
			if e.Kind != 20 && e.Kind != 254 {
				continue
			}
			d := absBot(e.X-p.X) + absBot(e.Y-104)
			if d < best {
				core, best, found = e, d, true
			}
		}
		if !found {
			return ButtonUp
		}
		b.BaseTarget, b.BaseTargetSeen = core, o.Tick
	}
	prone := core.Y >= 110 && p.Y >= 160
	originY, originX := 144, 1
	if prone {
		originY, originX = 156, -1
	}
	depth := float64(originY-core.Y) / 64
	depth = min(.8, max(0, depth))
	target := int((float64(core.X) - 128*depth - float64(originX)) / (1 - depth))
	target = max(34, min(222, target))
	pad := botToward(p.X, target, 2)
	if prone && pad == 0 {
		pad = ButtonDown
	}
	for _, e := range o.Enemies {
		if e.Kind == 1 && absBot(e.X-p.X) < 24 && e.Y > p.Y-52 && e.Y < p.Y+24 && p.Y >= 160 {
			return ButtonDown
		}
		if (e.Kind == 17 || e.Kind == 18) && absBot(e.X-p.X) < 36 && e.Y > 125 && b.JumpCooldown == 0 {
			b.JumpCooldown = 45
			return pad&^ButtonDown | ButtonA
		}
	}
	if core.Y < 96 && absBot(p.X-target) < 5 && b.PhaseTicks%55 == 0 && b.JumpCooldown == 0 {
		pad |= ButtonA
		b.JumpCooldown = 45
	}
	return pad
}
func (b *Bot) baseBoss(o BotObservation) Buttons {
	p := o.Players[o.Actor]
	target := 128
	best := 1000
	plating := false
	hasCannons := false
	for _, e := range o.Enemies {
		if e.Kind == 8 && o.Seats == 1 {
			hasCannons = true
		}
	}
	for _, e := range o.Enemies {
		if !hasCannons && (e.Kind == 16 || e.Kind == 28) && e.Sprite >= 5 && e.Y < p.Y {
			target = e.X + 1
			if e.Kind == 16 && e.ID >= 0 && e.ID < botTrackLimit && b.EnemySeen[e.ID] {
				direction := 1.25
				if e.X < b.PreviousEnemies[e.ID].X {
					direction = -1.25
				}
				speed := 3.0
				if p.Weapon&0x10 != 0 || p.Weapon&15 == 4 {
					speed = 4
				}
				lead := float64(e.X) + direction*float64(max(0, p.Y-24-e.Y))/speed
				if lead > 176 {
					lead = 352 - lead
				}
				if lead < 80 {
					lead = 160 - lead
				}
				target = int(lead) + 1
			}
			pad := botToward(p.X, target, 7)
			if evade := b.simpleBossEvade(o); evade != 0 {
				pad = evade
			}
			return pad
		}
	}
	for _, e := range o.Enemies {
		if e.Kind != 8 && e.Kind != 10 || hasCannons && e.Kind != 8 {
			continue
		}
		d := absBot(e.X - p.X)
		if d < best {
			offset := 1
			if e.Kind == 8 {
				offset = 10
			}
			target, best, plating = e.X+offset, d, true
		}
	}
	for _, e := range o.Enemies {
		if !plating && (e.Kind == 16 || e.Kind == 28) && e.Y < p.Y && e.Sprite >= 5 {
			d := absBot(e.X - p.X)
			if d < best {
				best = d
				target = e.X
			}
		}
	}
	pad := botToward(p.X, target, 3)
	if evade := b.simpleBossEvade(o); evade != 0 {
		pad = evade
	}
	return pad
}
func (b *Bot) climb(o BotObservation) Buttons {
	p := o.Players[o.Actor]
	grounded := o.supportAt(p.X, p.Y+16) != TerrainEmpty && p.Y == b.LastY && p.Sprite != 8 && p.Sprite != 9
	if grounded {
		if b.JumpCooldown == 0 {
			b.JumpTargetActive = false
		}
		peer := o.Players[o.Actor^1]
		if o.Seats == 2 && peer.Alive && peer.Y > p.Y+32 && p.Y < 160 {
			return b.climbAvoid(o, 0) | ButtonUp
		}
		tx, ty, platform, found := b.planJump(o, true)
		target, approachFound := botClimbApproach(o, p.X, p.Y+16)
		lateral := approachFound && ty <= p.Y+4 && absBot(tx-p.X) >= 12 && absBot(tx-target) < absBot(p.X-target)
		if found && (ty < p.Y-12 || lateral) && b.JumpCooldown == 0 {
			b.JumpTargetX, b.JumpTargetActive = tx, true
			b.JumpPlatform = platform + 1
			b.JumpCooldown = 14
			return botToward(p.X, tx, 0) | ButtonA | ButtonUp
		}
		// A higher ledge may require a short downward detour. Use this
		// only after sustained stagnation, preserving the ordinary upper
		// peer wait and requiring a reachable visible landing.
		if approachFound && b.StillTicks > 120 && b.ViewStillTicks > 240 && b.JumpCooldown == 0 {
			if x, _, platform, ok := b.planJumpRoute(o, true, true); ok {
				b.JumpTargetX, b.JumpTargetActive, b.JumpPlatform = x, true, platform+1
				b.JumpCooldown = 14
				return botToward(p.X, x, 0) | ButtonA | ButtonUp
			}
		}
		// A safe jump on the same broad ledge can reveal the next part of a
		// vertical viewport; it does not assume an offscreen map or boss trigger.
		if !approachFound && p.Y < 120 && b.ViewStillTicks > 240 && b.JumpCooldown == 0 {
			left, right := p.X, p.X
			for left > 28 && o.supportAt(left-1, p.Y+16) != TerrainEmpty {
				left--
			}
			for right < 228 && o.supportAt(right+1, p.Y+16) != TerrainEmpty {
				right++
			}
			if right-left >= 32 {
				center := max(left+4, min(right-4, p.X))
				if center != p.X {
					return botToward(p.X, center, 0) | ButtonUp
				}
				b.JumpTargetX, b.JumpTargetActive, b.JumpPlatform = p.X, true, 0
				b.JumpCooldown = 14
				return ButtonA | ButtonUp
			}
		}
		if approachFound {
			return b.climbAvoid(o, botToward(p.X, target, 2)) | ButtonUp
		}
		return b.climbAvoid(o, botToward(p.X, 128, 4)) | ButtonUp
	}
	if !b.JumpTargetActive && p.Y >= b.LastY && (p.Sprite == 8 || p.Sprite == 9) {
		if target, found := b.planDescent(o); found {
			// Keep the landing target: neutral input preserves original air
			// momentum, so reuse the normal corrective steering below.
			b.JumpTargetX, b.JumpTargetActive, b.JumpPlatform = target, true, 0
		}
	}
	if b.JumpTargetActive {
		if b.JumpPlatform != 0 {
			for _, e := range o.Enemies {
				if e.ID+1 == b.JumpPlatform && e.Kind == 16 {
					b.JumpTargetX = e.X
				}
			}
		}
		pad := botToward(p.X, b.JumpTargetX, 0)
		if pad == 0 {
			if b.LastPad&ButtonLeft != 0 {
				pad = ButtonRight
			} else {
				pad = ButtonLeft
			}
		}
		if escape := b.airEscape(o); escape != 0 {
			pad = escape
		}
		return pad | ButtonUp
	}
	return botToward(p.X, 128, 3) | ButtonUp
}

func botClimbApproach(o BotObservation, x, foot int) (int, bool) {
	best, target := 10000, x
	found := false
	for _, e := range o.Enemies {
		if e.Kind == 16 && e.Sprite == 72 && e.Y-16 < foot-16 && e.Y-16 >= foot-60 {
			best, target, found = absBot(e.X-x)+(foot-e.Y+16)/4, e.X, true
		}
	}
	for y := max(0, foot-60); y < foot-16; y += 4 {
		for xx := 28; xx <= 228; xx += 4 {
			kind := o.terrainAt(xx, y)
			if (kind != TerrainPlatform && kind != TerrainSolid) || o.terrainAt(xx, y-4) != TerrainEmpty {
				continue
			}
			if o.terrainAt(xx-4, y) == TerrainEmpty || o.terrainAt(xx+4, y) == TerrainEmpty {
				continue
			}
			score := absBot(xx-x) + (foot-y)/4
			if score < best {
				best, target, found = score, xx, true
			}
		}
	}
	return target, found
}
func (b *Bot) nearDanger(o BotObservation) bool {
	p := o.Players[o.Actor]
	for _, e := range o.Enemies {
		if e.Kind == 0 || e.Kind == 2 || e.Kind == 3 || e.Sprite < 5 {
			continue
		}
		dx, dy := e.X-p.X, e.Y-p.Y
		if absBot(dx) < 26 && dy > -25 && dy < 15 {
			return true
		}
		if e.ID >= 0 && e.ID < botTrackLimit && b.EnemySeen[e.ID] {
			old := b.PreviousEnemies[e.ID]
			if old.Kind != e.Kind {
				continue
			}
			vx, vy := e.X-old.X, e.Y-old.Y
			if absBot(vx) < 10 && absBot(vy) < 10 && absBot(dx+vx*8) < 14 && absBot(dy+vy*8) < 22 {
				return true
			}
		}
	}
	return false
}
func botLanding(o BotObservation, x, foot int, vertical bool) (int, int, bool) {
	bestScore := 100000
	tx, ty := x, foot
	found := false
	for yy := max(8, foot-60); yy < foot-12; yy += 4 {
		for xx := 8; xx < 248; xx += 4 {
			dx := xx - x
			if absBot(dx) > 56 || !vertical && dx < 8 {
				continue
			}
			// A higher platform must be reached before the descending feet pass
			// its top. Use the known public jump envelope, with landing margin.
			height := float64(foot - yy)
			discriminant := 4.0625*4.0625 - 2*(35.0/256.0)*height
			if discriminant < 0 || float64(absBot(dx)+10) > (4.0625+math.Sqrt(discriminant))/(35.0/256.0) {
				continue
			}
			kind := o.terrainAt(xx, yy)
			if (kind != TerrainPlatform && kind != TerrainSolid) || o.terrainAt(xx, yy-4) != TerrainEmpty {
				continue
			}
			if o.terrainAt(xx-4, yy) != kind || o.terrainAt(xx+4, yy) != kind {
				continue
			}
			score := absBot(dx)*2 + (yy-(foot-60))*2
			if score < bestScore {
				bestScore, tx, ty, found = score, xx, yy, true
			}
		}
	}
	return tx, ty, found
}
func botToward(x, target, tolerance int) Buttons {
	if x < target-tolerance {
		return ButtonRight
	}
	if x > target+tolerance {
		return ButtonLeft
	}
	return 0
}
func absBot(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// A flat strip with visible support on both sides of a small occluded region
// gives a bounded walking option below overhead claws. Unknown cells retain
// their unknown meaning; an observed gap or descent forbids this option.
func (b *Bot) clawGroundRoute(o BotObservation) (Buttons, bool) {
	p := o.Players[o.Actor]
	if o.Vertical || o.Scene != 0 || !o.GridKnown || !p.Alive || absBot(p.Y-b.LastY) > 1 || p.Y+16 >= 224 || !(p.Sprite >= 1 && p.Sprite <= 7 || p.Sprite >= 13 && p.Sprite <= 23) {
		return 0, false
	}
	seen := b.PlayerSeenAt[o.Actor]
	if !p.Visible && (seen == 0 || o.Tick < seen || o.Tick-seen > 8) {
		return 0, false
	}
	relevant, blocked := false, false
	for _, e := range o.Enemies {
		if e.Family != "machine-claw" || e.X < p.X-16 || e.X > p.X+80 || e.Y > p.Y+8 {
			continue
		}
		relevant = true
		if e.X >= p.X+8 && e.X <= p.X+28 && e.Y >= p.Y-24 {
			blocked = true
		}
	}
	if !relevant {
		return 0, false
	}
	supported := func(x int) bool {
		k := o.terrainAt(x, p.Y+16)
		return o.terrainKnownAt(x, p.Y+16) && (k == TerrainPlatform || k == TerrainSolid)
	}
	left := false
	for d := 0; d <= 16; d += 4 {
		left = left || supported(p.X-d)
	}
	right := 0
	for d := 16; d <= 32; d += 4 {
		if supported(p.X + d) {
			right = p.X + d
			break
		}
	}
	if !left || right == 0 {
		return 0, false
	}
	for x := p.X; x <= right; x++ {
		if o.terrainKnownAt(x, p.Y+16) && !supported(x) {
			return 0, false
		}
	}
	if blocked {
		return 0, true
	}
	return ButtonRight, true
}

// A previously visible downward fire column remains a route obstacle through
// brief visual gaps. This remembers an observed location, not an active flag
// or emitter clock. A lower currently visible platform is required; the
// original neutral Down+A admission and fall envelope remain authoritative.
func (b *Bot) verticalFlameDropRoute(o BotObservation) (Buttons, bool) {
	p := o.Players[o.Actor]
	if o.Vertical || o.Scene != 0 || !o.GridKnown || !p.Alive || b.JumpCooldown != 0 || b.JumpTargetActive || absBot(p.Y-b.LastY) > 1 || !(p.Sprite >= 1 && p.Sprite <= 7 || p.Sprite >= 13 && p.Sprite <= 23) {
		return 0, false
	}
	seen := b.PlayerSeenAt[o.Actor]
	if !p.Visible && (seen == 0 || o.Tick < seen || o.Tick-seen > 8) {
		return 0, false
	}
	blocksRoute := func(e BotEnemy) bool {
		return e.Family == "machine-flame" && e.Kind == 16 && e.X >= p.X+16 && e.X <= p.X+112
	}
	relevant := false
	for _, e := range o.Enemies {
		relevant = relevant || blocksRoute(e)
	}
	for id, e := range b.PreviousEnemies {
		seen := b.TrackSeenAt[id]
		if seen == 0 || o.Tick < seen || o.Tick-seen > 96 {
			continue
		}
		e.X -= b.CameraX - b.TrackCameraX[id]
		relevant = relevant || blocksRoute(e)
	}
	if !relevant {
		return 0, false
	}
	x, y, found := b.planDropThrough(o)
	if !found || y <= p.Y+16 {
		return 0, false
	}
	b.JumpTargetX, b.JumpTargetActive, b.JumpCooldown = x, true, 14
	return ButtonDown | ButtonA, true
}
