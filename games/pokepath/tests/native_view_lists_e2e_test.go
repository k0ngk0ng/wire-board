package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func assertPublicListValues(t *testing.T, value reflect.Value, path string) {
	t.Helper()
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if !value.IsNil() {
			assertPublicListValues(t, value.Elem(), path)
		}
		return
	}
	switch value.Kind() {
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			field := value.Type().Field(index)
			tag := strings.Split(field.Tag.Get("json"), ",")
			if field.PkgPath != "" || tag[0] == "-" {
				continue
			}
			child := value.Field(index)
			if child.Kind() == reflect.Slice && child.Len() == 0 && strings.Contains(field.Tag.Get("json"), "omitempty") {
				continue
			}
			childPath := path
			if tag[0] != "" {
				childPath += "." + tag[0]
			}
			assertPublicListValues(t, child, childPath)
		}
	case reflect.Slice:
		if value.IsNil() {
			t.Errorf("public array %s would encode as null instead of []", path)
		}
		for index := 0; index < value.Len(); index++ {
			assertPublicListValues(t, value.Index(index), fmt.Sprintf("%s[%d]", path, index))
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			assertPublicListValues(t, iterator.Value(), fmt.Sprintf("%s[%v]", path, iterator.Key().Interface()))
		}
	}
}

func assertPublicViewLists(t *testing.T, state *engine.State) {
	t.Helper()
	before, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []struct {
		name string
		read func(*engine.State) engine.View
	}{{"transient", engine.ProjectTransient}, {"independent", engine.Project}} {
		view := project.read(state)
		assertPublicListValues(t, reflect.ValueOf(view), project.name)
		encoded, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err = json.Unmarshal(encoded, &result); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"team", "box", "items", "towers", "enemies", "projectiles", "events", "tiles", "paths"} {
			if _, ok := result[key].([]any); !ok {
				t.Errorf("%s JSON %s is not an array: %v", project.name, key, result[key])
			}
		}
	}
	after, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("public array normalization changed saved nil collections, random state or rules data")
	}
}

func TestNativeViewListsSaveLoadFormationKeepsEmptyBoxIterableE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "charmander", 1)
	waApply(t, state, 0, engine.Action{Type: "saveTeam", Slot: 0})
	waApply(t, state, 0, engine.Action{Type: "loadTeam", Slot: 0})
	if state.Box != nil {
		t.Fatal("regression fixture did not preserve the original formation's nil empty box")
	}
	assertPublicViewLists(t, state)
	if state.Box != nil {
		t.Fatal("projection rewrote the empty-box save representation")
	}
	waEvidence(t, "view-lists-formation", state, map[string]any{"actions": []string{"saveTeam", "loadTeam", "ProjectTransient", "Project"}, "emptyBoxJSON": "[]", "saveNilPreserved": true})
}

func TestNativeViewListsEmptyWorldAndBoxProjectionRemainReadOnlyE2E(t *testing.T) {
	for _, boxed := range []bool{false, true} {
		state := waScenario(t)
		if boxed {
			id := waPartner(t, state, "treecko", 1)
			waApply(t, state, 0, engine.Action{Type: "team", PokemonID: id, Mode: "box"})
		}
		state.Team, state.Inventory, state.Towers, state.Enemies, state.Projectiles, state.Events = nil, nil, nil, nil, nil, nil
		assertPublicViewLists(t, state)
		encoded, err := engine.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := engine.Unmarshal(encoded)
		if err != nil {
			t.Fatal(err)
		}
		assertPublicViewLists(t, restored)
	}
}

func TestNativeViewListsActualProjectileAndAttackModeNestedArraysE2E(t *testing.T) {
	for _, key := range []string{"treecko", "heatmor", "scyther"} {
		t.Run(key, func(t *testing.T) {
			state := waScenario(t)
			waDeploy(t, state, key, 1)
			waBattle(t, state)
			waEnemy(t, state, engine.Point{X: state.Towers[0].X + 80, Y: state.Towers[0].Y})
			waAdvance(t, state, 20)
			if key == "treecko" && len(state.Projectiles) == 0 || key == "heatmor" && len(state.AttackModes.Effects) == 0 || key == "scyther" && state.AttackModes.Towers[state.Team[0].ID].Chain == nil {
				t.Fatal("actual fixture did not create its expected projectile, effect or chain")
			}
			assertPublicViewLists(t, state)
			waSnapshotContinuation(t, state, 40)
			assertPublicViewLists(t, state)
			waEvidence(t, "view-lists-"+key, state, map[string]any{"actualAdvance": true, "nestedArrays": true, "savedNilPreserved": true})
		})
	}
}
