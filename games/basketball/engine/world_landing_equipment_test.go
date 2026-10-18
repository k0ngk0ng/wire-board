package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type landingEquipmentOracle struct {
	worldActionFixture
	Label                                  string
	Actor                                  uint8
	Prepared, AfterAnimation, BeforeMotion WorldActionState
	Held, Pressed                          [4]Buttons
	Shoes                                  [4]uint8
	RNGCalls                               int
}

func landingEquipmentRows(t *testing.T) []landingEquipmentOracle {
	t.Helper()
	var rows []landingEquipmentOracle
	readPropAttackFixture(t, "testdata/landing-equipment.json.gz", &rows)
	if len(rows) != 6092 {
		t.Fatal("incomplete original landing oracle", len(rows))
	}
	return rows
}
func (r landingEquipmentOracle) landingContext(t *testing.T) WorldActionContext {
	c := r.context(t)
	c.ActorHeldButtons = r.Held
	c.ActorPressedButtons = &r.Pressed
	c.JumpEnhancements = &r.Shoes
	return c
}

// Failure inventory was recorded in LANDING_EQUIPMENT_VERIFICATION.md before
// fixtures/tests/implementation. These compare every world field at ROM stages.
func TestOriginalLandingEquipmentSelection(t *testing.T) {
	for i, r := range landingEquipmentRows(t) {
		got := r.AfterAnimation
		selectOriginalLandingAction(&got, r.Actor, r.landingContext(t))
		if got != r.BeforeMotion {
			t.Fatalf("selection %d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(r.BeforeMotion), "world"))
		}
	}
}
func TestOriginalLandingEquipmentFullCalls(t *testing.T) {
	for i, r := range landingEquipmentRows(t) {
		got := WorldActionStep{State: r.Before}
		c := r.landingContext(t)
		court, e := originalWorldMotionCourt(&got, c, r.Actor)
		if e != nil {
			t.Fatal(i, e)
		}
		if e = advanceWorldLandingPlayer(&got, r.Actor, c, court); e != nil {
			t.Fatal(i, e)
		}
		if got.State != r.After {
			t.Fatalf("full call %d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestOriginalLandingEquipmentWorldDispatch(t *testing.T) {
	for i, r := range landingEquipmentRows(t) {
		got := WorldActionStep{State: r.Before}
		c := r.landingContext(t)
		court, e := originalWorldMotionCourt(&got, c, r.Actor)
		if e != nil {
			t.Fatal(i, e)
		}
		if e = advanceWorldPlayer(&got, r.Actor, c, court); e != nil {
			t.Fatal(i, e)
		}
		if got.State != r.After {
			t.Fatalf("dispatch %d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestOriginalLandingEquipmentCommittedBeforeMotion(t *testing.T) {
	for i, r := range landingEquipmentRows(t) {
		got := WorldActionStep{State: r.Before}
		c := r.landingContext(t)
		read := false
		court := FlatCourt{originalMotion: &courtMotionBinding{object: r.Actor, court: 1,
			load: func() CourtFloorState {
				read = true
				if got.State != r.BeforeMotion {
					t.Fatalf("before motion %d: %s", i, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.BeforeMotion), "world"))
				}
				return worldFloorState(got.State, c)
			},
			store: func(s CourtFloorState, selected uint8) {
				storeWorldFloorState(&got.State, s, selected)
				got.State.Actors[r.Actor].SpecialShot = 33
			},
		}}
		if e := advanceWorldLandingPlayer(&got, r.Actor, c, court); e != nil {
			t.Fatal(i, e)
		}
		if !read || got.State.Actors[r.Actor].SpecialShot != 33 {
			t.Fatal("later motion write lost", i)
		}
	}
}
func TestOriginalLandingEquipmentRestoreAndMissingShoeSource(t *testing.T) {
	for i, r := range landingEquipmentRows(t) {
		if r.RNGCalls == 0 {
			continue
		}
		var restored WorldActionState
		data, e := json.Marshal(r.Before)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(data, &restored); e != nil {
			t.Fatal(e)
		}
		got := WorldActionStep{State: restored}
		c := r.landingContext(t)
		court, e := originalWorldMotionCourt(&got, c, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldLandingPlayer(&got, r.Actor, c, court); e != nil || got.State != r.After {
			t.Fatal("restore", i, e)
		}
		c.JumpEnhancements = nil
		noSource := r.AfterAnimation
		selectOriginalLandingAction(&noSource, r.Actor, c)
		if noSource.Random != r.Before.Random {
			t.Fatal("missing shoe source consumed RNG", i)
		}
	}
}
