package engine

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestArenaBackdropJSONPreservesCanonicalValue(t *testing.T) {
	state := ChuWorldState{Stream: make([]byte, 1024)}
	field := reflect.ValueOf(&state).Elem().FieldByName("BackdropColor")
	if !field.IsValid() || field.Kind() != reflect.Uint8 {
		t.Fatal("world is missing its canonical backdrop byte")
	}
	zero, err := json.Marshal(state)
	if err != nil || bytes.Contains(zero, []byte("BackdropColor")) {
		t.Fatal("default backdrop must preserve historical JSON bytes", err)
	}
	field.SetUint(42)
	raw, err := json.Marshal(state)
	if err != nil || !bytes.HasSuffix(raw, []byte(`,"BackdropColor":42}`)) {
		t.Fatal("serialization projection lost canonical backdrop or legacy field order", err)
	}
	var restored ChuWorldState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	got := reflect.ValueOf(restored).FieldByName("BackdropColor")
	if got.Uint() != 42 {
		t.Fatal("restored backdrop differs from canonical byte", got.Uint())
	}
	again, err := json.Marshal(restored)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("backdrop save/restore changes JSON bytes", err)
	}
}
