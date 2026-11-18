package pokepath_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestNativePlatformDexSharedAcrossFourSeatsAndSave(t *testing.T) {
	module, ctx, match := nativeStart(t, 4)
	ctx.Actor = 3
	match = nativeAction(t, module, ctx, match, `"type":"wave"`)
	ctx.Actor = -1
	for step := 0; step < 120; step++ {
		ctx.Now = ctx.Now.Add(time.Second)
		var err error
		match, err = module.OnTimer(ctx, match)
		if err != nil {
			t.Fatal(err)
		}
		if nativeObserve(t, module, match, 0).Wave > 1 {
			break
		}
	}
	readRecords := func(match gameapi.Match, seat int) map[string]struct {
		Defeated int `json:"defeated"`
		Escaped  int `json:"escaped"`
	} {
		view, err := module.View(match, seat)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		var value struct {
			Progression struct {
				EnemyStats map[string]struct {
					Defeated int `json:"defeated"`
					Escaped  int `json:"escaped"`
				} `json:"enemyStats"`
			} `json:"progression"`
		}
		if err := json.Unmarshal(payload, &value); err != nil {
			t.Fatal(err)
		}
		return value.Progression.EnemyStats
	}
	before := readRecords(match, 0)
	if before["rattata"].Escaped != 3 || before["rattata"].Defeated != 0 {
		view := nativeObserve(t, module, match, 0)
		t.Fatalf("original first wave without a tower must register three escapes, not discoveries: %+v at %dms, wave %d, health %d", before, view.Clock, view.Wave, view.Health)
	}
	for _, seat := range []int{-1, 1, 2, 3} {
		if got := readRecords(match, seat); !reflect.DeepEqual(got, before) {
			t.Fatalf("seat %d has a different authoritative dex: %+v", seat, got)
		}
	}
	ctx.Actor = 0
	frozen, err := module.SaveControl(ctx, match, true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	var cold gameapi.Match
	if err := json.Unmarshal(data, &cold); err != nil {
		t.Fatal(err)
	}
	ctx.Now = ctx.Now.Add(72 * time.Hour)
	after, err := module.ResumeSaved(ctx, cold)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRecords(after, 3); !reflect.DeepEqual(got, before) {
		t.Fatalf("cold continuation reset or advanced enemy statistics: %+v", got)
	}
}
