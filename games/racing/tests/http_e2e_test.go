package racing_test

// Failure scenarios, written before the racing implementation:
//   - An unpublished game appears in the lobby or bypasses admin publication.
//   - Invalid capacity/options are admitted, or supported 2–6 seat rooms fail.
//   - One driver's revision invalidates another driver's simultaneous input.
//   - Replayed nonces/sequence numbers, old epochs, observers or forged positions
//     drive a vehicle; countdown input moves the car before the starting lights.
//   - SQLite recovery changes race identity, setup, controls or progress.
//   - The host never schedules computer/autoplay input, takeover resets the race,
//     or timeout/reclaim changes another player's independent input deadline.
//   - A large sequence jump exhausts the controller's next sequence; switching
//     that seat to autoplay then rolls back every frame and freezes every car.
// These tests use real authenticated HTTP requests and the production module.
// They never mutate game snapshots, call private host methods or replace time.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

type httpRaceCar struct {
	Seat, Seq, Skin, Lap, Checkpoint, Rank int
	X, Y, Speed, Progress, Throttle        float64
	LastInputAt, IdleAt, FinishAt          int64
}

type httpRaceView struct {
	Phase, Epoch                                 string
	ServerAt, StartAt, FinishDeadline, MaxRaceAt int64
	Options                                      struct {
		Track string
		Laps  int
	}
	Cars     []httpRaceCar
	Controls struct{ CanDrive, CanReset, CanSkin bool }
}

func httpRaceState(t *testing.T, c *gametest.Client) httpRaceView {
	t.Helper()
	var v httpRaceView
	b, err := json.Marshal(gametest.Data(c.Room()))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func httpDrive(v httpRaceView, seat int, throttle float64) map[string]any {
	return map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[seat].Seq + 1, "steer": 0, "throttle": throttle, "brake": false, "drift": false}
}

// Keep module-specific, repeatable evidence under the game's own artifact path.
func httpRaceEvidence(t *testing.T, h *gametest.HTTP) func(string) {
	t.Helper()
	trace := []any{}
	t.Cleanup(func() {
		dir, err := os.Getwd()
		if err != nil {
			t.Error(err)
			return
		}
		for {
			if _, err = os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				t.Error("repository not found")
				return
			}
			dir = parent
		}
		dir = filepath.Join(dir, ".local", "racing", "http")
		if base := os.Getenv("WIRE_BOARD_RACING_EVIDENCE"); base != "" {
			dir = filepath.Join(base, "http")
		}
		if err = os.MkdirAll(dir, 0755); err != nil {
			t.Error(err)
			return
		}
		b, err := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real HTTP, authenticated sessions, production racing module, real clock and SQLite restore", "trace": trace}, "", "  ")
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+".json"), b, 0600)
		}
		if err != nil {
			t.Error(err)
		}
	})
	return func(label string) {
		t.Helper()
		views := []any{}
		for _, c := range h.People {
			views = append(views, c.State()["room"])
		}
		trace = append(trace, map[string]any{"label": label, "views": views, "spectator": h.Spectator.State()["room"]})
	}
}

// A bad response token may be rejected by AcceptStale (409) before Apply (400).
// Both reject it; neither is allowed to become a successful drive.
func httpRejectDrive(t *testing.T, c *gametest.Client, action map[string]any) {
	t.Helper()
	r := c.Room()
	body, err := json.Marshal(c.ActionRequest(action))
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", c.Base+"/api/rooms/"+r["id"].(string), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 && res.StatusCode != 409 {
		var response any
		_ = json.NewDecoder(res.Body).Decode(&response)
		t.Fatalf("invalid drive accepted: status %d response %v", res.StatusCode, response)
	}
}

func TestRacingHTTPPublicationConfigurationAndInputIsolation(t *testing.T) {
	tracks := []string{"metro", "harbor", "woodland", "coast", "canyon"}
	laps := []int{1, 3, 5}
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, game.Module(), "racing")
			record := httpRaceEvidence(t, h)
			h.AssertUnpublished()
			track, lapCount := tracks[(n-2)%len(tracks)], laps[(n-2)%3]
			options := map[string]any{"track": track, "laps": lapCount}
			if n == 2 {
				options, lapCount = map[string]any{}, 3
			}
			h.Start(options)
			c := h.People[0]
			initial := httpRaceState(t, c)
			if initial.Phase != "countdown" || initial.StartAt <= initial.ServerAt || initial.Epoch == "" || len(initial.Cars) != n || initial.Options.Track != track || initial.Options.Laps != lapCount {
				t.Fatalf("invalid configured countdown: %+v", initial)
			}
			if !initial.Controls.CanDrive || !initial.Controls.CanSkin {
				t.Fatal("countdown does not accept prepared input and skin")
			}
			observer := httpRaceState(t, h.Spectator)
			if observer.Controls.CanDrive || observer.Controls.CanReset || observer.Controls.CanSkin {
				t.Fatal("spectator has controls")
			}
			h.Spectator.Action(httpDrive(observer, 0, 1), 400)
			room := c.Room()
			id, version := room["id"].(string), room["version"]
			requests := make([]map[string]any, n)
			for p, person := range h.People {
				requests[p] = person.ActionRequest(httpDrive(initial, p, .5))
				requests[p]["version"] = version
			}
			for p, person := range h.People {
				person.Request("POST", "/api/rooms/"+id, requests[p], 200)
			}
			after := httpRaceState(t, c)
			for p, car := range after.Cars {
				if car.Seq != 1 || car.Lap != 0 || car.X != initial.Cars[p].X || car.Y != initial.Cars[p].Y {
					t.Fatalf("simultaneous input lost or moved before start: seat %d car %+v", p, car)
				}
			}
			c.Request("POST", "/api/rooms/"+id, requests[0], 200)
			if httpRaceState(t, c).Cars[0].Seq != 1 {
				t.Fatal("nonce replay applied twice")
			}
			httpRejectDrive(t, c, requests[0]["action"].(map[string]any))
			for name, mutate := range map[string]func(map[string]any){
				"old epoch":         func(a map[string]any) { a["epoch"] = "old-race" },
				"zero seq":          func(a map[string]any) { a["seq"] = 0 },
				"excess throttle":   func(a map[string]any) { a["throttle"] = 1.1 },
				"excess steering":   func(a map[string]any) { a["steer"] = -1.1 },
				"forged coordinate": func(a map[string]any) { a["x"] = 9999 },
			} {
				a := httpDrive(after, 0, .5)
				mutate(a)
				httpRejectDrive(t, c, a)
				if httpRaceState(t, c).Cars[0].Seq != after.Cars[0].Seq {
					t.Fatalf("%s consumed input", name)
				}
			}
			c.Action(map[string]any{"type": "skin", "epoch": initial.Epoch, "skin": 4}, 200)
			before := httpRaceState(t, c)
			h.Restart()
			restored := httpRaceState(t, c)
			if restored.Epoch != before.Epoch || restored.StartAt != before.StartAt || restored.MaxRaceAt != before.MaxRaceAt || restored.Options != before.Options {
				t.Fatalf("countdown recovery changed race: before %+v after %+v", before, restored)
			}
			for p, car := range restored.Cars {
				expected := before.Cars[p]
				if car.Throttle != 0 || car.LastInputAt < expected.LastInputAt {
					t.Fatal("restart revived held throttle or rolled back control time", car)
				}
				// Match progress/cosmetics/sequence stay exact; startup neutralizes
				// held human control and refreshes only its input timestamps.
				expected.Throttle = 0
				expected.LastInputAt, expected.IdleAt = car.LastInputAt, car.IdleAt
				if !reflect.DeepEqual(car, expected) {
					t.Fatal("restart changed race progress", car, expected)
				}
			}
			record("admin publication, supported room options, concurrent stale revisions, rejected replay/forgery, observer read-only and real SQLite recovery")
		})
	}
}

func TestRacingHTTPRejectInvalidConfiguration(t *testing.T) {
	h := gametest.NewHTTP(t, 1, game.Module(), "racing")
	record := httpRaceEvidence(t, h)
	c := h.People[0]
	c.Request("POST", "/api/admin/games/racing", map[string]bool{"enabled": true}, 200)
	for _, input := range []struct {
		capacity int
		options  map[string]any
	}{
		{1, map[string]any{}}, {7, map[string]any{}},
		{2, map[string]any{"track": "unknown"}}, {2, map[string]any{"laps": 2}},
		{2, map[string]any{"laps": 0}}, {2, map[string]any{"laps": 1, "speed": 200}},
	} {
		c.Request("POST", "/api/rooms", map[string]any{"kind": "racing", "name": "invalid racing configuration", "capacity": input.capacity, "options": input.options}, 400)
	}
	record("invalid capacities, unknown track, unsupported laps and forged configuration fields rejected")
}

func httpAwaitRace(t *testing.T, c *gametest.Client, within time.Duration, predicate func(httpRaceView) bool) httpRaceView {
	t.Helper()
	until := time.Now().Add(within)
	var v httpRaceView
	for time.Now().Before(until) {
		v = httpRaceState(t, c)
		if predicate(v) {
			return v
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("race did not reach expected state in %s: %+v", within, v)
	return v
}

func TestRacingHTTPRejectSequenceExhaustion(t *testing.T) {
	for _, seq := range []int64{1025, 9007199254740991} {
		t.Run(fmt.Sprint(seq), func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, game.Module(), "racing")
			record := httpRaceEvidence(t, h)
			h.Start(map[string]any{"track": "coast", "laps": 1})
			c := h.People[0]
			before := httpRaceState(t, c)
			if before.Cars[0].Seq != 0 {
				t.Fatal("sequence exhaustion test did not start from seq 0")
			}
			malicious := httpDrive(before, 0, 1)
			malicious["seq"] = seq
			c.Action(malicious, 400)
			after := httpRaceState(t, c)
			if after.Cars[0].Seq != 0 {
				t.Fatal("rejected sequence exhausted the controller counter")
			}
			c.Action(httpDrive(after, 0, .75), 200)
			for _, person := range h.People {
				person.Command("autoplay", map[string]any{"enabled": true}, 200)
			}
			httpAwaitRace(t, c, 8*time.Second, func(v httpRaceView) bool {
				return v.Phase == "racing" && v.ServerAt > before.StartAt+500 &&
					v.Cars[0].Seq > 1 && v.Cars[1].Seq > 0 &&
					math.Hypot(v.Cars[0].X-before.Cars[0].X, v.Cars[0].Y-before.Cars[0].Y) > 10 &&
					math.Hypot(v.Cars[1].X-before.Cars[1].X, v.Cars[1].Y-before.Cars[1].Y) > 10
			})
			record("malicious sequence jump rejected; seq 1 accepted; real autoplay advanced simulation clock and both vehicles")
		})
	}
}

func TestRacingHTTPComputerTakeoverAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 1, game.Module(), "racing")
	record := httpRaceEvidence(t, h)
	c := h.People[0]
	c.Request("POST", "/api/admin/games/racing", map[string]bool{"enabled": true}, 200)
	c.Request("POST", "/api/rooms", map[string]any{"kind": "racing", "name": "computer and takeover", "capacity": 2, "options": map[string]any{"track": "coast", "laps": 1}}, 201)
	c.Command("add_bot", nil, 200)
	c.Command("ready", nil, 200)
	c.Command("start", nil, 200)
	h.Spectator.Request("POST", "/api/rooms/"+c.Room()["id"].(string)+"/watch", map[string]any{}, 200)
	initial := httpRaceState(t, c)
	for _, seq := range []int64{1025, 9007199254740991} {
		malicious := httpDrive(initial, 0, 1)
		malicious["seq"] = seq
		c.Action(malicious, 400)
	}
	c.Action(httpDrive(initial, 0, .75), 200)
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	before := httpAwaitRace(t, c, 12*time.Second, func(v httpRaceView) bool {
		return v.Phase == "racing" && v.ServerAt > initial.StartAt && v.Cars[0].Seq > 1 && v.Cars[1].Seq > 0 && v.Cars[0].Speed > 10 && v.Cars[1].Speed > 10
	})
	c.Command("autoplay", map[string]any{"enabled": false}, 200)
	after := httpRaceState(t, c)
	if c.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] == true || after.Epoch != before.Epoch || after.StartAt != before.StartAt || after.Cars[0].Throttle != 0 || after.Cars[0].Lap != before.Cars[0].Lap || after.Cars[0].Progress < before.Cars[0].Progress {
		t.Fatalf("takeover did not preserve race and release throttle: before %+v after %+v", before, after)
	}
	c.Action(httpDrive(after, 0, .75), 200)
	manual := httpRaceState(t, c)
	if manual.Cars[0].Seq != after.Cars[0].Seq+1 {
		t.Fatal("manual input after takeover was not accepted")
	}
	record("real host scheduled computer and autoplay; human reclaimed and drove")
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	before = httpRaceState(t, c)
	h.Restart()
	after = httpRaceState(t, c)
	if c.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] != true || after.Epoch != before.Epoch || after.StartAt != before.StartAt || after.MaxRaceAt != before.MaxRaceAt || after.Options != before.Options {
		t.Fatal("SQLite recovery reset match or lost autoplay")
	}
	for p := range before.Cars {
		if after.Cars[p].Lap < before.Cars[p].Lap || after.Cars[p].Seq < before.Cars[p].Seq {
			t.Fatal("SQLite recovery lost progress or input sequence")
		}
	}
	httpAwaitRace(t, c, 3*time.Second, func(v httpRaceView) bool {
		return v.Cars[0].Seq > after.Cars[0].Seq && v.Cars[1].Seq > after.Cars[1].Seq
	})
	record("real SQLite restart preserved identity, progress and autoplay; both computer controllers resumed")
	if os.Getenv("WIRE_BOARD_RACING_HTTP_FULL") != "1" {
		return
	}
	finished := httpAwaitRace(t, c, 3*time.Minute, func(v httpRaceView) bool { return v.Phase == "finished" })
	if c.Room()["status"] != "finished" || finished.FinishDeadline == 0 {
		t.Fatal("computer race ended without a natural finish")
	}
	for _, car := range finished.Cars {
		if car.FinishAt == 0 || car.Lap != finished.Options.Laps {
			t.Fatal("computer failed to complete the circuit naturally", car)
		}
	}
	httpRejectDrive(t, c, httpDrive(finished, 0, 1))
	record("computer/autoplay completed every lap through real wall-clock HTTP host")
	h.Restart()
	if !reflect.DeepEqual(finished, httpRaceState(t, c)) {
		t.Fatal("completed race changed after SQLite recovery")
	}
}

func TestRacingHTTPDroppedInputIdleAndAutoPlayReclaim(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "racing")
	record := httpRaceEvidence(t, h)
	h.Start(map[string]any{"track": "woodland", "laps": 1})
	c, other := h.People[0], h.People[1]
	v := httpAwaitRace(t, c, 6*time.Second, func(v httpRaceView) bool { return v.Phase == "racing" })
	// Drive seat 0 long enough to establish motion, then drop its entire stream.
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		v = httpRaceState(t, c)
		c.Action(httpDrive(v, 0, 1), 200)
		other.Action(httpDrive(httpRaceState(t, other), 1, 0), 200)
		time.Sleep(100 * time.Millisecond)
	}
	moving := httpRaceState(t, c)
	if moving.Cars[0].Speed <= 10 || math.Hypot(moving.Cars[0].X-v.Cars[0].X, moving.Cars[0].Y-v.Cars[0].Y) <= 0 {
		t.Fatal("real drive input never moved the car")
	}
	until = time.Now().Add(1100 * time.Millisecond)
	for time.Now().Before(until) {
		other.Action(httpDrive(httpRaceState(t, other), 1, 0), 200)
		time.Sleep(100 * time.Millisecond)
	}
	stopped := httpRaceState(t, c)
	if stopped.Cars[0].Speed >= moving.Cars[0].Speed || stopped.Cars[0].LastInputAt != moving.Cars[0].LastInputAt {
		t.Fatalf("dropped input kept accelerating or manufactured new input: before %+v after %+v", moving.Cars[0], stopped.Cars[0])
	}
	record("dropping the real input stream causes automatic braking")
	until = time.Now().Add(18 * time.Second)
	for time.Now().Before(until) {
		other.Action(httpDrive(httpRaceState(t, other), 1, 0), 200)
		r := c.Room()
		for _, seat := range r["seats"].([]any) {
			if seat.(map[string]any)["autoPlay"] == true {
				t.Fatal("idle real-time player was automatically taken over")
			}
		}
		if r["turnDeadline"] != float64(0) {
			t.Fatal("real-time action countdown remains", r["turnDeadline"])
		}
		time.Sleep(200 * time.Millisecond)
	}
	record("idle human stays manual beyond the former 15 second timeout")
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	if c.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("manual autoplay unavailable")
	}
	before := httpRaceState(t, c)
	c.Command("autoplay", map[string]any{"enabled": false}, 200)
	after := httpRaceState(t, c)
	if after.Epoch != before.Epoch || after.StartAt != before.StartAt || after.Cars[1].LastInputAt != before.Cars[1].LastInputAt || after.Cars[1].IdleAt != before.Cars[1].IdleAt || after.Cars[0].Throttle != 0 {
		t.Fatal("manual reclaim reset race, retained throttle or changed opponent input deadline")
	}
	c.Action(httpDrive(after, 0, .5), 200)
	record("manual autoplay and reclaim preserve the race and opponent deadlines")
}
