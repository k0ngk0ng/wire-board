package contra

import "errors"

type EnemyTerrainRequest struct{ X, Y, Code uint8 }
type EnemyTerrainResponse struct {
	AttributeLookupWritten bool
	AttributeLookup        uint8
	Accepted               bool
	// EB1B's clipped coordinate path succeeds without locating an address.
	// A caller must not use a made-up zero address for later collision changes.
	AddressValid bool
	Address      uint16
}

// requestTerrain is the original EB1B/EB10 entity-coordinate adapter. The
// callback owns the real graphics budget and packet construction; retry marks
// the EB10 variant which delays the entity by one frame after queue refusal.
func (s *EnemySystem) requestTerrain(slot int, code uint8, c EnemyStepContext, retry bool) (EnemyTerrainResponse, error) {
	if slot < 0 || slot >= len(s.Slots) {
		return EnemyTerrainResponse{}, errors.New("invalid terrain entity slot")
	}
	e := &s.Slots[slot]
	if e.X < 12 || e.Y < 12 {
		return EnemyTerrainResponse{Accepted: true}, nil
	}
	if c.ChangeTerrain == nil {
		return EnemyTerrainResponse{}, errors.New("missing original terrain queue callback")
	}
	result, err := c.ChangeTerrain(EnemyTerrainRequest{X: e.X - 12, Y: e.Y - 12, Code: code})
	if err != nil {
		return EnemyTerrainResponse{}, err
	}
	if !result.Accepted && retry {
		e.AnimationDelay = 1
	}
	return result, nil
}
