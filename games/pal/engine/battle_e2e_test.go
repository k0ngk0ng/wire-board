package engine_test

// Written before the battle adapter. Isolated failure cases: original stats or
// enemies replaced by fixture values; wrong field clock in combat; a timed
// batch drives the next role; observing a suspended action corrupts C locals;
// a fresh saved runtime repeats rewards or changes HP, targets, RNG or time.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func battleRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	wasm, err := os.ReadFile(filepath.Join(root, ".local/pal/battle-fixtures/team-1-party-3/rules.wasm"))
	if err != nil {
		t.Fatal("build --battle-fixture 1 first", err)
	}
	r, err := engine.NewRuntime(context.Background(), wasm, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalBattleSavedActionsAndTiming(t *testing.T) {
	ctx := context.Background()
	r := battleRuntime(t)
	s, err := r.StartCampaignForPlayers(ctx, 12345, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; !(s.View.Battle.Active && s.View.Battle.State == 1); i++ {
		if i > 512 {
			t.Fatal("original battle did not accept commands")
		}
		s, err = r.Step(ctx, s, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Event.Value != 40 || len(s.View.Battle.Players) != 3 || s.View.Battle.Players[1].HP != 28 || s.View.Battle.Enemies[0].HP != 30 {
		t.Fatal("original resource stats/timing changed", s.View.Battle, s.Event)
	}
	for _, corrupt := range []func(*engine.Snapshot){
		func(x *engine.Snapshot) { x.View.Battle.Current = 3 },
		func(x *engine.Snapshot) { x.View.Battle.State = 6 },
		func(x *engine.Snapshot) { x.View.Battle.Players = nil },
		func(x *engine.Snapshot) { x.View.Battle.Enemies = make([]engine.BattleEnemy, 6) },
	} {
		bad := s
		corrupt(&bad)
		if _, err = r.Encode(bad); err == nil {
			t.Fatal("invalid battle snapshot accepted")
		}
	}
	// Even a 250 ms timer batch cannot submit a held input to the next role.
	advanced, err := r.AdvanceTimed(ctx, s, engine.Input{Direction: 0, Keys: 0x400}, 40, 250)
	if err != nil {
		t.Fatal(err)
	}
	s = advanced.Snapshot
	if s.View.Battle.Players[0].Action != 1 || s.View.Battle.Players[1].State == 2 || s.View.HeldDirection != 4 {
		t.Fatal("one input crossed a role boundary", s.View.Battle)
	}
	for i := 0; s.View.Battle.State != 1 || s.View.Battle.Current != 1; i++ {
		if i > 128 {
			t.Fatal("next original role unavailable")
		}
		s, err = r.Step(ctx, s, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := r.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	fresh := battleRuntime(t)
	restored, err := fresh.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	restored, err = fresh.ReleaseInputs(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.View.Battle, restored.View.Battle) || s.View.Clock != restored.View.Clock {
		t.Fatal("release advanced saved combat")
	}
	for i := 0; s.Event.Kind != engine.SceneComplete; i++ {
		if i > 1024 {
			t.Fatal("original battle did not finish")
		}
		keys := uint32(0)
		if s.View.Battle.Active && s.View.Battle.State != 0 {
			keys = 2
		}
		input := engine.Input{Direction: 4, Keys: keys}
		s, err = r.StepWithInput(ctx, s, input)
		if err != nil {
			t.Fatal(err)
		}
		restored, err = fresh.StepWithInput(ctx, restored, input)
		if err != nil {
			t.Fatal(err)
		}
		if s.Event != restored.Event || !reflect.DeepEqual(s.View, restored.View) {
			t.Fatal("fresh runtime diverged during combat")
		}
	}
	if s.Event.Value != 3 || s.View.Cash != 3 || s.View.Battle.Active {
		t.Fatal("real victory/reward missing", s.Event, s.View.Cash)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "isolated original battle, timed input boundary, fresh runtime saved continuation and actual reward; not full campaign", "saveBytes": len(encoded), "final": s.View.Battle, "cash": s.View.Cash}, "", "  ")
	if err = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/native-battle-go.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
