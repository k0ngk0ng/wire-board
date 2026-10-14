package snooker_test

// Failure scenarios specified before bot implementation: absent hooks; actions
// from idle/spectator seats; movement settled early; repeated ready/place loops;
// a missing cue, free ball or tied black stalls; doubles skips a partner; restore
// changes decisions; reclaim resets the table; bots resign instead of playing.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func botStep(t *testing.T, mod gameapi.Module, ctx gameapi.Context, m gameapi.Match) gameapi.Match {
	t.Helper()
	if mod.BotAction == nil || mod.Reclaim == nil {
		t.Fatal("computer players and autoplay hooks are required")
	}
	before := append([]byte{}, m.Data...)
	raw, e := mod.BotAction(ctx, m)
	if e != nil || raw == nil {
		t.Fatalf("bot stalled in %s seat %d: %v", state(t, m).Phase, ctx.Actor, e)
	}
	if !bytes.Equal(before, m.Data) {
		t.Fatal("planner mutated its input")
	}
	var restored gameapi.Match
	b, _ := json.Marshal(m)
	if e = json.Unmarshal(b, &restored); e != nil {
		t.Fatal(e)
	}
	again, e := mod.BotAction(ctx, restored)
	if e != nil || !bytes.Equal(raw, again) {
		t.Fatal("restore changed the decision", e)
	}
	var a game.Action
	if e = json.Unmarshal(raw, &a); e != nil {
		t.Fatal(e)
	}
	if a.Type == "resign" || a.Type == "concede_frame" || a.Type == "claim_frame" || a.Type == "rerack" {
		t.Fatal("bot used a shortcut instead of playing", a)
	}
	next, e := mod.Apply(ctx, m, raw)
	if e != nil {
		t.Fatal("invalid bot action", string(raw), e)
	}
	if e = mod.ValidateMatch(next, ctx.Players); e != nil {
		t.Fatal(e)
	}
	return next
}

func TestSnookerBotsCompleteMatches(t *testing.T) {
	for _, n := range []int{2, 4} {
		for _, bestOf := range []int{1, 3, 5} {
			t.Run(fmt.Sprintf("%d-players-best-of-%d", n, bestOf), func(t *testing.T) {
				mod := game.Module()
				ctx := gameapi.Context{Players: n, Now: time.Unix(100000, 0)}
				opts := map[string]any{"bestOf": bestOf}
				if n == 4 {
					opts["teams"] = []int{0, 0, 1, 1}
				}
				m, e := mod.Start(ctx, mustAction(opts))
				if e != nil {
					t.Fatal(e)
				}
				records := []any{map[string]any{"scope": "production BotAction and Apply, natural rack, accelerated system clock", "initial": state(t, m)}}
				defer func() { trace(t, records) }()
				shots := 0
				seats := map[int]int{}
				maxDecision := time.Duration(0)
				for step := 0; step < 1500 && m.Result == nil; step++ {
					s := state(t, m)
					if mod.BotAction == nil {
						t.Fatal("computer players are unsupported")
					}
					if m.Deadline != 0 {
						t.Fatal("introduced a human shot deadline")
					}
					for p := -1; p <= n; p++ {
						if p >= 0 && p < n && (s.Phase == "frame_end" && !s.Ready[p] || s.Phase != "frame_end" && s.Phase != "rolling" && p == s.Turn) {
							continue
						}
						ctx.Actor = p
						raw, e := mod.BotAction(ctx, m)
						if e != nil || raw != nil {
							t.Fatal("idle seat acted", p, s.Phase, string(raw), e)
						}
					}
					ctx.Now = ctx.Now.Add(time.Second)
					if s.Phase == "rolling" {
						ctx.Now = time.UnixMilli(m.SystemDeadline - 1)
						m = finishStroke(t, mod, ctx, m)
						continue
					}
					ctx.Actor = s.Turn
					if s.Phase == "frame_end" {
						for p, ready := range s.Ready {
							if !ready {
								ctx.Actor = p
								break
							}
						}
					}
					started := time.Now()
					next := botStep(t, mod, ctx, m)
					maxDecision = max(maxDecision, time.Since(started))
					after := state(t, next)
					if after.ShotID > s.ShotID {
						shots++
						seats[ctx.Actor]++
						records = append(records, map[string]any{"shot": after.ShotID, "seat": ctx.Actor, "strike": after.Flight.Strike, "scores": s.Scores, "target": s.Target})
					}
					m = next
				}
				final := state(t, m)
				records = append(records, map[string]any{"final": final, "shots": shots, "seats": seats, "maxDecisionMs": maxDecision.Milliseconds()})
				if m.Result == nil || final.FinishReason == "resigned" {
					t.Fatal("bot match stalled", shots, final.Phase)
				}
				if len(seats) != n {
					t.Fatal("some seats never played", seats)
				}
				for _, r := range final.Results {
					if r.Reason != "black" {
						t.Fatal("did not finish naturally", r)
					}
				}
				if final.FramesWon[final.Winner] != bestOf/2+1 {
					t.Fatal("wrong match victory", final.FramesWon)
				}
				t.Logf("%d strokes, %d frames, seats %v, max two decisions %v", shots, len(final.Results), seats, maxDecision)
			})
		}
	}
}

func TestSnookerBotsSpecialPhasesAndReclaim(t *testing.T) {
	mod, ctx, m := begin(t, 4)
	if mod.BotAction == nil || mod.Reclaim == nil {
		t.Fatal("missing bot hooks")
	}
	for _, kind := range []string{"inhand_free", "bank_miss", "touch_return"} {
		t.Run(kind, func(t *testing.T) {
			seeded := seedSpecial(t, m, kind)
			records := []any{map[string]any{"seededLayout": kind}}
			for step := 0; step < 4; step++ {
				s := state(t, seeded)
				ctx.Actor = s.Turn
				unchanged, e := mod.Reclaim(ctx, seeded)
				if e != nil || !reflect.DeepEqual(unchanged, seeded) {
					t.Fatal("reclaim reset the table or clock", e)
				}
				if s.Phase == "rolling" {
					break
				}
				seeded = botStep(t, mod, ctx, seeded)
				records = append(records, state(t, seeded))
			}
			if state(t, seeded).Phase != "rolling" {
				t.Fatal("special layout never produced a shot")
			}
			trace(t, records)
		})
	}
	m = onlyBlack(t, m, [2]int{0, 7})
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": -1.5707963267948966, "power": .25})
	m = finishStroke(t, mod, ctx, m)
	for _, phase := range []string{"black_choice", "black_player", "aim", "aim"} {
		s := state(t, m)
		if s.Phase != phase {
			t.Fatal("unexpected decider phase", s.Phase, phase)
		}
		ctx.Actor = s.Turn
		m = botStep(t, mod, ctx, m)
	}
	if state(t, m).Phase != "rolling" {
		t.Fatal("black decider stalled")
	}
	trace(t, m)
	// A human team's eligible concession must receive a response.
	mod, ctx, m = begin(t, 4)
	s := state(t, m)
	s.Scores = [2]int{200, 0}
	m.Data = mustAction(s)
	for _, p := range []int{1, 3} {
		ctx.Actor = p
		m = action(t, mod, ctx, m, map[string]any{"type": "concede_frame"})
	}
	ctx.Actor = state(t, m).Turn
	m = botStep(t, mod, ctx, m)
	if m.Result == nil {
		t.Fatal("bot ignored opponent's concession")
	}
	trace(t, m)
}
