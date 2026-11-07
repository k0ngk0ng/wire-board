package contra

// BotMachineNozzleVisionRecipes identifies fixed, visible emitter mouths. It
// says nothing about their timers, collision state or future flame length.
// Japanese Energy Zone tileset 9e9e-a57e, supertile20's top-right quadrant:
// E4/82 is the left-facing green mouth (also D/1A/5E's lower-right quadrant).
// The retained group0 palette and CHR preset5 come from the same original map
// header used by the public artwork catalog. The anchor is the visible center.
func BotMachineNozzleVisionRecipes() []BotVisionBackgroundRecipe {
	return []BotVisionBackgroundRecipe{{
		Level: 5, Variant: RegularMap, Table: "base", Codes: []uint8{0x20},
		CropX: 16, CropY: 0, Width: 8, Height: 16, AnchorX: 20, AnchorY: 8,
		Label: BotVisionLabel{Class: "enemy", Family: "machine-flame-nozzle-left", Player: -1, Kind: 17},
	}}
}
