package engine

import "errors"

// originalAIZone reads the cache produced before board/rim collision. Standalone
// component callers may omit it and derive a zone from their supplied positions;
// a complete world must supply the phase cache, including its initial zero values.
func originalAIZone(view AIControlView, actor uint8) (CourtZone, error) {
	if actor >= 4 {
		return 0, errors.New("invalid zone-cache actor")
	}
	if view.CachedZones != nil {
		zone := view.CachedZones[actor]
		if zone > 0x75 || zone&15 > 5 {
			return 0, errors.New("invalid original cached court zone")
		}
		return zone, nil
	}
	position := view.Players[actor]
	if actor == view.Actor {
		position = view.Position
	}
	return CourtZoneForActor(position, actor)
}
