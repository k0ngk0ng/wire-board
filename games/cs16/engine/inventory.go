package engine

import "errors"

func InitInventory(player *Player) {
	pistol, ammo, reserve := "glock18", 20, 40
	if player.Team == "ct" {
		pistol, ammo, reserve = "usp", 12, 24
	}
	player.Inventory = []WeaponState{{ID: "knife", Ammo: 1}, {ID: pistol, Ammo: ammo, Reserve: reserve}}
	player.Weapon = pistol
	player.NextAttack = 0
	SyncWeapon(player)
}

func SyncWeapon(player *Player) {
	index := weaponIndex(player, player.Weapon)
	if index < 0 {
		bestSlot := 99
		for _, item := range player.Inventory {
			weapon, ok := WeaponByID(item.ID)
			if ok && weapon.Slot < bestSlot && (weapon.Slot <= 3 || item.Ammo > 0) {
				player.Weapon = item.ID
				bestSlot = weapon.Slot
			}
		}
		index = weaponIndex(player, player.Weapon)
	}
	player.Ammo, player.Reserve = 0, 0
	if index >= 0 {
		player.Ammo, player.Reserve = player.Inventory[index].Ammo, player.Inventory[index].Reserve
	}
}

func CanBuy(state *State, seat int) bool {
	if seat < 0 || seat >= len(state.Players) || state.Finished || state.Phase != "freeze" && state.Phase != "live" || state.Frame > state.BuyEndsAt {
		return false
	}
	return InBuyZone(state, seat)
}

func InBuyZone(state *State, seat int) bool {
	if seat < 0 || seat >= len(state.Players) {
		return false
	}
	p := &state.Players[seat]
	if !p.Alive {
		return false
	}
	for _, zone := range state.World.BuyZones {
		if (zone.Team == "" || zone.Team == p.Team) && combatInside(combatPosition(p), zone) {
			return true
		}
	}
	if len(state.World.BuyZones) == 0 {
		spawns := state.World.SpawnsT
		if p.Team == "ct" {
			spawns = state.World.SpawnsCT
		}
		for _, spawn := range spawns {
			if Distance(combatPosition(p), Vec3{spawn.X, spawn.Y, spawn.Z}) <= 200*Unit {
				return true
			}
		}
	}
	return false
}

func Buy(state *State, seat int, item string) error {
	if !CanBuy(state, seat) {
		return errors.New("当前不在购买区域或购买时间内")
	}
	p := &state.Players[seat]
	helmetOwned := p.Helmet && (p.Armor > 0 || p.ArmorRemainder > 0)
	price := 0
	switch item {
	case "vest":
		if p.Armor >= 100 {
			return errors.New("护甲已满")
		}
		price = 650
	case "vesthelm":
		if p.Armor >= 100 && helmetOwned {
			return errors.New("防弹衣与头盔已齐全")
		}
		price = 1000
		if p.Armor >= 100 {
			price = 350
		} else if helmetOwned {
			price = 650
		}
	case "defuser":
		if p.Team != "ct" || p.Defuser || state.World.Mode != "bomb" {
			return errors.New("当前不能购买拆弹钳")
		}
		price = 200
	case "nvgs":
		if p.NightVisionOwned {
			return errors.New("已持有夜视仪")
		}
		price = 1250
	case "primammo", "secammo":
		slot := 1
		if item == "secammo" {
			slot = 2
		}
		index := -1
		var weapon Weapon
		for i, owned := range p.Inventory {
			w, ok := WeaponByID(owned.ID)
			if ok && w.Slot == slot {
				index, weapon = i, w
				break
			}
		}
		if index < 0 || p.Inventory[index].Reserve >= weapon.Reserve {
			return errors.New("没有可补充的弹药")
		}
		price = weapon.AmmoPrice
		if p.Money < price {
			return errors.New("资金不足")
		}
		reserve := min(weapon.Reserve, p.Inventory[index].Reserve+weapon.AmmoPack)
		for i, owned := range p.Inventory {
			w, ok := WeaponByID(owned.ID)
			if ok && w.AmmoType == weapon.AmmoType {
				p.Inventory[i].Reserve = reserve
			}
		}
		p.Money -= price
		SyncWeapon(p)
		Emit(state, Event{Kind: "buy", Seat: seat, Weapon: item, Value: price})
		return nil
	default:
		weapon, ok := WeaponByID(item)
		if !ok || weapon.Price <= 0 || weapon.Team != "" && weapon.Team != p.Team {
			return errors.New("该队伍不能购买此物品")
		}
		index := weaponIndex(p, item)
		if index >= 0 && (weapon.Slot != 4 || p.Inventory[index].Ammo >= weapon.Ammo) {
			return errors.New("已持有该物品")
		}
		if p.Money < weapon.Price {
			return errors.New("资金不足")
		}
		if weapon.Slot == 4 {
			if index >= 0 {
				p.Inventory[index].Ammo++
			} else {
				p.Inventory = append(p.Inventory, WeaponState{ID: item, Ammo: 1})
			}
		} else {
			reserve := 0
			for i := len(p.Inventory) - 1; i >= 0; i-- {
				owned, valid := WeaponByID(p.Inventory[i].ID)
				if valid && owned.AmmoType == weapon.AmmoType {
					reserve = p.Inventory[i].Reserve
				}
				if valid && owned.Slot == weapon.Slot {
					dropWeapon(state, p, i)
				}
			}
			p.Inventory = append(p.Inventory, WeaponState{ID: item, Ammo: weapon.Ammo, Reserve: min(reserve, weapon.Reserve)})
			p.Weapon = item
			p.NextAttack = state.Frame + weaponDeployFrames(item)
		}
		p.Money -= weapon.Price
		SyncWeapon(p)
		Emit(state, Event{Kind: "buy", Seat: seat, Weapon: item, Value: weapon.Price})
		return nil
	}
	if p.Money < price {
		return errors.New("资金不足")
	}
	p.Money -= price
	switch item {
	case "vest":
		p.Armor, p.ArmorRemainder = 100, 0
		p.Helmet = helmetOwned
	case "vesthelm":
		p.Armor, p.ArmorRemainder, p.Helmet = 100, 0, true
	case "defuser":
		p.Defuser = true
	case "nvgs":
		p.NightVisionOwned = true
	}
	Emit(state, Event{Kind: "buy", Seat: seat, Weapon: item, Value: price})
	return nil
}

func dropWeapon(state *State, player *Player, index int) {
	if index < 0 || index >= len(player.Inventory) {
		return
	}
	item := player.Inventory[index]
	if item.ID == "knife" {
		return
	}
	if weapon, valid := WeaponByID(item.ID); valid && weapon.AmmoType != "" {
		for other, owned := range player.Inventory {
			definition, known := WeaponByID(owned.ID)
			if other != index && known && definition.AmmoType == weapon.AmmoType {
				item.Reserve = 0
				break
			}
		}
	}
	if item.ID == "c4" {
		if state.Bomb.Carrier == player.Seat {
			placeDroppedBomb(state, player)
		}
	} else {
		state.NextID++
		item.ReloadUntil, item.BurstLeft = 0, 0
		forward := Direction(player.Yaw, 0)
		point, velocity := Position(*player), droppedItemVelocity(player)
		if player.Alive {
			point = combatAddRay(point, forward, 10*Unit)
			velocity = Vec3{int(int64(forward.X) * 400 * Unit / NormalUnit), int(int64(forward.Y) * 400 * Unit / NormalUnit), 0}
		}
		trace := TraceWorld(state, Position(*player), point, -1)
		state.Drops = append(state.Drops, Drop{ID: state.NextID, X: trace.End.X, Y: trace.End.Y, Z: trace.End.Z, Yaw: player.Yaw, VX: velocity.X, VY: velocity.Y, VZ: velocity.Z, Weapon: item, Frame: state.Frame, Moving: true})
		trimDynamicDrops(state)
	}
	player.Inventory = append(player.Inventory[:index], player.Inventory[index+1:]...)
}

func DropWeapons(state *State, seat int) {
	if seat < 0 || seat >= len(state.Players) {
		return
	}
	p := &state.Players[seat]
	if !p.Alive {
		p.NightVision = false
	}
	for i := len(p.Inventory) - 1; i >= 0; i-- {
		if p.Inventory[i].ID != "knife" {
			dropWeapon(state, p, i)
		}
	}
	if !p.Alive && p.Defuser {
		state.NextID++
		velocity := droppedItemVelocity(p)
		state.Drops = append(state.Drops, Drop{ID: state.NextID, X: p.X, Y: p.Y, Z: p.Z, VX: velocity.X, VY: velocity.Y, VZ: velocity.Z, Moving: true, Weapon: WeaponState{ID: "defuser", Ammo: 1}, Frame: state.Frame})
		p.Defuser = false
		trimDynamicDrops(state)
	}
	SyncWeapon(p)
}

func PickupDrop(state *State, seat int) {
	if state.Finished || state.Phase != "live" && state.Phase != "freeze" || seat < 0 || seat >= len(state.Players) || !state.Players[seat].Alive {
		return
	}
	p := &state.Players[seat]
	for i, drop := range state.Drops {
		if drop.SpawnID > 0 || drop.Moving {
			continue
		}
		if Distance(combatPosition(p), Vec3{drop.X, drop.Y, drop.Z}) > 48*Unit {
			continue
		}
		if drop.Weapon.ID == "defuser" {
			if p.Team != "ct" || p.Defuser {
				continue
			}
			p.Defuser = true
			state.Drops = append(state.Drops[:i], state.Drops[i+1:]...)
			Emit(state, Event{Kind: "pickup", Seat: seat, Weapon: "defuser", X: p.X, Y: p.Y, Z: p.Z})
			return
		}
		weapon, ok := WeaponByID(drop.Weapon.ID)
		if !ok {
			continue
		}
		occupied, same, count := false, -1, 0
		for _, item := range p.Inventory {
			owned, valid := WeaponByID(item.ID)
			if valid && owned.ID == weapon.ID {
				count++
				same = weaponIndex(p, item.ID)
			}
			if valid && owned.Slot == weapon.Slot && weapon.Slot != 4 {
				occupied = true
			}
		}
		if same >= 0 && weapon.Slot <= 2 {
			amount := min(weapon.Reserve-p.Inventory[same].Reserve, max(0, drop.Weapon.Ammo)+max(0, drop.Weapon.Reserve))
			if amount <= 0 {
				continue
			}
			syncSharedAmmo(p, weapon.AmmoType, p.Inventory[same].Reserve+amount)
			state.Drops = append(state.Drops[:i], state.Drops[i+1:]...)
			SyncWeapon(p)
			Emit(state, Event{Kind: "pickup", Seat: seat, Weapon: weapon.ID, Value: amount, X: p.X, Y: p.Y, Z: p.Z})
			return
		}
		if weapon.Slot == 4 {
			limit := 1
			if weapon.ID == "flashbang" {
				limit = 2
			}
			occupied = count >= limit
		}
		if occupied {
			continue
		}
		item := drop.Weapon
		if weapon.Slot <= 2 && weapon.AmmoType != "" {
			shared := 0
			for _, owned := range p.Inventory {
				definition, known := WeaponByID(owned.ID)
				if known && definition.AmmoType == weapon.AmmoType {
					shared = max(shared, owned.Reserve)
				}
			}
			item.Reserve = min(weapon.Reserve, shared+max(0, item.Reserve))
		}
		p.Inventory = append(p.Inventory, item)
		if weapon.Slot <= 2 && weapon.AmmoType != "" {
			syncSharedAmmo(p, weapon.AmmoType, item.Reserve)
		}
		state.Drops = append(state.Drops[:i], state.Drops[i+1:]...)
		p.Weapon = weapon.ID
		p.NextAttack = state.Frame + weaponDeployFrames(weapon.ID)
		SyncWeapon(p)
		Emit(state, Event{Kind: "pickup", Seat: seat, Weapon: weapon.ID, X: p.X, Y: p.Y, Z: p.Z})
		return
	}
}
