package engine

// clearOriginalLandingSpecialShot is the shared 8BBD entry write at 8BD7.
// Dispatcher aliases 06/16/26/36/46 all clear the acting player's canonical
// 0554 flag once, before animation and EEFF. No owner or ball flag is involved.
func clearOriginalLandingSpecialShot(actor *WorldActionActor) {
	if actor.Player.Entered {
		return
	}
	switch actor.Player.Action {
	case 0x06, 0x16, 0x26, 0x36, 0x46:
		actor.SpecialShot = 0
	}
}
