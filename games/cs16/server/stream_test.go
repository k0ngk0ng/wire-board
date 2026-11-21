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
	advanced := next.Data.(View)
	raw, _ := json.Marshal(advanced)
	if advanced.Brushes != nil || bytes.Contains(raw, []byte(`"brushes"`)) {
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
	if err != nil || update.Unchanged || update.Data.(View).Brushes == nil {
		t.Fatal("changed brush table omitted", err)
	}
	state.World.Round++
	round, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	reset, err := streamView(round, 0, update.Cursor)
	if err != nil || reset.Unchanged || reset.Data.(View).Brushes == nil {
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
	emptyRaw, _ := json.Marshal(empty.Data)
	if !bytes.Contains(emptyRaw, []byte(`"brushes":[]`)) {
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
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"} {
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
	body, err := json.Marshal(map[string]any{"passed": true, "scope": "10-seat initial and recovery dynamic projections, no static Map payload", "maps": sizes})
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
	if err != nil || permission.Unchanged || !permission.Data.(View).CatchingUp || permission.Data.(View).CanAct {
		t.Fatal("catchup permission change was skipped", err)
	}
	ctx.Actor = 0
	reclaimed, err := reclaim(ctx, queued)
	if err != nil || reclaimed.SystemDeadline != queued.SystemDeadline {
		t.Fatal("handoff postponed rule deadline", err)
	}
}
