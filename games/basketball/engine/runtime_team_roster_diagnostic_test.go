package engine

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// Explicit diagnostics only: ordinary engine tests must not expand the roster
// matrix. Failure cases: missing/extra/malformed selection, out-of-range teams
// or characters, duplicate teammates, unsupported court, original AI/action
// gaps, clock/score/restore drift, frame exhaustion and repeat nondeterminism.
// This is a cold integration gate, not a ROM trace or a menu-legality oracle.
func TestRuntimeTeamRosterDiagnostic(t *testing.T) {
	if os.Getenv("BASKETBALL_RUNTIME_DIAGNOSTIC") != "1" {
		t.Skip("opt in with BASKETBALL_RUNTIME_DIAGNOSTIC=1 and BASKETBALL_RUNTIME_SELECTION JSON")
	}
	var selection struct {
		Court      *uint8  `json:"court"`
		Mode       *uint8  `json:"mode,omitempty"`
		Teams      []uint8 `json:"teams"`
		Characters []uint8 `json:"characters"`
	}
	decoder := json.NewDecoder(strings.NewReader(os.Getenv("BASKETBALL_RUNTIME_SELECTION")))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selection); err != nil {
		t.Fatalf("explicit diagnostic selection: %v", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("diagnostic selection must contain exactly one JSON value")
	}
	if selection.Court == nil || *selection.Court > 1 || len(selection.Teams) != 2 || len(selection.Characters) != 4 {
		t.Fatal("diagnostic requires court 0/1, exactly two teams and four characters")
	}
	if selection.Teams[0] == selection.Teams[1] || selection.Characters[0] == selection.Characters[1] || selection.Characters[2] == selection.Characters[3] {
		t.Fatal("diagnostic requires distinct teams and distinct characters within each team")
	}
	config := DefaultRuntimeConfig()
	config.Match.Court = *selection.Court
	if selection.Mode != nil {
		if mode := *selection.Mode; mode != 0 && mode != 1 && mode != 128 && mode != 129 {
			t.Fatal("diagnostic original mode must be0/1/128/129")
		}
		config.Match.Mode = *selection.Mode
	}
	copy(config.Match.Teams[:], selection.Teams)
	copy(config.Match.Characters[:], selection.Characters)
	config.Match.Seed = 0x12345678
	config.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	if err := validateRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("diagnostic_config=%s", encoded)
	verifyColdVideoRuntime(t, config, true)
}
