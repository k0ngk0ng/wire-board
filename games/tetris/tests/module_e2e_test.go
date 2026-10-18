package tetris_test

// Failure scenarios and protocol are specified in ../CONTRACT.md before
// implementation. These are production Module flows, not a duplicate engine.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/tetris/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func raw(v any) json.RawMessage {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}

func begin(t *testing.T, n int, mode string) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	mod := game.Module()
	ctx := gameapi.Context{Players: n, Now: time.Unix(100000, 0)}
	m, e := mod.Start(ctx, raw(map[string]any{"mode": mode}))
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(m, n); e != nil {
		t.Fatal(e)
	}
	return mod, ctx, m
}

func view(t *testing.T, mod gameapi.Module, m gameapi.Match, seat int) game.View {
	t.Helper()
	v, e := mod.View(m, seat)
	if e != nil {
		t.Fatal(e)
	}
	var out game.View
	if e = json.Unmarshal(raw(v), &out); e != nil {
		t.Fatal(e)
	}
	return out
}

func trace(t *testing.T, value any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "tetris", "module")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(value, "", "  ")
	name := strings.ReplaceAll(t.Name(), "/", "-")
	if e := os.WriteFile(filepath.Join(dir, name+".json"), b, 0644); e != nil {
		t.Fatal(e)
	}
}

func step(t *testing.T, mod gameapi.Module, ctx *gameapi.Context, m gameapi.Match) gameapi.Match {
	t.Helper()
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	for _, actor := range append([]int{}, m.Actors...) {
		if m.Result != nil {
			break
		}
		ctx.Actor = actor
		before := append([]byte{}, m.Data...)
		action, e := mod.BotAction(*ctx, m)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(before, m.Data) {
			t.Fatal("bot mutated the match")
		}
		if len(action) == 0 {
			continue
		}
		var a struct {
			Type string `json:"type"`
		}
		if e = json.Unmarshal(action, &a); e != nil || a.Type != "inputs" {
			t.Fatalf("bot action is not an inputs batch: %s (%v)", action, e)
		}
		m, e = mod.Apply(*ctx, m, action)
		if e != nil {
			t.Fatalf("bot action rejected %s: %v", action, e)
		}
	}
	ctx.Actor = -1
	next, e := mod.OnTimer(*ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(next, ctx.Players); e != nil {
		t.Fatal(e)
	}
	return next
}

func TestTetrisCompleteBotGames(t *testing.T) {
	for _, mode := range []string{"survival", "sprint"} {
		for n := 1; n <= 4; n++ {
			t.Run(fmt.Sprintf("%s-%d-seats", mode, n), func(t *testing.T) {
				mod, ctx, m := begin(t, n, mode)
				initial := view(t, mod, m, -1)
				records := []any{map[string]any{"scope": "production Start/BotAction/Apply/OnTimer; accelerated clock only", "initial": initial}}
				defer func() { trace(t, records) }()
				for steps := 0; steps < 7000 && m.Result == nil; steps++ {
					m = step(t, mod, &ctx, m)
					if steps%100 == 0 {
						v := view(t, mod, m, -1)
						records = append(records, map[string]any{"at": v.ServerAt, "players": v.Players})
					}
				}
				v := view(t, mod, m, -1)
				records = append(records, map[string]any{"final": v, "result": m.Result})
				if m.Result == nil {
					t.Fatal("bot game did not finish")
				}
				if e := mod.ValidateMatch(m, n); e != nil {
					t.Fatal(e)
				}
				if len(m.Result.Ranks) != n || len(m.Result.Scores) != n {
					t.Fatalf("result sizes ranks=%v scores=%v", m.Result.Ranks, m.Result.Scores)
				}
				if n == 1 {
					if !m.Result.Unrated || len(m.Result.Winners) != 1 || m.Result.Winners[0] != 0 || m.Result.Ranks[0] != 1 {
						t.Fatalf("practice result=%+v", m.Result)
					}
				} else if m.Result.Unrated {
					t.Fatalf("multiplayer result unexpectedly unrated: %+v", m.Result)
				}
				for i, p := range v.Players {
					if p.Seat != i {
						t.Fatalf("seat order %d=%d", i, p.Seat)
					}
					if p.Queue != nil || p.Hold != 0 || p.Seq != 0 || (p.Active != nil && p.Active.GhostY != nil) {
						t.Fatalf("public view leaked private data for seat %d: %+v", i, p)
					}
				}
				if mode == "sprint" && !m.Result.Draw {
					winner := m.Result.Winners[0]
					if v.Players[winner].Lines < 40 {
						// A time-limit finish can end before 40 lines; a natural
						// sprint finish must have a 40-line winner.
						if m.Result.Details != nil {
							var d map[string]any
							_ = json.Unmarshal(m.Result.Details, &d)
							if d["reason"] == "sprint_finished" {
								t.Fatalf("sprint winner has %d lines", v.Players[winner].Lines)
							}
						}
					}
				}
			})
		}
	}
}

func TestTetrisViewPrivacyAndControls(t *testing.T) {
	mod, ctx, m := begin(t, 3, "survival")
	ctx.Now = time.UnixMilli(m.SystemDeadline)
	ctx.Actor = -1
	m, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	for seat := 0; seat < 3; seat++ {
		v := view(t, mod, m, seat)
		if len(v.Players) != 3 {
			t.Fatalf("players=%d", len(v.Players))
		}
		for i, p := range v.Players {
			if i == seat {
				if p.Queue == nil || len(p.Queue) != 3 || p.Active == nil || p.Active.GhostY == nil {
					t.Fatalf("own private data missing for seat %d: %+v", seat, p)
				}
				if !v.Controls.CanPlay {
					t.Fatalf("own controls disabled for seat %d", seat)
				}
			} else if p.Queue != nil || p.Hold != 0 || p.Seq != 0 || (p.Active != nil && p.Active.GhostY != nil) {
				t.Fatalf("opponent private data leaked to seat %d: %+v", seat, p)
			}
		}
	}
	spectator := view(t, mod, m, -1)
	if spectator.Controls.CanPlay || spectator.Controls.CanHold {
		t.Fatalf("spectator controls=%+v", spectator.Controls)
	}
	for _, p := range spectator.Players {
		if p.Queue != nil || p.Hold != 0 || p.Seq != 0 || (p.Active != nil && p.Active.GhostY != nil) {
			t.Fatalf("spectator private data leaked: %+v", p)
		}
	}
}

func TestTetrisRejectsInvalidActions(t *testing.T) {
	mod, ctx, m := begin(t, 2, "survival")
	ctx.Now = time.UnixMilli(m.SystemDeadline)
	ctx.Actor = -1
	m, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = 0
	v := view(t, mod, m, 0)
	base := map[string]any{"type": "inputs", "epoch": v.Epoch, "seq": 1, "events": []map[string]any{{"kind": "left"}}}
	cases := []map[string]any{
		{"type": "inputs", "epoch": "wrong", "seq": 2, "events": []map[string]any{{"kind": "left"}}},
		{"type": "inputs", "epoch": v.Epoch, "seq": 0, "events": []map[string]any{{"kind": "left"}}},
		{"type": "inputs", "epoch": v.Epoch, "seq": 1, "events": []map[string]any{{"kind": "teleport"}}},
		{"type": "inputs", "epoch": v.Epoch, "seq": 1, "events": []map[string]any{{"kind": "left", "x": 99}}},
	}
	for _, action := range cases {
		if _, e := mod.Apply(ctx, m, raw(action)); e == nil {
			t.Fatalf("invalid action accepted: %v", action)
		}
	}
	if _, e := mod.Apply(ctx, m, raw(base)); e != nil {
		t.Fatalf("valid action rejected: %v", e)
	}
	ctx.Actor = -1
	if _, e := mod.Apply(ctx, m, raw(map[string]any{"type": "inputs", "epoch": v.Epoch, "seq": 2, "events": []map[string]any{{"kind": "left"}}})); e == nil {
		t.Fatal("spectator action accepted")
	}
}

func TestTetrisRestorePreservesGameplay(t *testing.T) {
	mod, ctx, m := begin(t, 2, "survival")
	ctx.Now = time.UnixMilli(m.SystemDeadline)
	ctx.Actor = -1
	m, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(50 * time.Millisecond)
	action := raw(map[string]any{"type": "inputs", "epoch": view(t, mod, m, 0).Epoch, "seq": 1, "events": []map[string]any{{"kind": "hardDrop"}}})
	m, e = mod.Apply(ctx, m, action)
	if e != nil {
		t.Fatal(e)
	}
	before := view(t, mod, m, -1)
	restored, e := mod.Restore(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	after := view(t, mod, restored, -1)
	if before.Epoch != after.Epoch || before.ServerAt != after.ServerAt || before.MatchEndsAt != after.MatchEndsAt {
		t.Fatalf("restore changed match identity: before=%+v after=%+v", before, after)
	}
	for i := range before.Players {
		b, a := before.Players[i], after.Players[i]
		if b.Lines != a.Lines || b.Score != a.Score || b.Alive != a.Alive || strings.Join(b.Board, "") != strings.Join(a.Board, "") {
			t.Fatalf("restore changed player %d: before=%+v after=%+v", i, b, a)
		}
	}
	if restored.SystemDeadline != m.SystemDeadline {
		t.Fatalf("restore changed a live system deadline: %d -> %d", m.SystemDeadline, restored.SystemDeadline)
	}
}
