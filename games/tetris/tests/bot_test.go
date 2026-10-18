package tetris_test

// Failure inventory: the computer reads the match seed, an opponent's bag or
// queue, mutates the authoritative match, or reclaim resets another seat's
// board/deadline.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func started(t *testing.T, n int, mode string) (gameapi.Module, *gameapi.Context, gameapi.Match) {
	t.Helper()
	mod, ctx, m := begin(t, n, mode)
	ctx.Now = time.UnixMilli(m.SystemDeadline)
	ctx.Actor = -1
	m, err := mod.OnTimer(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	return mod, &ctx, m
}

func TestTetrisBotIgnoresHiddenOpponentDataAndDoesNotMutate(t *testing.T) {
	mod, ctx, m := started(t, 2, "survival")
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(time.Second)
	before := append([]byte{}, m.Data...)
	action, err := mod.BotAction(*ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(action) == 0 {
		t.Fatal("bot did not act when due")
	}
	if string(before) != string(m.Data) {
		t.Fatal("bot mutated the match")
	}

	var data map[string]any
	if err = json.Unmarshal(m.Data, &data); err != nil {
		t.Fatal(err)
	}
	data["seed"] = float64(987654321)
	players := data["players"].([]any)
	other := players[1].(map[string]any)
	other["queue"] = []any{float64(1), float64(2), float64(3), float64(4), float64(5)}
	other["bag"] = []any{float64(6), float64(7)}
	other["rand"] = float64(123456)
	changedData, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	changed := m
	changed.Data = changedData
	otherAction, err := mod.BotAction(*ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(action, otherAction) {
		t.Fatalf("bot output changed with hidden opponent data:\n%s\n%s", action, otherAction)
	}
}

func TestTetrisReclaimOnlyRestoresOwnWindow(t *testing.T) {
	mod, ctx, m := started(t, 2, "survival")
	before := view(t, mod, m, -1)
	beforeDeadline := m.Deadline
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(500 * time.Millisecond)
	reclaimed, err := mod.Reclaim(*ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, mod, reclaimed, -1)
	if before.Epoch != after.Epoch || before.ServerAt != after.ServerAt {
		t.Fatalf("reclaim changed match identity: %+v -> %+v", before, after)
	}
	for seat := 0; seat < 2; seat++ {
		b, a := before.Players[seat], after.Players[seat]
		if b.Lines != a.Lines || b.Score != a.Score || b.Alive != a.Alive ||
			!reflect.DeepEqual(b.Board, a.Board) || !reflect.DeepEqual(b.Active, a.Active) {
			t.Fatalf("reclaim changed seat %d: %+v -> %+v", seat, b, a)
		}
	}
	beforeData := map[string]any{}
	afterData := map[string]any{}
	_ = json.Unmarshal(m.Data, &beforeData)
	_ = json.Unmarshal(reclaimed.Data, &afterData)
	idle := func(data map[string]any, seat int) float64 {
		players := data["players"].([]any)
		return players[seat].(map[string]any)["idleAt"].(float64)
	}
	if idle(afterData, 0) <= idle(beforeData, 0) {
		t.Fatalf("reclaim did not move this seat's window: %v -> %v", idle(beforeData, 0), idle(afterData, 0))
	}
	if idle(afterData, 1) != idle(beforeData, 1) {
		t.Fatalf("reclaim changed another seat's window: %v -> %v", idle(beforeData, 1), idle(afterData, 1))
	}
	if reclaimed.Deadline != beforeDeadline {
		t.Fatalf("reclaim changed the earliest surviving deadline: %d -> %d", beforeDeadline, reclaimed.Deadline)
	}
	if len(reclaimed.TimeoutActors) != 1 || reclaimed.TimeoutActors[0] != 1 {
		t.Fatalf("reclaim changed another seat's timeout ownership: %v", reclaimed.TimeoutActors)
	}
}
