package xiangqi_test

// Before implementation: palace/river/horse-leg/elephant-eye/cannon-screen
// violations, facing generals and exposed check, stalemate confused with draw,
// perpetual check/chase confused with harmless repetition, protected targets
// and uncrossed pawns misclassified, late replies reset clocks, lost physical
// piece identity after restore, illegal or stale actions mutate the board.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/xiangqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func state(c *gametest.Client) game.View {
	var s game.View
	b, _ := json.Marshal(gametest.Data(c.Room()))
	if e := json.Unmarshal(b, &s); e != nil {
		panic(e)
	}
	return s
}
func request(v game.View, kind string) map[string]any {
	return map[string]any{"type": kind, "round": v.Round, "ply": v.Ply}
}
func sq(v string) int { return int(v[0]-'a') + (9-int(v[1]-'0'))*9 }
func move(h *gametest.HTTP, u string) game.View {
	v := state(h.People[0])
	a := request(v, "move")
	a["from"], a["to"] = sq(u[:2]), sq(u[2:])
	h.People[v.Turn].Action(a, 200)
	return state(h.People[0])
}
func record(t *testing.T, name string, v any) {
	t.Helper()
	p := filepath.Join("..", "..", "..", ".local", "xiangqi", "http")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "evidence": v}, "", "  ")
	if e := os.WriteFile(filepath.Join(p, name+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}

// Test-owned middle game fixture. No FEN option is accepted in production.
func position(fen string) gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		if e != nil {
			return v, e
		}
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Board = [90]int{}
		s.IDs = [90]int{}
		i, id := 0, 1
		codes := "kaehrcp"
		for _, ch := range strings.Split(fen, " ")[0] {
			if ch == '/' {
				continue
			}
			if ch >= '1' && ch <= '9' {
				i += int(ch - '0')
				continue
			}
			code := strings.IndexRune(codes, []rune(strings.ToLower(string(ch)))[0]) + 1
			if code < 1 || i >= 90 {
				return v, fmt.Errorf("bad fixture")
			}
			if ch >= 'a' {
				code = -code
			}
			s.Board[i], s.IDs[i] = code, id
			i++
			id++
		}
		if i != 90 {
			return v, fmt.Errorf("bad fixture length %d", i)
		}
		s.Red, s.Turn = 0, 0
		s.History = []game.Record{{Key: game.PositionKey(s.Board, s.Turn), Seat: -1}}
		v.Data, _ = json.Marshal(s)
		v.Attention = []int{0}
		v.TimeoutActors = []int{0}
		return v, nil
	}
	return m
}
func reject(t *testing.T, h *gametest.HTTP, u string) {
	t.Helper()
	v := state(h.People[0])
	a := request(v, "move")
	a["from"], a["to"] = sq(u[:2]), sq(u[2:])
	h.People[v.Turn].Action(a, 400)
	if !reflect.DeepEqual(v, state(h.People[0])) {
		t.Fatal("illegal move mutated state")
	}
}
func TestXiangqiHTTPInitialAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "xiangqi")
	h.AssertUnpublished()
	h.Start(map[string]any{})
	v := state(h.People[0])
	if len(v.Legal) != 44 || v.Turn != v.Red {
		t.Fatal("bad initial position", len(v.Legal), v)
	}
	for _, u := range []string{"b0b2", "c0d1", "a3b3", "e0e2"} {
		reject(t, h, u)
	}
	a := request(v, "move")
	a["from"], a["to"] = sq("h2"), sq("e2")
	h.People[1-v.Turn].Action(a, 400)
	h.Spectator.Action(a, 400)
	move(h, "h2e2")
	move(h, "h9g7")
	before := state(h.People[0])
	h.Restart()
	if !reflect.DeepEqual(before, state(h.People[0])) {
		t.Fatal("restore changed position/history/deadline")
	}
	record(t, "initial-restore", before)
}
func TestXiangqiHTTPMovementConstraints(t *testing.T) {
	for _, tc := range []struct {
		name, fen string
		bad       []string
		good      string
	}{
		{"horse-leg", "4k4/9/9/9/9/4P4/9/9/1P7/1H2K4", []string{"b0a2", "b0c2"}, "b0d1"},
		{"elephant-eye", "4k4/9/9/9/9/4P4/9/9/3P5/2E1K4", []string{"c0e2"}, "c0a2"},
		{"elephant-river", "4k4/9/9/9/9/2E1P4/9/9/9/4K4", []string{"c4e6"}, "c4a2"},
		{"advisor-palace", "4k4/9/9/9/9/4P4/9/3A5/9/4K4", []string{"d2c3", "d2e3", "d2d1"}, "d2e1"},
		{"general-palace", "3k5/9/9/9/9/9/9/9/9/5K3", []string{"f0g0", "f0f2"}, "f0f1"},
		{"pawn-crossed", "4k4/9/9/9/2P6/4P4/9/9/9/4K4", []string{"c5c4", "c5e5"}, "c5d5"},
		{"facing-generals", "4k4/9/9/9/9/4R4/9/9/9/4K4", []string{"e4d4"}, "e4e5"},
		{"exposed-check", "3k5/9/9/9/9/4r4/9/4R4/9/4K4", []string{"e2d2"}, "e2e4"},
		{"cannon-screen", "4k4/9/9/9/9/r3P4/9/P8/9/C3K4", []string{"a0a2", "a0a3"}, "a0a4"},
		{"cannon-no-screen", "4k4/9/9/9/9/r3P4/9/9/9/C3K4", []string{"a0a4"}, "a0a3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position(tc.fen), "xiangqi")
			h.Start(map[string]any{})
			for _, u := range tc.bad {
				reject(t, h, u)
			}
			v := move(h, tc.good)
			record(t, "movement-"+tc.name, v)
		})
	}
}
func TestXiangqiHTTPLongCheckAndHarmlessRepetition(t *testing.T) {
	for _, tc := range []struct {
		name, fen string
		cycle     []string
		winner    int
	}{
		{"long-check", "4k4/3R5/9/9/9/9/9/9/9/5K3", []string{"d8e8", "e9d9", "e8d8", "d9e9"}, 1},
		{"quiet", "rheakaehr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RHEAKAEHR", []string{"b0c2", "b9c7", "c2b0", "c7b9"}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position(tc.fen), "xiangqi")
			h.Start(map[string]any{})
			for n := 0; n < 12; n++ {
				move(h, tc.cycle[n%4])
				if n == 7 {
					v := state(h.People[0])
					if v.Phase != "playing" {
						t.Fatal("ended without warning opportunity")
					}
					if tc.winner >= 0 && v.Warning == "" {
						t.Fatal("missing repetition warning")
					}
					h.Restart()
				}
			}
			v := state(h.People[0])
			if v.Phase != "finished" || v.Winner != tc.winner {
				t.Fatal("wrong repeat outcome", v)
			}
			record(t, tc.name, v)
		})
	}
}
func TestXiangqiHTTPLongChaseRoots(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(fmt.Sprint(protected), func(t *testing.T) {
			row := "3h5"
			if protected {
				row = "3h1r3"
			}
			fen := "4k4/9/9/" + row + "/R8/4P4/9/9/9/4K4"
			h := gametest.NewHTTP(t, 2, position(fen), "xiangqi")
			h.Start(map[string]any{})
			for n := 0; n < 12; n++ {
				move(h, []string{"a5a6", "d6f5", "a6a5", "f5d6"}[n%4])
				if n == 7 {
					h.Restart()
				}
			}
			v := state(h.People[0])
			winner := 1
			if protected {
				winner = -1
			}
			if v.Phase != "finished" || v.Winner != winner {
				t.Fatal("protected-root chase wrong", v)
			}
			record(t, fmt.Sprintf("chase-protected-%v", protected), v)
		})
	}
}
func TestXiangqiHTTPStalemateAndAgreement(t *testing.T) {
	h := gametest.NewHTTP(t, 2, position("4k4/9/3R1R3/9/9/4P4/9/9/9/4K4"), "xiangqi")
	h.Start(map[string]any{})
	// Rooks deny d9/f9; a horizontal rook on rank 8 denies e8 without checking e9.
	v := move(h, "d7d8")
	if v.Phase != "finished" || v.Winner != 0 || v.Reason != "stalemate_loss" {
		t.Fatal("xiangqi stalemate must lose", v)
	}
	record(t, "stalemate-loss", v)
	h = gametest.NewHTTP(t, 2, game.Module(), "xiangqi")
	h.Start(map[string]any{})
	v = state(h.People[0])
	a := request(v, "move")
	a["from"], a["to"], a["offerDraw"] = sq("h2"), sq("e2"), true
	h.People[v.Turn].Action(a, 200)
	v = state(h.People[0])
	a = request(v, "respond_draw")
	a["accept"] = true
	h.People[v.Turn].Action(a, 200)
	v = state(h.People[0])
	if v.Winner != -1 || v.Reason != "agreement" {
		t.Fatal(v)
	}
	record(t, "agreed-draw", v)
}

// WXF 20.7–20.8: an exchange on one leg of a repeated chase does not
// neutralize the other leg where a blocked horse cannot capture back.
func TestXiangqiHTTPMixedChaseExchange(t *testing.T) {
	h := gametest.NewHTTP(t, 2, position("4k4/9/2hP5/9/9/3HP4/9/9/9/4K4"), "xiangqi")
	h.Start(map[string]any{})
	for n := 0; n < 12; n++ {
		move(h, []string{"d4e6", "c7b5", "e6d4", "b5c7"}[n%4])
	}
	v := state(h.People[0])
	if v.Phase != "finished" || v.Winner != 1 || v.Reason != "perpetual_chase" {
		t.Fatal("one reciprocal leg concealed perpetual chase", v.Winner, v.Reason)
	}
	record(t, "mixed-chase-exchange", v)
}

func countedPosition(fen string, count int) gameapi.Module {
	m := position(fen)
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.NoCapture = count
		v.Data, _ = json.Marshal(s)
		return v, e
	}
	return m
}
func TestXiangqiHTTPNaturalLimitAndCapture(t *testing.T) {
	for _, tc := range []struct {
		name, fen, move string
		ended           bool
		count           int
	}{
		{"pawn-does-not-reset", "4k4/9/9/9/9/4P4/9/9/9/4K4", "e4e5", true, 100},
		{"capture-resets", "4k4/9/9/9/3h5/3RP4/9/9/9/4K4", "d4d5", false, 0},
		{"quiet-boundary", "4k4/9/9/9/9/R3P4/9/9/9/4K4", "a4a5", true, 100},
		{"stalemate-first", "4k4/9/3R1R3/9/9/4P4/9/9/9/4K4", "d7d8", true, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, countedPosition(tc.fen, 99), "xiangqi")
			h.Start(map[string]any{})
			v := move(h, tc.move)
			if (v.Phase == "finished") != tc.ended || v.NoCapture != tc.count {
				t.Fatal(v)
			}
			if tc.name == "stalemate-first" {
				if v.Winner != 0 || v.Reason != "stalemate_loss" {
					t.Fatal("draw overrode win")
				}
			} else if tc.ended && v.Reason != "fifty_move" {
				t.Fatal(v.Reason)
			}
			if !tc.ended && (len(v.History) != 1 || len(v.Captured[0]) != 1) {
				t.Fatal("capture did not reset history or account for captured horse")
			}
			record(t, tc.name, v)
		})
	}
}
func TestXiangqiHTTPStaleReplayAndDrawClock(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "xiangqi")
	h.Start(map[string]any{})
	v := state(h.People[0])
	a := request(v, "move")
	a["from"], a["to"], a["offerDraw"] = sq("h2"), sq("e2"), true
	c := h.People[v.Turn]
	payload := c.ActionRequest(a)
	url := "/api/rooms/" + c.Room()["id"].(string)
	c.Request("POST", url, payload, 200)
	c.Request("POST", url, payload, 200)
	after := state(c)
	if after.Ply != 1 {
		t.Fatal("replay moved twice")
	}
	stale := request(v, "move")
	stale["from"], stale["to"] = sq("h9"), sq("g7")
	h.People[after.Turn].Action(stale, 400)
	for _, a := range []map[string]any{
		{"type": "move", "round": after.Round, "ply": after.Ply, "from": 0, "to": 90},
		{"type": "move", "round": after.Round, "ply": after.Ply, "to": 1},
		{"type": "resign", "round": after.Round, "ply": after.Ply, "from": 0},
	} {
		h.People[after.Turn].Action(a, 400)
	}
	if !reflect.DeepEqual(after, state(c)) {
		t.Fatal("rejected requests changed board")
	}
	r := request(after, "respond_draw")
	r["accept"] = false
	h.People[after.Turn].Action(r, 200)
	declined := state(c)
	if declined.Deadline != after.Deadline || declined.Offer != -1 || declined.Board != after.Board {
		t.Fatal("offer response changed turn clock or board")
	}
	h.Restart()
	if !reflect.DeepEqual(declined, state(c)) {
		t.Fatal("restore altered response")
	}
	record(t, "stale-replay-draw-clock", declined)
}

func TestXiangqiHTTPChaseExceptions(t *testing.T) {
	for _, tc := range []struct {
		name, fen string
		cycle     []string
		winner    int
	}{
		{"cannon-protected-rook", "4kr3/9/9/2P2r3/C1P6/4P4/9/9/9/4K4", []string{"a5a6", "f6f5", "a6a5", "f5f6"}, 1},
		{"horse-protected-rook", "4k4/9/9/9/6rr1/3H5/4P4/9/9/4K4", []string{"d4e6", "g5f5", "e6d4", "f5g5"}, 1},
		{"two-targets", "4k4/9/9/3c5/R4c3/4P4/9/9/9/4K4", []string{"a5a6", "e9d9", "a6a5", "d9e9"}, -1},
		{"king-exempt", "4k4/9/9/9/9/4P4/9/9/5c3/4K4", []string{"e0f0", "f1e1", "f0e0", "e1f1"}, -1},
		{"pawn-exempt", "4k4/9/3r5/4P4/9/4P4/9/9/9/4K4", []string{"e6d6", "d7e7", "d6e6", "e7d7"}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position(tc.fen), "xiangqi")
			h.Start(map[string]any{})
			for n := 0; n < 12; n++ {
				move(h, tc.cycle[n%4])
			}
			v := state(h.People[0])
			if v.Phase != "finished" || v.Winner != tc.winner {
				t.Fatal("wrong chase exception", v.Winner, v.Reason)
			}
			record(t, "chase-"+tc.name, v)
		})
	}
}

// WXF 2018 PDF p.32, figure 4: red captures the elephant, then rook and
// cannon shuttles each answer check by giving check. Both must draw.
func TestXiangqiHTTPOfficialMutualChecks(t *testing.T) {
	h := gametest.NewHTTP(t, 2, position("3akr3/5c3/1P2e4/4R4/9/9/9/9/9/4CK3"), "xiangqi")
	h.Start(map[string]any{})
	move(h, "e6e7")
	move(h, "f8e8")
	for n := 0; n < 14 && state(h.People[0]).Phase == "playing"; n++ {
		v := move(h, []string{"e7f7", "e8f8", "f7e7", "f8e8"}[n%4])
		if !v.Last.Check {
			t.Fatal("official mutual-check move did not check", n, v.Last)
		}
	}
	v := state(h.People[0])
	if v.Phase != "finished" || v.Winner != -1 || v.Reason != "repetition" {
		t.Fatal("mutual checks were not drawn", v.Winner, v.Reason)
	}
	record(t, "official-figure-4-mutual-check", v)
}

// The apparent defending rook cannot leave the f-file: doing so uncovers
// the red cannon behind its pawn and checks its own general (a false root).
func TestXiangqiHTTPPinnedFalseRoot(t *testing.T) {
	h := gametest.NewHTTP(t, 2, position("5k3/9/9/4cr3/9/3R5/5P3/3C5/9/3K1C3"), "xiangqi")
	h.Start(map[string]any{})
	for n := 0; n < 12; n++ {
		move(h, []string{"d4e4", "e6d6", "e4d4", "d6e6"}[n%4])
	}
	v := state(h.People[0])
	if v.Phase != "finished" || v.Winner != 1 || v.Reason != "perpetual_chase" {
		t.Fatal("geometric defender mistaken for a legal recapture", v.Winner, v.Reason)
	}
	record(t, "pinned-false-root", v)
}

// Two rooks alternate attacking one stationary target while an elephant
// directly blocks the preceding attack. Merely comparing old/new attack pairs
// misses this chase. An uncrossed pawn remains exempt under the same pattern.
func TestXiangqiHTTPAlternatingAttackers(t *testing.T) {
	for _, kind := range []string{"c", "p"} {
		t.Run(kind, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position("4R4/9/4ek3/9/R3"+kind+"4/9/9/9/9/3K5"), "xiangqi")
			h.Start(map[string]any{})
			for n := 0; n < 24; n++ {
				move(h, []string{"a5b5", "e7c5", "e9e8", "c5e7", "b5a5", "e7c5", "e8e9", "c5e7"}[n%8])
			}
			v := state(h.People[0])
			winner := 1
			if kind == "p" {
				winner = -1
			}
			if v.Phase != "finished" || v.Winner != winner {
				t.Fatal("alternating attackers/uncrossed pawn wrong", v.Winner, v.Reason)
			}
			record(t, "alternating-attackers-"+kind, v)
		})
	}
}

func TestXiangqiHTTPIgnoredAttackIsNotPursuit(t *testing.T) {
	h := gametest.NewHTTP(t, 2, position("9/9/5k3/9/R3c4/9/9/9/9/3K5"), "xiangqi")
	h.Start(map[string]any{})
	for n := 0; n < 12; n++ {
		move(h, []string{"a5b5", "f7f8", "b5a5", "f8f7"}[n%4])
	}
	v := state(h.People[0])
	if v.Winner != -1 || v.Reason != "repetition" {
		t.Fatal("ignored unchanged attack treated as chase", v.Winner, v.Reason)
	}
	record(t, "ignored-attack-not-pursuit", v)
}
