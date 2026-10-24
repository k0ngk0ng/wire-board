package tabletennis

// Before the browser predictor: bind its short visual extrapolation to actual
// server flight samples. This fixture never generates a match result.
import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPredictionOracle(t *testing.T) {
	rows := []any{}
	for _, phase := range []string{"serve", "rally"} {
		for seat := 0; seat < 2; seat++ {
			for ends := 0; ends < 2; ends++ {
				for _, kind := range []string{"drive", "chop", "block"} {
					for _, power := range []float64{.35, .85, 1} {
						s := fixture(t, 2)
						s.Epoch = "oracle-v1"
						s.Order = []int{0, 1}
						s.InitialOrder = []int{0, 1}
						s.Ends = ends
						s.Phase = phase
						s.Expected = seat
						s.Receiver = 1 - seat
						s.Flight = nil
						s.ServerAt = 1000
						s.PhaseUntil = 1000
						if phase == "rally" {
							s.Flight = &Flight{Spin: -.95, ContactStart: 1000, Segments: []Segment{{At: 1000, Until: 2000, P: Vec{.15, 1.3, sign(s.side(s.Options.Teams[seat])) * .9}, V: Vec{}, A: Vec{}}}}
						}
						m, e := s.match()
						if e != nil {
							t.Fatal(e)
						}
						view, e := project(m, seat)
						if e != nil {
							t.Fatal(e)
						}
						a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: kind, Aim: .65, Power: power, At: 1130}
						at := a.At
						if phase == "serve" {
							at += 180
						}
						s.launch(a, seat, at, phase == "serve")
						points := []any{}
						// The display predictor deliberately stops before a physical
						// bounce/net outcome. Fast shots can reach it inside 200ms.
						horizon := int64(200)
						for _, event := range s.Flight.Events {
							if event.At > at && event.At-at <= horizon {
								horizon = max(1, event.At-at-2)
							}
						}
						for _, delta := range []int64{0, horizon / 4, horizon / 2, horizon} {
							p, _ := s.Flight.at(at + delta)
							points = append(points, map[string]any{"at": at + delta, "p": p})
						}
						rows = append(rows, map[string]any{"name": fmt.Sprintf("%s-seat%d-ends%d-%s-%.2f", phase, seat, ends, kind, power), "view": view, "seat": seat, "stroke": map[string]any{"kind": kind, "aim": a.Aim, "power": power}, "at": a.At, "points": points})
					}
				}
			}
		}
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	b = append(b, '\n')
	path := filepath.Join("..", "tests", "testdata", "prediction-oracle.json")
	if os.Getenv("TT_UPDATE_ORACLE") == "1" {
		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	expected, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	// ARM may fuse multiply/add where amd64 rounds separately. Compare only
	// sampled coordinates with a sub-picometre tolerance; keep the complete
	// view, input, sample timestamps and schema exact. Do not regenerate the
	// independent client reference merely to match a runner's architecture.
	var want, got []map[string]any
	if json.Unmarshal(expected, &want) != nil || json.Unmarshal(b, &got) != nil || len(want) != len(got) {
		t.Fatal("invalid prediction reference shape")
	}
	for i := range want {
		wp, gp := want[i]["points"].([]any), got[i]["points"].([]any)
		if len(wp) != len(gp) {
			t.Fatal("prediction sample count changed")
		}
		for j := range wp {
			w := wp[j].(map[string]any)["p"].(map[string]any)
			g := gp[j].(map[string]any)["p"].(map[string]any)
			for _, axis := range []string{"x", "y", "z"} {
				if math.Abs(w[axis].(float64)-g[axis].(float64)) > 1e-12 {
					t.Fatalf("%s sample %d %s differs: want %v got %v", want[i]["name"], j, axis, w[axis], g[axis])
				}
				g[axis] = w[axis]
			}
		}
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatal("client prediction reference differs from server physics; review and regenerate explicitly")
	}
}
