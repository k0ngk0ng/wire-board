package cs16

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func observerFixture() *matchState {
	world := engine.Map{ID: "observer-fixture", Planes: []engine.Plane{{X: engine.NormalUnit, Dist: 50 * engine.Unit}, {X: engine.NormalUnit, Dist: 60 * engine.Unit}}, Nodes: []engine.Node{{Plane: 0, Front: 1, Back: engine.ContentsEmpty}, {Plane: 1, Front: engine.ContentsEmpty, Back: engine.ContentsSolid}}, Root: 0}
	return &matchState{Schema: 1, Epoch: "observer-test", BaseAt: 1000, World: engine.State{World: world, MapID: world.ID, Players: []engine.Player{{Seat: 0, Team: "t", Alive: false, Life: 1}, {Seat: 1, Team: "ct", Alive: true, Life: 1, X: 150 * engine.Unit}, {Seat: 2, Team: "t", Alive: true, Life: 1}, {Seat: 3, Team: "t", Alive: false, Life: 1, X: 100 * engine.Unit}}}}
}

func TestPublicEventsKeepObjectiveAndEconomyPrivate(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Alive = true
	state.World.Bomb = engine.Bomb{Carrier: -1, Planted: true, X: 10 * engine.Unit, Y: 600 * engine.Unit, Z: 25 * engine.Unit, Defuser: 1, DefuseAt: 999}
	state.World.Events = []engine.Event{{ID: 1, Kind: "bomb_planted", Seat: 0, Target: -1, X: 10 * engine.Unit, Y: 600 * engine.Unit, Z: 25 * engine.Unit}, {ID: 2, Kind: "kill", Seat: 0, Target: 1, X: 10 * engine.Unit, Y: 600 * engine.Unit, Z: 25 * engine.Unit}, {ID: 3, Kind: "shot", Seat: 0, Target: -1, X: 10 * engine.Unit, Y: 600 * engine.Unit, Z: 25 * engine.Unit}, {ID: 4, Kind: "buy", Seat: 0, Target: -1, Weapon: "hegrenade", Value: 300}}
	state.World.EventID = 4
	projected := projection(state, 1)
	raw, _ := json.Marshal(projected.Events)
	var events []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&events) != nil {
		t.Fatal("event projection invalid")
	}
	if len(events) != 3 {
		t.Fatal("unexpected visible event count", len(events))
	}
	for _, event := range events {
		switch event["type"] {
		case "bomb_planted":
			if event["x"] != nil || event["y"] != nil || event["z"] != nil || event["global"] != true || event["seat"] != json.Number("-1") || event["target"] != json.Number("-1") {
				t.Fatal("global objective event leaked hidden details", event)
			}
		case "kill":
			if event["x"] != nil || event["y"] != nil || event["z"] != nil || event["seat"] != json.Number("0") || event["target"] != json.Number("1") {
				t.Fatal("kill feed changed public identities or leaked position", event)
			}
		case "shot":
			if event["x"] == nil || event["global"] == true {
				t.Fatal("audible local shot lost spatial position", event)
			}
		}
	}
	for _, event := range projection(state, -1).Events {
		if event.Kind == "buy" {
			t.Fatal("spectator received private purchase data")
		}
	}
	owned := false
	for _, event := range projection(state, 0).Events {
		owned = owned || event.Kind == "buy"
	}
	if !owned {
		t.Fatal("owner lost purchase confirmation")
	}
	if projection(state, 0).Bomb.Defuser != -1 || projection(state, 0).Bomb.DefuseAt != 0 {
		t.Fatal("terrorist view disclosed hidden defuse progress")
	}
	if projection(state, 1).Bomb.Defuser != 1 || projection(state, 1).Bomb.DefuseAt != 999 {
		t.Fatal("CT lost team defuse progress")
	}
}

func TestDeadTeamVisionProjection(t *testing.T) {
	state := observerFixture()
	blocked := projection(state, 0)
	if blocked.Players[1].Visible || blocked.Players[1].X != nil {
		t.Fatal("dead player saw enemy through wall")
	}
	state.World.Players[3].Alive = true
	teamView := projection(state, 0)
	if !teamView.Players[1].Visible || teamView.Players[1].X == nil {
		t.Fatal("live teammate vision missing from dead observer")
	}
	if !pointVisible(state, 0, engine.Vec3{X: 150 * engine.Unit}) {
		t.Fatal("live teammate point vision missing")
	}
	if teamView.Players[1].Money != nil || teamView.Players[1].Inventory != nil {
		t.Fatal("team vision leaked enemy inventory")
	}
	state.World.Players[2].Alive = false
	state.World.Players[3].Alive = false
	noTeam := projection(state, 0)
	if noTeam.Players[1].Visible || noTeam.Players[1].X != nil {
		t.Fatal("dead player retained enemy positions after team elimination")
	}
	if pointVisible(state, 0, engine.Vec3{X: 150 * engine.Unit}) {
		t.Fatal("point vision survived team elimination")
	}
	spectator := projection(state, -1)
	if !spectator.Players[1].Visible || spectator.Players[1].X == nil || spectator.Players[1].Money != nil {
		t.Fatal("independent spectator scope incorrect")
	}
	directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "server")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"passed": true, "scope": "dead team line-of-sight union with occlusion, team elimination and independent observer", "blocked": blocked, "teamView": teamView, "noTeam": noTeam, "spectator": spectator})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, t.Name()+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPredictionProjectionOmitsQueueSchedulerFrames(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Input = engine.Input{Seq: 5, Life: 1, MoveY: 500, QueuedFrame: 123}
	state.World.Players[0].Previous = engine.Input{Seq: 4, Life: 1, MoveY: 1000, QueuedFrame: 120}
	value := projection(state, 0)
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"queuedFrame"`)) {
		t.Fatal("private scheduler metadata entered prediction projection")
	}
	if value.Players[0].Input == nil || value.Players[0].Input.MoveY != 500 || value.Players[0].Previous.MoveY != 1000 {
		t.Fatal("logical prediction input lost")
	}
	if state.World.Players[0].Input.QueuedFrame != 123 || state.World.Players[0].Previous.QueuedFrame != 120 {
		t.Fatal("projection changed persistent queue clocks")
	}
}
