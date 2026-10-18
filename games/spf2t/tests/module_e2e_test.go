package spf2t_test

// Production module flows with an accelerated clock. Failure scenarios are
// listed in ../CONTRACT.md before the implementation.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/spf2t/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func raw(v any) json.RawMessage {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}

func viewOf(t *testing.T, mod gameapi.Module, m gameapi.Match, seat int) game.View {
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

func begin(t *testing.T, format string) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	mod := game.Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0)}
	m, e := mod.Start(ctx, raw(game.Options{Format: format}))
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(m, 2); e != nil {
		t.Fatal(e)
	}
	v := viewOf(t, mod, m, -1)
	for seat := 0; seat < 2; seat++ {
		ctx.Actor = seat
		m, e = mod.Apply(ctx, m, raw(game.Action{Type: "select", Epoch: v.Epoch, Character: v.Characters[seat].ID}))
		if e != nil {
			t.Fatal(e)
		}
	}
	ctx.Actor = -1
	return mod, ctx, m
}

func trace(t *testing.T, value any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "spf2t", "module")
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
		m, e = mod.Apply(*ctx, m, action)
		if e != nil {
			t.Fatalf("bot action rejected: %v (%s)", e, action)
		}
	}
	ctx.Actor = -1
	next, e := mod.OnTimer(*ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(next, 2); e != nil {
		t.Fatal(e)
	}
	return next
}

func TestSPF2TCompleteBotGames(t *testing.T) {
	for _, format := range []string{game.FormatSingle, game.FormatBestOf3} {
		t.Run(format, func(t *testing.T) {
			mod, ctx, m := begin(t, format)
			initial := viewOf(t, mod, m, -1)
			records := []any{map[string]any{"scope": "production Start/BotAction/Apply/OnTimer; accelerated clock only", "initial": initial}}
			defer func() { trace(t, records) }()
			for steps := 0; steps < 7000 && m.Result == nil; steps++ {
				m = step(t, mod, &ctx, m)
				if steps%200 == 0 {
					v := viewOf(t, mod, m, -1)
					records = append(records, map[string]any{"at": v.ServerAt, "phase": v.Phase, "round": v.Round, "players": v.Players})
				}
			}
			final := viewOf(t, mod, m, -1)
			records = append(records, map[string]any{"final": final, "result": m.Result})
			if m.Result == nil {
				t.Fatal("bot game did not finish")
			}
			if e := mod.ValidateMatch(m, 2); e != nil {
				t.Fatal(e)
			}
			if len(m.Result.Ranks) != 2 || len(m.Result.Scores) != 2 {
				t.Fatalf("result sizes ranks=%v scores=%v", m.Result.Ranks, m.Result.Scores)
			}
			if m.Result.Draw {
				if len(m.Result.Winners) != 0 || m.Result.Ranks[0] != 1 || m.Result.Ranks[1] != 1 {
					t.Fatalf("invalid draw result: %+v", m.Result)
				}
			} else {
				if len(m.Result.Winners) != 1 || m.Result.Ranks[m.Result.Winners[0]] != 1 {
					t.Fatalf("invalid winner result: %+v", m.Result)
				}
			}
			spectator := viewOf(t, mod, m, -1)
			for i, p := range spectator.Players {
				if p.Seq != 0 {
					t.Fatalf("spectator saw seat %d seq", i)
				}
			}
		})
	}
}

func TestSPF2TSelectionTimeoutAndSameCharacter(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(200000, 0)}
	m, e := mod.Start(ctx, raw(game.Options{Format: game.FormatBestOf3}))
	if e != nil {
		t.Fatal(e)
	}
	v := viewOf(t, mod, m, -1)
	if len(v.Characters) != 8 {
		t.Fatalf("characters = %d", len(v.Characters))
	}
	// Both seats pick the same character.
	for seat := 0; seat < 2; seat++ {
		ctx.Actor = seat
		m, e = mod.Apply(ctx, m, raw(game.Action{Type: "select", Epoch: v.Epoch, Character: "sakura"}))
		if e != nil {
			t.Fatal(e)
		}
	}
	v = viewOf(t, mod, m, 0)
	if v.Players[0].Character != "sakura" || v.Players[1].Character != "sakura" {
		t.Fatalf("same-character selection lost: %+v", v.Players)
	}
	// A fresh match auto-selects both after the deadline.
	m2, e := mod.Start(ctx, raw(game.Options{Format: game.FormatSingle}))
	if e != nil {
		t.Fatal(e)
	}
	v2 := viewOf(t, mod, m2, -1)
	ctx.Actor = -1
	ctx.Now = time.UnixMilli(v2.SelectUntil)
	m2, e = mod.OnTimer(ctx, m2)
	if e != nil {
		t.Fatal(e)
	}
	v2 = viewOf(t, mod, m2, -1)
	if v2.Phase != game.PhaseCountdown {
		t.Fatalf("timeout phase = %s", v2.Phase)
	}
	for i, p := range v2.Players {
		if !p.Selected || p.Character == "" {
			t.Fatalf("seat %d was not auto-selected: %+v", i, p)
		}
	}
}

func TestSPF2TRejectsInvalidAndStaleActions(t *testing.T) {
	mod, ctx, m := begin(t, game.FormatBestOf3)
	v := viewOf(t, mod, m, 0)
	ctx.Actor = 0
	cases := []game.Action{
		{Type: "inputs", Epoch: "wrong", Seq: 1, Events: []game.Input{{Kind: "left"}}},
		{Type: "inputs", Epoch: v.Epoch, Seq: 0, Events: []game.Input{{Kind: "left"}}},
		{Type: "inputs", Epoch: v.Epoch, Seq: 1, Events: []game.Input{{Kind: "teleport"}}},
		{Type: "select", Epoch: v.Epoch, Character: "nobody"},
	}
	for _, a := range cases {
		if _, e := mod.Apply(ctx, m, raw(a)); e == nil {
			t.Fatalf("invalid action accepted: %+v", a)
		}
	}
	if mod.AcceptStale(ctx, m, raw(game.Action{Type: "inputs", Epoch: v.Epoch, Seq: 1, Events: []game.Input{{Kind: "left"}}})) {
		t.Fatal("stale input accepted during countdown")
	}
}
