package cs16

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func cursorFixture(t *testing.T) (gameapi.Context, gameapi.Match) {
	t.Helper()
	ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.Unix(100000, 0)}
	match, err := start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, match
}

func TestCursorBrushReuseAndRecovery(t *testing.T) {
	ctx, match := cursorFixture(t)
	first, err := streamView(match, 0, "")
	if err != nil || first.Unchanged {
		t.Fatal("first projection missing", err)
	}
	initial := first.Data.(View)
	if initial.Brushes == nil || len(*initial.Brushes) == 0 {
		t.Fatal("first projection missing original brush states")
	}
	repeated, err := streamView(match, 0, first.Cursor)
	if err != nil || !repeated.Unchanged || repeated.Data != nil || repeated.Cursor != first.Cursor {
		t.Fatal("same projection not skipped", err)
	}
	other, err := streamView(match, 1, first.Cursor)
	if err != nil || other.Unchanged || !other.Data.(View).Recover || other.Data.(View).You != 1 {
		t.Fatal("cursor crossed authenticated seat scope", err)
	}
	ctx.Now = ctx.Now.Add(30 * time.Millisecond)
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	next, err := streamView(match, 0, first.Cursor)
	if err != nil || next.Unchanged {
		t.Fatal("new rule frame skipped", err)
	}
	advanced := streamTestMerge(t, initial, next.Data)
	raw, _ := json.Marshal(next.Data)
	if advanced.Brushes == nil || bytes.Contains(raw, []byte(`"brushes"`)) {
		t.Fatal("unchanged brush table repeated")
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Brushes[0].Open = !state.World.Brushes[0].Open
	changed, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	update, err := streamView(changed, 0, next.Cursor)
	if err != nil || update.Unchanged || len(update.Data.(DeltaView).Collections["brushes"].Upsert) != 1 {
		t.Fatal("changed brush table omitted", err)
	}
	state.World.Round++
	round, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	reset, err := streamView(round, 0, update.Cursor)
	if err != nil || reset.Unchanged || len(reset.Data.(DeltaView).Collections["brushes"].Upsert) != len(state.World.Brushes) {
		t.Fatal("round reset did not send full brush state", err)
	}
	state.World.Brushes = []engine.BrushState{}
	emptyMatch, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	empty, err := streamView(emptyMatch, 0, reset.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Data.(DeltaView).Collections["brushes"].Remove) != len(*initial.Brushes) {
		t.Fatal("empty brush state cannot clear cache")
	}
	for _, cursor := range []string{"other:0:0", "malformed", first.Cursor + ":extra"} {
		recovery, recoverErr := streamView(match, 0, cursor)
		if recoverErr != nil || recovery.Unchanged || !recovery.Data.(View).Recover || recovery.Data.(View).Brushes == nil || len(recovery.Data.(View).RulesSHA256) != 64 {
			t.Fatal("invalid cursor did not fully recover", cursor, recoverErr)
		}
	}
	state.World.Brushes = *initial.Brushes
	state.Epoch = "123456789012345678901234"
	epoch, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := streamView(epoch, 0, reset.Cursor)
	if err != nil || recovery.Unchanged || !recovery.Data.(View).Recover || recovery.Data.(View).Brushes == nil {
		t.Fatal("new epoch reused prior brushes", err)
	}
}

func TestRecoveryUsesVerifiedStaticRulesReference(t *testing.T) {
	sizes := []map[string]any{}
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault", "fy_iceworld", "cs_bloodstrike"} {
		ctx := gameapi.Context{Players: 10, Actor: 0, Now: time.Unix(100000, 0)}
		raw, _ := json.Marshal(map[string]any{"map": mapID})
		match, err := start(ctx, raw)
		if err != nil {
			t.Fatal(err)
		}
		first, err := streamView(match, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		full, err := syncSnapshot(match, 0)
		if err != nil {
			t.Fatal(err)
		}
		resync, err := streamView(match, 0, "lost-cursor")
		if err != nil {
			t.Fatal(err)
		}
		row := map[string]any{"map": mapID}
		for index, value := range []any{first.Data, full, resync.Data} {
			body, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(body, &fields) != nil {
				t.Fatal("projection not JSON")
			}
			if fields["map"] != nil || bytes.Contains(body, []byte(`"planes"`)) || bytes.Contains(body, []byte(`"surfaces"`)) || bytes.Contains(body, []byte(`"nav"`)) || len(body) > 64000 {
				t.Fatal("static map leaked into recovery", mapID, len(body))
			}
			var identity string
			if json.Unmarshal(fields["rulesSHA256"], &identity) != nil || len(identity) != 64 {
				t.Fatal("recovery missing verified rules identity", mapID)
			}
			if value.(View).MapID != mapID {
				t.Fatal("recovery changed map identity")
			}
			row[[]string{"initialBytes", "snapshotBytes", "cursorRecoveryBytes"}[index]] = len(body)
			row["rulesSHA256"] = identity
		}
		ctx.Now = ctx.Now.Add(30 * time.Millisecond)
		advanced, err := onTimer(ctx, match)
		if err != nil {
			t.Fatal(err)
		}
		legacy := expandedMatchState(t, advanced)
		legacy.Schema = 1
		legacyMatch := advanced
		legacyMatch.Data, err = json.Marshal(legacy)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := restore(ctx, legacyMatch)
		if err != nil {
			t.Fatal("legacy map checkpoint did not restore", mapID, err)
		}
		for seat := range legacy.World.Players {
			engine.ClearInput(&legacy.World, seat)
		}
		recovered := expandedMatchState(t, restored)
		expectedWorld, _ := json.Marshal(legacy.World)
		actualWorld, _ := json.Marshal(recovered.World)
		if recovered.Epoch != legacy.Epoch || recovered.BaseAt != legacy.BaseAt || recovered.RulesSHA256 != legacy.RulesSHA256 || !bytes.Equal(expectedWorld, actualWorld) {
			t.Fatal("legacy map restore changed identity, full history or rule state", mapID)
		}
		legacyView, err := syncSnapshot(restored, 0)
		if err != nil || legacyView.(View).Schema != 1 || legacyView.(View).MapID != mapID || legacyView.(View).RulesSHA256 != legacy.RulesSHA256 || !legacyView.(View).Recover {
			t.Fatal("legacy map restore did not project schema1 recovery", mapID, err)
		}
		legacyBytes, _ := json.Marshal(legacyView)
		row["legacySchema1RecoveryBytes"], row["legacyHistoryFrames"] = len(legacyBytes), len(recovered.World.History)
		sizes = append(sizes, row)
		state, err := decode(match)
		if err != nil {
			t.Fatal(err)
		}
		state.RulesSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		tampered, err := envelope(state, false, ctx.Now.UnixMilli())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decode(tampered); err == nil {
			t.Fatal("mismatched persisted rule identity accepted", mapID)
		}
	}
	directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "server")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"passed": true, "scope": "six-map 10-seat initial, cursor recovery and legacy schema1 checkpoint recovery dynamic projections, no static Map payload", "maps": sizes})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "recovery-size.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCursorPermissionsAndInputDeadline(t *testing.T) {
	ctx, match := cursorFixture(t)
	first, err := streamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	input := inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[0].Life, MoveY: 1000}}
	raw, _ := json.Marshal(input)
	ctx.Now = ctx.Now.Add(31 * time.Millisecond)
	queued, err := apply(ctx, match, raw)
	if err != nil {
		t.Fatal(err)
	}
	if queued.SystemDeadline != match.SystemDeadline {
		t.Fatal("input postponed rule deadline")
	}
	unchanged, err := streamView(queued, 0, first.Cursor)
	if err != nil || !unchanged.Unchanged {
		t.Fatal("received-only input repeated entire projection", err)
	}
	state, err = decode(queued)
	if err != nil {
		t.Fatal(err)
	}
	state.ObservedAt = state.BaseAt + 201
	lagged, err := envelope(state, false, state.ObservedAt)
	if err != nil {
		t.Fatal(err)
	}
	permission, err := streamView(lagged, 0, first.Cursor)
	permissionView := streamTestMerge(t, first.Data.(View), permission.Data)
	if err != nil || permission.Unchanged || !permissionView.CatchingUp || permissionView.CanAct {
		t.Fatal("catchup permission change was skipped", err)
	}
	ctx.Actor = 0
	reclaimed, err := reclaim(ctx, queued)
	if err != nil || reclaimed.SystemDeadline != queued.SystemDeadline {
		t.Fatal("handoff postponed rule deadline", err)
	}
}
