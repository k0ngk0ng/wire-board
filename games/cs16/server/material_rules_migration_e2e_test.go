package cs16

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/data"
	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func materialRuleTransitions(t *testing.T) []materialRulesTransition {
	t.Helper()
	var rows []materialRulesTransition
	if err := json.Unmarshal([]byte(data.MaterialRulesCompatibilityJSON), &rows); err != nil || len(rows) != 4 {
		t.Fatal("expected four material-only original map transitions", err, len(rows))
	}
	return rows
}

func materialLegacySnapshot(state *matchState) legacyMatchState {
	return legacyMatchState{Schema: 1, Epoch: state.Epoch, BaseAt: state.BaseAt, ObservedAt: state.ObservedAt, RulesSHA256: state.RulesSHA256, World: state.World}
}

func TestMaterialRulesKnownLegacyRestorePreservesDynamicStateE2E(t *testing.T) {
	for _, row := range materialRuleTransitions(t) {
		t.Run(row.MapID, func(t *testing.T) {
			ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.Unix(100000, 0)}
			options, _ := json.Marshal(map[string]string{"map": row.MapID})
			match, err := start(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			state, err := decode(match)
			if err != nil {
				t.Fatal(err)
			}
			engine.Advance(&state.World, 3)
			ctx.Now = ctx.Now.Add(30 * time.Millisecond)
			state.World.Players[0].Health = 37
			state.World.Players[0].Input.MoveX = 1000
			for index := range state.World.Brushes {
				if state.World.Brushes[index].ID == 120 || index == 0 {
					state.World.Brushes[index].Open = true
					state.World.Brushes[index].Health = 0
				}
			}
			state.RulesSHA256 = row.From
			raw, _ := json.Marshal(materialLegacySnapshot(state))
			match.Data = raw
			if _, err = decode(match); err == nil {
				t.Fatal("ordinary decode accepted an old client rules identity before migration")
			}
			if _, err = decodeInputState(match, 0); err == nil {
				t.Fatal("input header accepted old rules without migration")
			}
			restored, err := restore(ctx, match)
			if err != nil {
				t.Fatal("known metadata-only legacy room did not restore", err)
			}
			actual, err := decode(restored)
			if err != nil {
				t.Fatal(err)
			}
			if err = loadHistory(actual); err != nil {
				t.Fatal(err)
			}
			for seat := range state.World.Players {
				engine.ClearInput(&state.World, seat)
			}
			expectedWorld, _ := json.Marshal(state.World)
			actualWorld, _ := json.Marshal(actual.World)
			if actual.RulesSHA256 != row.To || actual.Epoch != state.Epoch || actual.BaseAt != state.BaseAt || !bytes.Equal(expectedWorld, actualWorld) || len(actual.World.BrushDamageStates) != 0 {
				var expectedFields, actualFields map[string]json.RawMessage
				_ = json.Unmarshal(expectedWorld, &expectedFields)
				_ = json.Unmarshal(actualWorld, &actualFields)
				for key, expected := range expectedFields {
					if !bytes.Equal(expected, actualFields[key]) {
						t.Logf("world field %s differs: expected %d bytes actual %d bytes", key, len(expected), len(actualFields[key]))
					}
				}
				t.Fatal("material-only restore reset dynamic world, epoch, history or clock")
			}
			if projection(actual, 0).RulesSHA256 != rulesIdentities[row.MapID] {
				t.Fatal("restored view did not require current rules identity")
			}
		})
	}
}

func TestMaterialRulesUnknownAndCrossMapRestoreRejectE2E(t *testing.T) {
	rows := materialRuleTransitions(t)
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{strings.Repeat("0", 64), rows[1].From} {
		state.RulesSHA256 = identity
		match.Data, _ = json.Marshal(materialLegacySnapshot(state))
		before := bytes.Clone(match.Data)
		if _, err = restore(ctx, match); err == nil || !bytes.Equal(before, match.Data) {
			t.Fatal("unknown or cross-map legacy identity restored or mutated caller state", identity)
		}
	}
}

func TestBreakableDamageRemainderStaysPrivateE2E(t *testing.T) {
	state := observerFixture()
	state.World.BrushDamageStates = []engine.BrushDamageState{{ID: 1, HealthRemainder: 400000}}
	private, err := json.Marshal(state.World)
	if err != nil || !strings.Contains(string(private), "brushDamageStates") {
		t.Fatal("breakable fractional checkpoint missing", err)
	}
	for _, seat := range []int{-1, 0, 1} {
		raw, err := json.Marshal(projection(state, seat))
		if err != nil || strings.Contains(string(raw), "brushDamageStates") || strings.Contains(string(raw), "healthRemainder") {
			t.Fatal("breakable precision leaked into public projection", seat, err)
		}
	}
}
