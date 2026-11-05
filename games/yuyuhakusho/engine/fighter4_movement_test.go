package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFighter4MovementSourcesDriveCompleteAction(t *testing.T) {
	f, animations, _, _ := readChuAction(t, "chu-action-oracle")
	var rows []chuActionCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	commands := actionCommandLibrary(t)
	checked := 0
	for _, row := range rows {
		s := decodeChuAction(t, row.Before, row.ObjectID)
		if s.Body.ActionKind != 1 {
			continue
		}
		p := actionParameters(row.Before)
		want, err := StepChuAction(s, p, commands, animations)
		if err != nil || want.Body.X == s.Body.X {
			continue
		}
		movement := FighterMovementSpec{WalkSpeed: p.WalkSpeed, JumpParameters: p.JumpParameters, BurstSpeed: p.BurstSpeed, EnergyDrain: p.EnergyDrain}
		p = ChuActionParameters{MovementSources: map[uint32]FighterMovementSpec{s.MovementSource: movement}}
		before, _ := json.Marshal(p.MovementSources)
		got, err := StepChuAction(s, p, commands, animations)
		after, _ := json.Marshal(p.MovementSources)
		if err != nil || !reflect.DeepEqual(got, want) || string(before) != string(after) {
			t.Fatalf("current source did not select original complete-action movement: %v", err)
		}
		checked++
		if checked == 12 {
			break
		}
	}
	if checked == 0 {
		t.Fatal("original oracle has no advancing walk action")
	}
}

func TestFighter4MissingMovementSourceIsAtomic(t *testing.T) {
	f, animations, _, _ := readChuAction(t, "chu-action-oracle")
	var rows []chuActionCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) == 0 {
		t.Fatal("missing original complete action", err)
	}
	s, copy := decodeChuAction(t, rows[0].Before, rows[0].ObjectID), decodeChuAction(t, rows[0].Before, rows[0].ObjectID)
	p := actionParameters(rows[0].Before)
	p.MovementSources = map[uint32]FighterMovementSpec{}
	got, err := StepChuAction(s, p, actionCommandLibrary(t), animations)
	if err == nil || !reflect.DeepEqual(got, copy) || !reflect.DeepEqual(s, copy) {
		t.Fatal("missing current movement was accepted or mutated action")
	}
}
