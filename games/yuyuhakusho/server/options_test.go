package yuyuhakusho

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func waitingOptionsTest(t *testing.T, n int, raw string) Options {
	t.Helper()
	data, err := configure(n, json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	var o Options
	if err = json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	return o
}
func TestWaitingOptionsCapacityArenaAndCharacters(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		o := waitingOptionsTest(t, n, "")
		if len(o.Characters) != n || o.TeamBattle || o.Arena != 0 {
			t.Fatal("incomplete room defaults", n, o)
		}
		var raw json.RawMessage
		for _, arena := range []uint16{0, 2, 4, 6, 8, 10, 12, 14} {
			for id := uint8(0); id <= 10; id++ {
				characters := make([]uint8, n)
				for i := range characters {
					characters[i] = id
				}
				raw, _ = json.Marshal(Options{Arena: arena, Characters: characters, TeamBattle: n == 4})
				before := append([]byte(nil), raw...)
				actual, err := configure(n, raw)
				if err != nil || !bytes.Equal(raw, before) {
					t.Fatal("valid original configuration rejected or mutated", n, arena, id, err)
				}
				var got Options
				_ = json.Unmarshal(actual, &got)
				if !reflect.DeepEqual(got.Characters, characters) || got.Arena != arena || got.TeamBattle != (n == 4) {
					t.Fatal("room selection changed")
				}
			}
		}
	}
}
func TestWaitingOptionsInvalidRequestsAreAtomic(t *testing.T) {
	for _, tc := range []struct {
		n   int
		raw string
	}{{1, ""}, {5, ""}, {2, "null"}, {2, `{"arena":1}`}, {2, `{"arena":16}`}, {2, `{"arena":65536}`}, {2, `{"arena":"2"}`}, {2, `{"teamBattle":true}`}, {3, `{"teamBattle":true}`}, {2, `{"characters":[]}`}, {2, `{"characters":[0]}`}, {2, `{"characters":[0,11]}`}, {2, `{"characters":[0,-1]}`}, {2, `{"characters":[0,256]}`}, {2, `{"unknown":1}`}, {2, `{"arena":0,"arena":2}`}, {2, `{} {}`}} {
		raw := json.RawMessage(tc.raw)
		before := append([]byte(nil), raw...)
		out, err := configure(tc.n, raw)
		if err == nil || len(out) != 0 || !bytes.Equal(raw, before) {
			t.Fatal("invalid options accepted or partially committed", tc, err)
		}
	}
}
func TestWaitingPreparationSeatOwnershipAndHostRules(t *testing.T) {
	o := Options{Arena: 2, Characters: []uint8{3, 1, 9, 10}}
	raw, _ := json.Marshal(o)
	before := append([]byte(nil), raw...)
	call := func(ctx gameapi.PreparationContext, action string, ok bool) Options {
		t.Helper()
		out, err := prepare(ctx, raw, json.RawMessage(action))
		if (err == nil) != ok || !bytes.Equal(raw, before) {
			t.Fatal("preparation auth/atomicity differs", ctx, action, err)
		}
		if !ok {
			if len(out) != 0 {
				t.Fatal("failed preparation emitted partial options")
			}
			return Options{}
		}
		var got Options
		if err = json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	guest := gameapi.PreparationContext{Actor: 1, Capacity: 4, Bots: []bool{false, false, true}}
	got := call(guest, `{"type":"character","character":7}`, true)
	if !reflect.DeepEqual(got.Characters, []uint8{3, 7, 9, 10}) {
		t.Fatal("self selection changed other seats", got)
	}
	call(guest, `{"type":"character","seat":0,"character":7}`, false)
	call(guest, `{"type":"arena","arena":4}`, false)
	call(guest, `{"type":"mode","teamBattle":true}`, false)
	host := guest
	host.Actor = 0
	host.Host = true
	got = call(host, `{"type":"character","seat":2,"character":4}`, true)
	if !reflect.DeepEqual(got.Characters, []uint8{3, 1, 4, 10}) {
		t.Fatal("host bot selection changed another seat")
	}
	call(host, `{"type":"character","seat":1,"character":7}`, false)
	call(host, `{"type":"character","seat":3,"character":7}`, false)
	got = call(host, `{"type":"arena","arena":14}`, true)
	if got.Arena != 14 || !reflect.DeepEqual(got.Characters, o.Characters) {
		t.Fatal("arena preparation changed roster")
	}
	got = call(host, `{"type":"mode","teamBattle":true}`, true)
	if !got.TeamBattle {
		t.Fatal("four-seat team preparation was lost")
	}
	for _, action := range []string{`null`, `{"type":"unknown"}`, `{"type":"character"}`, `{"type":"arena"}`, `{"type":"mode"}`, `{"type":"mode","teamBattle":null}`, `{"type":"character","character":1,"arena":2}`, `{"type":"character","character":11}`, `{"type":"character","character":1,"seat":4}`, `{"type":"arena","arena":3}`, `{"type":"character","character":1,"automated":true}`, `{"type":"character","character":1,"character":2}`} {
		call(host, action, false)
	}
	missing := host
	missing.Actor = 3
	call(missing, `{"type":"character","character":1}`, false)
	malformed := host
	malformed.Bots = []bool{false, false, false, false, false}
	call(malformed, `{"type":"character","character":1}`, false)
}
func TestWaitingOriginalSlotAndControllerProjection(t *testing.T) {
	for _, tc := range []struct {
		n     int
		slots []uint8
	}{{2, []uint8{0, 3}}, {3, []uint8{0, 1, 3}}, {4, []uint8{0, 1, 2, 3}}} {
		o := waitingOptionsTest(t, tc.n, "")
		flags := make([]bool, tc.n)
		flags[tc.n-1] = true
		cfg, err := sessionOptions(o, flags, 0x12345678)
		if err != nil || len(cfg.Players) != tc.n {
			t.Fatal(err)
		}
		for seat, p := range cfg.Players {
			if p.Slot != tc.slots[seat] || p.CharacterID != o.Characters[seat] || p.Color != uint8(seat) || p.Computer != flags[seat] || p.HumanChannel != uint8(seat) {
				t.Fatal("seat/slot/channel/automation mapping differs", seat, p)
			}
		}
		if cfg.Seed != 0x12345678 || cfg.Arena != o.Arena || cfg.TeamBattle != o.TeamBattle {
			t.Fatal("configuration projection differs")
		}
		if _, err = sessionOptions(o, flags[:tc.n-1], 1); err == nil {
			t.Fatal("missing host automation flags accepted")
		}
	}
}
