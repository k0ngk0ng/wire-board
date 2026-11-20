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
	}
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
	return []string{"jacket", "biker", "bodyguard", "father", "richter"}
}

func validCharacter(id string) bool {
	for _, character := range Characters() {
		if character == id {
			return true
		}
	}
	return false
}
