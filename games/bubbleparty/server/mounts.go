package bubbleparty

// These values come from the retained 2002 ItemAttr Ship records: 40 original
// pixels per grid, 30 ms engine tick, and respectively 2/9/5 pixels per tick.
// Flying effects and the rail override are independently handled by movement
// and pickup rules, preserving each original ItemAttr ability.
func mountKind(item string) string {
	switch item {
	case "turtle", "turtle-fast", "owl", "airship", "airship-broken", "octopus", "octopus-fast", "railcar":
		return item
	default:
		return ""
	}
}

func validMount(kind string) bool { return kind == "" || mountKind(kind) != "" }

func effectiveMoveMS(p *Player) float64 {
	switch p.Mount {
	case "turtle", "airship-broken", "octopus", "railcar":
		return 600
	case "turtle-fast", "octopus-fast":
		return 1200.0 / 9
	case "airship":
		return 120
	case "owl":
		return 240
	default:
		if p.NativeSpeed > 0 {
			return 1200.0 / float64(p.NativeSpeed)
		}
		return float64(p.SpeedMS)
	}
}

func (s *State) absorbMountHit(p *Player, at int64, by int) bool {
	if p == nil || !p.Alive || p.TrappedUntil > 0 || p.Mount == "" || p.InvulnerableUntil > at {
		return false
	}
	kind := p.Mount
	p.Mount = ""
	// A 50-tick transition exists in the original client. This reconstruction
	// immediately shows the dismount and retains 1500ms water protection, so the
	// same persistent water column cannot also trap the newly unmounted player.
	p.InvulnerableUntil = at + 1500
	s.emit("dismount", p.Seat, p.X, p.Y, kind, by, at)
	return true
}

// ItemAttr effects 5 and 9 are m_fAbPassBox and m_fAbNotEatItem.
func mountPassesBoxes(kind string) bool { return kind == "airship" || kind == "airship-broken" }
func mountCanPickup(kind string) bool   { return !mountPassesBoxes(kind) }
