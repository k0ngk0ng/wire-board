package contra

// Integration inventory before implementation: reset per-frame state only at
// its original phase; enemies observe post-scroll/post-player values; spawning
// follows collisions; palette and sprite work preserve generation compatibility;
// sound order is retained; all 600 frames advance from one initial state with
// no recorded-after-state injection. Only pads and original pre-frame entropy
// are external inputs. Shipping entropy uses the separately approved adapter.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func activeWorldFromOriginal(t *testing.T, meta frameOracle, raw string) ActiveWorld {
	t.Helper()
	b, err := hex.DecodeString(raw)
	if err != nil || len(b) != 2048 {
		t.Fatal("invalid complete-world evidence")
	}
	values := func(fields []struct{ Name, Address string }, actor int) []uint8 {
		out := make([]uint8, len(fields))
		for i, field := range fields {
			a, e := strconv.ParseUint(field.Address, 0, 16)
			if e != nil {
				t.Fatal(e)
			}
			out[i] = b[int(a)+actor]
		}
		return out
	}
	s := frameOracleSnapshot{World: values(meta.WorldFields, 0), Collision: hex.EncodeToString(b[0x680:0x700]), Projectiles: hex.EncodeToString(b[0x368:0x4b8])}
	for i := range 2 {
		s.Players[i] = values(meta.PlayerFields, i)
	}
	s.Display.Codes = hex.EncodeToString(b[0x300:0x30a])
	s.Display.Y = hex.EncodeToString(b[0x31a:0x324])
	s.Display.X = hex.EncodeToString(b[0x334:0x33e])
	s.Display.Attributes = hex.EncodeToString(b[0x34e:0x358])
	combat, _ := combatFromOracle(t, meta, s)
	w := ActiveWorld{Combat: combat, GenerationBias: b[6], Random: RandomState{Value: b[0x34]}, HUD: b[0x37] != 0, Flicker: b[0x7eb]&1 != 0,
		Work:      TerrainWork{Value: uint16(b[0x12]) | uint16(b[0x13])<<8},
		Animation: CHRAnimation{Timer: b[0x5c], Phase: b[0x71], Pace: b[0x5d]}, Snow: SnowControlState{Visibility: b[0x5a], Cursor: b[0x5b]},
		Terrain: TerrainScrollState{AlternateGraphicsScreen: b[0x48], Axis: b[0x60], TransitionRemaining: b[0x61], TileAddress: uint16(b[0x62]) | uint16(b[0x63])<<8, AttributeAddress: uint16(b[0x66]) | uint16(b[0x67])<<8, MapOffset: b[0x69], TankScroll: b[0x77]},
		Header:  PaletteState{CollisionLimits: [3]uint8(b[0x49:0x4c]), CycleIndices: [4]uint8(b[0x4c:0x50]), PaletteIndices: [8]uint8(b[0x50:0x58])}}
	copy(w.Terrain.Supertiles[:], b[0x600:0x680])
	w.Presentation.Sprites.Rotation = b[0x35]
	copy(w.Presentation.Sprites.OAM[:], b[0x200:0x300])
	w.Presentation.Palette = PaletteOutput{RequestedColors: b[0x36], FadeTimer: b[0x74], Cycle: b[0x72], PauseCycle: b[0x78], FlashTimer: b[0x7d]}
	copy(w.Presentation.Palette.Desired[:], b[0x7c0:0x7e0])
	copy(w.Presentation.Patterns.Desired[:], b[0x7f0:0x7f8])
	fields := []int{0x30a, 0x324, 0x33e, 0x358, 0x4b8, 0x4c8, 0x4d8, 0x4e8, 0x4f8, 0x508, 0x518, 0x528, 0x538, 0x548, 0x558, 0x568, 0x578, 0x588, 0x598, 0x5a8, 0x5b8, 0x5c8, 0x5d8, 0x5e8}
	for i := range 16 {
		row := make([]uint8, len(fields))
		for j, a := range fields {
			row[j] = b[a+i]
		}
		w.Enemies.Slots[i] = enemyFromOracle(row)
	}
	w.Enemies.ScreenReadOffset = b[0x82]
	w.Enemies.Generator = SoldierGenerator{Routine: b[0x79], Timer: b[0x7a], X: b[0x7b], Y: b[0x7c], Screen: b[0x195], Generated: b[0x196]}
	word := func(a int) uint16 { return uint16(b[a]) | uint16(b[a+1])<<8 }
	w.Scores = ArcadeScores{HighScore: word(0x7e0), Hundreds: [2]uint16{word(0x7e2), word(0x7e4)}, NextLife: [2]uint16{word(0x3c), word(0x3e)}}
	return w
}

func TestOriginalContinuousActiveWorld(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/world-complete-original.json.gz")
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
	if !d.Original || len(d.Frames) != 600 {
		t.Fatal("incomplete original run")
	}
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-1.json")
	w := activeWorldFromOriginal(t, meta, d.Frames[0].Before)
	for i, row := range d.Frames {
		b, _ := hex.DecodeString(row.Before)
		after, _ := hex.DecodeString(row.After)
		if i > 0 {
			if _, err = w.Presentation.Commit(); err != nil {
				t.Fatal(i, err)
			}
		}
		w.Random.Value = b[0x34] // explicit original idle-time entropy input only
		inputs := [2]PlayerInput{}
		for actor := range 2 {
			inputs[actor] = PlayerInput{Held: Buttons(b[0xf1+actor]), Pressed: Buttons(b[0xf5+actor])}
		}
		commands, err := w.Advance(inputs, resources)
		if err != nil {
			t.Fatal(i, err)
		}
		want := activeWorldFromOriginal(t, meta, row.After)
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
			t.Fatal(i, "palette/pattern output differs")
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
		if hex.EncodeToString(queue) != hex.EncodeToString(after[0x700:0x700+int(after[0x21])]) {
			t.Fatalf("frame%d graphics got=%x want=%x used=%d", i, queue, after[0x700:0x700+int(after[0x21])], w.Presentation.Queue.Used)
		}
		if len(commands) != len(row.Commands) {
			t.Fatal(i, "sound count", commands, row.Commands)
		}
		for j := range commands {
			if commands[j] != row.Commands[j] {
				t.Fatal(i, "sound order", commands, row.Commands)
			}
		}
	}
	t.Log("600 complete active-world frames advanced continuously from one original initial state")
}
