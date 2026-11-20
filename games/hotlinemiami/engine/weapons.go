package engine

func Weapons() []Weapon {
	return []Weapon{
		{ID: "pistol", Name: "手枪", Ammo: 8, Cooldown: 15, Range: 680 * Unit, Speed: 20 * Unit, Pellets: 1, Spread: 2},
		{ID: "uzi", Name: "Uzi", Ammo: 20, Cooldown: 5, Range: 520 * Unit, Speed: 19 * Unit, Pellets: 1, Spread: 18},
		{ID: "m16", Name: "M16", Ammo: 15, Cooldown: 8, Range: 800 * Unit, Speed: 24 * Unit, Pellets: 1, Spread: 7},
		{ID: "shotgun", Name: "霰弹枪", Ammo: 4, Cooldown: 44, Range: 390 * Unit, Speed: 21 * Unit, Pellets: 6, Spread: 50},
		{ID: "knife", Name: "小刀", Cooldown: 15, Range: 31 * Unit, Melee: true},
		{ID: "bat", Name: "棒球棒", Cooldown: 27, Range: 45 * Unit, Melee: true},
		{ID: "katana", Name: "武士刀", Cooldown: 24, Range: 49 * Unit, Melee: true},
		{ID: "magnum", Name: "马格南", Ammo: 6, Cooldown: 28, Range: 760 * Unit, Speed: 26 * Unit, Pellets: 1, Spread: 1},
		{ID: "mp5", Name: "MP5", Ammo: 24, Cooldown: 6, Range: 620 * Unit, Speed: 21 * Unit, Pellets: 1, Spread: 10},
		{ID: "double-barrel", Name: "双管霰弹枪", Ammo: 2, Cooldown: 36, Range: 330 * Unit, Speed: 20 * Unit, Pellets: 8, Spread: 64},
		{ID: "axe", Name: "斧头", Cooldown: 38, Range: 55 * Unit, Melee: true},
		{ID: "machete", Name: "砍刀", Cooldown: 20, Range: 38 * Unit, Melee: true},
	}
}

func UnarmedWeapon() Weapon {
	return Weapon{ID: "", Cooldown: 22, Range: 25 * Unit, Melee: true}
}

func WeaponByID(id string) (Weapon, bool) {
	for _, weapon := range Weapons() {
		if weapon.ID == id {
			return weapon, true
		}
	}
	return Weapon{}, false
}

func Characters() []string {
	return []string{"jacket", "biker", "bodyguard", "father", "richter", "producer", "beard", "hobo"}
}

func validCharacter(id string) bool {
	for _, character := range Characters() {
		if character == id {
			return true
		}
	}
	return false
}
