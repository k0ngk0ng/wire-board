package engine

import "strings"

func stepGrenadeWeapon(state *State, p *Player, item *WeaponState) {
	if p.Input.Fire && !p.Previous.Fire && state.Frame >= p.NextAttack && item.Ammo > 0 && p.Action == "" {
		p.Action = "grenade:" + item.ID
		p.ActionSince = state.Frame
		Emit(state, Event{Kind: "grenade_pin", Seat: p.Seat, Weapon: item.ID})
	}
	if p.Action != "grenade:"+item.ID || p.Input.Fire || state.Frame < p.ActionSince+50 {
		return
	}
	if len(state.Grenades) >= 32 {
		p.Action = ""
		p.ActionSince = 0
		return
	}
	pitch := p.Pitch
	if pitch < 0 {
		pitch = -1820 + pitch*80/90
	} else {
		pitch = -1820 + pitch*100/90
	}
	speed := min(750*Unit, (16384-pitch)*6*Unit/182)
	direction := Direction(p.Yaw, pitch)
	start := combatAddRay(combatEye(p), direction, 16*Unit)
	trace := TraceWorld(state, combatEye(p), start, -1)
	start = trace.End
	state.NextID++
	fuse := int64(150)
	if item.ID == "smokegrenade" {
		fuse = 300
	}
	state.Grenades = append(state.Grenades, Grenade{ID: state.NextID, Owner: p.Seat, Kind: item.ID, X: start.X, Y: start.Y, Z: start.Z, VX: int(int64(direction.X)*int64(speed)/NormalUnit) + p.VX, VY: int(int64(direction.Y)*int64(speed)/NormalUnit) + p.VY, VZ: int(int64(direction.Z)*int64(speed)/NormalUnit) + p.VZ, ExplodesAt: state.Frame + fuse})
	item.Ammo--
	p.Action = ""
	p.ActionSince = 0
	p.NextAttack = state.Frame + 75
	Emit(state, Event{Kind: "throw", Seat: p.Seat, Weapon: item.ID, X: start.X, Y: start.Y, Z: start.Z})
	if item.Ammo <= 0 {
		index := weaponIndex(p, item.ID)
		p.Inventory = append(p.Inventory[:index], p.Inventory[index+1:]...)
		p.Weapon = ""
		SyncWeapon(p)
	}
}

func stepGrenades(state *State) {
	kept := make([]Grenade, 0, len(state.Grenades))
	for i := range state.Grenades {
		grenade := state.Grenades[i]
		if grenade.Active {
			if state.Frame < grenade.ExpiresAt {
				kept = append(kept, grenade)
			}
			continue
		}
		if state.Frame >= grenade.ExplodesAt {
			grenade.Active = true
			center := Vec3{grenade.X, grenade.Y, grenade.Z}
			if grenade.Kind == "hegrenade" {
				center = blastOrigin(state, center)
			}
			Emit(state, Event{Kind: "explode", Seat: grenade.Owner, Target: grenade.ID, Weapon: grenade.Kind, X: center.X, Y: center.Y, Z: center.Z})
			switch grenade.Kind {
			case "hegrenade":
				explodeHEAt(state, grenade.Owner, center)
			case "flashbang":
				explodeFlash(state, grenade)
			case "smokegrenade":
				grenade.ExpiresAt = state.Frame + 1800
				kept = append(kept, grenade)
			}
			continue
		}
		grenade.VZ -= 400 * Unit / FPS
		start := Vec3{grenade.X, grenade.Y, grenade.Z}
		end := Vec3{start.X + grenade.VX/FPS, start.Y + grenade.VY/FPS, start.Z + grenade.VZ/FPS}
		trace := TraceWorld(state, start, end, -1)
		grenade.X, grenade.Y, grenade.Z = trace.End.X, trace.End.Y, trace.End.Z
		if trace.Fraction < NormalUnit {
			dot := (int64(grenade.VX)*int64(trace.Normal.X) + int64(grenade.VY)*int64(trace.Normal.Y) + int64(grenade.VZ)*int64(trace.Normal.Z)) / NormalUnit
			grenade.VX = (grenade.VX - int(dot*int64(trace.Normal.X)*2/NormalUnit)) * 45 / 100
			grenade.VY = (grenade.VY - int(dot*int64(trace.Normal.Y)*2/NormalUnit)) * 45 / 100
			grenade.VZ = (grenade.VZ - int(dot*int64(trace.Normal.Z)*2/NormalUnit)) * 45 / 100
			if trace.Normal.Z > NormalUnit*7/10 && grenade.VZ < 30*Unit {
				grenade.VZ = 0
				grenade.VX = grenade.VX * 4 / 5
				grenade.VY = grenade.VY * 4 / 5
			}
		}
		kept = append(kept, grenade)
	}
	state.Grenades = kept
}

func explodeHE(state *State, grenade Grenade) {
	center := blastOrigin(state, Vec3{grenade.X, grenade.Y, grenade.Z})
	explodeHEAt(state, grenade.Owner, center)
}

func explodeHEAt(state *State, owner int, center Vec3) {
	source := center
	source.Z += Unit
	for seat := range state.Players {
		p := &state.Players[seat]
		if !p.Alive {
			continue
		}
		if !blastWaterAllowed(state, center, p.WaterLevel) {
			continue
		}
		damage := blastDamageUnits(source, combatPosition(p), 100, 350*Unit)
		damagePlayerUnitsFrom(state, owner, seat, damage, 500, 0, "hegrenade", center, true)
	}
	for index := range state.Hostages {
		h := state.Hostages[index]
		if h.Rescued || h.Health <= 0 {
			continue
		}
		damage := blastDamageUnits(source, Vec3{h.X, h.Y, h.Z}, 100, 350*Unit)
		group := hostageDamageStateFor(state, index).Hitgroup
		damageHostageUnits(state, owner, index, hostageHitUnits(damage, group))
	}
	damageBlastBrushes(state, owner, center, source, 100, 350*Unit, "hegrenade")
}

func explodeFlash(state *State, grenade Grenade) {
	center := Vec3{grenade.X, grenade.Y, grenade.Z}
	for seat := range state.Players {
		p := &state.Players[seat]
		if !p.Alive {
			continue
		}
		eye := combatEye(p)
		distance := Distance(center, eye)
		if distance >= 1500*Unit {
			continue
		}
		trace := TraceWorld(state, center, eye, -1)
		if trace.Fraction < NormalUnit {
			continue
		}
		direction := Direction(p.Yaw, p.Pitch)
		dot := int64(center.X-eye.X)*int64(direction.X) + int64(center.Y-eye.Y)*int64(direction.Y) + int64(center.Z-eye.Z)*int64(direction.Z)
		strength := 1000 * (1500*Unit - distance) / (1500 * Unit)
		if dot < 0 {
			strength = strength / 3
		}
		p.FlashStrength = max(p.FlashStrength, strength)
		p.FlashUntil = max(p.FlashUntil, state.Frame+int64(50+strength*450/1000))
		Emit(state, Event{Kind: "flash", Seat: grenade.Owner, Target: seat, Value: strength})
	}
}

func CancelGrenade(player *Player) {
	if strings.HasPrefix(player.Action, "grenade:") {
		player.Action = ""
		player.ActionSince = 0
	}
}

func DamageHostage(state *State, attacker, index, damage int) {
	damageHostageUnits(state, attacker, index, int64(damage)*damageUnit)
}
