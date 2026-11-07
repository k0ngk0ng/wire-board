package starfort

// Failure cases before implementation: start still allocates a route per seat;
// expansion loses towers or new skills; schema cannot reload the mixed rules;
// old central saves move; bots submit obsolete arrow specializations.
import (
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func TestCooperativeExpansionWithCurrentSkills(t *testing.T) {
	for n := 1; n <= 4; n++ {
		m, err := Module().Start(gameapi.Context{Players: n, Now: time.Unix(1900000000, 0)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		s, err := decode(m)
		if err != nil {
			t.Fatal(err)
		}
		if m.Schema != 5 || s.World.Layout != "" || len(s.World.Map.Routes) != 1 || s.SkillRules != 2 {
			t.Fatal("not a shared expanding world", m.Schema, s.World.Layout)
		}
		for _, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
			if s.passiveView(Tower{Kind: kind}) == nil {
				t.Fatal("missing passive", kind)
			}
		}
		for _, wave := range []int{6, 11, 16} {
			oldPads := append([]Pad{}, s.World.Map.Pads...)
			s.Wave = wave
			s.CompletedWaves = wave - 1
			s.describeWave()
			s.worldPrepare(s.ServerAt)
			if s.World.Choice == nil || s.World.BoonChoice == nil {
				t.Fatal("missing team votes", wave)
			}
			s.finishChoice("route", s.ServerAt)
			s.finishChoice("boon", s.ServerAt)
			if !reflect.DeepEqual(oldPads, s.World.Map.Pads[:len(oldPads)]) {
				t.Fatal("expansion moved pads")
			}
			m, err = s.Envelope()
			if err != nil {
				t.Fatal(err)
			}
			s, err = decode(m)
			if err != nil {
				t.Fatal(err)
			}
		}
		if s.World.Stage != 4 || s.World.Map.Width != 1920 {
			t.Fatal("expansion incomplete")
		}
	}
}
