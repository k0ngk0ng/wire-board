package contra

// Whole-world integration gate before pause support: entry must skip gameplay;
// an already paused frame still projects player/bullet sprites and animates;
// resume runs that prefix before combat, without double-incrementing Frame;
// sound 74 occurs only on pause entry. Original expected states are never input.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalContinuousPauseResumeWorld(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/world-paused-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d struct {
		Original bool `json:"source_is_original_execution"`
		Frames   []struct {
			Before, After string
			Commands      []WorldSoundCommand
		}
	}
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if !d.Original || len(d.Frames) != 180 {
		t.Fatal("incomplete paused original run")
	}
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-1.json")
	w := activeWorldFromOriginal(t, meta, d.Frames[0].Before)
	campaign := NewCampaign()
	pausedCount := 0
	for i, row := range d.Frames {
		b, _ := hex.DecodeString(row.Before)
		a, _ := hex.DecodeString(row.After)
		if i > 0 {
			if _, err = w.Presentation.Commit(); err != nil {
				t.Fatal(i, err)
			}
		}
		w.Random.Value = b[0x34]
		inputs := [2]PlayerInput{}
		for actor := range 2 {
			inputs[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
		}
		commands, err := w.AdvanceWithPause(inputs, resources, &campaign)
		if err != nil {
			t.Fatal(i, err)
		}
		want := activeWorldFromOriginal(t, meta, row.After)
		if campaign.Paused != (a[0x25] != 0) {
			t.Fatal(i, "pause state differs")
		}
		if campaign.Paused {
			pausedCount++
		}
		for name, pair := range map[string][2]any{"combat": {w.Combat, want.Combat}, "terrain": {w.Terrain, want.Terrain}, "header": {w.Header, want.Header}, "slots": {w.Enemies.Slots, want.Enemies.Slots}, "generator": {w.Enemies.Generator, want.Enemies.Generator}, "scores": {w.Scores, want.Scores}, "animation": {w.Animation, want.Animation}, "oam": {w.Presentation.Sprites.OAM, want.Presentation.Sprites.OAM}, "bias": {w.GenerationBias, want.GenerationBias}} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Fatalf("frame%d %s got%+v want%+v", i, name, pair[0], pair[1])
			}
		}
		if w.Enemies.ScreenReadOffset != want.Enemies.ScreenReadOffset || w.Random != want.Random || w.Presentation.Sprites.Rotation != want.Presentation.Sprites.Rotation {
			t.Fatal(i, "persistent work differs")
		}
		p, wp := w.Presentation.Palette, want.Presentation.Palette
		p.Applied = [32]uint8{}
		wp.Applied = [32]uint8{}
		if p != wp || w.Presentation.Patterns.Desired != want.Presentation.Patterns.Desired {
			t.Fatal(i, "palette/pattern output differs", p, wp)
		}
		var queue []uint8
		for _, packet := range w.Presentation.Queue.Packets {
			v, e := packet.Encode()
			if e != nil {
				t.Fatal(e)
			}
			queue = append(queue, v...)
		}
		if w.Presentation.Terminated {
			queue = append(queue, 0)
		}
		if hex.EncodeToString(queue) != hex.EncodeToString(a[0x700:0x700+int(a[0x21])]) {
			t.Fatal(i, "graphics differs")
		}
		if len(commands) != len(row.Commands) {
			t.Fatal(i, "sound count", commands, row.Commands)
		}
		for j := range commands {
			if commands[j] != row.Commands[j] {
				t.Fatal(i, "sound order")
			}
		}
	}
	if pausedCount != 100 {
		t.Fatal("wrong paused frame coverage", pausedCount)
	}
	t.Log("180 continuous original frames, 100 paused frames, two pause/resume cycles match")
}
