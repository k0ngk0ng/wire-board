package yuyuhakusho

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Failure range before the numeric-array fix: Go []uint8 silently becomes a
// base64 string in Configure/Prepare/State/View/SyncSnapshot. Go []uint8 decode
// accepts that string and masks the browser's number[] contract failure.
func moduleNumericArrayTest(t *testing.T, raw []byte, field string, expected []uint8) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	value := bytes.TrimSpace(object[field])
	if len(value) == 0 || value[0] != '[' {
		t.Fatalf("%s must be a raw JSON numeric array, got %s", field, value)
	}
	var numbers []int
	if err := json.Unmarshal(value, &numbers); err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(expected))
	for i, n := range expected {
		want[i] = int(n)
	}
	if !reflect.DeepEqual(numbers, want) {
		t.Fatal("numeric JSON values differ", field, numbers, want)
	}
}

func TestModuleWireUsesNumericSeatAndCharacterArrays(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprintf("seats-%d", n), func(t *testing.T) {
			mod := Module()
			for _, request := range []json.RawMessage{nil, json.RawMessage(`{"arena":2}`)} {
				configured, err := mod.Configure(n, request)
				if err != nil {
					t.Fatal(err)
				}
				moduleNumericArrayTest(t, configured, "characters", defaultOptions(n).Characters)
			}
			configured, err := mod.Configure(n, nil)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := mod.Prepare(gameapi.PreparationContext{Capacity: n, Actor: 0, Host: true, Bots: make([]bool, n)}, configured, json.RawMessage(`{"type":"character","character":10}`))
			if err != nil {
				t.Fatal(err)
			}
			expected := defaultOptions(n).Characters
			expected[0] = 10
			moduleNumericArrayTest(t, prepared, "characters", expected)
			mod, _, match := moduleStartTest(t, n, Options{Characters: expected}, make([]bool, n))
			var stored map[string]json.RawMessage
			if err = json.Unmarshal(match.Data, &stored); err != nil {
				t.Fatal(err)
			}
			moduleNumericArrayTest(t, stored["options"], "characters", expected)
			for _, seat := range []int{-1, 0, n - 1} {
				view, err := mod.View(match, seat)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(view)
				if err != nil {
					t.Fatal(err)
				}
				moduleNumericArrayTest(t, raw, "seatSlots", seatSlots(n))
				var fields map[string]json.RawMessage
				if err = json.Unmarshal(raw, &fields); err != nil {
					t.Fatal(err)
				}
				moduleNumericArrayTest(t, fields["options"], "characters", expected)
				snapshot, err := mod.SyncSnapshot(match, seat)
				if err != nil {
					t.Fatal(err)
				}
				raw, err = json.Marshal(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(raw, &fields); err != nil {
					t.Fatal(err)
				}
				moduleNumericArrayTest(t, fields["view"], "seatSlots", seatSlots(n))
				var outer map[string]json.RawMessage
				if err = json.Unmarshal(fields["view"], &outer); err != nil {
					t.Fatal(err)
				}
				moduleNumericArrayTest(t, outer["options"], "characters", expected)
			}
		})
	}
}
