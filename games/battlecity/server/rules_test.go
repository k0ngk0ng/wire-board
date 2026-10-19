package battlecity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/battlecity/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func testState(t *testing.T, players int, mapID string) (*State, int64) {
	t.Helper()
	now := int64(1_700_000_000_000)
	ctx := gameapi.Context{Players: players, Now: time.UnixMilli(now)}
	s, err := newState(ctx, Options{Map: mapID})
	if err != nil {
		t.Fatal(err)
	}
	for i := range s.Tanks {
		s.Tanks[i].Class = "balanced"
		s.Tanks[i].Selected = true
		s.Tanks[i].MaxHP = 1
		s.Tanks[i].HP = 1
	}
	s.beginCountdown(now)
	s.onTimer(now + Countdown)
	return &s, now + Countdown
}

func allDeadExcept(s *State, keep int) {
	for i := range s.Tanks {
		if i != keep {
			s.Tanks[i].Alive = false
		}
	}
}

func TestLifecyclePlayers(t *testing.T) {
	for _, players := range []int{2, 4, 6} {
		s, _ := testState(t, players, "classic-01")
		if s.Phase != PhasePlaying {
			t.Fatalf("%d players: phase %s", players, s.Phase)
		}
		if s.TeamSize != players/2 || len(s.Tanks) != players {
			t.Fatalf("%d players: team size %d tanks %d", players, s.TeamSize, len(s.Tanks))
		}
		for i := range s.Tanks {
			if !s.Tanks[i].Alive || s.Tanks[i].HP != 1 {
				t.Fatalf("%d players: seat %d not spawned", players, i)
			}
			if s.Tanks[i].Team != teamOf(i, s.TeamSize) {
				t.Fatalf("seat %d team", i)
			}
		}
	}
}

func TestSpawnSubsets(t *testing.T) {
	cases := map[int][]int{
		2: {6},
		4: {2, 10},
		6: {2, 6, 10},
	}
	for players, xs := range cases {
		s, _ := testState(t, players, "classic-01")
		for i := 0; i < s.TeamSize; i++ {
			wantX := float64(xs[i] * Cell)
			if s.Tanks[i].X != wantX || s.Tanks[i].Y != 2*Cell {
				t.Fatalf("%d players seat %d: got %v,%v want %v,%v", players, i, s.Tanks[i].X, s.Tanks[i].Y, wantX, 2*Cell)
			}
			bi := s.TeamSize + i
			if s.Tanks[bi].X != wantX || s.Tanks[bi].Y != 10*Cell {
				t.Fatalf("%d players bottom seat %d: got %v,%v", players, bi, s.Tanks[bi].X, s.Tanks[bi].Y)
			}
		}
	}
}

func TestMovementAndBrickCollision(t *testing.T) {
	s, start := testState(t, 2, "classic-01")
	allDeadExcept(s, 0)
	tank := &s.Tanks[0]
	tank.X, tank.Y, tank.Dir = 32, 32, DirRight
	// Clear a corridor and put a brick two cells to the right.
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 6; x++ {
			s.cells[y][x] = maps.Empty
		}
	}
	s.cells[2][4] = maps.Brick
	s.Broken = make([]byte, SubCols*SubRows)
	tank.Applied = Input{At: start, Move: "right"}
	tank.LastInputAt = start
	x0 := tank.X
	for i := 0; i < 20; i++ {
		s.stepTank(tank, start+int64(i+1)*StepMS, 0.02)
	}
	if tank.X <= x0 {
		t.Fatalf("tank did not move: %v -> %v", x0, tank.X)
	}
	if tank.X+Cell > 4*Cell {
		t.Fatalf("tank passed through brick: x=%v", tank.X)
	}
}

func TestWaterBlocksTank(t *testing.T) {
	s, start := testState(t, 2, "classic-01")
	allDeadExcept(s, 0)
	tank := &s.Tanks[0]
	tank.X, tank.Y, tank.Dir = 32, 32, DirRight
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 6; x++ {
			s.cells[y][x] = maps.Empty
		}
	}
	s.cells[2][4] = maps.Water
	s.Broken = make([]byte, SubCols*SubRows)
	tank.Applied = Input{At: start, Move: "right"}
	tank.LastInputAt = start
	for i := 0; i < 20; i++ {
		s.stepTank(tank, start+int64(i+1)*StepMS, 0.02)
	}
	if tank.X+Cell > 4*Cell {
		t.Fatalf("tank entered water: x=%v", tank.X)
	}
}

func TestBulletDestroysBrickSubcell(t *testing.T) {
	s, at := testState(t, 2, "classic-01")
	allDeadExcept(s, 0)
	tank := &s.Tanks[0]
	tank.X, tank.Y, tank.Dir = 32, 32, DirRight
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			s.cells[y][x] = maps.Empty
		}
	}
	s.cells[2][3] = maps.Brick
	s.Broken = make([]byte, SubCols*SubRows)
	s.fire(tank, at)
	if len(s.Bullets) != 1 {
		t.Fatalf("no bullet")
	}
	for i := 0; i < 5; i++ {
		s.stepBullets(at+int64(i+1)*StepMS, 0.02)
	}
	broken := false
	for sy := 4; sy <= 5; sy++ {
		for sx := 6; sx <= 7; sx++ {
			if s.Broken[sy*SubCols+sx] != 0 {
				broken = true
			}
		}
	}
	if !broken {
		t.Fatalf("brick subcell not destroyed: %v", s.Broken)
	}
	if len(s.Bullets) != 0 {
		t.Fatalf("bullet survived brick")
	}
}

func TestBulletKillsTankAndRespawns(t *testing.T) {
	s, at := testState(t, 2, "classic-01")
	allDeadExcept(s, 0)
	victim := &s.Tanks[1]
	victim.Alive = true
	victim.X, victim.Y = 64, 32
	victim.Class = "balanced"
	victim.MaxHP, victim.HP = 1, 1
	victim.Tech = 2
	victim.ShieldUntil = 0
	tank := &s.Tanks[0]
	tank.X, tank.Y, tank.Dir = 32, 32, DirRight
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			s.cells[y][x] = maps.Empty
		}
	}
	s.Broken = make([]byte, SubCols*SubRows)
	s.fire(tank, at)
	for i := 0; i < 10; i++ {
		s.stepBullets(at+int64(i+1)*StepMS, 0.02)
	}
	if victim.Alive {
		t.Fatal("victim survived")
	}
	if victim.Tech != 0 {
		t.Fatal("tech was not reset")
	}
	if victim.RespawnAt < at+RespawnMS {
		t.Fatalf("respawn at %d", victim.RespawnAt)
	}
	s.tryRespawns(victim.RespawnAt)
	if !victim.Alive {
		t.Fatal("victim did not respawn")
	}
	if victim.ShieldUntil < victim.RespawnAt+ShieldMS {
		t.Fatal("respawn shield missing")
	}
}

func TestShieldBlocksDamage(t *testing.T) {
	s, at := testState(t, 2, "classic-01")
	allDeadExcept(s, 0)
	victim := &s.Tanks[1]
	victim.Alive = true
	victim.X, victim.Y = 64, 32
	victim.ShieldUntil = at + 5000
	s.damageTank(victim, 0, at, "bullet")
	if victim.HP != 1 || !victim.Alive {
		t.Fatal("shield did not block")
	}
}

func TestBaseDestructionWins(t *testing.T) {
	s, at := testState(t, 2, "classic-01")
	s.Bases[0].Alive = true
	s.Bases[1].Alive = true
	b := Bullet{ID: 1, Seat: s.TeamSize, Team: TeamBottom, X: 6*Cell + 8, Y: 8, Dir: DirUp}
	if !s.bulletHitsTerrain(&b, b.X, b.Y, at) {
		t.Fatal("base bullet did not collide")
	}
	if s.Bases[0].Alive {
		t.Fatal("top base survived")
	}
	s.Tick = 1
	s.evaluateFrame()
	if s.Result == nil || s.Result.Draw || len(s.Result.Winners) != s.TeamSize {
		t.Fatalf("result %+v", s.Result)
	}
	for _, w := range s.Result.Winners {
		if s.Tanks[w].Team != TeamBottom {
			t.Fatal("wrong winner")
		}
	}
}

func TestSameTickBasesDraw(t *testing.T) {
	s, at := testState(t, 4, "classic-01")
	s.Tick = 3
	s.damageBaseForTest(0, at)
	s.damageBaseForTest(1, at)
	s.evaluateFrame()
	if s.Result == nil || !s.Result.Draw {
		t.Fatalf("expected draw: %+v", s.Result)
	}
	if len(s.Result.Winners) != 0 {
		t.Fatal("draw has winners")
	}
	for _, r := range s.Result.Ranks {
		if r != 1 {
			t.Fatal("draw ranks not 1")
		}
	}
}

func TestPowerUps(t *testing.T) {
	s, at := testState(t, 4, "classic-01")
	tank := &s.Tanks[0]
	tank.Tech = 0
	s.applyPowerUp(tank, "star", at)
	if tank.Tech != 1 {
		t.Fatal("star")
	}
	tank.ShieldUntil = 0
	s.applyPowerUp(tank, "helmet", at)
	if tank.ShieldUntil != at+8000 {
		t.Fatal("helmet")
	}
	enemy := &s.Tanks[2]
	enemy.Alive = true
	s.applyPowerUp(tank, "grenade", at)
	if enemy.Alive {
		t.Fatal("grenade")
	}
	enemy.Alive = true
	s.applyPowerUp(tank, "clock", at)
	if enemy.FreezeUntil != at+FreezeMS {
		t.Fatal("clock")
	}
	// Shovel repairs the ring and turns it solid steel.
	s.Broken = make([]byte, SubCols*SubRows)
	for _, idx := range ringIndexes(tank.Team) {
		s.Broken[idx] = 1
	}
	s.applyPowerUp(tank, "shovel", at)
	if s.ShovelUntil[tank.Team] != at+ShovelMS {
		t.Fatal("shovel")
	}
	p := maps.Ring(tank.Team)[0]
	if !s.subSolid(p.X*2, p.Y*2, at) {
		t.Fatal("shovel ring is not solid")
	}
	s.tickShovel(at + ShovelMS)
	if s.ShovelUntil[tank.Team] != 0 {
		t.Fatal("shovel did not expire")
	}
	if !s.subSolid(p.X*2, p.Y*2, at+ShovelMS) {
		t.Fatal("shovel ring was not restored to brick")
	}
}

func TestSpareTankInstantRespawn(t *testing.T) {
	s, at := testState(t, 2, "classic-01")
	tank := &s.Tanks[0]
	tank.Spare = true
	s.killTank(tank, 1, at, "bullet")
	if tank.RespawnAt != at {
		t.Fatalf("spare respawn %d", tank.RespawnAt)
	}
	s.tryRespawns(at)
	if !tank.Alive || tank.ShieldUntil != at+SpareMS {
		t.Fatal("spare did not respawn with shield")
	}
}

func TestInputValidationAndViewPrivacy(t *testing.T) {
	s, at := testState(t, 2, "classic-01")
	m, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	ctx := gameapi.Context{Actor: 0, Players: 2, Now: time.UnixMilli(at + 100)}
	raw := json.RawMessage(`{"type":"input","epoch":"` + s.Epoch + `","seq":1,"move":"up","fire":true}`)
	nm, err := apply(ctx, m, raw)
	if err != nil {
		t.Fatal(err)
	}
	if nm.Schema != 1 {
		t.Fatal("schema")
	}
	bad := json.RawMessage(`{"type":"input","epoch":"wrong","seq":2,"move":"up","fire":true}`)
	if _, err := apply(ctx, m, bad); err == nil {
		t.Fatal("bad epoch accepted")
	}
	jump := json.RawMessage(`{"type":"input","epoch":"` + s.Epoch + `","seq":2000,"move":"up","fire":true}`)
	if _, err := apply(ctx, m, jump); err == nil {
		t.Fatal("sequence jump accepted")
	}
	view, err := project(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(view)
	for _, secret := range []string{"seed", "pending", "applied"} {
		if strings.Contains(string(b), `"`+secret+`"`) {
			t.Fatalf("view leaks %s: %s", secret, b)
		}
	}
}

func TestEnvelope(t *testing.T) {
	s, at := testState(t, 6, "classic-01")
	m, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Actors) != 6 {
		t.Fatalf("actors %v", m.Actors)
	}
	if len(m.Attention) != 0 {
		t.Fatalf("attention %v", m.Attention)
	}
	if m.Deadline == 0 {
		t.Fatal("deadline not set")
	}
	s.Bases[1].Alive = false
	s.Bases[1].DestroyedBy = 0
	s.Tick = 1
	s.evaluateFrame()
	m, _ = s.match()
	if m.Result == nil || len(m.Actors) != 0 || m.SystemDeadline != 0 || m.Deadline != 0 {
		t.Fatalf("finished envelope %+v", m)
	}
	_ = at
}

func TestOnTimerCatchesUpToFutureDeadline(t *testing.T) {
	s, _ := testState(t, 2, "classic-01")
	now := s.ServerAt + 5000
	s.onTimer(now)
	m, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	if m.Result == nil && (m.SystemDeadline <= now || s.ServerAt < now) {
		t.Fatalf("timer did not catch up: server=%d deadline=%d now=%d", s.ServerAt, m.SystemDeadline, now)
	}
}
