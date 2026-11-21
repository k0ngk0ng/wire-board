package cs16_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNaturalMatchDiagnosticsPreserveExactSeedsAndSurvivors(t *testing.T) {
	initial := json.RawMessage(`{"world":{"mapId":"fy_iceworld","seed":18446744073709551615}}`)
	final := json.RawMessage(`{"world":{"mapId":"fy_iceworld","seed":9007199254740993,"frame":27000,"phase":"live","round":2,"players":[{"seat":0,"team":"t","alive":false,"health":0},{"seat":1,"team":"ct","alive":true,"life":2,"health":37,"x":12,"y":-34,"z":56,"weapon":"awp","ammo":0,"reserve":0}]}}`)
	value, err := naturalMatchDiagnostics(initial, final, map[string]int{"elimination": 1})
	if err != nil {
		t.Fatal(err)
	}
	if value.InitialSeed != "18446744073709551615" || value.FinalSeed != "9007199254740993" || value.Map != "fy_iceworld" || value.Frame != 27000 || value.Phase != "live" || value.Round != 2 || value.RoundReasons["elimination"] != 1 || len(value.Survivors) != 1 || value.Survivors[0].Seat != 1 || value.Survivors[0].Health != 37 || value.Survivors[0].Weapon != "awp" {
		t.Fatal("natural failure diagnostic lost reproducible state", value)
	}
	encoded := string(encode(value))
	for _, field := range []string{`"initialSeed":"18446744073709551615"`, `"finalSeed":"9007199254740993"`, `"phase":"live"`, `"round":2`, `"survivors":[`, `"roundReasons":{"elimination":1}`} {
		if !strings.Contains(encoded, field) {
			t.Fatal("compact failure log omitted required field", field, encoded)
		}
	}
	if _, err := naturalMatchDiagnostics(initial, json.RawMessage(`{"world":{"seed":-1}}`), nil); err == nil {
		t.Fatal("invalid private seed silently changed diagnostic")
	}
}
