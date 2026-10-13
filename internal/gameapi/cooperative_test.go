package gameapi

// Failure scenarios: a cooperative loss rejected for no winner; partial team
// victory; ranked teammates; mixed competitive/cooperative semantics; duplicate
// or invalid winners; old competitive empty winners newly accepted.
import (
	"encoding/json"
	"testing"
)

func TestCooperativeResults(t *testing.T) {
	for _, tc := range []struct {
		name, result string
		valid        bool
	}{
		{"win", `{"ranks":[1,1],"winners":[0,1],"cooperative":true}`, true},
		{"loss", `{"ranks":[1,1],"winners":[],"cooperative":true}`, true},
		{"solo loss", `{"ranks":[1],"winners":[],"cooperative":true}`, true},
		{"partial victory", `{"ranks":[1,1],"winners":[0],"cooperative":true}`, false},
		{"ranked teammates", `{"ranks":[1,2],"winners":[],"cooperative":true}`, false},
		{"competitive policy", `{"ranks":[1,1],"winners":[0,1],"cooperative":true,"team":true}`, false},
		{"duplicate winner", `{"ranks":[1,1],"winners":[0,0],"cooperative":true}`, false},
		{"invalid winner", `{"ranks":[1,1],"winners":[0,2],"cooperative":true}`, false},
		{"competitive empty", `{"ranks":[1,2],"winners":[]}`, false},
		{"competitive normal", `{"ranks":[1,2],"winners":[0]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			if err := json.Unmarshal([]byte(tc.result), &result); err != nil {
				t.Fatal(err)
			}
			m := Match{Data: json.RawMessage(`{}`), Result: &result}
			if err := m.Validate(len(result.Ranks)); (err == nil) != tc.valid {
				t.Fatalf("validation %v; valid=%v", err, tc.valid)
			}
		})
	}
}
