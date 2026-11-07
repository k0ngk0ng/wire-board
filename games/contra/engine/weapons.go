package contra

// Projectile is semantic, serializable state for one of the original sixteen
// projectile slots. Positions have separate preserved fractional accumulators;
// the original clear routine intentionally leaves position/fractions intact.
type Projectile struct {
	Sprite, Attributes, Kind                           uint8
	X, Y, XFraction, YFraction                         uint8
	VX, VY                                             int16
	Timer, Aim, Routine, Owner                         uint8
	RapidOrSpread, DistanceOrSpread                    uint8
	CenterX, CenterY, CenterXFraction, CenterYFraction uint8
}

type WeaponSystem struct{ Projectiles [16]Projectile }

func (p *Projectile) clear() {
	*p = Projectile{X: p.X, Y: p.Y, XFraction: p.XFraction, YFraction: p.YFraction}
}

// Fire follows Japanese $8001..$8489. It runs once for each active player after
// aim/ledge handling and before that player's movement. Slot allocation and
// sounds are consequences of these rules, not externally supplied outcomes.
func (s *WeaponSystem) Fire(r FireRequest) FireResult {
	out := FireResult{MachineGunTimer: r.MachineGunTimer, Recoil: r.Recoil}
	if r.Hidden || r.Electrocution != 0 || r.Water != 0 && r.Aim >= 3 && r.Aim < 7 {
		return out
	}
	weapon := r.Weapon & 15
	if r.Actor < 0 || r.Actor > 1 || weapon > 4 || r.Aim > 11 {
		panic("invalid Contra weapon state")
	}
	pressed := r.Pressed&ButtonB != 0
	if weapon == 1 || weapon == 4 {
		pressed = r.Held&ButtonB != 0
	}
	if !pressed {
		out.MachineGunTimer &= 15
		if out.MachineGunTimer < 7 {
			out.MachineGunTimer++
		}
		return out
	}
	r.ThrowWork.record(r.Jump)
	aim := r.Aim
	if r.Jump != 0 && (aim == 4 || aim == 5) {
		aim = 11
	}
	rapid := (r.Weapon >> 4) & 1
	indoor := r.StageType != 0 && r.StageType&0x80 == 0
	create := func(slot int) *Projectile {
		p := &s.Projectiles[slot]
		p.clear()
		p.Kind, p.Owner, p.Aim, p.Sprite = weapon+1, uint8(r.Actor), aim, weaponSprites[weapon]
		out.Recoil = 15
		out.Sounds = append(out.Sounds, weaponSounds[weapon])
		return p
	}
	find := func(limit, p2Start int) int {
		start := 0
		if r.Actor == 1 {
			start = p2Start
		}
		for i := start; i < start+limit; i++ {
			if s.Projectiles[i].Kind == 0 {
				return i
			}
		}
		return -1
	}
	initShot := func(p *Projectile) {
		if indoor {
			p.initIndoor(r, aim, rapid)
		} else {
			p.initOutdoor(r, weapon, aim, rapid)
		}
	}
	switch weapon {
	case 0, 2:
		slot := find(4, 10)
		if slot < 0 {
			return out
		}
		p := create(slot)
		initShot(p)
		if weapon == 2 {
			if indoor {
				p.DistanceOrSpread = 4
				p.CenterX = p.X
				p.CenterY = p.Y - 2
				p.RapidOrSpread = rapid
			} else {
				p.initFlameCenter(aim)
			}
		}
	case 1:
		out.MachineGunTimer++
		threshold := uint8(8)
		if out.MachineGunTimer >= 0x60 {
			threshold = 15
		}
		if out.MachineGunTimer&15 < threshold {
			return out
		}
		next := out.MachineGunTimer + 16
		if next >= 0x70 {
			next = 0
		}
		out.MachineGunTimer = next & 0xf0
		slot := find(6, 10)
		if slot < 0 {
			out.MachineGunTimer = 7
			return out
		}
		initShot(create(slot))
	case 3:
		for shot := uint8(0); shot < 5; shot++ {
			slot := find(10, 6)
			if slot < 0 {
				break
			}
			p := create(slot)
			p.CenterYFraction = shot
			if indoor {
				p.initIndoor(r, aim, rapid)
				p.CenterX = p.X
				p.CenterY = rapid
			} else {
				p.initMuzzle(r, aim)
				// 849F publishes the selected spread velocity table pointer's
				// low byte. It is a compatibility effect, not random entropy.
				out.GenerationBiasChanged = true
				out.GenerationBiasValue = [2]uint8{0xf4, 0x44}[rapid]
				angle := (spreadAngles[aim] + spreadDeltas[shot]) & 31
				p.VX, p.VY = spreadXVelocity[rapid][angle], spreadYVelocity[rapid][angle]
			}
		}
	case 4:
		rapid = 1
		start := 0
		if r.Actor == 1 {
			start = 10
		}
		if r.Pressed&ButtonB == 0 {
			for i := start; i < start+4; i++ {
				if s.Projectiles[i].Kind != 0 {
					return out
				}
			}
		}
		for i := 0; i < 4; i++ {
			p := create(start + i)
			initShot(p)
			level := 0
			if indoor {
				level = 1
			}
			p.Timer = laserDelays[level][3-i]
		}
	}
	return out
}

func (p *Projectile) initMuzzle(r FireRequest, aim uint8) {
	mode := 0
	if r.StageType != 0 {
		mode = 2
	}
	if r.Jump != 0 {
		mode++
	}
	offset := weaponMuzzles[mode][aim]
	p.X, p.Y = r.X+offset[0], r.Y+offset[1]
}

func (p *Projectile) initOutdoor(r FireRequest, weapon, aim, rapid uint8) {
	p.initMuzzle(r, aim)
	mode := uint8(0)
	if weapon == 2 {
		mode = 4
	}
	if weapon == 4 {
		mode = 2
	}
	v := weaponVelocities[mode+rapid][aim]
	p.VX, p.VY = v[0], v[1]
}

func (p *Projectile) initIndoor(r FireRequest, aim, rapid uint8) {
	if r.Work != nil {
		r.Work.RecordPerspectiveX(r.X)
	}
	dx, dy := uint8(0), uint8(0)
	if r.Jump == 0 {
		dx, dy = 255, 0xf4
		if aim != 4 && aim != 5 {
			dy = 0xe8
			if aim < 5 {
				dx = 1
			}
		}
	}
	p.X, p.Y = r.X+dx, r.Y+dy
	if dy == 0xf4 {
		p.Kind |= 0x80
	}
	if r.Jump != 0 && p.Y >= 0x98 {
		p.Y = 0x98
	}
	shift := 6 - rapid
	p.VY = -int16((uint16(0x40) << 8) >> shift)
	delta := int(r.X) - 128
	if delta < 0 {
		p.VX = int16((uint16(-delta) << 8) >> shift)
	} else {
		p.VX = -int16((uint16(delta) << 8) >> shift)
	}
	p.Timer = 42
	if rapid != 0 {
		p.Timer = 21
	}
}

func (p *Projectile) initFlameCenter(aim uint8) {
	data := flameOrigins[aim]
	p.Timer = data[0]
	p.CenterX = p.X + data[1]
	y := int(p.Y) + int(int8(data[2]))
	if y < 0 || y > 255 {
		p.clear()
		return
	}
	p.CenterY = uint8(y)
}
