package contra

// FireRequest is sampled after the player's aim/ledge update and before motion.
// Hidden means the original offscreen Y byte is nonzero. Controls and weapon
// data are trusted game state, never arbitrary client-supplied result values.
type FireRequest struct {
	ThrowWork                                          *BaseThrowWork
	Work                                               *TerrainWork `json:"-"`
	Actor                                              int
	X, Y                                               uint8
	Hidden                                             bool
	Aim, Weapon, Jump, Water, Electrocution, StageType uint8
	MachineGunTimer, Recoil                            uint8
	Held, Pressed                                      Buttons
}

// FireResult contains player updates and recovered compatibility effects.
// Projectile allocation remains owned by WeaponSystem; sounds are source event
// codes for the platform audio mapper, not a game-owned audio player.
type FireResult struct {
	MachineGunTimer, Recoil uint8
	Sounds                  []uint8
	GenerationBiasChanged   bool
	GenerationBiasValue     uint8
}
