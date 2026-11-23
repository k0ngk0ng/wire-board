package engine

import (
	"encoding/json"
	"sync"

	"github.com/k0ngk0ng/wire-board/games/cs16/data"
)

type combatHitbox struct {
	Bone  int  `json:"bone"`
	Group int  `json:"group"`
	Min   Vec3 `json:"min"`
	Max   Vec3 `json:"max"`
}

type combatBone struct {
	X  int `json:"x"`
	Y  int `json:"y"`
	Z  int `json:"z"`
	XX int `json:"xx"`
	XY int `json:"xy"`
	XZ int `json:"xz"`
	YX int `json:"yx"`
	YY int `json:"yy"`
	YZ int `json:"yz"`
	ZX int `json:"zx"`
	ZY int `json:"zy"`
	ZZ int `json:"zz"`
}

type combatModel struct {
	ID       string                  `json:"id"`
	Hitboxes []combatHitbox          `json:"hitboxes"`
	Poses    map[string][]combatBone `json:"poses"`
}

var combatModels map[string]combatModel
var combatModelOnce sync.Once

func loadCombatModels() {
	combatModelOnce.Do(func() {
		var entries []combatModel
		if json.Unmarshal([]byte(data.ModelRulesJSON), &entries) != nil {
			panic("CS人物碰撞数据无法解码")
		}
		combatModels = make(map[string]combatModel, len(entries))
		for _, entry := range entries {
			combatModels[entry.ID] = entry
		}
	})
}

type combatShot struct {
	Seat      int
	Life      int
	Weapon    Weapon
	Start     Vec3
	Yaw       int
	Pitch     int
	Spread    int
	ViewFrame int64
	Secondary bool
	Silenced  bool
	Burst     bool
	Damage    int
}

func StepCombat(state *State) {
	if state.Finished || state.Phase != "live" && state.Phase != "freeze" {
		return
	}
	live := state.Phase == "live"
	if live {
		stepGrenades(state)
	}
	shots := make([]combatShot, 0, len(state.Players))
	for seat := range state.Players {
		p := &state.Players[seat]
		stepFlashlight(state, p)
		if !p.Alive {
			continue
		}
		landing := p.LandingSpeed
		p.LandingSpeed = 0
		if live && landing >= 250*Unit {
			Emit(state, Event{Kind: "land", Seat: seat, Value: landing / Unit, X: p.X, Y: p.Y, Z: p.Z, Life: p.Life})
			p.StepAt = state.Frame + 30
		}
		if live && landing > 500*Unit && p.WaterLevel == 0 {
			DamagePlayer(state, -1, seat, (landing-500*Unit)*125/(600*Unit), 1000, 0, "fall")
			if !p.Alive {
				continue
			}
		}
		decayRecoil(p)
		if state.Frame >= p.FlashUntil {
			p.FlashStrength = 0
		}
		if p.Input.Select != "" && p.Input.Select != p.Weapon && weaponIndex(p, p.Input.Select) >= 0 {
			old := weaponIndex(p, p.Weapon)
			if old >= 0 {
				p.Inventory[old].ReloadUntil, p.Inventory[old].BurstLeft = 0, 0
				p.Inventory[old].Zoom, p.Inventory[old].ResumeZoom = 0, 0
			}
			p.Weapon, p.NextAttack = p.Input.Select, state.Frame+weaponDeployFrames(p.Input.Select)
			index := weaponIndex(p, p.Weapon)
			p.Inventory[index].Zoom, p.Inventory[index].ResumeZoom = 0, 0
			resetWeaponAccuracy(&p.Inventory[index], false)
			Emit(state, Event{Kind: "select", Seat: seat, Weapon: p.Weapon})
		}
		if p.Input.Drop && !p.Previous.Drop {
			index := weaponIndex(p, p.Weapon)
			if index >= 0 && p.Weapon != "knife" {
				dropWeapon(state, p, index)
				p.Weapon = ""
				p.NextAttack = state.Frame + 50
			}
		}
		if p.Input.Use && !p.Previous.Use {
			PickupDrop(state, seat)
		}
		SyncWeapon(p)
		weapon, ok := WeaponByID(p.Weapon)
		index := weaponIndex(p, p.Weapon)
		if !ok || index < 0 || weapon.ID == "c4" {
			continue
		}
		item := &p.Inventory[index]
		if weapon.Slot == 4 {
			if live {
				stepGrenadeWeapon(state, p, item)
			} else {
				CancelGrenade(p)
			}
			SyncWeapon(p)
			continue
		}
		if item.ResumeZoom > 0 && state.Frame >= p.NextAttack && item.ReloadUntil == 0 {
			item.Zoom, item.ResumeZoom = item.ResumeZoom, 0
		}
		if item.ReloadUntil > 0 {
			if live && weapon.Pellets > 1 && p.Input.Fire && item.Ammo > 0 {
				item.ReloadUntil, p.NextAttack = 0, state.Frame
			}
			if item.ReloadUntil > 0 && state.Frame >= item.ReloadUntil {
				amount := min(weapon.Ammo-item.Ammo, item.Reserve)
				if weapon.Pellets > 1 {
					amount = min(amount, 1)
				}
				item.Ammo += amount
				item.Reserve -= amount
				item.ReloadUntil = 0
				if weapon.Pellets > 1 && item.Ammo < weapon.Ammo && item.Reserve > 0 && (!live || !p.Input.Fire) {
					item.ReloadUntil = state.Frame + weapon.Reload
					p.NextAttack = item.ReloadUntil
				}
				syncSharedAmmo(p, weapon.AmmoType, item.Reserve)
				Emit(state, Event{Kind: "reload_end", Seat: seat, Weapon: weapon.ID})
			}
		}
		if item.ReloadUntil > 0 {
			SyncWeapon(p)
			continue
		}
		automaticReload := item.Ammo == 0 && item.BurstLeft == 0 && !p.Input.Secondary && (!p.Input.Fire && state.Frame > p.NextAttack || live && weapon.Pellets > 1 && p.WaterLevel < 3 && p.Input.Fire && state.Frame >= p.NextAttack)
		manualReload := p.Input.Reload && !p.Previous.Reload && state.Frame >= p.NextAttack
		if (manualReload || automaticReload) && weapon.Slot <= 2 && item.Ammo < weapon.Ammo && item.Reserve > 0 {
			duration := weapon.Reload
			if weapon.ID == "m3" {
				duration = 100
			}
			if weapon.ID == "xm1014" {
				duration = 85
			}
			item.ReloadUntil, item.Zoom, item.ResumeZoom = state.Frame+duration, 0, 0
			resetWeaponAccuracy(item, true)
			p.NextAttack = item.ReloadUntil
			Emit(state, Event{Kind: "reload", Seat: seat, Weapon: weapon.ID})
			continue
		}
		if p.Input.Secondary && !p.Previous.Secondary && state.Frame >= p.NextAttack && weapon.ID != "knife" {
			mode := secondaryMode(item)
			if mode {
				p.NextAttack = state.Frame + 30
				if weapon.ID == "m4a1" || weapon.ID == "usp" {
					p.NextAttack = state.Frame + 200
					if weapon.ID == "usp" {
						p.NextAttack = state.Frame + 300
					}
				}
				Emit(state, Event{Kind: "mode", Seat: seat, Weapon: weapon.ID})
			}
		}
		if !live {
			item.BurstLeft = 0
			SyncWeapon(p)
			continue
		}
		if item.BurstLeft > 0 && state.Frame >= item.BurstAt {
			if item.Ammo > 0 {
				shots = append(shots, makeCombatShot(state, p, item, weapon, true))
			}
			item.BurstLeft--
			item.BurstAt += 10
		}
		primary := p.Input.Fire && (weapon.Slot != 2 || item.Burst || !p.Previous.Fire)
		secondaryKnife := weapon.ID == "knife" && p.Input.Secondary && !p.Previous.Secondary
		if (primary || secondaryKnife) && state.Frame >= p.NextAttack && item.BurstLeft == 0 {
			if weapon.ID == "knife" {
				shot := combatShot{Seat: seat, Life: p.Life, Weapon: weapon, Start: combatEye(p), Yaw: p.Yaw, Pitch: p.Pitch, ViewFrame: state.Frame, Secondary: secondaryKnife, Damage: 15}
				item.Shots++
				item.LastShot = state.Frame
				shots = append(shots, shot)
				p.NextAttack = state.Frame + 35
				if secondaryKnife {
					p.NextAttack = state.Frame + 100
				}
				Emit(state, Event{Kind: "shot", Seat: seat, Weapon: weapon.ID, Value: map[bool]int{true: 1, false: 0}[secondaryKnife], X: shot.Start.X, Y: shot.Start.Y, Z: shot.Start.Z, Life: p.Life})
			} else if item.Ammo > 0 {
				shots = append(shots, makeCombatShot(state, p, item, weapon, false))
				if item.Burst {
					item.BurstLeft, item.BurstAt = min(2, item.Ammo), state.Frame+5
					if weapon.ID == "glock18" {
						item.BurstAt += 5
					}
				}
				cycle := cycleMicros(weapon.ID, item.Burst, item.Zoom)
				now := state.Frame * 10000
				if item.NextShotUS <= 0 || item.NextShotUS < now-cycle {
					item.NextShotUS = now
				}
				item.NextShotUS += cycle
				p.NextAttack = (item.NextShotUS + 9999) / 10000
			} else {
				p.NextAttack = state.Frame + 20
				Emit(state, Event{Kind: "empty", Seat: seat, Weapon: weapon.ID})
			}
		}
		if !p.Input.Fire && item.BurstLeft == 0 && state.Frame-item.LastShot > 40 {
			item.Shots = max(0, item.Shots-1)
		}
		SyncWeapon(p)
	}
	for _, shot := range shots {
		if shot.Weapon.ID == "knife" {
			resolveKnife(state, shot)
		} else {
			resolveGunshot(state, shot)
		}
	}
}

func secondaryMode(item *WeaponState) bool {
	switch item.ID {
	case "usp", "m4a1":
		item.Silenced = !item.Silenced
	case "glock18", "famas":
		item.Burst = !item.Burst
	case "aug", "sg552":
		if item.Zoom == 0 {
			item.Zoom = 55
		} else {
			item.Zoom = 0
		}
	case "awp", "scout", "g3sg1", "sg550":
		if item.Zoom == 0 {
			item.Zoom = 40
		} else if item.Zoom == 40 {
			item.Zoom = 15
			if item.ID == "awp" {
				item.Zoom = 10
			}
		} else {
			item.Zoom = 0
		}
	default:
		return false
	}
	return true
}

func syncSharedAmmo(p *Player, ammoType string, reserve int) {
	for i := range p.Inventory {
		weapon, ok := WeaponByID(p.Inventory[i].ID)
		if ok && weapon.AmmoType == ammoType {
			p.Inventory[i].Reserve = reserve
		}
	}
}

func makeCombatShot(state *State, p *Player, item *WeaponState, weapon Weapon, secondary bool) combatShot {
	spread := item.BurstSpread
	if !secondary {
		spread = spreadFor(p, item, weapon, state.Frame)
		item.BurstSpread = spread
	} else if weapon.ID == "glock18" {
		spread = 500
	}
	shot := combatShot{Seat: p.Seat, Life: p.Life, Weapon: weapon, Start: combatEye(p), Yaw: (p.Yaw + p.RecoilYaw*2 + 65536) % 65536, Pitch: max(-16000, min(16000, p.Pitch+p.RecoilPitch*2)), Spread: spread, ViewFrame: p.Input.ViewFrame, Secondary: secondary, Silenced: item.Silenced, Burst: item.Burst}
	if (weapon.ID == "awp" || weapon.ID == "scout") && item.Zoom > 0 {
		item.ResumeZoom, item.Zoom = item.Zoom, 0
	}
	item.Ammo--
	if !secondary {
		item.LastShot = state.Frame
		item.Shots++
		applyRecoil(state, p, item, weapon)
	}
	value := 0
	if item.Silenced {
		value = 1
	}
	if weapon.ID == "elite" {
		value = item.Ammo % 2
	}
	Emit(state, Event{Kind: "shot", Seat: p.Seat, Weapon: weapon.ID, X: shot.Start.X, Y: shot.Start.Y, Z: shot.Start.Z, Value: value, Life: p.Life})
	return shot
}

func combatRandom(state *State, maximum int) int {
	state.Seed ^= state.Seed << 13
	state.Seed ^= state.Seed >> 7
	state.Seed ^= state.Seed << 17
	if maximum <= 1 {
		return 0
	}
	return int(state.Seed % uint64(maximum))
}

func spreadFor(p *Player, item *WeaponState, weapon Weapon, frame int64) int {
	shots := item.Shots
	if !item.AccuracyInitialized {
		resetWeaponAccuracy(item, false)
	}
	accuracy := item.Accuracy
	air := !p.Grounded
	speedSquared := int64(p.VX)*int64(p.VX) + int64(p.VY)*int64(p.VY)
	moving := speedSquared > int64(140*Unit)*int64(140*Unit)
	spread := weapon.Spread
	if weapon.Slot == 2 {
		moving = speedSquared > 0
		base, movingBase, crouchBase, airBase := 1000, 1650, 750, 10000
		switch weapon.ID {
		case "usp":
			base, movingBase, crouchBase, airBase = 1000, 2250, 800, 12000
			if item.Silenced {
				base, movingBase, crouchBase, airBase = 1500, 2500, 1250, 13000
			}
		case "deagle":
			base, movingBase, crouchBase, airBase = 1300, 2500, 1150, 15000
		case "p228", "fiveseven":
			base, movingBase, crouchBase, airBase = 1500, 2550, 750, 15000
		case "elite":
			base, movingBase, crouchBase, airBase = 1000, 1750, 800, 13000
		}
		if item.Burst {
			base, movingBase, crouchBase, airBase = 3000, 1850, 950, 12000
		}
		coefficient := base
		if air {
			coefficient = airBase
		} else if moving {
			coefficient = movingBase
		} else if p.Crouch {
			coefficient = crouchBase
		}
		spread = coefficient * (10000 - accuracy) / 10000
		elapsed := int(frame-item.LastShot) * 100
		threshold, recovery, minimum, maximum := 3250, 2750, 6000, 9000
		switch weapon.ID {
		case "usp":
			threshold, minimum, maximum = 3000, 6000, 9200
		case "deagle":
			threshold, recovery, minimum = 4000, 3500, 5500
		case "p228":
			recovery = 3000
		case "fiveseven":
			threshold, recovery, minimum, maximum = 2750, 2500, 7250, 9200
		case "elite":
			minimum, maximum = 5500, 8800
		}
		if item.LastShot > 0 {
			accuracy -= (threshold - elapsed) * recovery / 10000
		}
		item.Accuracy = max(minimum, min(maximum, accuracy))
	} else if weapon.ID == "awp" || weapon.ID == "scout" || weapon.ID == "g3sg1" || weapon.ID == "sg550" {
		if weapon.ID == "awp" {
			spread = 10
			if p.Crouch {
				spread = 0
			}
			if speedSquared > int64(10*Unit)*int64(10*Unit) {
				spread = 1000
			}
			if moving {
				spread = 2500
			}
			if air {
				spread = 8500
			}
			if item.Zoom == 0 {
				spread += 800
			}
		}
		if weapon.ID == "scout" {
			spread = 70
			if p.Crouch {
				spread = 0
			}
			if speedSquared > int64(170*Unit)*int64(170*Unit) {
				spread = 750
			}
			if air {
				spread = 2000
			}
			if item.Zoom == 0 {
				spread += 250
			}
		}
		if weapon.ID == "g3sg1" {
			accuracy = 9800
			if item.LastShot > 0 {
				accuracy = min(9800, 5500+int(frame-item.LastShot)*30)
			}
			coefficient := 550
			if p.Crouch {
				coefficient = 350
			}
			if speedSquared > 0 {
				coefficient = 1500
			}
			if air {
				coefficient = 4500
			}
			if item.Zoom == 0 {
				coefficient += 250
			}
			spread = coefficient * (10000 - accuracy) / 10000
			item.Accuracy = accuracy
		}
		if weapon.ID == "sg550" {
			spread = 500 * (10000 - accuracy) / 10000
			if p.Crouch {
				spread = 400 * (10000 - accuracy) / 10000
			}
			if speedSquared > 0 {
				spread = 1500
			}
			if air {
				spread = 4500 * (10000 - accuracy) / 10000
			}
			if item.Zoom == 0 {
				spread += 250
			}
			if item.LastShot > 0 {
				item.Accuracy = min(9800, 6500+int(frame-item.LastShot)*35)
			} else {
				item.Accuracy = accuracy
			}
		}
	} else if weapon.Pellets == 1 {
		ground, move, airborne, offset := weapon.Spread, 700, 4000, 350
		switch weapon.ID {
		case "ak47":
			ground, move, airborne, offset = 275, 700, 4000, 400
		case "galil":
			ground, move, airborne, offset = 375, 700, 3000, 400
		case "famas":
			ground, move, airborne, offset = 200, 700, 3000, 300
		case "sg552":
			ground, move, airborne, offset = 200, 750, 4500, 350
		case "m249":
			ground, move, airborne, offset = 300, 950, 5000, 450
		case "mp5navy":
			ground, move, airborne, offset = 400, 400, 2000, 0
		case "mac10":
			ground, move, airborne, offset = 300, 300, 3750, 0
		case "tmp":
			ground, move, airborne, offset = 300, 300, 2500, 0
		case "ump45":
			ground, move, airborne, offset = 400, 400, 2400, 0
		case "p90":
			ground, move, airborne, offset = 450, 1150, 3000, 0
			moving = speedSquared > int64(170*Unit)*int64(170*Unit)
		}
		if item.Silenced && weapon.ID == "m4a1" {
			ground = 250
		}
		coefficient := ground
		spread = coefficient * accuracy / 10000
		if air {
			spread = offset + airborne*accuracy/10000
		} else if moving {
			spread = offset + move*accuracy/10000
		}
		n := shots + 1
		divisor, base, limit := 220, 3000, 10000
		power := n * n * n
		switch weapon.ID {
		case "ak47", "galil":
			divisor, base, limit = 200, 3500, 12500
		case "aug", "famas":
			divisor = 215
		case "m249":
			divisor, base, limit = 175, 4000, 9000
		case "mp5navy":
			power, divisor, base, limit = n*n*10, 2201, 4500, 7500
		case "tmp":
			divisor, base, limit = 200, 5500, 14000
		case "mac10":
			divisor, base, limit = 200, 6000, 16500
		case "ump45":
			power, divisor, base = n*n, 210, 5000
		case "p90":
			power, divisor, base = n*n, 175, 4500
		}
		if weapon.ID == "mp5navy" {
			item.Accuracy = min(limit, base+power*10000/divisor)
		} else {
			item.Accuracy = min(limit, base+(power/divisor)*10000)
		}
		if weapon.ID == "famas" && !item.Burst {
			spread += 100
		}
	}
	return max(0, spread)
}

func applyRecoil(state *State, p *Player, item *WeaponState, weapon Weapon) {
	if weapon.Slot == 2 || weapon.ID == "awp" || weapon.ID == "scout" {
		if weapon.ID != "glock18" {
			p.RecoilPitch = max(-16000, p.RecoilPitch-2*65536/360)
		}
		return
	}
	if weapon.ID == "g3sg1" || weapon.ID == "sg550" {
		amplitude := 10000
		if weapon.ID == "sg550" {
			amplitude = 5000
		}
		kick := 7500 + combatRandom(state, amplitude+1)
		p.RecoilPitch = max(-16000, p.RecoilPitch-kick*65536/3600000-p.RecoilPitch/4)
		p.RecoilYaw += (combatRandom(state, 15001) - 7500) * 65536 / 3600000
		return
	}
	if weapon.Pellets > 1 {
		low, high := 8, 11
		if p.Grounded {
			low, high = 4, 6
		}
		if weapon.ID == "xm1014" {
			low, high = 7, 10
			if p.Grounded {
				low, high = 3, 5
			}
		}
		p.RecoilPitch = max(-16000, p.RecoilPitch-(low+combatRandom(state, high-low+1))*65536/360)
		return
	}
	profile := recoilProfile(p, weapon.ID)
	up, lateral := profile[0], profile[1]
	if item.Shots > 1 {
		up += item.Shots * profile[2]
		lateral += item.Shots * profile[3]
	}
	pitchLimit, yawLimit := profile[4]*65536/3600000, profile[5]*65536/3600000
	p.RecoilPitch = max(-pitchLimit, p.RecoilPitch-up*65536/3600000)
	delta := lateral * 65536 / 3600000
	if item.KickDirection == 0 {
		delta = -delta
	}
	p.RecoilYaw = max(-yawLimit, min(yawLimit, p.RecoilYaw+delta))
	if combatRandom(state, profile[6]+1) == 0 {
		item.KickDirection = 1 - item.KickDirection
	}
}

func decayRecoil(p *Player) {
	length := int(Sqrt(int64(p.RecoilPitch)*int64(p.RecoilPitch) + int64(p.RecoilYaw)*int64(p.RecoilYaw)))
	if length == 0 {
		return
	}
	remaining := max(0, length-10*65536/(360*FPS)-length/(2*FPS))
	p.RecoilPitch = p.RecoilPitch * remaining / length
	p.RecoilYaw = p.RecoilYaw * remaining / length
}

func combatAddRay(start, direction Vec3, distance int) Vec3 {
	return Vec3{start.X + int(int64(direction.X)*int64(distance)/NormalUnit), start.Y + int(int64(direction.Y)*int64(distance)/NormalUnit), start.Z + int(int64(direction.Z)*int64(distance)/NormalUnit)}
}

func rayBoxFraction(start, end, minPoint, maxPoint Vec3) (int, bool) {
	entry, exit := int64(0), int64(NormalUnit)
	a, b, c, d := [3]int{start.X, start.Y, start.Z}, [3]int{end.X, end.Y, end.Z}, [3]int{minPoint.X, minPoint.Y, minPoint.Z}, [3]int{maxPoint.X, maxPoint.Y, maxPoint.Z}
	for i := range 3 {
		delta := int64(b[i] - a[i])
		if delta == 0 {
			if a[i] < c[i] || a[i] > d[i] {
				return 0, false
			}
			continue
		}
		one := int64(c[i]-a[i]) * NormalUnit / delta
		two := int64(d[i]-a[i]) * NormalUnit / delta
		if one > two {
			one, two = two, one
		}
		entry = max(entry, one)
		exit = min(exit, two)
		if entry > exit {
			return 0, false
		}
	}
	return int(entry), entry >= 0 && entry <= NormalUnit
}

func historicalTargets(state *State, frame int64) []HistoryPlayer {
	result := make([]HistoryPlayer, len(state.Players))
	for i, p := range state.Players {
		result[i] = HistoryPlayer{Seat: i, X: p.X, Y: p.Y, Z: p.Z, Yaw: p.Yaw, Weapon: p.Weapon, Alive: p.Alive, Life: p.Life, Crouch: p.Crouch}
	}
	if frame >= state.Frame || frame < state.Frame-20 {
		return result
	}
	for i := len(state.History) - 1; i >= 0; i-- {
		history := state.History[i]
		if history.Frame <= frame {
			for _, p := range history.Players {
				if p.Seat >= 0 && p.Seat < len(result) {
					if p.Life == state.Players[p.Seat].Life {
						result[p.Seat] = p
					} else {
						result[p.Seat].Alive = false
					}
				}
			}
			break
		}
	}
	return result
}

func nearestPlayerHit(state *State, shot combatShot, start, end Vec3, targets []HistoryPlayer, ignored map[int]bool) (int, int, int) {
	bestSeat, bestFraction, bestGroup := -1, NormalUnit+1, 0
	for _, target := range targets {
		if target.Seat == shot.Seat || ignored[target.Seat] || !target.Alive || target.Seat < 0 || target.Seat >= len(state.Players) || !state.Players[target.Seat].Alive || state.Players[target.Seat].Life != target.Life {
			continue
		}
		fraction, group, hit := modelHitFraction(target, state.Players[target.Seat].Character, start, end)
		if hit && fraction < bestFraction {
			bestSeat, bestFraction, bestGroup = target.Seat, fraction, group
		}
	}
	return bestSeat, bestFraction, bestGroup
}

func modelHitFraction(target HistoryPlayer, character string, start, end Vec3) (int, int, bool) {
	loadCombatModels()
	model, ok := combatModels[character]
	if !ok {
		model = combatModels["urban"]
	}
	pose := model.Poses["stand"]
	key := weaponExtension(target.Weapon) + "Stand"
	if target.Crouch {
		pose = model.Poses["crouch"]
		key = weaponExtension(target.Weapon) + "Crouch"
	}
	if selected := model.Poses[key]; len(selected) > 0 {
		pose = selected
	}
	direction := Direction(target.Yaw, 0)
	local := func(point Vec3) Vec3 {
		x, y := point.X-target.X, point.Y-target.Y
		return Vec3{int((int64(direction.X)*int64(x) + int64(direction.Y)*int64(y)) / NormalUnit), int((-int64(direction.Y)*int64(x) + int64(direction.X)*int64(y)) / NormalUnit), point.Z - target.Z}
	}
	a, b := local(start), local(end)
	best, group := NormalUnit+1, 0
	for _, box := range model.Hitboxes {
		if box.Group == 8 || box.Bone < 0 || box.Bone >= len(pose) {
			continue
		}
		bone := pose[box.Bone]
		inverse := func(point Vec3) Vec3 {
			x, y, z := int64(point.X-bone.X), int64(point.Y-bone.Y), int64(point.Z-bone.Z)
			return Vec3{int((int64(bone.XX)*x + int64(bone.YX)*y + int64(bone.ZX)*z) / NormalUnit), int((int64(bone.XY)*x + int64(bone.YY)*y + int64(bone.ZY)*z) / NormalUnit), int((int64(bone.XZ)*x + int64(bone.YZ)*y + int64(bone.ZZ)*z) / NormalUnit)}
		}
		fraction, hit := rayBoxFraction(inverse(a), inverse(b), box.Min, box.Max)
		if hit && fraction < best {
			best, group = fraction, box.Group
		}
	}
	return best, group, best <= NormalUnit
}

func resolveGunshot(state *State, shot combatShot) {
	targets := historicalTargets(state, shot.ViewFrame)
	pendingSeat, pendingHostage, pendingBrush, pendingGroup, pendingDamage := -1, -1, -1, 0, int64(0)
	flush := func() {
		if pendingSeat >= 0 {
			takeDamagePlayerUnitsFrom(state, shot.Seat, pendingSeat, pendingDamage, shot.Weapon.ArmorRatio, pendingGroup, shot.Weapon.ID, shot.Start, true)
		} else if pendingHostage >= 0 {
			damageHostageMultiUnits(state, shot.Seat, pendingHostage, pendingDamage, pendingGroup)
		} else if pendingBrush >= 0 {
			damageBreakableUnits(state, shot.Seat, pendingBrush, pendingDamage, shot.Weapon.ID)
		}
		pendingSeat, pendingHostage, pendingBrush, pendingDamage = -1, -1, -1, 0
	}
	for pellet := 0; pellet < max(1, shot.Weapon.Pellets); pellet++ {
		spreadX := combatRandom(state, 10001) + combatRandom(state, 10001) - 10000
		spreadY := combatRandom(state, 10001) + combatRandom(state, 10001) - 10000
		direction := Direction(shot.Yaw, shot.Pitch)
		right := Direction((shot.Yaw+16384)%65536, 0)
		up := Direction(shot.Yaw, shot.Pitch-16384)
		direction.X += int((int64(right.X)*int64(spreadX) + int64(up.X)*int64(spreadY)) * int64(shot.Spread) / 100000000)
		direction.Y += int((int64(right.Y)*int64(spreadX) + int64(up.Y)*int64(spreadY)) * int64(shot.Spread) / 100000000)
		direction.Z += int((int64(right.Z)*int64(spreadX) + int64(up.Z)*int64(spreadY)) * int64(shot.Spread) / 100000000)
		start, remaining := shot.Start, shot.Weapon.Range
		damage, modifier := shot.Weapon.Damage, shot.Weapon.RangeModifier
		if shot.Silenced && shot.Weapon.ID == "usp" {
			damage = 30
		}
		if shot.Silenced && shot.Weapon.ID == "m4a1" {
			damage, modifier = 33, 950
		}
		if shot.Burst && shot.Weapon.ID == "famas" && !shot.Secondary {
			damage = 34
		}
		if shot.Secondary && shot.Weapon.ID == "glock18" {
			damage, modifier = 18, 900
		}
		penetrations, power := max(1, shot.Weapon.Penetration), shot.Weapon.PenetrationPower
		ignored := map[int]bool{}
		for segments := 0; segments < 8 && remaining > 0 && damage > 0 && penetrations > 0; segments++ {
			end := combatAddRay(start, direction, remaining)
			wall := TraceWorld(state, start, end, -1)
			seat, fraction, group := nearestPlayerHit(state, shot, start, end, targets, ignored)
			hostage, hostageFraction, hostageGroup := nearestHostageHit(state, start, end, ignored)
			if hostage >= 0 && hostageFraction <= wall.Fraction && (seat < 0 || hostageFraction < fraction) {
				fraction, group = hostageFraction, hostageGroup
				seat = -1
			} else {
				hostage = -1
			}
			if seat < 0 || fraction > wall.Fraction {
				seat = -1
				if hostage < 0 {
					fraction = wall.Fraction
				}
			}
			if fraction >= NormalUnit {
				break
			}
			distance := int(int64(remaining) * int64(fraction) / NormalUnit)
			if shot.Weapon.Pellets > 1 {
				damage = 20 * (NormalUnit - fraction) / NormalUnit
			} else {
				damage = rangeDamage(damage, modifier, distance)
			}
			penetrations--
			if distance > penetrationDistance(shot.Weapon.AmmoType) {
				penetrations = 0
			}
			hit := combatAddRay(start, direction, distance)
			if seat >= 0 {
				if shot.Weapon.Pellets > 1 {
					if pendingSeat != seat {
						flush()
					}
					units := int64(damage) * damageUnit
					switch group {
					case 1:
						units *= 4
					case 3:
						units = units * 5 / 4
					case 6, 7:
						units = units * 3 / 4
					}
					pendingSeat, pendingGroup, pendingDamage = seat, group, pendingDamage+units
				} else {
					DamagePlayer(state, shot.Seat, seat, damage, shot.Weapon.ArmorRatio, group, shot.Weapon.ID)
				}
				ignored[seat] = true
				start = combatAddRay(hit, direction, 42*Unit)
				remaining = (remaining - distance) * 3 / 4
				damage = damage * 3 / 4
			} else if hostage >= 0 {
				if shot.Weapon.Pellets > 1 {
					if pendingHostage != hostage {
						flush()
					}
					units := hostageTraceUnits(int64(damage)*damageUnit, group)
					pendingHostage, pendingGroup, pendingDamage = hostage, group, pendingDamage+units
				} else {
					damageHostageTraceUnits(state, shot.Seat, hostage, int64(damage)*damageUnit, group)
				}
				ignored[10000+hostage] = true
				start = combatAddRay(hit, direction, 42*Unit)
				remaining = (remaining - distance) * 3 / 4
				damage = damage * 3 / 4
			} else {
				materialWeapon := shot.Weapon
				materialWeapon.PenetrationPower = power
				var damageModifier int
				power, damageModifier = materialPenetration(materialWeapon, combatMaterial(state, wall))
				if shot.Weapon.Pellets > 1 && wall.Brush >= 0 && brushReceivesDamage(state, wall.Brush) {
					if pendingBrush != wall.Brush {
						flush()
					}
					pendingBrush, pendingDamage = wall.Brush, pendingDamage+int64(damage)*damageUnit
				} else {
					damageBrush(state, shot.Seat, wall.End, damage, shot.Weapon.ID)
				}
				start = combatAddRay(hit, direction, power)
				remaining = (remaining - distance) / 2
				damage = penetrationDamage(damage, damageModifier)
			}
		}
	}
	flush()
}

func nearestHostageHit(state *State, start, end Vec3, ignored map[int]bool) (int, int, int) {
	best, fraction, group := -1, NormalUnit+1, 0
	for index, hostage := range state.Hostages {
		if hostage.Rescued || hostage.Health <= 0 || ignored[10000+index] {
			continue
		}
		yaw := 0
		if index < len(state.World.Hostages) {
			yaw = state.World.Hostages[index].Yaw
		}
		if hostage.Following >= 0 && hostage.Following < len(state.Players) {
			p := state.Players[hostage.Following]
			yaw = AngleTo(p.X-hostage.X, p.Y-hostage.Y)
		}
		target := HistoryPlayer{X: hostage.X, Y: hostage.Y, Z: hostage.Z, Yaw: yaw, Alive: true}
		hitFraction, hitGroup, hit := modelHitFraction(target, "hostage", start, end)
		if hit && hitFraction < fraction {
			best, fraction, group = index, hitFraction, hitGroup
		}
	}
	return best, fraction, group
}

func hostageHitDamage(damage, group int) int {
	return int(hostageHitUnits(int64(damage)*damageUnit, group) / damageUnit)
}

func penetrationDistance(ammoType string) int {
	switch ammoType {
	case "9mm", "357sig":
		return 800 * Unit
	case "45acp":
		return 500 * Unit
	case "50ae":
		return 1000 * Unit
	case "762nato":
		return 5000 * Unit
	case "556nato", "556natobox":
		return 4000 * Unit
	case "338magnum":
		return 8000 * Unit
	case "57mm":
		return 2000 * Unit
	}
	return 0
}

func materialPenetration(weapon Weapon, material string) (int, int) {
	power, damage := weapon.PenetrationPower/Unit, 500
	switch material {
	case "M":
		power, damage = power*15/100, 200
	case "C":
		power = power / 4
	case "G":
		power, damage = power/2, 400
	case "V":
		power, damage = power/2, 450
	case "T":
		power, damage = power*65/100, 300
	case "P":
		power, damage = power*40/100, 450
	case "W":
		damage = 600
	}
	return power * Unit, damage
}

func combatMaterial(state *State, trace TraceResult) string {
	point := trace.End
	brushID := -1
	if trace.Brush >= 0 && trace.Brush < len(state.World.Brushes) {
		brush := state.World.Brushes[trace.Brush]
		brushID = brush.ID
		progress := 0
		direction := 1
		for _, dynamic := range state.Brushes {
			if dynamic.ID == brush.ID {
				progress = dynamic.Progress
				if dynamic.Direction < 0 {
					direction = -1
				}
				break
			}
		}
		origin := Add(brush.Origin, Scale(brush.Move, progress, 1000))
		point = Sub(point, origin)
		if brush.Kind == "func_door_rotating" {
			angle := brush.Angle
			if angle == 0 {
				angle = 16384
			}
			angle = angle * progress / 1000 * direction
			if brush.Flags&2 != 0 {
				angle = -angle
			}
			point = rotateZ(point, -angle)
		}
	}
	candidates := state.World.SurfaceIndex[trace.Plane]
	if len(candidates) == 0 {
		for index, face := range state.World.Surfaces {
			if face.Plane == trace.Plane {
				candidates = append(candidates, index)
			}
		}
	}
	for _, index := range candidates {
		if index < 0 || index >= len(state.World.Surfaces) {
			continue
		}
		face := state.World.Surfaces[index]
		if face.Brush != brushID || len(face.Vertices) < 3 || face.Plane < 0 || face.Plane >= len(state.World.Planes) {
			continue
		}
		plane := state.World.Planes[face.Plane]
		if Abs(planeDistance(plane, point)) > Unit {
			continue
		}
		axis := 0
		if Abs(plane.Y) > Abs(plane.X) {
			axis = 1
		}
		if Abs(plane.Z) > max(Abs(plane.X), Abs(plane.Y)) {
			axis = 2
		}
		project := func(p Vec3) (int, int) {
			if axis == 0 {
				return p.Y, p.Z
			}
			if axis == 1 {
				return p.X, p.Z
			}
			return p.X, p.Y
		}
		px, py := project(point)
		positive, negative := false, false
		for i, a := range face.Vertices {
			b := face.Vertices[(i+1)%len(face.Vertices)]
			ax, ay := project(a)
			bx, by := project(b)
			cross := int64(bx-ax)*int64(py-ay) - int64(by-ay)*int64(px-ax)
			epsilon := int64(max(Abs(bx-ax), Abs(by-ay))) * int64(Unit/16)
			positive = positive || cross > epsilon
			negative = negative || cross < -epsilon
			if positive && negative {
				break
			}
		}
		if !positive || !negative {
			if face.Material != "" {
				return face.Material
			}
			return "C"
		}
	}
	return "C"
}

func rangeDamage(damage, modifier, distance int) int {
	return exponentialRangeDamage(damage, modifier, distance)
}

func damageBrush(state *State, attacker int, point Vec3, damage int, weapon string) bool {
	for index, brush := range state.World.Brushes {
		if !brushReceivesDamage(state, index) {
			continue
		}
		bounds := brush.Bounds
		bounds.Min.X -= Unit
		bounds.Min.Y -= Unit
		bounds.Min.Z -= Unit
		bounds.Max.X += Unit
		bounds.Max.Y += Unit
		bounds.Max.Z += Unit
		if !combatInside(point, bounds) {
			continue
		}
		return damageBreakableUnits(state, attacker, index, int64(damage)*damageUnit, weapon)
	}
	return false
}

func resolveKnife(state *State, shot combatShot) {
	rangeUnits, damage := 48*Unit, max(15, shot.Damage)
	if shot.Secondary {
		rangeUnits, damage = 32*Unit, 65
	}
	end := combatAddRay(shot.Start, Direction(shot.Yaw, shot.Pitch), rangeUnits)
	wall := TraceWorld(state, shot.Start, end, -1)
	targets := historicalTargets(state, state.Frame)
	seat, fraction, group := nearestPlayerHit(state, shot, shot.Start, end, targets, map[int]bool{})
	if fraction >= NormalUnit {
		seat, fraction = -1, NormalUnit+1
	}
	hostageIndex, hostageFraction, hostageGroup := nearestHostageHit(state, shot.Start, end, map[int]bool{})
	if hostageIndex >= 0 && hostageFraction < NormalUnit && (seat < 0 || hostageFraction < fraction) {
		seat, fraction, group = -1, hostageFraction, hostageGroup
	} else {
		hostageIndex = -1
	}
	if seat < 0 && hostageIndex < 0 && wall.Fraction >= NormalUnit {
		wall = TraceWorld(state, shot.Start, end, 1)
		for _, target := range targets {
			if target.Seat == shot.Seat || !target.Alive || !state.Players[target.Seat].Alive {
				continue
			}
			bottom, top := -36*Unit, 36*Unit
			if target.Crouch {
				bottom, top = -18*Unit, 18*Unit
			}
			minPoint := Vec3{target.X - 32*Unit, target.Y - 32*Unit, target.Z + bottom - 18*Unit}
			maxPoint := Vec3{target.X + 32*Unit, target.Y + 32*Unit, target.Z + top + 18*Unit}
			if hit, valid := rayBoxFraction(shot.Start, end, minPoint, maxPoint); valid && hit < fraction && hit < NormalUnit {
				seat, hostageIndex, fraction, group = target.Seat, -1, hit, 0
			}
		}
		for index, hostage := range state.Hostages {
			if hostage.Rescued || hostage.Health <= 0 {
				continue
			}
			minPoint := Vec3{hostage.X - 26*Unit, hostage.Y - 26*Unit, hostage.Z - 18*Unit}
			maxPoint := Vec3{hostage.X + 26*Unit, hostage.Y + 26*Unit, hostage.Z + 80*Unit}
			if hit, valid := rayBoxFraction(shot.Start, end, minPoint, maxPoint); valid && hit < fraction && hit < NormalUnit {
				seat, hostageIndex, fraction, group = -1, index, hit, 0
			}
		}
	}
	value := 0
	if shot.Secondary {
		value = 1
	}
	if seat < 0 && hostageIndex < 0 || fraction >= NormalUnit || fraction > wall.Fraction {
		kind, point := "knife_air", shot.Start
		if wall.Fraction < NormalUnit {
			kind, point = "knife_wall", wall.End
			state.Players[shot.Seat].NextAttack = state.Frame + 40
			if shot.Secondary {
				state.Players[shot.Seat].NextAttack = state.Frame + 110
			}
			damageBrush(state, shot.Seat, point, damage, "knife")
		}
		Emit(state, Event{Kind: kind, Seat: shot.Seat, Target: -1, Weapon: "knife", Value: value, X: point.X, Y: point.Y, Z: point.Z, Life: shot.Life})
		return
	}
	point := combatAddRay(shot.Start, Direction(shot.Yaw, shot.Pitch), int(int64(rangeUnits)*int64(fraction)/NormalUnit))
	if shot.Secondary {
		state.Players[shot.Seat].NextAttack = state.Frame + 110
	} else {
		state.Players[shot.Seat].NextAttack = state.Frame + 40
	}
	if hostageIndex >= 0 {
		Emit(state, Event{Kind: "knife_hit", Seat: shot.Seat, Target: -1, Weapon: "knife", Value: value, Hitgroup: group, X: point.X, Y: point.Y, Z: point.Z, Life: shot.Life})
		damageHostageTraceUnits(state, shot.Seat, hostageIndex, int64(damage)*damageUnit, group)
		return
	}
	target := state.Players[seat]
	if shot.Secondary {
		difference := ((target.Yaw - shot.Yaw + 32768) & 65535) - 32768
		if difference >= -6711 && difference <= 6711 {
			damage *= 3
		}
	}
	Emit(state, Event{Kind: "knife_hit", Seat: shot.Seat, Target: seat, Weapon: "knife", Value: value, Hitgroup: group, X: point.X, Y: point.Y, Z: point.Z, Life: shot.Life})
	DamagePlayer(state, shot.Seat, seat, damage, 850, group, "knife")
}

const damageUnit int64 = 800000

func DamagePlayer(state *State, attacker, target, damage, armorRatio, hitgroup int, weapon string) {
	source, hasSource := Vec3{}, false
	if weapon == "c4" {
		source, hasSource = bombPosition(state.Bomb), true
	} else if attacker >= 0 && attacker < len(state.Players) {
		source, hasSource = Position(state.Players[attacker]), true
	}
	damagePlayerFrom(state, attacker, target, damage, armorRatio, hitgroup, weapon, source, hasSource)
}

func damagePlayerFrom(state *State, attacker, target, damage, armorRatio, hitgroup int, weapon string, source Vec3, hasSource bool) {
	damagePlayerUnitsFrom(state, attacker, target, int64(damage)*damageUnit, armorRatio, hitgroup, weapon, source, hasSource)
}

func damagePlayerUnitsFrom(state *State, attacker, target int, damageUnits int64, armorRatio, hitgroup int, weapon string, source Vec3, hasSource bool) {
	switch hitgroup {
	case 1:
		damageUnits *= 4
	case 3:
		damageUnits = damageUnits * 5 / 4
	case 6, 7:
		damageUnits = damageUnits * 3 / 4
	}
	takeDamagePlayerUnitsFrom(state, attacker, target, damageUnits, armorRatio, hitgroup, weapon, source, hasSource)
}

func takeDamagePlayerUnitsFrom(state *State, attacker, target int, damageUnits int64, armorRatio, hitgroup int, weapon string, source Vec3, hasSource bool) {
	if target < 0 || target >= len(state.Players) || damageUnits <= 0 {
		return
	}
	p := &state.Players[target]
	if !p.Alive {
		return
	}
	teamAttack := attacker >= 0 && attacker < len(state.Players) && attacker != target && state.Players[attacker].Team == p.Team
	if teamAttack && !state.Options.FriendlyFire {
		return
	}
	fx := impactEffect(*p, hitgroup, weapon)

	if teamAttack && weapon != "hegrenade" {
		damageUnits = damageUnits * 35 / 100
	}
	armorUnits := int64(p.Armor)*damageUnit + int64(p.ArmorRemainder)
	armored := armorUnits > 0 && hitgroup != 6 && hitgroup != 7 && (hitgroup != 1 || p.Helmet)
	if armored {
		healthDamage := damageUnits * int64(max(0, min(1000, armorRatio))) / 1000
		armorDamage := (damageUnits - healthDamage) / 2
		if armorDamage > armorUnits {
			absorbed := armorUnits * 2
			if weapon == "hegrenade" || weapon == "c4" {
				absorbed = armorUnits
			}
			damageUnits -= absorbed
			p.Armor, p.ArmorRemainder = 0, 0
		} else {
			armorUnits -= armorDamage
			p.Armor, p.ArmorRemainder = int(armorUnits/damageUnit), int(armorUnits%damageUnit)
			damageUnits = healthDamage
		}
		if p.Armor == 0 && p.ArmorRemainder == 0 {
			p.Helmet = false
		}
	}
	damage := int(damageUnits / damageUnit)
	p.Health = max(0, p.Health-damage)
	Emit(state, Event{Kind: "hit", Seat: attacker, Target: target, Weapon: weapon, Value: damage, Hitgroup: hitgroup, X: p.X, Y: p.Y, Z: p.Z, Life: p.Life, FX: fx})
	if p.Health > 0 {
		return
	}
	p.Alive = false
	p.DeathAt, p.DeathHitgroup = state.Frame, hitgroup
	p.DeathVX, p.DeathVY, p.DeathVZ = p.VX, p.VY, p.VZ
	applyDeathImpulse(state, p, attacker, damage, hitgroup, weapon, source, hasSource)
	p.Flashlight = false
	p.Deaths++
	p.Input = Input{Life: p.Life, Yaw: p.Yaw, Pitch: p.Pitch}
	p.Pending = nil
	p.Action = ""
	p.ActionSince = 0
	if attacker >= 0 && attacker < len(state.Players) {
		killer := &state.Players[attacker]
		if attacker == target {
			killer.Kills--
			killer.Money = max(0, killer.Money-300)
		} else if teamAttack {
			killer.Kills--
			killer.Money = max(0, killer.Money-3300)
		} else {
			killer.Kills++
			killer.Money = min(16000, killer.Money+300)
		}
	}
	DropWeapons(state, target)
	Emit(state, Event{Kind: "kill", Seat: attacker, Target: target, Weapon: weapon, Value: hitgroup, Hitgroup: hitgroup, Life: p.Life})
}

func impactEffect(player Player, hitgroup int, weaponID string) int {
	weapon, known := WeaponByID(weaponID)
	if !known || weapon.Slot > 2 && weaponID != "knife" {
		return 0
	}
	if hitgroup == 1 && player.Helmet {
		return 2
	}
	if hitgroup >= 2 && hitgroup <= 5 && (player.Armor > 0 || player.ArmorRemainder > 0 || player.Helmet) {
		return 0
	}
	return 1
}
