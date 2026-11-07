package contra

// Failure scenarios before the EB1B/EB10 adapter: subtracting coordinates wraps
// a clipped request into the opposite side; a no-op invents a valid address;
// queue refusal is treated as success; retry delay changes on accepted/no-op;
// renderer errors are suppressed; a callback alters the authoritative enemy.
import (
	"errors"
	"testing"
)

func TestEnemyTerrainRequestAdapter(t *testing.T) {
	s := EnemySystem{}
	s.Slots[3] = Enemy{X: 20, Y: 30, AnimationDelay: 7, Routine: 4}
	before := s.Slots
	calls := 0
	c := EnemyStepContext{ChangeTerrain: func(r EnemyTerrainRequest) (EnemyTerrainResponse, error) {
		calls++
		if r != (EnemyTerrainRequest{X: 8, Y: 18, Code: 0x45}) {
			t.Fatal("request coordinates", r)
		}
		return EnemyTerrainResponse{Accepted: true, AddressValid: true, Address: 0x2201}, nil
	}}
	r, err := s.requestTerrain(3, 0x45, c, true)
	if err != nil || !r.Accepted || !r.AddressValid || r.Address != 0x2201 || calls != 1 || s.Slots != before {
		t.Fatal("accepted request", r, err, s.Slots[3])
	}
	c.ChangeTerrain = func(EnemyTerrainRequest) (EnemyTerrainResponse, error) { return EnemyTerrainResponse{}, nil }
	r, err = s.requestTerrain(3, 0, c, false)
	if err != nil || r.Accepted || s.Slots[3].AnimationDelay != 7 {
		t.Fatal("EB1B retry", r, err, s.Slots[3])
	}
	r, err = s.requestTerrain(3, 0, c, true)
	if err != nil || r.Accepted || s.Slots[3].AnimationDelay != 1 {
		t.Fatal("EB10 retry", r, err, s.Slots[3])
	}
	for _, xy := range [][2]uint8{{11, 40}, {40, 11}, {0, 0}} {
		s.Slots[3] = Enemy{X: xy[0], Y: xy[1], AnimationDelay: 7}
		r, err = s.requestTerrain(3, 0, EnemyStepContext{}, true)
		if err != nil || !r.Accepted || r.AddressValid || s.Slots[3].AnimationDelay != 7 {
			t.Fatal("clipped no-op", r, err)
		}
	}
	s.Slots[3] = Enemy{X: 20, Y: 30, AnimationDelay: 7}
	if _, err = s.requestTerrain(3, 0, EnemyStepContext{}, true); err == nil {
		t.Fatal("missing queue callback accepted")
	}
	want := errors.New("bad queue")
	c.ChangeTerrain = func(EnemyTerrainRequest) (EnemyTerrainResponse, error) { return EnemyTerrainResponse{}, want }
	if _, err = s.requestTerrain(3, 0, c, true); !errors.Is(err, want) || s.Slots[3].AnimationDelay != 7 {
		t.Fatal("queue error changed state", err)
	}
}
