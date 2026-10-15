package gameapi

// Failure cases before implementation: valid draws rejected; two winners used
// to fake a draw; single-player/cooperative draws; unequal ranks; unfinished
// actors/deadlines remaining; ordinary invalid empty-winner results accepted.
import (
	"encoding/json"
	"testing"
)

func TestDrawResultContract(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		players   int
		valid     bool
	}{
		{"two players", `{"draw":true,"ranks":[1,1],"winners":[]}`, 2, true},
		{"team draw", `{"draw":true,"team":true,"ranks":[1,1,1,1],"winners":[]}`, 4, true},
		{"declared winner", `{"draw":true,"ranks":[1,1],"winners":[0]}`, 2, false},
		{"two winners", `{"draw":true,"ranks":[1,1],"winners":[0,1]}`, 2, false},
		{"unequal ranks", `{"draw":true,"ranks":[1,2],"winners":[]}`, 2, false},
		{"cooperative", `{"draw":true,"cooperative":true,"ranks":[1,1],"winners":[]}`, 2, false},
		{"solo", `{"draw":true,"ranks":[1],"winners":[]}`, 1, false},
		{"unspecified outcome", `{"ranks":[1,1],"winners":[]}`, 2, false},
		{"ordinary win", `{"ranks":[1,2],"winners":[0]}`, 2, true},
		{"ordinary tie winners", `{"ranks":[1,1],"winners":[0,1]}`, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r Result
			if err := json.Unmarshal([]byte(tc.raw), &r); err != nil {
				t.Fatal(err)
			}
			m := Match{Data: json.RawMessage(`{}`), Result: &r}
			if err := m.Validate(tc.players); (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid {
				m.Actors = []int{0}
				if m.Validate(tc.players) == nil {
					t.Fatal("finished result retained actor")
				}
			}
		})
	}
}
