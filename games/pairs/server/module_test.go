package pairs

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func TestRaceAllSeatsAndBots(t *testing.T) {
	mod := Module()
	for n := 1; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			ctx := gameapi.Context{Players: n, Now: time.Unix(1700000000, int64(n)*7919)}
			match, e := mod.Start(ctx, json.RawMessage(`{}`))
			if e != nil {
				t.Fatal(e)
			}
			v, _ := mod.View(match, 0)
			raw, _ := json.Marshal(v)
			var before View
			json.Unmarshal(raw, &before)
			if before.Players[0].Board != nil {
				t.Fatal("countdown leaked board")
			}
			ctx.Actor = -1
			ctx.Now = ctx.Now.Add(3 * time.Second)
			match, e = mod.OnTimer(ctx, match)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := decode(match)
			for p := 1; p < n; p++ {
				if !reflect.DeepEqual(s.Players[0].Board, s.Players[p].Board) {
					t.Fatal("unequal starts")
				}
			}
			for step := 0; step < 600 && match.Result == nil; step++ {
				ctx.Actor = step % n
				ctx.Now = ctx.Now.Add(time.Second)
				action, err := mod.BotAction(ctx, match)
				if err != nil || action == nil {
					t.Fatal("bot stalled", err)
				}
				match, err = mod.Apply(ctx, match, action)
				if err != nil {
					t.Fatal(err)
				}
				if err = mod.ValidateMatch(match, n); err != nil {
					t.Fatal(err)
				}
				projected, _ := mod.View(match, ctx.Actor)
				raw, _ := json.Marshal(projected)
				var fields map[string]any
				json.Unmarshal(raw, &fields)
				if fields["seed"] != nil || fields["solution"] != nil {
					t.Fatal("answer leaked")
				}
			}
			if match.Result == nil || len(match.Result.Winners) != 1 {
				t.Fatal("race never completed")
			}
			s, _ = decode(match)
			winner := match.Result.Winners[0]
			if TileCount(*s.Players[winner].Board) != 0 {
				t.Fatal("winner did not clear")
			}
			for p, rank := range match.Result.Ranks {
				if p != winner && rank != 2 {
					t.Fatal("losers ranked by progress")
				}
			}
		})
	}
}
func TestRevisionsToolsAndClocks(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(1700000000, 1), Actor: -1}
	m, _ := mod.Start(ctx, json.RawMessage(`{}`))
	ctx.Now = ctx.Now.Add(3 * time.Second)
	m, _ = mod.OnTimer(ctx, m)
	s, _ := decode(m)
	ctx.Actor = 0
	a := Action{Type: "hint", Round: s.Round, Revision: s.Players[0].Revision}
	raw, _ := json.Marshal(a)
	next, e := mod.Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := decode(next)
	if after.Players[0].Tools.Hint != 2 || after.Players[1].Due != s.Players[1].Due {
		t.Fatal("tool or other clock changed")
	}
	if mod.AcceptStale(ctx, next, raw) {
		t.Fatal("own stale request accepted")
	}
	if _, e = mod.Apply(ctx, next, raw); e == nil {
		t.Fatal("duplicate revision replayed")
	}
	ctx.Actor = 1
	a.Revision = s.Players[1].Revision
	raw, _ = json.Marshal(a)
	if !mod.AcceptStale(ctx, next, raw) {
		t.Fatal("other player's action blocked")
	}
	next, e = mod.Apply(ctx, next, raw)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(121 * time.Second)
	before, _ := decode(next)
	next, e = mod.Reclaim(ctx, next)
	if e != nil {
		t.Fatal(e)
	}
	after, _ = decode(next)
	if after.Players[1].Due != before.Players[1].Due || after.Players[0].Due <= ctx.Now.UnixMilli() {
		t.Fatal("reclaim changed unrelated deadline")
	}
	if _, e = mod.Apply(ctx, next, json.RawMessage(`{"type":"win","round":1,"revision":0}`)); e == nil {
		t.Fatal("client declared win")
	}
	if _, e = mod.Configure(7, nil); e == nil {
		t.Fatal("invalid seats")
	}
	if _, e = mod.Configure(2, json.RawMessage(`{"seed":1}`)); e == nil {
		t.Fatal("client fixed difficulty")
	}
}
