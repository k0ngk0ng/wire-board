package racing

// Failure inventory: docs/research/racing-remake/PLAN.md.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func TestRemakeFiveLongTracksAndLegacyGeometry(t *testing.T) {
	tracks := Tracks()
	if len(tracks) != 5 {
		t.Fatalf("need exactly five selectable tracks, got %d", len(tracks))
	}
	ids := map[string]bool{}
	for _, track := range tracks {
		if ids[track.ID] {
			t.Fatal("duplicate track")
		}
		ids[track.ID] = true
		_, length := pathLengths(track)
		if length < 11000 {
			t.Fatalf("track %s remains short: %.0f", track.ID, length)
		}
		if len(track.Points) > 320 {
			t.Fatalf("unbounded snapshot geometry %s: %d", track.ID, len(track.Points))
		}
		for _, p := range track.Points {
			if p.X < track.RoadWidth || p.Y < track.RoadWidth || p.X > track.Width-track.RoadWidth || p.Y > track.Height-track.RoadWidth {
				t.Fatalf("road outside world: %s %+v", track.ID, p)
			}
		}
		ctx := gameapi.Context{Players: 6, Now: time.UnixMilli(1700000000000)}
		raw, _ := json.Marshal(Options{Track: track.ID, Laps: 5})
		m, err := start(ctx, raw)
		if err != nil {
			t.Fatal(err)
		}
		s, err := decode(m)
		if err != nil {
			t.Fatal(err)
		}
		if s.MaxRaceAt-s.StartAt < 900000 {
			t.Fatalf("long endurance race keeps old short cap: %d", s.MaxRaceAt-s.StartAt)
		}
		// A saved historical geometry must remain authoritative after restoring.
		s.Track.Points[2].X += 1
		s.MaxRaceAt += 1234
		m, _ = s.match()
		got, err := restore(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		back, _ := decode(got)
		if !reflect.DeepEqual(s.Track, back.Track) || s.MaxRaceAt != back.MaxRaceAt {
			t.Fatal("restoring replaced saved route or race deadline")
		}
	}
	for _, id := range []string{"metro", "harbor", "coast", "woodland", "canyon"} {
		if !ids[id] {
			t.Fatal("missing", id)
		}
	}
	if Module().Name != "极品飞车" {
		t.Fatal("public game name not updated")
	}
}
