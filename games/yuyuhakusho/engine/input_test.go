package engine

import (
	"encoding/json"
	"os"
	"testing"
)

// This test was added before input.go. The fixture must come from complete
// original-ROM calls whose observer passed the full coldboot equivalence check.
func TestControllerAgainstOriginalCalls(t *testing.T) {
	type record struct {
		Frame   int    `json:"frame"`
		Channel int    `json:"channel"`
		Six     bool   `json:"six_buttons"`
		Nibbles []byte `json:"raw_nibbles"`
		Swap    bool   `json:"swap_ac"`
		Before  []byte `json:"before"`
		After   []byte `json:"after"`
		D0      uint32 `json:"entry_d0"`
		D1      uint32 `json:"entry_d1"`
	}
	raw, err := os.ReadFile("testdata/input-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []record
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("empty original trace")
	}
	for _, r := range rows {
		previous := Buttons(r.Before[0]) | Buttons(r.Before[5])<<8
		held := Buttons(r.Nibbles[0]^15) | Buttons(r.Nibbles[1]^15)<<4
		device := MD3
		if r.Six {
			// MOVE.B preserves register high bytes. The original six-button
			// routine propagates their unused high nibble; retain it as evidence.
			held |= Buttons(r.D0&0xff00) | Buttons((r.D1&0xff00)<<4) | Buttons(r.Nibbles[2]^15)<<8
			device = MD6
		}
		got, err := SampleController(previous, held, device, r.Swap)
		if err != nil {
			t.Fatalf("frame %d channel %d: %v", r.Frame, r.Channel, err)
		}
		actual := []Buttons{got.Current, got.Previous, got.Changed, got.Pressed, got.Released}
		for i, a := range actual {
			want := Buttons(r.After[i]) | Buttons(r.After[i+5])<<8
			if a != want {
				t.Fatalf("frame %d channel %d field %d: got %03x want %03x", r.Frame, r.Channel, i, a, want)
			}
		}
	}
}

func TestControllerRejectsUnsupportedInputs(t *testing.T) {
	for _, buttons := range []Buttons{0x1000, 0x8000} {
		if err := ValidateControllerButtons(buttons); err == nil {
			t.Fatal("accepted non-button bits from platform input")
		}
	}
	if _, err := SampleController(0, 0, ControllerDevice(0), false); err == nil {
		t.Fatal("accepted unknown controller model")
	}
}

// These are boundary expectations from the static instruction sequence, not
// natural-game coverage. In particular MD6 -> MD3 clears the extension history.
func TestControllerBoundaryEdges(t *testing.T) {
	cases := []struct {
		name           string
		previous, held Buttons
		device         ControllerDevice
		swap           bool
		want           ControllerFrame
	}{
		{"press_base_and_extension", 0, 0x440, MD6, false, ControllerFrame{Current: 0x440, Changed: 0x440, Pressed: 0x440}},
		{"held_is_not_new_press", 0x440, 0x440, MD6, false, ControllerFrame{Current: 0x440, Previous: 0x440}},
		{"release_both_groups", 0x440, 0, MD6, false, ControllerFrame{Previous: 0x440, Changed: 0x440, Released: 0x440}},
		{"swap_before_edges", 0x40, 0x20, MD6, true, ControllerFrame{Current: 0x40, Previous: 0x40}},
		{"three_key_clears_extension_history", 0x440, 0x40, MD3, false, ControllerFrame{Current: 0x40, Previous: 0x40}},
		{"opposing_directions_are_preserved_for_game_rules", 0, 0xf, MD6, false, ControllerFrame{Current: 0xf, Changed: 0xf, Pressed: 0xf}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SampleController(tc.previous, tc.held, tc.device, tc.swap)
			if err != nil || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}
