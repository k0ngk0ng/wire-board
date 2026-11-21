package yuyuhakusho

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestRandomSelectionsResolveOnceAtMatchStart(t *testing.T) {
	mod := Module()
	raw := json.RawMessage(`{"arena":65535,"characters":[255,9,255,255],"teamBattle":true,"oneVsThree":true}`)
	waiting, err := mod.Configure(4, raw)
	if err != nil {
		t.Fatal(err)
	}
	var selected Options
	if err = json.Unmarshal(waiting, &selected); err != nil || selected.Arena != 65535 || selected.Characters[0] != 255 {
		t.Fatal("random selection resolved before start", err, selected)
	}
	seenCharacters, seenArenas := map[uint8]bool{}, map[uint16]bool{}
	for run := 0; run < 256; run++ {
		ctx := moduleContextTest(4, 1000, []bool{false, true, true, true})
		m, err := mod.Start(ctx, waiting)
		if err != nil {
			t.Fatal(err)
		}
		saved := moduleCheckpointTest(t, mod, m, 0)
		if saved.World.Match.BaseTeams != [4]uint8{1, 4, 4, 4} || saved.World.Match.Grouping != 6 {
			t.Fatal("one-versus-three teams differ", saved.World.Match.BaseTeams)
		}
		var state State
		if err = json.Unmarshal(m.Data, &state); err != nil {
			t.Fatal(err)
		}
		if state.Options.Arena > 14 || state.Options.Arena&1 != 0 || state.Options.Characters[1] != 9 {
			t.Fatal("random arena or fixed character changed", state.Options)
		}
		seenArenas[state.Options.Arena] = true
		for _, character := range state.Options.Characters {
			if character > 10 {
				t.Fatal("unresolved random character", state.Options)
			}
			seenCharacters[character] = true
		}
		before := state.Options
		ctx.Actor = 0
		m = moduleActionTest(t, mod, ctx, m, map[string]any{"type": "loaded", "epoch": state.Epoch})
		ctx.Now = time.UnixMilli(1100)
		m, err = mod.Restore(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(m.Data, &state); err != nil || !reflect.DeepEqual(before, state.Options) {
			t.Fatal("loading or reconnect rerolled selections", err)
		}
	}
	if len(seenCharacters) != 11 || len(seenArenas) != 8 {
		t.Fatal("random selections do not reach the complete catalog", seenCharacters, seenArenas)
	}
	t.Logf("256 starts: all %d characters and %d arenas resolved once", len(seenCharacters), len(seenArenas))
}

func TestRandomAndOneVsThreePreparationAuthorization(t *testing.T) {
	mod := Module()
	raw, err := mod.Configure(4, nil)
	if err != nil {
		t.Fatal(err)
	}
	host := gameapi.PreparationContext{Actor: 0, Capacity: 4, Host: true, Bots: []bool{false, false, true, true}}
	guest := host
	guest.Actor, guest.Host = 1, false
	for _, tc := range []struct {
		ctx    gameapi.PreparationContext
		action string
		valid  bool
	}{
		{host, `{"type":"arena","arena":65535}`, true},
		{host, `{"type":"character","seat":2,"character":255}`, true},
		{guest, `{"type":"character","character":255}`, true},
		{host, `{"type":"mode","teamBattle":true,"oneVsThree":true}`, true},
		{guest, `{"type":"arena","arena":65535}`, false},
		{guest, `{"type":"mode","teamBattle":true,"oneVsThree":true}`, false},
		{host, `{"type":"character","seat":1,"character":255}`, false},
		{host, `{"type":"arena","arena":65534}`, false},
		{host, `{"type":"character","character":254}`, false},
		{host, `{"type":"mode","teamBattle":false,"oneVsThree":true}`, false},
		{host, `{"type":"arena","arena":0,"oneVsThree":true}`, false},
		{host, `{"type":"mode","teamBattle":true,"oneVsThree":null}`, false},
	} {
		_, err = mod.Prepare(tc.ctx, raw, json.RawMessage(tc.action))
		if (err == nil) != tc.valid {
			t.Fatalf("authorization differs: %s valid=%t err=%v", tc.action, tc.valid, err)
		}
	}
	for _, n := range []int{2, 3} {
		if _, err = mod.Configure(n, json.RawMessage(`{"teamBattle":true,"oneVsThree":true}`)); err == nil {
			t.Fatal("one-versus-three accepted without four seats", n)
		}
	}
	team, err := mod.Prepare(host, raw, json.RawMessage(`{"type":"mode","teamBattle":true,"oneVsThree":true}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{`{"type":"mode","teamBattle":true}`, `{"type":"mode","teamBattle":false}`} {
		next, err := mod.Prepare(host, team, json.RawMessage(action))
		var fields map[string]any
		if err != nil || json.Unmarshal(next, &fields) != nil || fields["oneVsThree"] == true {
			t.Fatal("switching mode retained one-versus-three", action, err)
		}
	}
}

func TestOneVsThreeCompleteComputerMatchAndTakeover(t *testing.T) {
	mod := Module()
	raw := json.RawMessage(`{"characters":[3,1,0,10],"teamBattle":true,"oneVsThree":true}`)
	ctx := moduleContextTest(4, 1000, []bool{true, true, true, true})
	m, err := mod.Start(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	for step := 1; step <= 4000 && m.Result == nil; step++ {
		now := int64(1000 + step*100)
		m = moduleTickTest(t, mod, ctx, m, now)
		if step == 100 {
			ctx.Now = time.UnixMilli(now)
			ctx.Actor = 1
			before := moduleCheckpointTest(t, mod, m, 1)
			m, err = mod.Reclaim(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			after := moduleCheckpointTest(t, mod, m, 1)
			if before.World.Match.BaseTeams != after.World.Match.BaseTeams || before.Frame != after.Frame {
				t.Fatal("manual takeover changed teams or clock")
			}
			m, err = mod.Restore(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if m.Result == nil || !m.Result.Team || !m.Result.Unrated || m.Result.Draw {
		t.Fatal("computer match did not finish with team result", m.Result)
	}
	cp := moduleCheckpointTest(t, mod, m, -1)
	want := []int{0}
	if cp.World.Match.WinnerCode == 4 {
		want = []int{1, 2, 3}
	}
	if !reflect.DeepEqual(m.Result.Winners, want) {
		t.Fatal("winning side mapped as two-versus-two", m.Result.Winners, want)
	}
	t.Logf("completed frame=%d winner=%d seats=%v", cp.Frame, cp.World.Match.WinnerCode, m.Result.Winners)
}
