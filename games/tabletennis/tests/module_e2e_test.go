package tabletennis_test

// Written before production implementation. Natural complete games exercise
// the public Module only: no ball/score/result injection or bot-only rules.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/tabletennis/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func raw(v any) json.RawMessage {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func projected(t *testing.T, mod gameapi.Module, m gameapi.Match, seat int) game.View {
	t.Helper()
	v, e := mod.View(m, seat)
	if e != nil {
		t.Fatal(e)
	}
	var d game.View
	if e = json.Unmarshal(raw(v), &d); e != nil {
		t.Fatal(e)
	}
	return d
}
func TestTableTennisNaturalCompleteGames(t *testing.T) {
	for _, teams := range [][]int{{0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1}, {0, 1, 1, 0}} {
		for _, best := range []int{1, 3} {
			for _, level := range []string{"easy", "normal", "hard"} {
				t.Run(fmt.Sprintf("%v-best%d-%s", teams, best, level), func(t *testing.T) {
					mod := game.Module()
					n := len(teams)
					ctx := gameapi.Context{Players: n, Now: time.Unix(100000, 0), Automated: make([]bool, n)}
					for i := range ctx.Automated {
						ctx.Automated[i] = true
					}
					m, e := mod.Start(ctx, raw(map[string]any{"teams": teams, "bestOf": best, "difficulty": level, "court": "club"}))
					if e != nil {
						t.Fatal(e)
					}
					if e = mod.ValidateMatch(m, n); e != nil {
						t.Fatal(e)
					}
					dir := filepath.Join("..", "..", "..", ".local", "tabletennis", "bots")
					if e = os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					path := filepath.Join(dir, fmt.Sprintf("%v-%d-%s.jsonl", teams, best, level))
					f, e := os.Create(path)
					if e != nil {
						t.Fatal(e)
					}
					defer f.Close()
					enc := json.NewEncoder(f)
					_ = enc.Encode(map[string]any{"scope": "natural full production module game; accelerated clock only", "initial": m})
					strokes := 0
					events := map[string]int{}
					seen := 0
					for step := 0; step < 60000 && m.Result == nil; step++ {
						ctx.Now = ctx.Now.Add(50 * time.Millisecond)
						for seat := 0; seat < n; seat++ {
							ctx.Actor = seat
							before := sha256.Sum256(m.Data)
							a, err := mod.BotAction(ctx, m)
							if err != nil {
								t.Fatal(err)
							}
							if before != sha256.Sum256(m.Data) {
								t.Fatal("bot mutated match")
							}
							if len(a) > 0 {
								var typ struct{ Type string }
								_ = json.Unmarshal(a, &typ)
								if typ.Type != "swing" {
									t.Fatal("bot bypassed ordinary swing")
								}
								next, err := mod.Apply(ctx, m, a)
								if err != nil {
									t.Fatalf("bot rejected: %s %v", a, err)
								}
								m = next
								strokes++
								_ = enc.Encode(map[string]any{"at": ctx.Now.UnixMilli(), "actor": seat, "action": a})
							}
						}
						ctx.Actor = -1
						m, e = mod.OnTimer(ctx, m)
						if e != nil {
							t.Fatal(e)
						}
						if e = mod.ValidateMatch(m, n); e != nil {
							t.Fatal(e)
						}
						v := projected(t, mod, m, -1)
						for _, ev := range v.Events {
							if ev.ID > seen {
								events[ev.Type]++
								seen = ev.ID
								_ = enc.Encode(ev)
							}
						}
						if step%200 == 0 {
							_ = enc.Encode(map[string]any{"snapshot": m})
						}
					}
					if m.Result == nil {
						t.Fatal("natural game did not finish")
					}
					v := projected(t, mod, m, -1)
					if strokes < 20 || events["hit"] < 10 || events["bounce"] < 10 {
						t.Fatalf("no genuine rallies: strokes=%d events=%v", strokes, events)
					}
					if m.Result.Draw || len(m.Result.Winners) != n/2 || m.Result.Team != (n == 4) {
						t.Fatalf("wrong outcome: %+v", m.Result)
					}
					for _, g := range v.Games {
						if g.Score[0] < 11 && g.Score[1] < 11 {
							t.Fatal("not an eleven-point game")
						}
						d := g.Score[0] - g.Score[1]
						if d < 2 && d > -2 {
							t.Fatal("missing two-point lead")
						}
					}
					_ = enc.Encode(map[string]any{"passed": true, "strokes": strokes, "events": events, "final": m})
					t.Logf("%d strokes, %d points, %d games", strokes, events["point"], len(v.Games))
				})
			}
		}
	}
}
func TestTableTennisConfigurationAndRestore(t *testing.T) {
	mod := game.Module()
	for _, n := range []int{0, 1, 3, 5} {
		if _, e := mod.Configure(n, nil); e == nil {
			t.Fatal("unsupported seats accepted")
		}
	}
	for _, o := range []any{map[string]any{"bestOf": 2}, map[string]any{"teams": []int{0, 0}}, map[string]any{"court": "unknown"}, map[string]any{"difficulty": "magic"}, map[string]any{"score": []int{11, 0}}} {
		if _, e := mod.Configure(2, raw(o)); e == nil {
			t.Fatalf("invalid option %v", o)
		}
	}
	ctx := gameapi.Context{Players: 4, Now: time.Unix(100000, 0)}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	restore, e := mod.Restore(ctx, m)
	if e != nil || string(restore.Data) != string(m.Data) {
		t.Fatal("restore changed active match")
	}
	v := projected(t, mod, m, -1)
	if v.Controls.CanSwing {
		t.Fatal("spectator can swing")
	}
	ctx.Actor = 2
	ctx.Now = ctx.Now.Add(time.Second)
	reclaim, e := mod.Reclaim(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	rv := projected(t, mod, reclaim, -1)
	if rv.Epoch != v.Epoch || rv.Rally != v.Rally || rv.Score != v.Score || rv.PhaseUntil != v.PhaseUntil {
		t.Fatal("reclaim restarted world")
	}
}

// Natural module starts and ordinary serve inputs, before broadening the serve
// power response. No ball, net flag or outcome is inserted into the match.
func TestTableTennisServePowerAndLetFlow(t *testing.T) {
	mod := game.Module()
	foundLet := false
	var slow, fast float64
	for _, kind := range []string{"drive", "chop", "block"} {
		for percent := 0; percent <= 100; percent++ {
			ctx := gameapi.Context{Players: 4, Now: time.Unix(100000, 0)}
			m, e := mod.Start(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			v := projected(t, mod, m, 0)
			ctx.Now = time.UnixMilli(v.PhaseUntil)
			ctx.Actor = -1
			m, e = mod.OnTimer(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			v = projected(t, mod, m, 0)
			ctx.Actor = v.Expected
			v = projected(t, mod, m, ctx.Actor)
			input := map[string]any{"type": "swing", "epoch": v.Epoch, "rally": v.Rally, "shot": v.Shot, "seq": 1, "kind": kind, "aim": 0, "power": float64(percent) / 100, "at": ctx.Now.UnixMilli()}
			m, e = mod.Apply(ctx, m, raw(input))
			if e != nil {
				t.Fatal(e)
			}
			ctx.Now = ctx.Now.Add(250 * time.Millisecond)
			ctx.Actor = -1
			m, e = mod.OnTimer(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			v = projected(t, mod, m, 0)
			f := v.Flight
			if f == nil {
				t.Fatal("serve did not produce flight")
			}
			if kind == "drive" {
				speed := f.Segments[0].V.Z
				if speed < 0 {
					speed = -speed
				}
				if percent == 0 {
					slow = speed
				}
				if percent == 100 {
					fast = speed
				}
			}
			fromSide := v.Options.Teams[v.LastHitter] ^ v.Ends
			count := 0
			for _, event := range f.Events {
				if event.Type != "bounce" {
					continue
				}
				wantSide := fromSide
				if count > 0 {
					wantSide = 1 - fromSide
				}
				if count < 2 && (f.ReceiveAt > 0 || f.Reason == "let") && event.Side != wantSide {
					t.Fatal("serve skipped own/opponent bounce")
				}
				if count < 2 && (f.ReceiveAt > 0 || f.Reason == "let") && ((wantSide == 0 && event.P.X < -.02) || (wantSide == 1 && event.P.X > .02)) {
					t.Fatal("serve not diagonal")
				}
				count++
			}
			if f.Reason == "let" {
				foundLet = true
				server := v.LastHitter
				oldRally := v.Rally
				ctx.Now = time.UnixMilli(f.EndAt + game.LateGraceMS + 1)
				m, e = mod.OnTimer(ctx, m)
				if e != nil {
					t.Fatal(e)
				}
				after := projected(t, mod, m, server)
				if after.Score != [2]int{} || after.Expected != server || after.Rally <= oldRally {
					t.Fatal("let awarded a point or changed server")
				}
			}
		}
	}
	if fast < slow*1.3 {
		t.Fatalf("serve force has too little effect: %.3f to %.3f", slow, fast)
	}
	if !foundLet {
		t.Fatal("ordinary input range cannot produce a legal net-let serve")
	}
}

// The game broadcasts at realtime cadence. Retain history in the saved state,
// but do not resend dozens of obsolete impact effects in every public packet.
func TestTableTennisPublicEffectWindowStaysBounded(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0), Automated: []bool{true, true}}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	for step := 0; step < 5000 && m.Result == nil; step++ {
		ctx.Now = ctx.Now.Add(50 * time.Millisecond)
		for seat := 0; seat < 2; seat++ {
			ctx.Actor = seat
			a, err := mod.BotAction(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			if a != nil {
				m, err = mod.Apply(ctx, m, a)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		ctx.Actor = -1
		m, e = mod.OnTimer(ctx, m)
		if e != nil {
			t.Fatal(e)
		}
		v := projected(t, mod, m, -1)
		if len(v.Events) > 16 {
			t.Fatal("old effect history inflated every realtime packet")
		}
		if len(v.Events) > 0 && v.Events[len(v.Events)-1].ID > 64 {
			return
		}
	}
	t.Fatal("bounded effect stream did not carry new impacts")
}
