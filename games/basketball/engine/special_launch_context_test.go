package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestOriginalSpecialLaunchContext(t *testing.T) {
	f, e := os.Open("testdata/special-launch-context.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		worldActionFixture
		Kind          string
		Actor         uint8
		Prepared      WorldActionState
		SecondPlayerX Fixed
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 20 {
		t.Fatal(len(rows))
	}
	for i, r := range rows {
		s := r.Before
		c := r.context(t)
		source := PassLaunchContext{SecondPlayerX: r.SecondPlayerX}
		if r.Kind == "player" {
			a := s.Actors[r.Actor]
			ctx := LaunchedPlayerContext{Actor: r.Actor, Attachment: s.PlayerLinks[r.Actor], Random: s.Random, Computer: c.Controls[r.Actor]&128 != 0, HasBall: s.Holder == int8(r.Actor), SpecialShot: a.SpecialShot, ShotCycle: a.ShotCycle, HighArc: s.MountFlags[s.SelectedHoop-7]&2 != 0, AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}}
			if _, e = PrepareLaunchedPlayer(a.Player, ctx); e == nil || !strings.Contains(e.Error(), "second-player X context") {
				t.Fatal("missing context did not reject original diagonal", i, e)
			}
			step, e := PrepareLaunchedPlayer(a.Player, ctx, source)
			if e != nil {
				t.Fatal(i, e)
			}
			applyLaunchedPreparation(&s, r.Actor, step)
		} else {
			ctx := teamSpecialContext(s, c)
			if _, e = PrepareOriginalTeamSpecialBall(s.Ball, ctx); e == nil || !strings.Contains(e.Error(), "second-player X context") {
				t.Fatal("missing ball context did not reject original diagonal", i, e)
			}
			step, e := PrepareOriginalTeamSpecialBall(s.Ball, ctx, source)
			if e != nil {
				t.Fatal(i, e)
			}
			shake := uint8(0)
			applyTeamSpecialStep(&s, &shake, step)
		}
		if s != r.Prepared {
			t.Fatalf("call%d %s X69=%d: %s", i, r.Kind, uint8(r.SecondPlayerX), firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.Prepared), "world"))
		}
	}
}

func TestWorldSpecialLaunchReadsCurrentSecondPlayerX(t *testing.T) {
	f, e := os.Open("testdata/special-launch-context.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		worldActionFixture
		Kind          string
		Actor         uint8
		Prepared      WorldActionState
		SecondPlayerX Fixed
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	for i, row := range rows {
		r := WorldActionStep{State: row.Before}
		ctx := row.context(t)
		// This is the authoritative state immediately before this actor/object,
		// not an input-stage cached coordinate or the projectile's fraction.
		if r.State.Actors[1].Player.Body.Position.X != row.SecondPlayerX {
			t.Fatal("oracle source is not the launch-time player1 coordinate")
		}
		court, e := originalWorldMotionCourt(&r, ctx, row.Actor)
		if e != nil {
			t.Fatal(e)
		}
		load := court.originalMotion.load
		observed := false
		court.originalMotion.load = func() CourtFloorState {
			observed = true
			body := r.State.Ball.Body
			want := row.Prepared.Ball.Body
			if row.Kind == "player" {
				body = r.State.Actors[row.Actor].Player.Body
				want = row.Prepared.Actors[row.Actor].Player.Body
			}
			if body != want {
				t.Fatalf("call%d launched before motion: got %+v want %+v", i, body, want)
			}
			return load()
		}
		if row.Kind == "player" {
			e = advanceWorldLaunchedPlayer(&r, row.Actor, ctx, court)
		} else {
			e = advanceWorldTeamSpecialBall(&r, ctx, court)
		}
		if e != nil {
			t.Fatalf("world call%d %s: %v", i, row.Kind, e)
		}
		if !observed {
			t.Fatal("world launch skipped original motion")
		}
	}
}
