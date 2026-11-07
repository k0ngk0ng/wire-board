package pvz_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
)

// Explicit scenarios enter the real Wasm through its public load/step/apply
// interface. They are never selectable from a production room.
func TestPvZFullPresentationFixtures(t *testing.T) {
	out := os.Getenv("PVZ_PRESENTATION_OUT")
	if out == "" {
		t.Skip("browser presentation fixture only")
	}
	fixtures := map[string]any{}
	for _, scene := range []string{"day", "fog"} {
		s, err := engine.New(engine.Options{Scene: scene}, 2, "presentation-"+scene)
		if err != nil {
			t.Fatal(err)
		}
		s.Phase, s.Tick, s.BattleStartTick, s.PhaseEndTick = "battle", 1000, 600, 10000
		s.NextSunTick, s.NextBrainTick = 1001, 1001
		s.Players[0].Money = 1000
		s.Players[0].Deck = []string{"peashooter", "sunflower", "wallnut", "cherrybomb"}
		s.Plants = []engine.Plant{{ID: 1, Owner: 0, Kind: "wallnut", Row: 0, Col: 1, HP: 1, MaxHP: 4000, Layer: "body", Flags: map[string]int64{}}}
		z, _ := engine.ZombieSpec("zombie")
		s.Zombies = []engine.Zombie{{ID: 2, Owner: 1, Kind: "zombie", Row: 0, X: 1750, HP: 1, MaxHP: z.HP, State: "walking", SpeedMilli: 10000, Allegiance: "zombies", Flags: map[string]int64{}}}
		s.Projectiles = []engine.Projectile{{ID: 3, Owner: 0, Row: 0, X: 1400, Y: 500, VX: 208, Kind: "pea", Damage: 20, BornAt: 990}, {ID: 4, Owner: 0, Row: 1, X: 1000, Y: 1500, VX: 208, Kind: "pea", Damage: 20, BornAt: 990}}
		s.Mowers[4] = engine.Mower{Row: 4, State: "moving", X: 10100}
		s.Craters = []engine.Crater{{ID: 5, Row: 4, Col: 4, Until: 1001}}
		s.Terrain = []engine.Terrain{{ID: 6, Row: 4, FromX: 0, ToX: 1000, Owner: 1, Kind: "ice", Until: 1001}}
		s.Pickups = []engine.Pickup{{ID: 7, Owner: 0, Row: 3, X: 1000, Y: 3500, Kind: "sun", Amount: 25, AutoAt: 1001, ExpiresAt: 1001}}
		s.NextID = 7
		if scene == "fog" {
			s.Projection = true
		}
		if err := engine.Validate(s); err != nil {
			t.Fatalf("%s: %v", scene, err)
		}
		// Motion fields are wire data, deliberately described before the engine
		// learns to emit them. JSON avoids coupling this red test to new Go fields.
		raw, _ := json.Marshal(s)
		var state map[string]any
		_ = json.Unmarshal(raw, &state)
		for _, q := range state["projectiles"].([]any) {
			q.(map[string]any)["motionX"] = 208
		}
		state["zombies"].([]any)[0].(map[string]any)["motionX"] = -10
		state["mowers"].([]any)[4].(map[string]any)["motionX"] = 300
		fixtures[scene] = map[string]any{"snapshot": state, "map": s.Map, "options": s.Options, "protocol": 1, "epoch": s.Epoch, "round": 1, "phase": s.Phase, "tick": s.Tick, "serverAt": 100000, "phaseUntil": 550000, "players": s.Players, "plants": s.Plants, "zombies": s.Zombies, "projectiles": s.Projectiles, "mowers": s.Mowers, "events": s.Events, "rounds": s.Rounds, "canAct": true, "wireMode": map[bool]string{true: "projection", false: "replay"}[s.Projection], "catalog": map[string]any{"plants": []any{}, "zombies": []any{}}}
	}
	raw, err := json.Marshal(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "fixtures.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
}
