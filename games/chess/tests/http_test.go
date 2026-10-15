package chess_test

// Written before the module. Failure inventory: illegal/self-check/wrong-seat
// moves; replay and stale round; unsafe king captures; castling through check
// or after a rook returns; en passant expiry/pins; missing promotion choice;
// mate vs stalemate; insufficient material false positives; lost repetition
// history; irrelevant en-passant rights splitting identical positions; claimed
// 3-fold/50-move draws confused with automatic 5-fold/75; intended claim moves
// incorrectly played; mate losing priority to 75 moves; offers reset clocks;
// bots miss mate/return illegal moves or mutate state; restart loses control.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	engine "github.com/corentings/chess/v2"
	game "github.com/k0ngk0ng/wire-board/games/chess/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func state(c *gametest.Client) game.View {
	raw, _ := json.Marshal(gametest.Data(c.Room()))
	var v game.View
	_ = json.Unmarshal(raw, &v)
	return v
}
func request(v game.View, kind string) map[string]any {
	return map[string]any{"type": kind, "round": v.Round, "ply": v.Ply}
}
func move(h *gametest.HTTP, uci string) game.View {
	v := state(h.People[0])
	a := request(v, "move")
	a["uci"] = uci
	h.People[v.Turn].Action(a, 200)
	return state(h.People[0])
}
func record(t *testing.T, name string, value any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "chess", "http")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "evidence": value}, "", "  ")
	if e := os.WriteFile(filepath.Join(dir, name+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

// Explicit middle-game fixture, never an accepted production room option.
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
		f, e := engine.FEN(fen)
		if e != nil {
			return v, e
		}
		p := engine.NewGame(f).Position()
		parts := strings.Fields(p.String())
		ep := false
		for _, mv := range p.ValidMoves() {
			if mv.HasTag(engine.EnPassant) {
				ep = true
			}
		}
		if !ep {
			parts[3] = "-"
		}
		s.White, s.Turn = 0, 0
		if p.Turn() == engine.Black {
			s.Turn = 1
		}
		s.FEN, s.InitialFEN = fen, fen
		s.Keys = []string{strings.Join(parts[:4], " ")}
		v.Attention = []int{s.Turn}
		v.TimeoutActors = []int{s.Turn}
		v.Data, _ = json.Marshal(s)
		return v, nil
	}
	return m
}

func TestChessHTTPMovesAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "chess")
	h.AssertUnpublished()
	a := h.People[0]
	a.Request("POST", "/api/admin/games/chess", map[string]bool{"enabled": true}, 200)
	for _, opts := range []map[string]any{{"fen": "8/8/8/8/8/8/8/8 w - - 0 1"}, {"difficulty": "unknown"}, {"turnSeconds": 1}} {
		a.Request("POST", "/api/rooms", map[string]any{"kind": "chess", "capacity": 2, "name": "拒绝非法选项", "options": opts}, 400)
	}
	h.Start(map[string]any{})
	v := state(a)
	if len(v.Board) != 64 || len(v.Legal) != 20 || v.Turn != v.White || v.Ply != 0 {
		t.Fatal("bad standard setup", v)
	}
	for _, uci := range []string{"e2e5", "a1a8", "e1e2", "a9a1", "e2e4q"} {
		r := request(v, "move")
		r["uci"] = uci
		h.People[v.Turn].Action(r, 400)
	}
	r := request(v, "move")
	r["uci"] = "e2e4"
	h.People[1-v.Turn].Action(r, 400)
	h.Spectator.Action(r, 400)
	old := v
	payload := h.People[v.Turn].ActionRequest(r)
	url := "/api/rooms/" + a.Room()["id"].(string)
	h.People[v.Turn].Request("POST", url, payload, 200)
	h.People[v.Turn].Request("POST", url, payload, 200)
	v = state(a)
	if v.Ply != 1 || v.Board[28] != "wP" || v.Board[12] != "" {
		t.Fatal("wrong/repeated pawn move", v)
	}
	r = request(old, "move")
	r["uci"] = "e7e5"
	h.People[v.Turn].Action(r, 400)
	move(h, "e7e5")
	move(h, "g1f3")
	before := state(a)
	h.Restart()
	if !reflect.DeepEqual(before, state(a)) {
		t.Fatal("restore changed FEN/history/deadline")
	}
	h.Record("natural moves and restore")
	record(t, "natural-lifecycle", state(a))
}

func TestChessHTTPNaturalMate(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "chess")
	h.Start(map[string]any{})
	white := state(h.People[0]).White
	for _, u := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		move(h, u)
	}
	v := state(h.People[0])
	if v.Phase != "finished" || v.Reason != "checkmate" || v.Winner != 1-white || v.SAN[3] != "Qh4#" {
		t.Fatal("mate not recognized", v)
	}
	if v.CheckSquare != 4 {
		t.Fatal("wrong checked king", v.CheckSquare)
	}
	h.Restart()
	record(t, "natural-fools-mate", state(h.People[0]))
}

func TestChessHTTPCastling(t *testing.T) {
	for _, tc := range []struct {
		name, fen, uci string
		valid          bool
		king, rook     int
	}{
		{"short", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", true, 6, 5},
		{"long", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1c1", true, 2, 3},
		{"through-check", "r3kr1r/8/8/8/8/8/8/R3K2R w KQ - 0 1", "e1g1", false, 0, 0},
		{"in-check", "4r1k1/8/8/8/8/8/8/R3K2R w KQ - 0 1", "e1g1", false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position(tc.fen), "chess")
			h.Start(map[string]any{})
			v := state(h.People[0])
			r := request(v, "move")
			r["uci"] = tc.uci
			status := 400
			if tc.valid {
				status = 200
			}
			h.People[0].Action(r, status)
			after := state(h.People[0])
			if tc.valid && (after.Board[tc.king] != "wK" || after.Board[tc.rook] != "wR" || after.Last.RookTo != tc.rook) {
				t.Fatal(after)
			}
			if !tc.valid && !reflect.DeepEqual(v, after) {
				t.Fatal("illegal castle mutated state")
			}
			record(t, "castle-"+tc.name, after)
		})
	}
	h := gametest.NewHTTP(t, 2, position("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1"), "chess")
	h.Start(map[string]any{})
	first := state(h.People[0]).Keys[0]
	for _, u := range []string{"h1h2", "h8h7", "h2h1", "h7h8"} {
		move(h, u)
	}
	v := state(h.People[0])
	r := request(v, "move")
	r["uci"] = "e1g1"
	h.People[0].Action(r, 400)
	if v.Keys[len(v.Keys)-1] == first {
		t.Fatal("repetition ignored lost castling rights")
	}
	record(t, "returned-rook-lost-rights", v)
}

func TestChessHTTPEnPassant(t *testing.T) {
	h := gametest.NewHTTP(t, 2, position("k7/8/8/3pP3/8/8/8/4K3 w - d6 0 1"), "chess")
	h.Start(map[string]any{})
	v := move(h, "e5d6")
	if v.Board[35] != "" || v.Board[36] != "" || v.Board[43] != "wP" || v.Last.CaptureAt != 35 {
		t.Fatal("wrong en passant capture", v)
	}
	record(t, "en-passant", v)
	h = gametest.NewHTTP(t, 2, position("k7/8/8/3pP3/8/8/8/4K3 w - d6 0 1"), "chess")
	h.Start(map[string]any{})
	move(h, "e1f1")
	move(h, "a8b8")
	v = state(h.People[0])
	r := request(v, "move")
	r["uci"] = "e5d6"
	h.People[0].Action(r, 400)
	for _, pinned := range []bool{false, true} {
		t.Run(fmt.Sprintf("pinned-%v", pinned), func(t *testing.T) {
			fen := "k7/8/8/3pP3/8/8/8/4K3 w - d6 0 1"
			if pinned {
				fen = "k3r3/8/8/3pP3/8/8/8/4K3 w - d6 0 1"
			}
			h := gametest.NewHTTP(t, 2, position(fen), "chess")
			h.Start(map[string]any{})
			if pinned {
				r := request(state(h.People[0]), "move")
				r["uci"] = "e5d6"
				h.People[0].Action(r, 400)
			}
			for range 2 {
				for _, u := range []string{"e1f1", "a8b8", "f1e1", "b8a8"} {
					move(h, u)
				}
			}
			v := state(h.People[0])
			if contains(v.Claims, "threefold") != pinned {
				t.Fatal("irrelevant/legal EP repetition distinction lost", v.Claims, v.Keys)
			}
			record(t, fmt.Sprintf("ep-repeat-pinned-%v", pinned), v)
		})
	}
}

func TestChessHTTPPromotionAndStalemate(t *testing.T) {
	for _, piece := range []string{"q", "r", "b", "n"} {
		t.Run(piece, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position("7k/P7/8/8/8/8/8/4K3 w - - 0 1"), "chess")
			h.Start(map[string]any{})
			v := state(h.People[0])
			r := request(v, "move")
			r["uci"] = "a7a8"
			h.People[0].Action(r, 400)
			v = move(h, "a7a8"+piece)
			if v.Board[56] != "w"+strings.ToUpper(piece) || v.Last.Promotion != piece {
				t.Fatal("wrong promotion", v)
			}
			if (piece == "b" || piece == "n") && (v.Phase != "finished" || v.Reason != "dead_position") {
				t.Fatal("minor promotion not dead position", v)
			}
			record(t, "promotion-"+piece, v)
		})
	}
	h := gametest.NewHTTP(t, 2, position("7k/5K2/8/6Q1/8/8/8/8 w - - 0 1"), "chess")
	h.Start(map[string]any{})
	v := move(h, "g5g6")
	if v.Reason != "stalemate" || v.Winner != -1 || v.Phase != "finished" {
		t.Fatal("stalemate is not a loss", v)
	}
	record(t, "stalemate", v)
	h = gametest.NewHTTP(t, 2, position("7k/8/8/8/4r3/3B4/8/K7 w - - 0 1"), "chess")
	h.Start(map[string]any{})
	v = move(h, "d3e4")
	if v.Reason != "dead_position" {
		t.Fatal("K+B vs K not ended", v)
	}
	record(t, "dead-position-capture", v)
	h = gametest.NewHTTP(t, 2, position("7k/8/8/8/8/8/3NN3/K7 w - - 0 1"), "chess")
	h.Start(map[string]any{})
	v = move(h, "a1a2")
	if v.Phase != "playing" {
		t.Fatal("KNN vs K can still mate cooperatively", v)
	}
	record(t, "two-knights-not-dead", v)
}

func TestChessHTTPRepetitionClaimsAndAutomatic(t *testing.T) {
	cycle := []string{"g1f3", "g8f6", "f3g1", "f6g8"}
	h := gametest.NewHTTP(t, 2, game.Module(), "chess")
	h.Start(map[string]any{})
	for _, u := range cycle {
		move(h, u)
	}
	v := state(h.People[0])
	bad := request(v, "claim_draw")
	bad["reason"] = "threefold"
	h.People[v.Turn].Action(bad, 400)
	for _, u := range cycle {
		move(h, u)
	}
	v = state(h.People[0])
	if v.Phase != "playing" || !contains(v.Claims, "threefold") {
		t.Fatal("threefold must be claimable, not automatic", v)
	}
	h.Restart()
	for range 2 {
		for _, u := range cycle {
			move(h, u)
		}
	}
	v = state(h.People[0])
	if v.Reason != "fivefold" || v.Phase != "finished" {
		t.Fatal("fivefold did not end", v)
	}
	record(t, "fivefold-after-restore", v)
	h = gametest.NewHTTP(t, 2, game.Module(), "chess")
	h.Start(map[string]any{})
	for i := 0; i < 7; i++ {
		move(h, cycle[i%4])
	}
	v = state(h.People[0])
	before := v.FEN
	if !contains(v.ClaimMoves["f6g8"], "threefold") {
		t.Fatal("intended repetition claim missing", v.ClaimMoves)
	}
	r := request(v, "claim_draw")
	r["reason"], r["uci"] = "threefold", "f6g8"
	h.People[v.Turn].Action(r, 200)
	v = state(h.People[0])
	if v.Reason != "threefold_claim" || v.Ply != 7 || v.FEN != before || v.ClaimMove != "f6g8" {
		t.Fatal("claim played the intended move", v)
	}
	record(t, "intended-threefold", v)
}

func TestChessHTTPMoveCountDraws(t *testing.T) {
	for _, half := range []int{99, 100, 149} {
		t.Run(fmt.Sprint(half), func(t *testing.T) {
			fen := fmt.Sprintf("7k/8/8/8/8/8/8/R3K3 w - - %d 76", half)
			h := gametest.NewHTTP(t, 2, position(fen), "chess")
			h.Start(map[string]any{})
			v := state(h.People[0])
			if half == 149 {
				v = move(h, "a1a2")
				if v.Reason != "seventy_five_move" {
					t.Fatal(v)
				}
			} else {
				r := request(v, "claim_draw")
				r["reason"] = "fifty_move"
				if half == 99 {
					h.People[0].Action(r, 400)
					r["uci"] = "a1a2"
				}
				h.People[0].Action(r, 200)
				v = state(h.People[0])
				if v.Reason != "fifty_move_claim" || v.Ply != 0 || v.FEN != fen {
					t.Fatal(v)
				}
			}
			record(t, fmt.Sprintf("half-clock-%d", half), v)
		})
	}
	h := gametest.NewHTTP(t, 2, position("7k/8/5KQ1/8/8/8/8/8 w - - 149 76"), "chess")
	h.Start(map[string]any{})
	v := move(h, "g6g7")
	if v.Reason != "checkmate" || v.Winner != 0 {
		t.Fatal("75-move draw overrode mate", v)
	}
	record(t, "mate-before-75", v)
}

func TestChessHTTPDrawOfferAndResign(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "chess")
	h.Start(map[string]any{})
	v := state(h.People[0])
	r := request(v, "move")
	r["uci"], r["offerDraw"] = "e2e4", true
	h.People[v.Turn].Action(r, 200)
	v = state(h.People[0])
	deadline := v.Deadline
	if v.Offer != v.White {
		t.Fatal("offer not attached to move")
	}
	reply := request(v, "respond_draw")
	reply["accept"] = false
	h.People[v.Turn].Action(reply, 200)
	v = state(h.People[0])
	if v.Deadline != deadline || v.Offer != -1 {
		t.Fatal("reply reset clock", v)
	}
	r = request(v, "move")
	r["uci"], r["offerDraw"] = "e7e5", true
	h.People[v.Turn].Action(r, 200)
	v = state(h.People[0])
	reply = request(v, "respond_draw")
	reply["accept"] = true
	h.People[v.Turn].Action(reply, 200)
	v = state(h.People[0])
	if v.Reason != "agreement" {
		t.Fatal(v)
	}
	record(t, "agreed-draw", v)
	h.People[0].Command("rematch", nil, 200)
	for _, c := range h.People {
		c.Command("ready", nil, 200)
	}
	h.People[0].Command("start", nil, 200)
	v = state(h.People[0])
	h.People[1-v.Turn].Action(request(v, "resign"), 200)
	v = state(h.People[0])
	if v.Reason != "resign" || v.Winner != v.White {
		t.Fatal(v)
	}
	record(t, "natural-resign", v)
}

func TestChessBotLegalMateHTTP(t *testing.T) {
	m := position("7k/8/5KQ1/8/8/8/8/8 w - - 0 1")
	h := gametest.NewHTTP(t, 2, m, "chess")
	h.Start(map[string]any{})
	v := state(h.People[0])
	raw, _ := json.Marshal(v.State)
	match := gameapi.Match{Schema: 1, Data: raw, Actors: []int{0, 1}, TimeoutActors: []int{0}, Deadline: v.Deadline}
	before := append([]byte(nil), raw...)
	in, e := m.BotSearch.Input(gameapi.Context{Actor: 0, Players: 2, Now: time.Now()}, match)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	proposal, e := m.BotSearch.Think(ctx, in)
	cancel()
	if e != nil {
		t.Fatal(e)
	}
	var a map[string]any
	_ = json.Unmarshal(proposal, &a)
	h.People[0].Action(a, 200)
	if state(h.People[0]).Reason != "checkmate" || !reflect.DeepEqual(before, []byte(match.Data)) {
		t.Fatal("bot missed mate or mutated snapshot", a)
	}
	record(t, "bot-mate", state(h.People[0]))
}

// FIDE Arbiter Manual 2026, PDF page 17 (printed p.10): after h5 neither
// bishop can cross the frozen pawn barrier or ever check the opposing king.
// Replacing the white bishop with a knight/queen must NOT be called dead.
func TestChessHTTPFrozenDeadPosition(t *testing.T) {
	for _, piece := range []string{"B", "N", "Q"} {
		t.Run(piece, func(t *testing.T) {
			fen := "8/2b1k3/7p/p1p1p1p1/PpP1P1PP/1P1" + piece + "K3/8/8 w - - 0 1"
			h := gametest.NewHTTP(t, 2, position(fen), "chess")
			h.Start(map[string]any{})
			v := move(h, "h4h5")
			if (v.Reason == "dead_position") != (piece == "B") {
				t.Fatal("frozen-position proof wrong", v)
			}
			record(t, "frozen-wall-"+piece, v)
		})
	}
}

func TestChessHTTPResignMatingPotential(t *testing.T) {
	for _, tc := range []struct {
		name, fen string
		draw      bool
	}{
		{"bare-king", "7k/8/8/8/8/8/8/K2Q4 w - - 0 1", true},
		{"two-knights", "5nnk/8/8/8/8/8/8/K7 w - - 0 1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, position(tc.fen), "chess")
			h.Start(map[string]any{})
			v := state(h.People[0])
			h.People[0].Action(request(v, "resign"), 200)
			v = state(h.People[0])
			if (v.Winner < 0) != tc.draw {
				t.Fatal("resignation ignored mating possibility", v)
			}
			record(t, "resign-potential-"+tc.name, v)
		})
	}
}
