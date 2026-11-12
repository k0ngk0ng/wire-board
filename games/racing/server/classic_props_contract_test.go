package racing

// Cross-language prop collision failure inventory precedes implementation in
// ../tests/classic-props-failures.md. These are isolated numerical probes of
// production collision rules, separate from ordinary-drive Module E2E evidence.
import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClassicPropCollisionClientParity(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	out := filepath.Join(root, ".local/racing-nfs-backend/props-parity")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	rows := []any{}
	for _, id := range []string{"france", "coastal", "uk", "park"} {
		track := classicRules.byID[id].track
		catalog := classicProps.current[track.Geometry]
		for _, prop := range catalog.Props {
			edge := track.Barriers[prop.RuleEdges[0]]
			dx, dy := edge.B.X-edge.A.X, edge.B.Y-edge.A.Y
			length := math.Hypot(dx, dy)
			nx, ny := 1., 0.
			if length > 1e-8 {
				nx, ny = -dy/length, dx/length
			}
			for _, vehicle := range []string{"p911", "fm50", "ldia", "mcf1", "ccor"} {
				for _, mode := range []string{"low", "high", "broken", "above", "away"} {
					props := newClassicPropState(catalog)
					if mode == "broken" {
						props.Broken[prop.Index/8] |= 1 << uint(prop.Index%8)
					}
					speed := 4.
					if mode == "high" {
						speed = 13
					} else if mode == "away" {
						speed = -4
					}
					height := edge.Bottom
					if mode == "above" {
						height = edge.Top + 100
					}
					motion := ClassicMotionState{X: ((edge.A.X+edge.B.X)/2 + nx*3) / 10, Z: ((edge.A.Y+edge.B.Y)/2 + ny*3) / 10, VX: -nx * speed, VZ: -ny * speed, Angle: math.Atan2(-ny, -nx), Gear: 1, RPM: 2000}
					before := append([]byte{}, props.Broken...)
					state := State{Props: props, Cars: make([]Car, 1), Events: []Event{}}
					actual := collideClassicRoadWithProps(track, motion, vehicle, height, classicCollisionProps(&state, 0, 20))
					rows = append(rows, map[string]any{"track": id, "prop": prop.Index, "mode": mode, "vehicle": vehicle, "height": height, "motion": motion, "before": before, "after": props.Broken, "expected": actual, "events": state.Events})
				}
			}
		}
	}
	payload, _ := json.Marshal(rows)
	fixture := filepath.Join(out, "go-probes.json")
	if err := os.WriteFile(fixture, payload, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--experimental-strip-types", filepath.Join(root, "games/racing/tests/classic-props-parity.mjs"), fixture, filepath.Join(out, "report.json"))
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prop collision client parity: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
