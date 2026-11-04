package presentation

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func hudRead(t *testing.T, name, hashKey string) []byte {
	t.Helper()
	compressed, err := os.ReadFile("../data/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("../data/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if manifest["pass"] != true || manifest[hashKey] != hex.EncodeToString(h[:]) || manifest["gzip_sha256"] != hex.EncodeToString(zh[:]) {
		t.Fatal("unbound HUD data")
	}
	return raw
}

func loadHUD(t *testing.T) HUDCatalog {
	t.Helper()
	c, err := DecodeHUDCatalog(hudRead(t, "hud-assets", "catalog_sha256"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type hudReference struct {
	Events []struct {
		Frame int
		Event engine.ChuWorldEvent
	}
	Checks []struct {
		Frame, Cursor int
		Cells         string
	}
	Calls []struct {
		Name          string
		Event         engine.ChuWorldEvent
		Before, After string
		NextNameTile  uint16 `json:"next_name_tile"`
	}
}

func TestHUDOriginalCallsAndNaturalWindow(t *testing.T) {
	catalog := loadHUD(t)
	var source hudReference
	if err := json.Unmarshal(hudRead(t, "hud-reference", "fixture_sha256"), &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Calls) < 55 || len(source.Checks) != 20 || len(source.Events) < 100 {
		t.Fatal("incomplete HUD source")
	}
	for _, call := range source.Calls {
		state, err := NewHUD(catalog)
		if err != nil {
			t.Fatal(err)
		}
		before := dataBytes(t, call.Before, 512)
		for i := range state.Cells {
			state.Cells[i] = binary.BigEndian.Uint16(before[i*2:])
		}
		state.NextNameTile = call.NextNameTile
		state, err = ApplyHUDWorldEvents(state, []engine.ChuWorldEvent{call.Event}, nil, catalog)
		if err != nil {
			t.Fatal(call.Name, err)
		}
		after := make([]byte, 512)
		for i, w := range state.Cells {
			binary.BigEndian.PutUint16(after[i*2:], w)
		}
		if !bytes.Equal(after, dataBytes(t, call.After, 512)) {
			t.Fatalf("original HUD call cells differ: %s", call.Name)
		}
	}
	raw, _ := arenaReadGzip(t, "arena-assets-reference")
	var frames struct {
		Frames []struct {
			arenaOriginalFrame
			RAM string
		}
	}
	if err := json.Unmarshal(raw, &frames); err != nil {
		t.Fatal(err)
	}
	state, err := NewHUD(catalog)
	if err != nil {
		t.Fatal(err)
	}
	arena, _ := loadArena(t)
	cursor, windowPixels := 0, 0
	for i, check := range source.Checks {
		if check.Frame != frames.Frames[i].Frame || check.Cursor < cursor || check.Cursor > len(source.Events) {
			t.Fatal("invalid HUD event cursor")
		}
		var events []engine.ChuWorldEvent
		for _, event := range source.Events[cursor:check.Cursor] {
			events = append(events, event.Event)
		}
		cursor = check.Cursor
		cellsRaw := dataBytes(t, check.Cells, 512)
		cells := make([]uint16, 256)
		for i := range cells {
			cells[i] = binary.BigEndian.Uint16(cellsRaw[i*2:])
		}
		state, err = ApplyHUDWorldEvents(state, events, cells, catalog)
		if err != nil {
			t.Fatalf("frame%d: %v", check.Frame, err)
		}
		window, err := HUDWindow(state)
		if err != nil {
			t.Fatal(err)
		}
		f := frames.Frames[i].arenaOriginalFrame
		vram := dataBytes(t, f.ActiveVRAM, 65536)
		// Every visible window cell and all of its referenced indexed tiles
		// must be constructed from original assets and events, not copied from
		// this expected window map.
		for row := 0; row < 6; row++ {
			for col := 0; col < 40; col++ {
				pos := row*64 + col
				actual := window.Cells[pos]
				want := binary.BigEndian.Uint16(vram[0xd000+pos*2:])
				if actual.Flags != want&0xf800 || window.Tiles[actual.Tile] != Tile(*(*[32]byte)(vram[int(want&2047)*32 : int(want&2047)*32+32])) {
					t.Fatalf("window frame%d row%d col%d differs", check.Frame, row, col)
				}
			}
		}
		input := arenaObservedFrame(t, f)
		input.Window = &window
		bg, err := RasterizeArena(arena, input)
		if err != nil {
			t.Fatal(err)
		}
		sprites := arenaOriginalSprites(t, f, input.Rows)
		video := dataBytes(t, f.Video, f.Width*f.Height*2)
		for p, color := range bg.Colors {
			if bg.Sources[p] == 3 && !sprites.Opaque[p] {
				windowPixels++
				if color != binary.LittleEndian.Uint16(video[p*2:]) {
					t.Fatal("original window pixel mismatch", check.Frame, p)
				}
			}
		}
	}
	if windowPixels != 91756 {
		t.Fatal("window pixel coverage changed", windowPixels)
	}
	t.Logf("%d original calls and %d ordered events,20 reconstructed windows, %d window pixels zero differences", len(source.Calls), len(source.Events), windowPixels)
}

func TestHUDFreshSessionConstructsWindowWithoutReferenceState(t *testing.T) {
	catalog := loadHUD(t)
	compressed, err := os.ReadFile("../data/chu-world-rules.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	library, err := engine.DecodeChuWorldRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, players := range []int{2, 3, 4} {
		options := engine.ChuSessionOptions{Seed: 0x12345678, Difficulty: 6}
		slots := []uint8{0, 3}
		if players == 3 {
			slots = []uint8{0, 1, 3}
		} else if players == 4 {
			slots = []uint8{0, 1, 2, 3}
		}
		for i, slot := range slots {
			options.Players = append(options.Players, engine.ChuSessionPlayer{Slot: slot, CharacterID: 9, Color: uint8(i), Computer: true})
		}
		world, events, err := engine.NewChuSession(options, library)
		if err != nil {
			t.Fatal(err)
		}
		hud, err := NewHUD(catalog)
		if err != nil {
			t.Fatal(err)
		}
		hud, err = ApplyHUDWorldEvents(hud, events, world.Cells[:256], catalog)
		if err != nil {
			t.Fatal(err)
		}
		world, err = engine.CompleteChuSessionResource(world, engine.DefaultChuSessionResource())
		if err != nil {
			t.Fatal(err)
		}
		for tick := 0; tick < 500; tick++ {
			world, events, _, err = engine.StepChuSessionFrame(world, [8]engine.Buttons{}, library)
			if err != nil {
				t.Fatal(err)
			}
			hud, err = ApplyHUDWorldEvents(hud, events, world.Cells[:256], catalog)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = HUDWindow(hud); err != nil {
				t.Fatalf("fresh %d players tick%d: %v", players, tick, err)
			}
		}
		if hud.NextNameTile != uint16(32+12*players) {
			t.Fatal("fresh names were not constructed", players, hud.NextNameTile)
		}
		if len(hud.Slots) < 16+players*12 {
			t.Fatal("fresh indexed glyphs unavailable")
		}
	}
}

func TestHUDErrorsAreAtomicAndRuleSyncPreservesPictures(t *testing.T) {
	catalog := loadHUD(t)
	state, err := NewHUD(catalog)
	if err != nil {
		t.Fatal(err)
	}
	start := []engine.ChuWorldEvent{{Kind: "start", Start: engine.MatchStartEvent{Kind: "words", Offset: 16, Words: []uint16{0xa001, 0xa00b}}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "name", Offset: 16, Glyphs: [3]uint16{358, 0, 0}}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "portrait", Attributes: 0xa300, X: 184, Y: 144, Parameter: 20}}}
	state, err = ApplyHUDWorldEvents(state, start, nil, catalog)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(state)
	for _, events := range [][]engine.ChuWorldEvent{
		{{Kind: "start", Start: engine.MatchStartEvent{Kind: "words", Offset: 511, Words: []uint16{1}}}},
		{{Kind: "start", Start: engine.MatchStartEvent{Kind: "name", Offset: 510, Glyphs: [3]uint16{358, 0, 0}}}},
		{{Kind: "start", Start: engine.MatchStartEvent{Kind: "name", Offset: 16, Glyphs: [3]uint16{65534, 0, 0}}}},
		{{Kind: "start", Start: engine.MatchStartEvent{Kind: "portrait", Attributes: 0xa300, X: 0, Y: 0, Parameter: 20}}},
		{{Kind: "start", Start: engine.MatchStartEvent{Kind: "resource", Source: 1}}},
		{{Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: 32, TargetTile: 16}}},
		{{Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: 31, TargetTile: 1}}},
		{{Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: 31, TargetTile: 16}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "broken"}}},
	} {
		got, err := ApplyHUDWorldEvents(state, events, nil, catalog)
		after, _ := json.Marshal(state)
		if err == nil || !reflect.DeepEqual(got, state) || !bytes.Equal(before, after) {
			t.Fatal("invalid HUD event was not atomic")
		}
	}
	if _, err := ApplyHUDWorldEvents(state, nil, make([]uint16, 255), catalog); err == nil {
		t.Fatal("truncated rule cells accepted")
	}
	cells := make([]uint16, 256)
	cells[9] = 0xa00c
	got, err := ApplyHUDWorldEvents(state, nil, cells, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cells[9] != 0xa00c || got.Cells[8] != state.Cells[8] || got.Cells[139] != state.Cells[139] {
		t.Fatal("rule cells overwrote pictures or failed to update bar")
	}
	bad := got
	bad.Slots = map[uint16]Tile{}
	if _, err := HUDWindow(bad); err == nil {
		t.Fatal("missing HUD tile silently painted transparent")
	}
	raw := hudRead(t, "hud-assets", "catalog_sha256")
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	data["rom_sha256"] = "wrong"
	encoded, _ := json.Marshal(data)
	if _, err := DecodeHUDCatalog(encoded); err == nil {
		t.Fatal("wrong HUD identity accepted")
	}
}
