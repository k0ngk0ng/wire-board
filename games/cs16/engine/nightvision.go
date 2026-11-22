package engine

func stepNightVision(state *State, player *Player) {
	if !player.Alive || !player.NightVisionOwned {
		player.NightVision = false
	}
	if !player.Alive || !player.Input.NightVision || player.Previous.NightVision || state.Frame < player.NightVisionAt {
		return
	}
	player.NightVisionAt = state.Frame + 30
	if !player.NightVisionOwned {
		return
	}
	player.NightVision = !player.NightVision
	value := 0
	if player.NightVision {
		value = 1
	}
	Emit(state, Event{Kind: "nightvision", Seat: player.Seat, Target: -1, Weapon: "nvgs", Value: value, Life: player.Life})
}
