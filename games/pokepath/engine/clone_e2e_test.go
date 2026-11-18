package engine

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func alterCloneBranch(value reflect.Value) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			alterCloneBranch(value.Elem())
		}
	case reflect.Struct:
		for field := 0; field < value.NumField(); field++ {
			alterCloneBranch(value.Field(field))
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			alterCloneBranch(value.Index(index))
		}
	case reflect.Map:
		if value.IsNil() {
			return
		}
		if value.Len() == 0 {
			value.SetMapIndex(reflect.Zero(value.Type().Key()), reflect.Zero(value.Type().Elem()))
			return
		}
		iterator := value.MapRange()
		for iterator.Next() {
			entry := reflect.New(value.Type().Elem()).Elem()
			entry.Set(iterator.Value())
			alterCloneBranch(entry)
			value.SetMapIndex(iterator.Key(), entry)
		}
	case reflect.String:
		value.SetString(value.String() + "甲")
	case reflect.Bool:
		value.SetBool(!value.Bool())
	case reflect.Int, reflect.Int64, reflect.Int32:
		value.SetInt(value.Int() + 1)
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint8:
		value.SetUint(value.Uint() + 1)
	case reflect.Float64, reflect.Float32:
		value.SetFloat(value.Float() + 1)
	}
}

func TestCoreClonedBattleCollectionsAreIndependentBeforeAndAfterRestoreE2E(t *testing.T) {
	state := coreMotionOwner(t, "pineco")
	coreApply(t, state, 0, Action{Type: "wave"})
	tower := state.Towers[0]
	id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + 40, Y: tower.Y}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	nearby := enemyFor(state, id)
	nearby.Entered, nearby.BaseSpeed, nearby.HP, nearby.MaxHP = true, 0, 100000, 100000
	if err := Advance(state, 700); err != nil {
		t.Fatal(err)
	}
	if len(state.World.Zones) != 1 || state.World.Zones[0].SourceTower == nil {
		t.Fatal("the real persistent field did not retain its source")
	}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		enemy.PassiveFlags = map[string]bool{"sample": true}
		enemy.PassiveValues = map[string]float64{"sample": .5}
		enemy.PassiveTimers = map[string]int64{"sample": 111}
		enemy.PassiveCounters = map[string]int{"sample": 2}
		enemy.Conditions = []Condition{{Kind: "slow", Stacks: 1, Magnitude: .4, DurationMS: 2500, RemainingMS: 1100}}
	}
	before, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, restore := range []bool{false, true} {
		original := state
		if restore {
			original, err = Unmarshal(before)
			if err != nil {
				t.Fatal(err)
			}
		}
		branch, err := Clone(original)
		if err != nil {
			t.Fatal(err)
		}
		serialized, err := json.Marshal(branch)
		if err != nil || !bytes.Equal(serialized, before) {
			t.Fatal("clone changed nil, empty or populated state before any branch operation", err)
		}
		alterCloneBranch(reflect.ValueOf(branch))
		after, err := Marshal(original)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("a mutated branch contaminated a live or restored battle", restore, err)
		}
	}
}
