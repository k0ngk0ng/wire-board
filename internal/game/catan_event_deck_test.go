package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func eventDeckJSON(t *testing.T, d catanEventDeck) []byte {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCatanEventDeckNewYear(t *testing.T) {
	d := newCatanEventDeck()
	for cycle := uint64(1); cycle <= 4; cycle++ {
		// On subsequent cycles next() already returned the first normal card.
		first := len(d.Discard)
		if d.Cycle != cycle || first > 1 {
			t.Fatalf("unexpected new cycle: %+v", d)
		}
		bottom := slices.Clone(d.DrawPile[:5])
		seen := make(map[int]bool)
		for _, id := range d.Discard {
			seen[id] = true
		}
		for count := first; count < 31; count++ {
			got, err := d.next()
			if err != nil || got.NewYear || got.Card < 0 || got.Card >= 36 || seen[got.Card] || slices.Contains(bottom, got.Card) {
				t.Fatalf("draw %d of cycle %d: %+v, %v", count+1, cycle, got, err)
			}
			seen[got.Card] = true
			if err := d.validate(); err != nil {
				t.Fatal(err)
			}
		}
		if len(seen) != 31 || !slices.Equal(d.DrawPile[:5], bottom) || len(d.DrawPile) != 6 {
			t.Fatalf("five hidden cards must survive until New Year: %+v", d)
		}
		got, err := d.next()
		if err != nil || !got.NewYear || got.Card < 0 || got.Card >= 36 {
			t.Fatalf("New Year must draw a normal card in the same action: %+v, %v", got, err)
		}
		if d.Cycle != cycle+1 || len(d.DrawPile) != 36 || !slices.Equal(d.Discard, []int{got.Card}) {
			t.Fatalf("New Year must reset the discard and reshuffle all cards: %+v", d)
		}
		if err := d.validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatanEventDeckRestoreEveryPosition(t *testing.T) {
	d := newCatanEventDeck()
	for count := 0; count <= 31; count++ {
		data := eventDeckJSON(t, d)
		var restored catanEventDeck
		if err := json.Unmarshal(data, &restored); err != nil || !reflect.DeepEqual(d, restored) {
			t.Fatalf("restore after %d normal draws: %v", count, err)
		}
		before, err := d.view()
		if err != nil {
			t.Fatal(err)
		}
		after, err := restored.view()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("public view changed after restart: %v", err)
		}
		if count == 31 {
			next, err := restored.next()
			if err != nil || !next.NewYear || restored.Cycle != 2 || len(restored.Discard) != 1 {
				t.Fatalf("restore at New Year lost reshuffle boundary: %+v, %v", next, err)
			}
			break
		}
		expected := d.DrawPile[len(d.DrawPile)-1]
		x, err := restored.next()
		if err != nil || x.Card != expected || x.NewYear {
			t.Fatalf("restore lost next card: %+v, %v", x, err)
		}
		// Advancing the restored instance must not mutate the saved instance.
		if string(eventDeckJSON(t, d)) != string(data) {
			t.Fatal("restored deck aliases live state")
		}
		y, err := d.next()
		if err != nil || x != y || !reflect.DeepEqual(d, restored) {
			t.Fatalf("continuation differs after restart: %v", err)
		}
	}
}

func TestCatanEventDeckPublicView(t *testing.T) {
	d := newCatanEventDeck()
	for count := 0; count <= 31; count++ {
		v, err := d.view()
		if err != nil {
			t.Fatal(err)
		}
		if v.Remaining != 36-count || v.UntilNewYear != 31-count || v.Cycle != 1 || !slices.Equal(v.Revealed, d.Discard) {
			t.Fatalf("incorrect public counts: %+v", v)
		}
		if count == 0 && v.Current != nil || count > 0 && (v.Current == nil || *v.Current != d.Discard[count-1]) {
			t.Fatalf("incorrect current card: %+v", v)
		}
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var public map[string]json.RawMessage
		if err := json.Unmarshal(data, &public); err != nil {
			t.Fatal(err)
		}
		for key := range public {
			if !slices.Contains([]string{"cycle", "revealed", "current", "remaining", "untilNewYear"}, key) {
				t.Fatalf("unexpected public field: %s", key)
			}
		}
		// Hidden order (even the bottom five) must not influence the public view.
		other := d
		other.DrawPile = slices.Clone(d.DrawPile)
		other.DrawPile[0], other.DrawPile[len(other.DrawPile)-1] = other.DrawPile[len(other.DrawPile)-1], other.DrawPile[0]
		if count == 31 { // Top is New Year; swap only the hidden normal cards.
			other.DrawPile = slices.Clone(d.DrawPile)
			other.DrawPile[0], other.DrawPile[4] = other.DrawPile[4], other.DrawPile[0]
		}
		hiddenView, err := other.view()
		if err != nil || !reflect.DeepEqual(v, hiddenView) {
			t.Fatalf("hidden order leaked: %v", err)
		}
		before := eventDeckJSON(t, d)
		if count > 0 {
			v.Revealed[0] = -100
			*v.Current = -100
		}
		if string(before) != string(eventDeckJSON(t, d)) {
			t.Fatal("public view aliases private deck")
		}
		if count < 31 {
			if _, err := d.next(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCatanEventDeckRejectCorruptSave(t *testing.T) {
	mutations := map[string]func(*catanEventDeck){
		"zero cycle":             func(d *catanEventDeck) { d.Cycle = 0 },
		"missing card":           func(d *catanEventDeck) { d.DrawPile = d.DrawPile[:len(d.DrawPile)-1] },
		"extra card":             func(d *catanEventDeck) { d.DrawPile = append(d.DrawPile, 0) },
		"duplicate across piles": func(d *catanEventDeck) { d.DrawPile[0] = d.Discard[0] },
		"negative id":            func(d *catanEventDeck) { d.DrawPile[0] = -1 },
		"unknown id":             func(d *catanEventDeck) { d.DrawPile[0] = 37 },
		"misplaced New Year":     func(d *catanEventDeck) { d.DrawPile[4], d.DrawPile[5] = d.DrawPile[5], d.DrawPile[4] },
		"discarded New Year":     func(d *catanEventDeck) { d.DrawPile[5], d.Discard[0] = d.Discard[0], d.DrawPile[5] },
		"drew bottom card":       func(d *catanEventDeck) { d.Discard = append(d.Discard, d.DrawPile[5:]...); d.DrawPile = d.DrawPile[:5] },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			d := newCatanEventDeck()
			if _, err := d.next(); err != nil {
				t.Fatal(err)
			}
			mutate(&d)
			bad := eventDeckJSON(t, d)
			if _, err := d.next(); err == nil || string(bad) != string(eventDeckJSON(t, d)) {
				t.Fatal("invalid draw must fail without mutation")
			}
			if _, err := d.view(); err == nil {
				t.Fatal("invalid deck produced a public view")
			}
			existing := newCatanEventDeck()
			before := eventDeckJSON(t, existing)
			if err := json.Unmarshal(bad, &existing); err == nil || string(before) != string(eventDeckJSON(t, existing)) {
				t.Fatal("invalid save must fail without mutation")
			}
		})
	}
	for _, bad := range []string{"null", "{}", `{"cycle":1,"drawPile":[`, `{"cycle":-1}`, `{"cycle":"1"}`} {
		d := newCatanEventDeck()
		before := eventDeckJSON(t, d)
		if err := json.Unmarshal([]byte(bad), &d); err == nil || string(before) != string(eventDeckJSON(t, d)) {
			t.Fatalf("malformed save must fail atomically: %s", bad)
		}
	}
}

func TestCatanEventDeckCycleOverflow(t *testing.T) {
	d := newCatanEventDeck()
	d.Cycle = ^uint64(0)
	for range 31 {
		if _, err := d.next(); err != nil {
			t.Fatal(err)
		}
	}
	before := eventDeckJSON(t, d)
	if _, err := d.next(); err == nil || string(before) != string(eventDeckJSON(t, d)) {
		t.Fatal("cycle overflow must fail atomically")
	}
}
