package engine

var gunCatalog = []Weapon{
	{ID: "glock18", Name: "Glock 18", Slot: 2, Price: 400, Ammo: 20, Reserve: 120, Damage: 25, ArmorRatio: 525, Cycle: 15, Reload: 220, Range: 8192 * Unit, RangeModifier: 750, Penetration: 1, PenetrationPower: 21 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 100, AmmoType: "9mm", AmmoPrice: 20, AmmoPack: 30},
	{ID: "usp", Name: "USP", Slot: 2, Price: 500, Ammo: 12, Reserve: 100, Damage: 34, ArmorRatio: 500, Cycle: 15, Reload: 270, Range: 4096 * Unit, RangeModifier: 790, Penetration: 1, PenetrationPower: 15 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 100, AmmoType: "45acp", AmmoPrice: 25, AmmoPack: 12},
	{ID: "p228", Name: "P228", Slot: 2, Price: 600, Ammo: 13, Reserve: 52, Damage: 32, ArmorRatio: 625, Cycle: 15, Reload: 270, Range: 4096 * Unit, RangeModifier: 800, Penetration: 1, PenetrationPower: 25 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 150, AmmoType: "357sig", AmmoPrice: 50, AmmoPack: 13},
	{ID: "deagle", Name: "Desert Eagle", Slot: 2, Price: 650, Ammo: 7, Reserve: 35, Damage: 54, ArmorRatio: 750, Cycle: 23, Reload: 220, Range: 4096 * Unit, RangeModifier: 810, Penetration: 2, PenetrationPower: 30 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 130, AmmoType: "50ae", AmmoPrice: 40, AmmoPack: 7},
	{ID: "elite", Name: "Dual Elites", Slot: 2, Team: "t", Price: 800, Ammo: 30, Reserve: 120, Damage: 36, ArmorRatio: 525, Cycle: 8, Reload: 450, Range: 8192 * Unit, RangeModifier: 750, Penetration: 1, PenetrationPower: 21 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 100, AmmoType: "9mm", AmmoPrice: 20, AmmoPack: 30},
	{ID: "fiveseven", Name: "Five-SeveN", Slot: 2, Team: "ct", Price: 750, Ammo: 20, Reserve: 100, Damage: 20, ArmorRatio: 750, Cycle: 15, Reload: 270, Range: 4096 * Unit, RangeModifier: 885, Penetration: 1, PenetrationPower: 30 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 150, AmmoType: "57mm", AmmoPrice: 50, AmmoPack: 50},
	{ID: "m3", Name: "M3", Slot: 1, Price: 1700, Ammo: 8, Reserve: 32, Damage: 20, ArmorRatio: 500, Cycle: 88, Reload: 45, Range: 3000 * Unit, RangeModifier: 700, Penetration: 0, Speed: 230 * Unit, Pellets: 9, Spread: 675, AmmoType: "buckshot", AmmoPrice: 65, AmmoPack: 8},
	{ID: "xm1014", Name: "XM1014", Slot: 1, Price: 3000, Ammo: 7, Reserve: 32, Damage: 20, ArmorRatio: 500, Cycle: 25, Reload: 30, Range: 3048 * Unit, RangeModifier: 700, Penetration: 0, Speed: 240 * Unit, Pellets: 6, Spread: 725, Automatic: true, AmmoType: "buckshot", AmmoPrice: 65, AmmoPack: 8},
	{ID: "mp5navy", Name: "MP5 Navy", Slot: 1, Price: 1500, Ammo: 30, Reserve: 120, Damage: 26, ArmorRatio: 500, Cycle: 8, Reload: 263, Range: 8192 * Unit, RangeModifier: 840, Penetration: 1, PenetrationPower: 21 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 40, Automatic: true, AmmoType: "9mm", AmmoPrice: 20, AmmoPack: 30},
	{ID: "tmp", Name: "TMP", Slot: 1, Team: "ct", Price: 1250, Ammo: 30, Reserve: 120, Damage: 20, ArmorRatio: 500, Cycle: 7, Reload: 212, Range: 8192 * Unit, RangeModifier: 850, Penetration: 1, PenetrationPower: 21 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 30, Automatic: true, AmmoType: "9mm", AmmoPrice: 20, AmmoPack: 30},
	{ID: "p90", Name: "P90", Slot: 1, Price: 2350, Ammo: 50, Reserve: 100, Damage: 21, ArmorRatio: 750, Cycle: 7, Reload: 340, Range: 8192 * Unit, RangeModifier: 885, Penetration: 1, PenetrationPower: 30 * Unit, Speed: 245 * Unit, Pellets: 1, Spread: 45, Automatic: true, AmmoType: "57mm", AmmoPrice: 50, AmmoPack: 50},
	{ID: "mac10", Name: "MAC-10", Slot: 1, Team: "t", Price: 1400, Ammo: 30, Reserve: 100, Damage: 29, ArmorRatio: 475, Cycle: 7, Reload: 315, Range: 8192 * Unit, RangeModifier: 820, Penetration: 1, PenetrationPower: 15 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 30, Automatic: true, AmmoType: "45acp", AmmoPrice: 25, AmmoPack: 12},
	{ID: "ump45", Name: "UMP45", Slot: 1, Price: 1700, Ammo: 25, Reserve: 100, Damage: 30, ArmorRatio: 500, Cycle: 10, Reload: 350, Range: 8192 * Unit, RangeModifier: 820, Penetration: 1, PenetrationPower: 15 * Unit, Speed: 250 * Unit, Pellets: 1, Spread: 40, Automatic: true, AmmoType: "45acp", AmmoPrice: 25, AmmoPack: 12},
	{ID: "ak47", Name: "AK-47", Slot: 1, Team: "t", Price: 2500, Ammo: 30, Reserve: 90, Damage: 36, ArmorRatio: 775, Cycle: 10, Reload: 245, Range: 8192 * Unit, RangeModifier: 980, Penetration: 2, PenetrationPower: 39 * Unit, Speed: 221 * Unit, Pellets: 1, Spread: 275, Automatic: true, AmmoType: "762nato", AmmoPrice: 80, AmmoPack: 30},
	{ID: "m4a1", Name: "M4A1", Slot: 1, Team: "ct", Price: 3100, Ammo: 30, Reserve: 90, Damage: 32, ArmorRatio: 700, Cycle: 9, Reload: 305, Range: 8192 * Unit, RangeModifier: 970, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 230 * Unit, Pellets: 1, Spread: 200, Automatic: true, AmmoType: "556nato", AmmoPrice: 60, AmmoPack: 30},
	{ID: "galil", Name: "Galil", Slot: 1, Team: "t", Price: 2000, Ammo: 35, Reserve: 90, Damage: 30, ArmorRatio: 775, Cycle: 9, Reload: 245, Range: 8192 * Unit, RangeModifier: 980, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 240 * Unit, Pellets: 1, Spread: 375, Automatic: true, AmmoType: "556nato", AmmoPrice: 60, AmmoPack: 30},
	{ID: "famas", Name: "FAMAS", Slot: 1, Team: "ct", Price: 2250, Ammo: 25, Reserve: 90, Damage: 30, ArmorRatio: 700, Cycle: 9, Reload: 330, Range: 8192 * Unit, RangeModifier: 960, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 240 * Unit, Pellets: 1, Spread: 200, Automatic: true, AmmoType: "556nato", AmmoPrice: 60, AmmoPack: 30},
	{ID: "aug", Name: "AUG", Slot: 1, Team: "ct", Price: 3500, Ammo: 30, Reserve: 90, Damage: 32, ArmorRatio: 700, Cycle: 9, Reload: 330, Range: 8192 * Unit, RangeModifier: 960, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 240 * Unit, Pellets: 1, Spread: 200, Automatic: true, AmmoType: "556nato", AmmoPrice: 60, AmmoPack: 30},
	{ID: "sg552", Name: "SG552", Slot: 1, Team: "t", Price: 3500, Ammo: 30, Reserve: 90, Damage: 33, ArmorRatio: 700, Cycle: 9, Reload: 300, Range: 8192 * Unit, RangeModifier: 955, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 235 * Unit, Pellets: 1, Spread: 200, Automatic: true, AmmoType: "556nato", AmmoPrice: 60, AmmoPack: 30},
	{ID: "scout", Name: "Scout", Slot: 1, Price: 2750, Ammo: 10, Reserve: 90, Damage: 75, ArmorRatio: 850, Cycle: 125, Reload: 200, Range: 8192 * Unit, RangeModifier: 980, Penetration: 3, PenetrationPower: 39 * Unit, Speed: 260 * Unit, Pellets: 1, Spread: 70, Automatic: true, AmmoType: "762nato", AmmoPrice: 80, AmmoPack: 30},
	{ID: "awp", Name: "AWP", Slot: 1, Price: 4750, Ammo: 10, Reserve: 30, Damage: 115, ArmorRatio: 975, Cycle: 145, Reload: 250, Range: 8192 * Unit, RangeModifier: 990, Penetration: 3, PenetrationPower: 45 * Unit, Speed: 210 * Unit, Pellets: 1, Spread: 10, Automatic: true, AmmoType: "338magnum", AmmoPrice: 125, AmmoPack: 10},
	{ID: "g3sg1", Name: "G3SG1", Slot: 1, Team: "t", Price: 5000, Ammo: 20, Reserve: 90, Damage: 80, ArmorRatio: 825, Cycle: 25, Reload: 350, Range: 8192 * Unit, RangeModifier: 980, Penetration: 3, PenetrationPower: 39 * Unit, Speed: 210 * Unit, Pellets: 1, Spread: 55, Automatic: true, AmmoType: "762nato", AmmoPrice: 80, AmmoPack: 30},
	{ID: "sg550", Name: "SG550", Slot: 1, Team: "ct", Price: 4200, Ammo: 30, Reserve: 90, Damage: 70, ArmorRatio: 725, Cycle: 25, Reload: 335, Range: 8192 * Unit, RangeModifier: 980, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 210 * Unit, Pellets: 1, Spread: 50, Automatic: true, AmmoType: "556nato", AmmoPrice: 60, AmmoPack: 30},
	{ID: "m249", Name: "M249", Slot: 1, Price: 5750, Ammo: 100, Reserve: 200, Damage: 32, ArmorRatio: 750, Cycle: 10, Reload: 470, Range: 8192 * Unit, RangeModifier: 970, Penetration: 2, PenetrationPower: 35 * Unit, Speed: 220 * Unit, Pellets: 1, Spread: 300, Automatic: true, AmmoType: "556natobox", AmmoPrice: 60, AmmoPack: 30},
}

var utilityCatalog = []Weapon{
	{ID: "knife", Name: "小刀", Slot: 3, Damage: 20, ArmorRatio: 850, Cycle: 35, Range: 48 * Unit, Speed: 250 * Unit, Pellets: 1},
	{ID: "hegrenade", Name: "高爆手雷", Slot: 4, Price: 300, Ammo: 1, Damage: 100, Cycle: 100, Speed: 250 * Unit},
	{ID: "flashbang", Name: "闪光弹", Slot: 4, Price: 200, Ammo: 2, Cycle: 100, Speed: 250 * Unit},
	{ID: "smokegrenade", Name: "烟雾弹", Slot: 4, Price: 300, Ammo: 1, Cycle: 100, Speed: 250 * Unit},
	{ID: "c4", Name: "C4", Slot: 5, Ammo: 1, Speed: 250 * Unit},
}

func Weapons() []Weapon {
	result := append([]Weapon(nil), gunCatalog...)
	for i := range result {
		result[i].CycleUS = cycleMicros(result[i].ID, false, 0)
		result[i].Cycle = (result[i].CycleUS + 9999) / 10000
	}
	return result
}

func WeaponByID(id string) (Weapon, bool) {
	for _, weapon := range gunCatalog {
		if weapon.ID == id {
			weapon.CycleUS = cycleMicros(id, false, 0)
			weapon.Cycle = (weapon.CycleUS + 9999) / 10000
			return weapon, true
		}
	}
	for _, weapon := range utilityCatalog {
		if weapon.ID == id {
			return weapon, true
		}
	}
	return Weapon{}, false
}

func cycleMicros(id string, burst bool, zoom int) int64 {
	if burst {
		if id == "glock18" {
			return 500000
		}
		if id == "famas" {
			return 550000
		}
	}
	if zoom != 0 && (id == "aug" || id == "sg552") {
		return 135000
	}
	switch id {
	case "glock18", "usp", "p228", "fiveseven":
		return 150000
	case "deagle":
		return 225000
	case "elite":
		return 75000
	case "m3":
		return 875000
	case "xm1014", "g3sg1", "sg550":
		return 250000
	case "mp5navy":
		return 75000
	case "tmp", "mac10":
		return 70000
	case "p90":
		return 66000
	case "ump45", "m249":
		return 100000
	case "ak47":
		return 95500
	case "m4a1", "galil":
		return 87500
	case "famas", "aug", "sg552":
		return 82500
	case "scout":
		return 1250000
	case "awp":
		return 1450000
	}
	return 350000
}

func weaponIndex(player *Player, id string) int {
	for i := range player.Inventory {
		if player.Inventory[i].ID == id {
			return i
		}
	}
	return -1
}

func combatPosition(player *Player) Vec3 { return Vec3{player.X, player.Y, player.Z} }

func combatEye(player *Player) Vec3 {
	return Eye(*player)
}

func WeaponMoveSpeed(player *Player) int {
	weapon, ok := WeaponByID(player.Weapon)
	if !ok {
		return 250 * Unit
	}
	index := weaponIndex(player, player.Weapon)
	if index >= 0 && player.Inventory[index].Zoom != 0 {
		switch player.Weapon {
		case "awp", "g3sg1", "sg550":
			return 150 * Unit
		case "scout":
			return 220 * Unit
		case "sg552":
			return 200 * Unit
		}
	}
	return weapon.Speed
}

func weaponDeployFrames(id string) int64 {
	switch id {
	case "awp":
		return 145
	case "scout":
		return 125
	}
	return 75
}

func weaponExtension(id string) string {
	switch id {
	case "ak47", "galil":
		return "ak47"
	case "m4a1", "awp", "scout", "sg550":
		return "rifle"
	case "aug", "famas", "p90", "ump45":
		return "carbine"
	case "g3sg1", "mp5navy", "sg552":
		return "mp5"
	case "tmp", "mac10", "glock18", "usp", "p228", "deagle", "fiveseven":
		return "onehanded"
	case "elite":
		return "dualpistols"
	case "m3":
		return "shotgun"
	case "xm1014", "m249":
		return "m249"
	case "knife":
		return "knife"
	case "hegrenade", "flashbang", "smokegrenade":
		return "grenade"
	case "c4":
		return "c4"
	}
	return ""
}

func resetWeaponAccuracy(item *WeaponState, reload bool) {
	accuracy := 2000
	switch item.ID {
	case "glock18", "p228", "deagle", "sg550":
		accuracy = 9000
	case "usp", "fiveseven":
		accuracy = 9200
	case "elite":
		accuracy = 8800
	case "mp5navy", "ump45":
		accuracy = 0
	case "mac10":
		accuracy = 1500
	}
	if reload {
		switch item.ID {
		case "aug", "famas", "mac10":
			accuracy = 0
		case "sg550":
			if item.AccuracyInitialized {
				accuracy = item.Accuracy
			}
		}
	}
	item.Accuracy, item.AccuracyInitialized = accuracy, true
	item.Shots, item.LastShot, item.NextShotUS = 0, 0, 0
}

func combatInside(point Vec3, bounds Bounds) bool {
	return point.X >= bounds.Min.X && point.X <= bounds.Max.X && point.Y >= bounds.Min.Y && point.Y <= bounds.Max.Y && point.Z >= bounds.Min.Z && point.Z <= bounds.Max.Z
}

var recoilProfiles = map[string][4][7]int{
	"ak47":    {{15000, 4500, 2250, 500, 65000, 25000, 7}, {20000, 10000, 5000, 3500, 90000, 60000, 5}, {9000, 3500, 1500, 250, 55000, 15000, 9}, {10000, 3750, 1750, 375, 57500, 17500, 8}},
	"m4a1":    {{10000, 4500, 2800, 450, 37500, 30000, 7}, {12000, 5000, 2300, 1500, 55000, 35000, 6}, {6000, 3000, 2000, 125, 32500, 20000, 7}, {6500, 3500, 2500, 150, 35000, 22500, 7}},
	"galil":   {{10000, 4500, 2800, 450, 37500, 30000, 7}, {12000, 5000, 2300, 1500, 55000, 35000, 6}, {6000, 3000, 2000, 125, 32500, 20000, 7}, {6500, 3500, 2500, 150, 35000, 22500, 7}},
	"famas":   {{10000, 4500, 2750, 500, 40000, 25000, 7}, {12500, 4500, 2200, 1800, 55000, 40000, 5}, {5750, 3250, 2000, 110, 32500, 20000, 8}, {6250, 3750, 2500, 125, 35000, 22500, 8}},
	"aug":     {{10000, 4500, 2750, 500, 40000, 25000, 7}, {12500, 4500, 2200, 1800, 55000, 40000, 5}, {5750, 3250, 2000, 110, 32500, 20000, 8}, {6250, 3750, 2500, 125, 35000, 22500, 8}},
	"sg552":   {{10000, 4500, 2800, 400, 42500, 25000, 7}, {12500, 4500, 2200, 1800, 60000, 40000, 5}, {6000, 3500, 2000, 125, 37000, 20000, 10}, {6250, 3750, 2500, 125, 40000, 22500, 9}},
	"m249":    {{11000, 5000, 3000, 600, 40000, 30000, 8}, {18000, 6500, 4500, 1250, 50000, 35000, 8}, {7500, 3250, 2500, 250, 35000, 25000, 9}, {8000, 3500, 3000, 300, 37500, 30000, 9}},
	"mp5navy": {{5000, 2750, 2000, 300, 30000, 20000, 10}, {9000, 4750, 3500, 425, 50000, 30000, 6}, {2250, 1500, 1000, 150, 20000, 10000, 10}, {2500, 1750, 1250, 200, 22500, 12500, 10}},
	"tmp":     {{8000, 4000, 2000, 300, 30000, 25000, 7}, {11000, 5000, 3500, 450, 45000, 35000, 6}, {7000, 3500, 1250, 250, 25000, 20000, 10}, {7250, 3750, 1500, 250, 27500, 22500, 9}},
	"mac10":   {{9000, 4500, 2500, 350, 35000, 27500, 7}, {13000, 5500, 4000, 500, 47500, 37500, 5}, {7500, 4000, 1750, 300, 27500, 25000, 10}, {7750, 4250, 2000, 300, 30000, 27500, 9}},
	"ump45":   {{5500, 3000, 2250, 300, 35000, 25000, 10}, {1250, 6500, 5500, 475, 55000, 40000, 10}, {2500, 1750, 1250, 200, 22500, 12500, 10}, {2750, 2000, 1500, 225, 25000, 15000, 10}},
	"p90":     {{4500, 3000, 2000, 275, 40000, 22500, 7}, {9000, 4500, 3500, 400, 52500, 35000, 4}, {2750, 2000, 1250, 200, 30000, 10000, 9}, {3000, 2250, 1250, 200, 32500, 12500, 8}},
}

func recoilProfile(p *Player, id string) [7]int {
	profiles := recoilProfiles[id]
	moving := p.VX != 0 || p.VY != 0
	movingFirst := id == "ak47" || id == "m4a1" || id == "galil" || id == "famas" || id == "aug" || id == "sg552"
	if movingFirst && moving {
		return profiles[0]
	}
	if !p.Grounded {
		return profiles[1]
	}
	if moving {
		return profiles[0]
	}
	if p.Crouch {
		return profiles[2]
	}
	return profiles[3]
}
