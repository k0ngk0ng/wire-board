package contra

// Controlled original terminal boundary before enabling final presentation.
// This proves the caller after exit/transition, not a natural eight-stage win.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalFinalCampaignWorldBoundary(t *testing.T) {
	f, e := os.Open("../../../docs/research/nes-contra/evidence/world-finish-original.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct {
		Original bool `json:"source_is_original_execution"`
		Frames   []struct {
			Before, After string
			Commands      []WorldSoundCommand
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if !d.Original || len(d.Frames) != 1 {
		t.Fatal("missing original final boundary")
	}
	meta := loadFrameOracle(t)
	row := d.Frames[0]
	w := guardianWorldFromOriginal(t, meta, row.Before)
	want := guardianWorldFromOriginal(t, meta, row.After)
	commands, e := w.Advance(1, [2]PlayerInput{}, levelResources(t, "stage-8.json"))
	if e != nil {
		t.Fatal(e)
	}
	if w.Campaign.Outcome != CampaignCleared || w.Campaign.Pending != CampaignNoWork || w.Campaign.Loops != want.Campaign.Loops || w.Campaign.DelayLow != want.Campaign.DelayLow {
		t.Fatal("final decision differs", w.Campaign, want.Campaign)
	}
	for name, pair := range map[string][2]any{"combat": {w.Active.Combat, want.Active.Combat}, "sprites": {w.Active.Presentation.Sprites, want.Active.Presentation.Sprites}, "terrain": {w.Active.Terrain, want.Active.Terrain}, "score": {w.Active.Scores, want.Active.Scores}, "bias": {w.Active.GenerationBias, want.Active.GenerationBias}} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Fatal(name, pair)
		}
	}
	if len(commands) != len(row.Commands) {
		t.Fatal("terminal sounds differ", commands, row.Commands)
	}
	before, _ := json.Marshal(w)
	if v, e := w.Advance(2, [2]PlayerInput{}, levelResources(t, "stage-8.json")); e != nil || len(v) != 0 {
		t.Fatal("clear emitted again", e)
	}
	after, _ := json.Marshal(w)
	if string(before) != string(after) {
		t.Fatal("clear repeated")
	}
}
