package racing

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func mustMarshalRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func startGapRace(t *testing.T, players, laps int, track string) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	mod := Module()
	ctx := gameapi.Context{Players: players, Now: time.Unix(1794500000, 0)}
	m, err := mod.Start(ctx, mustMarshalRaw(t, map[string]any{"track": track, "laps": laps}))
	if err != nil {
		t.Fatal(err)
	}
	return mod, ctx, m
}

func projectGapView(t *testing.T, mod gameapi.Module, m gameapi.Match, seat int) View {
	t.Helper()
	v, err := mod.View(m, seat)
	if err != nil {
		t.Fatal(err)
	}
	var out View
	if err := json.Unmarshal(mustMarshalRaw(t, v), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTimeGapCountdownAndLeaderZero(t *testing.T) {
	mod, ctx, m := startGapRace(t, 4, 1, "france")
	v := projectGapView(t, mod, m, 0)
	for _, c := range v.Cars {
		if c.GapMS != 0 || c.GapLaps != 0 {
			t.Fatalf("countdown gap must be zero, got seat=%d gapMs=%d gapLaps=%d", c.Seat, c.GapMS, c.GapLaps)
		}
	}
	ctx.Now = ctx.Now.Add(3000 * time.Millisecond)
	var err error
	m, err = mod.OnTimer(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	for step := 1; step <= 45; step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		v = projectGapView(t, mod, m, 0)
		ctx.Actor = 0
		m, err = mod.Apply(ctx, m, mustMarshalRaw(t, map[string]any{
			"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "steer": 0, "throttle": 1, "brake": false, "drift": false,
		}))
		if err != nil {
			t.Fatal(err)
		}
		ctx.Actor = 1
		m, err = mod.Apply(ctx, m, mustMarshalRaw(t, map[string]any{
			"type": "drive", "epoch": v.Epoch, "seq": v.Cars[1].Seq + 1, "steer": 0, "throttle": 0.45, "brake": false, "drift": false,
		}))
		if err != nil {
			t.Fatal(err)
		}
		ctx.Actor = -1
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
	}
	v = projectGapView(t, mod, m, 0)
	if v.Cars[0].Rank != 1 || v.Cars[0].GapMS != 0 || v.Cars[0].GapLaps != 0 {
		t.Fatalf("leader must have rank 1 and zero gap, got %+v", v.Cars[0])
	}
	if v.Cars[1].GapMS <= 200 || v.Cars[1].GapMS > 15000 {
		t.Fatalf("trailing car must report positive realistic gapMs, got %d", v.Cars[1].GapMS)
	}
	if v.Cars[2].GapMS <= v.Cars[1].GapMS {
		t.Fatalf("stopped car must trail further than moving car: stopped=%d moving=%d", v.Cars[2].GapMS, v.Cars[1].GapMS)
	}
}

func TestTimeGapUsesExactFinishDeltaAndLaps(t *testing.T) {
	track, ok := classicTrackByID("france")
	if !ok {
		t.Fatal("france track missing")
	}
	info := classicTrackInfoFor(track)
	s := State{
		Physics:  3,
		Options:  Options{Track: "france", Laps: 3, TimeOfDay: "day"},
		Phase:    "finishing",
		Epoch:    "gaptest0001",
		ServerAt: 200000,
		StartAt:  100000,
		Track:    track,
		Cars: []Car{
			{Seat: 0, Lap: 3, Progress: 3 * info.length, FinishAt: 185240},
			{Seat: 1, Lap: 3, Progress: 3 * info.length, FinishAt: 187890},
			{Seat: 2, Lap: 1, Progress: 1.4 * info.length},
		},
	}
	rank(&s)
	updateTimeGaps(&s)
	if s.Cars[0].GapMS != 0 || s.Cars[0].GapLaps != 0 {
		t.Fatalf("finished leader gap must be zero: %+v", s.Cars[0])
	}
	if s.Cars[1].GapMS != 2650 || s.Cars[1].GapLaps != 0 {
		t.Fatalf("finished runner-up must use exact finishAt delta 2650ms, got gapMs=%d gapLaps=%d", s.Cars[1].GapMS, s.Cars[1].GapLaps)
	}
	if s.Cars[2].GapLaps != 1 {
		t.Fatalf("car 1.6 laps behind must report GapLaps=1, got %d", s.Cars[2].GapLaps)
	}
}

func TestTimeGapGateCrossingAccuracyAndLegacySaveCompat(t *testing.T) {
	track, ok := classicTrackByID("germany")
	if !ok {
		t.Fatal("germany track missing")
	}
	info := classicTrackInfoFor(track)
	s := State{
		Physics:  3,
		Options:  Options{Track: "germany", Laps: 3, TimeOfDay: "day"},
		Phase:    "racing",
		Epoch:    "gaptest0002",
		ServerAt: 130000,
		StartAt:  100000,
		Track:    track,
		Cars: []Car{
			{Seat: 0, Lap: 0, Progress: 0.40 * info.length, Speed: 400},
			{Seat: 1, Lap: 0, Progress: 0.30 * info.length, Speed: 20},
		},
	}
	steps := 250
	for k := 0; k < steps; k++ {
		p0 := (0.25 + 0.15*float64(k)/float64(steps)) * info.length
		p1 := (0.25 + 0.15*float64(k+1)/float64(steps)) * info.length
		at := int64(125000) + int64(k+1)*stepMS
		recordGapGateCrossing(&s, &s.Cars[0], p0, p1, at)
	}
	rank(&s)
	updateTimeGaps(&s)
	if math.Abs(float64(s.Cars[1].GapMS-3333)) > 150 {
		t.Fatalf("gate crossing gap should reflect leader crossing time (~3333ms) even when trailing car brakes, got %d", s.Cars[1].GapMS)
	}
	mod, ctx, m := startGapRace(t, 6, 1, "france")
	vBytes := mustMarshalRaw(t, projectGapView(t, mod, m, 0))
	if len(vBytes) > 8192 {
		t.Fatalf("6-car view exceeded 8KiB budget: %d", len(vBytes))
	}
	if _, err := mod.Restore(ctx, m); err != nil {
		t.Fatalf("restore with time gap fields failed: %v", err)
	}
}
