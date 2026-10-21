package bubbleparty

// All failure boundaries precede implementation in
// docs/research/bubble-classic-maps/IMPLEMENTATION.md.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"strings"
	"testing"
	"time"
)

func TestNativeMapCatalogAndAllSeatSpawns(t *testing.T) {
	n := 0
	for _, id := range mapIDs() {
		if !strings.HasPrefix(id, "classic2002-") {
			continue
		}
		n++
		cells := mapCells(id, 1)
		if len(cells) != Cols*Rows {
			t.Fatal(id, "wrong dimensions")
		}
		for _, players := range []int{2, 4, 6, 8} {
			seen := map[[2]int]bool{}
			for team := 0; team < 2; team++ {
				points := mapSpawnsFor(id, players/2, team)
				if len(points) != players/2 {
					t.Fatal(id, "missing spawns")
				}
				for _, p := range points {
					if p[0] < 0 || p[1] < 0 || p[0] >= Cols || p[1] >= Rows || cells[p[1]*Cols+p[0]] != CellEmpty || seen[p] {
						t.Fatal(id, players, "invalid/duplicate spawn", p)
					}
					seen[p] = true
				}
			}
		}
	}
	if n != 112 {
		t.Fatalf("classic map count %d", n)
	}
	if mapCells("community10", 1)[8] != 4 {
		t.Fatal("district wooden road box remains fixed")
	}
}
func TestNativeCharacterSelectionBeforeCountdown(t *testing.T) {
	ctx := gameapi.Context{Players: 2, Now: time.UnixMilli(1700000000000)}
	mod := Module()
	m, e := mod.Start(ctx, json.RawMessage(`{"map":"community10","roundSeconds":180}`))
	if e != nil {
		t.Fatal(e)
	}
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	if s.Phase != "select" {
		t.Fatal("missing character stage", s.Phase)
	}
	for seat, id := range []int{1, 2} {
		ctx.Actor = seat
		raw, _ := json.Marshal(map[string]any{"type": "character", "epoch": s.Epoch, "character": id, "confirm": true})
		m, e = mod.Apply(ctx, m, raw)
		if e != nil {
			t.Fatal(e)
		}
	}
	s, e = decode(m)
	if e != nil {
		t.Fatal(e)
	}
	if s.Phase != PhaseCountdown || s.StartAt != ctx.Now.UnixMilli()+Countdown {
		t.Fatal("all confirmed did not start countdown", s.Phase)
	}
	if s.Players[0].MaxBombs != 2 || s.Players[0].Range != 1 || s.Players[1].MaxBombs != 1 || s.Players[1].Range != 2 || math.Abs(effectiveMoveMS(&s.Players[0])-300) > .001 {
		t.Fatal("original character defaults lost", s.Players)
	}
	s.onTimer(s.StartAt)
	m, _ = s.match()
	ctx.Now = time.UnixMilli(s.ServerAt)
	if _, e = mod.Apply(ctx, m, mustJSON(map[string]any{"type": "character", "epoch": s.Epoch, "character": 0, "confirm": true})); e == nil {
		t.Fatal("character can change after start")
	}
	restored, e := mod.Restore(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	again, _ := decode(restored)
	if again.Players[0].Character != 1 || again.Players[0].NativeSpeed != 4 {
		t.Fatal("restore changed profile")
	}
}

// Realtime bots are only scheduled at SystemDeadline. Selection must wake the
// host before its 20-second user deadline, without automatically taking humans.
func TestNativeSelectionSchedulesBotFrames(t *testing.T) {
	ctx := gameapi.Context{Players: 2, Now: time.UnixMilli(1700000000000)}
	m, err := Module().Start(ctx, json.RawMessage(`{"map":"community10"}`))
	if err != nil {
		t.Fatal(err)
	}
	s, _ := decode(m)
	if m.SystemDeadline > s.ServerAt+FrameMS || len(m.TimeoutActors) != 0 {
		t.Fatal("selection cannot schedule timely bots", m.SystemDeadline, s.ServerAt)
	}
	before := s.SelectUntil
	s.onTimer(s.ServerAt + FrameMS)
	next, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	if next.SystemDeadline <= m.SystemDeadline || s.SelectUntil != before || s.Phase != PhaseSelect {
		t.Fatal("selection frame stalled or altered human deadline")
	}
}
