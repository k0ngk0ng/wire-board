package bubbleparty

// Failure scenarios:
// - odd/oversized rooms, invalid maps/options, asymmetric maps, blocked spawns
// - bombs that do not block, blasts that pass walls, chain reactions skipped
// - self/team damage, shields, crate drops, item stat overflow
// - players walking through walls/bombs, stale or duplicate inputs accepted
// - matches that never end, wrong team ranks, fabricated draws/winners
// - bot games stuck, restore resetting the match, spectators controlling play

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func newMatch(t *testing.T, players int, mapID string, seed uint64) (*State, gameapi.Match) {
	t.Helper()
	raw, _ := json.Marshal(Options{Map: mapID, RoundSeconds: 300})
	ctx := gameapi.Context{Players: players, Now: time.Unix(1000, 0)}
	m, err := startWithSeed(ctx, raw, seed)
	if err != nil {
		t.Fatal(err)
	}
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	return s, m
}

func beginPlaying(t *testing.T, s *State) {
	t.Helper()
	s.onTimer(s.StartAt)
	if s.Phase != PhasePlaying {
		t.Fatalf("phase after countdown: %s", s.Phase)
	}
}

func TestConfigureAndSymmetricMaps(t *testing.T) {
	for _, players := range []int{2, 4, 6, 8} {
		for _, id := range mapIDs() {
			if _, err := configure(players, json.RawMessage(`{"map":"`+id+`","roundSeconds":180}`)); err != nil {
				t.Fatalf("players %d map %s: %v", players, id, err)
			}
			s, _ := newMatch(t, players, id, 42)
			if s.TeamSize != players/2 || len(s.Players) != players {
				t.Fatalf("bad teams %d %s: %d/%d", players, id, s.TeamSize, len(s.Players))
			}
			cells := mapCells(s.MapID, s.Seed)
			if len(cells) != Cols*Rows {
				t.Fatalf("map cells %s: %d", id, len(cells))
			}
			for y := 0; y < Rows; y++ {
				for x := 0; x < Cols; x++ {
					mx, my := Cols-1-x, Rows-1-y
					if id != "community10" && cells[y*Cols+x] != cells[my*Cols+mx] {
						t.Fatalf("map %s not 180 symmetric at %d,%d", id, x, y)
					}
				}
			}
			for _, p := range s.Players {
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						x, y := p.X+dx, p.Y+dy
						if id != "community10" && x >= 1 && x < Cols-1 && y >= 1 && y < Rows-1 && cells[y*Cols+x] == CellWall {
							t.Fatalf("map %s wall next to spawn %d: %d,%d", id, p.Seat, p.X, p.Y)
						}
					}
				}
				if cells[p.Y*Cols+p.X] != CellEmpty {
					t.Fatalf("map %s spawn blocked seat %d", id, p.Seat)
				}
			}
			if id != "community10" && !reachable(cells, s.Players[0].X, s.Players[0].Y) {
				t.Fatalf("map %s spawn area disconnected", id)
			}
		}
	}
	for _, players := range []int{1, 3, 5, 7, 10} {
		if _, err := configure(players, nil); err == nil {
			t.Fatalf("players %d accepted", players)
		}
	}
	if _, err := configure(4, json.RawMessage(`{"map":"nope","roundSeconds":300}`)); err == nil {
		t.Fatal("invalid map accepted")
	}
	if _, err := configure(4, json.RawMessage(`{"map":"community10","roundSeconds":30}`)); err == nil {
		t.Fatal("short round accepted")
	}
}

func TestTurnOrderAndTeamSplit(t *testing.T) {
	s, _ := newMatch(t, 8, "community10", 7)
	for i, p := range s.Players {
		if p.Seat != i || p.Team != i/(len(s.Players)/2) {
			t.Fatalf("bad seat/team %d: %+v", i, p)
		}
	}
	beginPlaying(t, s)
	for _, p := range s.Players {
		if !p.Alive {
			t.Fatalf("spawn failed %d", p.Seat)
		}
	}
	if s.Sudden || s.SafeInset != 0 {
		t.Fatal("sudden death started early")
	}
}

func TestBombDestroysCrateAndBlocksMovement(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 11)
	beginPlaying(t, s)
	p := &s.Players[0]
	cx, cy := -1, -1
	for _, d := range directions {
		x, y := p.X+d[0], p.Y+d[1]
		if s.cell(x, y) == CellCrate {
			cx, cy = x, y
			break
		}
	}
	if cx < 0 {
		cx, cy = p.X+1, p.Y
		s.setCell(cx, cy, CellCrate)
	}
	s.placeBomb(p, s.ServerAt)
	if s.cell(p.X, p.Y) != CellBomb {
		t.Fatal("bomb did not occupy the cell")
	}
	if s.canWalk(p, cx, cy) {
		t.Fatal("crate is walkable")
	}
	s.advance(s.ServerAt + BombFuseMS + 200)
	if s.cell(cx, cy) != CellEmpty {
		t.Fatalf("crate survived blast: %d", s.cell(cx, cy))
	}
	if s.cell(p.X, p.Y) != CellEmpty {
		t.Fatal("bomb cell did not clear")
	}
}

func TestChainBlastShieldAndElimination(t *testing.T) {
	s, _ := newMatch(t, 4, "community10", 13)
	beginPlaying(t, s)
	a, b, c := &s.Players[0], &s.Players[1], &s.Players[2]
	s.Phase = PhasePlaying
	s.setCell(a.X, a.Y, CellEmpty)
	s.setCell(b.X, b.Y, CellEmpty)
	s.setCell(c.X, c.Y, CellEmpty)
	b.X, b.Y = a.X+1, a.Y
	c.X, c.Y = a.X+2, a.Y
	s.setCell(b.X, b.Y, CellEmpty)
	s.setCell(c.X, c.Y, CellEmpty)
	s.placeBomb(a, s.ServerAt)
	s.Bombs[0].Range = 3
	s.placeBomb(b, s.ServerAt)
	s.Bombs[1].Range = 3
	b.Shield = true
	s.advance(s.ServerAt + BombFuseMS + 100)
	if !a.Alive || !c.Alive || a.TrappedUntil <= s.ServerAt || c.TrappedUntil <= s.ServerAt {
		t.Fatalf("blast did not trap living players: a=%+v c=%+v", a, c)
	}
	if !b.Alive || b.Shield {
		t.Fatalf("shield did not absorb first hit: alive=%v shield=%v", b.Alive, b.Shield)
	}
}

func TestItemsApplyAndCap(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 17)
	beginPlaying(t, s)
	p := &s.Players[0]
	baseSpeed, baseBombs, baseRange := p.SpeedMS, p.MaxBombs, p.Range
	s.Items = append(s.Items, Item{ID: 1, X: p.X, Y: p.Y, Type: ItemSpeed})
	s.Items = append(s.Items, Item{ID: 2, X: p.X, Y: p.Y, Type: ItemBomb})
	s.Items = append(s.Items, Item{ID: 3, X: p.X, Y: p.Y, Type: ItemRange})
	s.Items = append(s.Items, Item{ID: 4, X: p.X, Y: p.Y, Type: ItemShield})
	s.pickupItems(s.ServerAt)
	if p.SpeedMS >= baseSpeed || p.MaxBombs != baseBombs+1 || p.Range != baseRange+1 || !p.Shield {
		t.Fatalf("items not applied: %+v", p)
	}
	for i := 0; i < 20; i++ {
		s.Items = append(s.Items, Item{ID: 10 + i, X: p.X, Y: p.Y, Type: ItemBomb})
	}
	s.pickupItems(s.ServerAt)
	if p.MaxBombs > MaxBombCapacity || p.Range > MaxBombRange || p.SpeedMS < MinMoveMS {
		t.Fatalf("item caps violated: %+v", p)
	}
}

func TestTeamWinAndDraw(t *testing.T) {
	s, _ := newMatch(t, 4, "community10", 19)
	beginPlaying(t, s)
	for i := range s.Players {
		if s.Players[i].Team == 1 {
			s.eliminate(&s.Players[i], s.ServerAt, -1)
		}
	}
	s.advance(s.ServerAt + 200)
	if s.Result == nil || !s.Result.Team || s.Result.Draw || len(s.Result.Winners) != 2 {
		t.Fatalf("team win missing: %+v", s.Result)
	}
	for i := range s.Players {
		want := 1
		if s.Players[i].Team == 1 {
			want = 2
		}
		if s.Result.Ranks[i] != want {
			t.Fatalf("bad rank %d: %+v", i, s.Result)
		}
	}

	d, _ := newMatch(t, 2, "community10", 23)
	beginPlaying(t, d)
	d.MatchEndsAt = d.ServerAt + 40
	d.advance(d.ServerAt + 100)
	if d.Result == nil || !d.Result.Draw || len(d.Result.Winners) != 0 {
		t.Fatalf("equal-survivor timeout should draw: %+v", d.Result)
	}
}

func TestInputValidationAndRestore(t *testing.T) {
	s, m := newMatch(t, 2, "community10", 29)
	beginPlaying(t, s)
	m, _ = s.match()
	ctx := gameapi.Context{Actor: 0, Now: time.Unix(1100, 0), Players: 2}
	var err error
	if m, err = apply(ctx, m, json.RawMessage(`{"type":"input","epoch":"`+s.Epoch+`","seq":1,"move":"up","bomb":true}`)); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if _, err := apply(ctx, m, json.RawMessage(`{"type":"input","epoch":"`+s.Epoch+`","seq":1,"move":"up","bomb":true}`)); err == nil {
		t.Fatal("replayed seq accepted")
	}
	if _, err := apply(gameapi.Context{Actor: 0, Now: time.Unix(1100, 0), Players: 2}, m, json.RawMessage(`{"type":"input","epoch":"bad","seq":2,"move":"up"}`)); err == nil {
		t.Fatal("wrong epoch accepted")
	}
	if _, err := apply(gameapi.Context{Actor: 1, Now: time.Unix(1100, 0), Players: 2}, m, json.RawMessage(`{"type":"input","epoch":"`+s.Epoch+`","seq":1,"move":"down"}`)); err != nil {
		t.Fatalf("second player input rejected: %v", err)
	}
}
