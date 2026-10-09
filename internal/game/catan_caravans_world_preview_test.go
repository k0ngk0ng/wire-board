package game

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

// Cross-language check: the waiting-room preview must mark exactly the same
// locations as the installed game, including edge candidates and tied distances.
func TestCatanCaravansWorldPreviewParity(t *testing.T) {
	type sample struct {
		Map   *CatanNewWorldMap `json:"map"`
		Holes []int             `json:"holes"`
	}
	samples := []sample{}
	for _, n := range []int{3, 6} {
		for i := 0; i < 12; i++ {
			s, e := newCatanCaravansWorld(n)
			if e != nil {
				t.Fatal(e)
			}
			samples = append(samples, sample{s.Catan.Caravans.WorldBase, s.Catan.Caravans.Map.WateringHoles})
		}
	}
	data, _ := json.Marshal(samples)
	script := `import {caravanWorldPreview} from './web/src/catan-caravan-world-preview.ts';let input='';for await (const chunk of process.stdin) input+=chunk;for(const x of JSON.parse(input)){const got=caravanWorldPreview(x.map);if(JSON.stringify(got)!==JSON.stringify(x.holes))throw new Error(JSON.stringify({got,want:x.holes}));}`
	cmd := exec.Command("node", "--experimental-strip-types", "--input-type=module", "-e", script)
	cmd.Dir = "../.."
	cmd.Stdin = bytes.NewReader(data)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("preview parity: %v %s", e, out)
	}
}
