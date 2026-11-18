package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestOriginalBattleAssetPlanAndFreshSavedContinuation(t *testing.T) {
	if os.Getenv("PAL_BATTLE_ASSETS_E2E") != "1" {
		t.Skip("explicit original battle asset candidate")
	}
	root := filepath.Join("..", "..", "..")
	ctx := context.Background()
	const legacy = ".local/pal/screen-input/battle-current/rules.wasm"
	const candidate = ".local/pal/dense-battle-candidate/battle/rules.wasm"
	load := func(relative string) *Runtime {
		t.Helper()
		wasm, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		r, err := NewRuntime(ctx, wasm, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close(ctx) })
		return r
	}
	step := func(r *Runtime, s Snapshot, key uint32) Snapshot {
		t.Helper()
		next, err := r.StepWithInput(ctx, s, Input{Direction: 4, Keys: key})
		if err != nil {
			t.Fatal(err)
		}
		return next
	}
	start := func(r *Runtime, roles []int) Snapshot {
		t.Helper()
		s, err := r.StartCampaignForPlayers(ctx, 12345, 1)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 32 && s.View.Battle.State != 1; i++ {
			s = step(r, s, 0)
		}
		if s.View.Battle.State != 1 || !s.View.Battle.Active || !slices.Equal(s.View.Party, roles) {
			t.Fatal("original command selection did not open")
		}
		return s
	}
	old, current := load(legacy), load(candidate)
	before, after := start(old, []int{0, 1, 2}), start(current, []int{0, 1, 2})
	if len(before.View.BattleAssets) != 0 || !reflect.DeepEqual(before.View.Battle, after.View.Battle) {
		t.Fatal("old core or original battle state changed at the command gate")
	}
	for _, id := range []int{6, 27, 28, 29, 50} {
		if !slices.Contains(after.View.BattleAssets, [2]int{8, id}) {
			t.Fatal("missing original spell atlas", id, after.View.BattleAssets)
		}
	}
	if len(after.View.BattleAssets) > 19 || slices.Contains(after.View.BattleAssets, [2]int{8, 54}) {
		t.Fatal("prefetch contains unrelated resources", after.View.BattleAssets)
	}
	before = step(old, before, 32)
	after = step(current, after, 32)
	before = step(old, before, 2)
	after = step(current, after, 2)
	for i := 0; i < 16; i++ {
		before, after = step(old, before, 0), step(current, after, 0)
	}
	encoded, err := current.Encode(after)
	if err != nil {
		t.Fatal(err)
	}
	fresh := load(candidate)
	restored, err := fresh.Decode(encoded)
	if err != nil || !slices.Equal(restored.View.BattleAssets, after.View.BattleAssets) {
		t.Fatal("fresh runner lost the battle resource plan", err)
	}
	for i := 0; i < 32; i++ {
		before, after, restored = step(old, before, 0), step(current, after, 0), step(fresh, restored, 0)
		if !reflect.DeepEqual(before.View.Battle, after.View.Battle) || before.View.Seed != after.View.Seed || before.View.Clock != after.View.Clock || before.Event != after.Event || before.View.Cash != after.View.Cash {
			t.Fatal("advisory plan changed original battle rules", i)
		}
		if !reflect.DeepEqual(after.View, restored.View) || after.MemoryID != restored.MemoryID || after.Event != restored.Event {
			t.Fatal("fresh save diverged during the original spell", i)
		}
	}
	bad := after
	bad.View.BattleAssets = [][2]int{{8, 50}, {8, 50}}
	if _, err := fresh.Encode(bad); err == nil {
		t.Fatal("duplicate metadata accepted in a native snapshot")
	}
	alternate := start(load(".local/pal/dense-battle-candidate/alternate/rules.wasm"), []int{3, 2, 5})
	if !slices.ContainsFunc(alternate.View.Battle.Enemies, func(enemy BattleEnemy) bool { return enemy.Object == 530 }) {
		t.Fatal("alternate fixture did not reach the original enemy caster")
	}
	for _, asset := range [][2]int{{6, 14}, {6, 16}, {8, 0}, {8, 44}, {8, 46}} {
		if !slices.Contains(alternate.View.BattleAssets, asset) {
			t.Fatal("alternate fixture missed original enemy or summon asset", asset, alternate.View.BattleAssets)
		}
	}
	if slices.Contains(alternate.View.BattleAssets, [2]int{8, 50}) {
		t.Fatal("alternate party fetched Xiaoyao's absent cooperative effect")
	}
	t.Logf("regular plan %v; alternate plan %v", after.View.BattleAssets, alternate.View.BattleAssets)
}
