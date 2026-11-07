package contra

// Failure scenarios defined before the snow rules: stage-specific dispatch or
// properties taken from US bytes; spawns read a source slot before initialization
// when that same slot is reused; ice blast disables player collision; saucer
// descent uses the US A8/A9 instead of Japanese A9/AA boundary; joint scrolling
// subtracts a multi-pixel scroll instead of one; sound/order/removal drift.

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type snowOracleState struct {
	Enemies                                      [][]uint8
	World                                        []uint8
	Players                                      []EnemyTarget
	Indices, Palette, Supertiles, Terrain, Queue string
	Used                                         uint8
}
type snowOracleEvent struct {
	Kind                                                             string
	Code, X, Y, Used, AfterUsed, Carry, AttributeLookup, Count, Fade uint8
	Address                                                          uint16
	Indices                                                          string
}
type snowOracleRecord struct {
	ID, Frame, Slot int
	Before, After   snowOracleState
	Events          []snowOracleEvent
}

func snowRecords(t *testing.T, path string) []snowOracleRecord {
	t.Helper()
	f, err := os.Open(path)
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
		Records  []snowOracleRecord
	}
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if !d.Original {
		t.Fatal("not original execution")
	}
	return d.Records
}
func snowContext(t *testing.T, b snowOracleState) (EnemyStepContext, CollisionWindow) {
	t.Helper()
	raw, err := hex.DecodeString(b.Terrain)
	if err != nil || len(raw) != 128 {
		t.Fatal(err)
	}
	terrain := CollisionWindow(raw)
	w := b.World
	c := EnemyStepContext{Level: w[2], GameLoop: w[3], Random: w[4], Frame: w[0], WeaponStrength: w[1], SceneType: w[6], Vertical: w[7] != 0, Screen: w[8], FrameScroll: w[10], AttackEnabled: w[19] != 0, Scroll: CollisionScroll{Y: w[21], X: w[22], NameTable: w[23]}}
	copy(c.Players[:], b.Players)
	return c, terrain
}
func TestSnowSmallOriginal(t *testing.T) {
	for _, scope := range []string{"natural", "small-cases"} {
		rows := snowRecords(t, "testdata/original-enemies-snow-"+scope+".json.gz")
		wantCount := 4501
		if scope == "small-cases" {
			wantCount = 104
		}
		if len(rows) != wantCount {
			t.Fatal("snow oracle coverage", scope, len(rows))
		}
		for _, r := range rows {
			before := enemyPoolFromOracle(r.Before.Enemies)
			s := EnemySystem{Slots: before, JointScroll: r.Before.World[16]}
			c, terrain := snowContext(t, r.Before)
			c.Terrain = &terrain
			var err error
			switch before[r.Slot].Kind {
			case 16:
				err = s.StepIceGenerator(r.Slot, c)
			case 17:
				err = s.StepIceGrenade(r.Slot, c)
			case 19:
				err = s.StepIceJoint(r.Slot, c)
			case 21:
				err = s.StepMiniUFO(r.Slot, c)
			case 22:
				err = s.StepSnowBomb(r.Slot, c)
			default:
				t.Fatal("unexpected small snow kind", before[r.Slot].Kind)
			}
			if err != nil {
				t.Fatalf("%s record%d: %v", scope, r.ID, err)
			}
			want := enemyPoolFromOracle(r.After.Enemies)
			if s.Slots != want {
				for i, e := range s.Slots {
					if e != want[i] {
						t.Fatalf("%s record%d kind%d state%d slot%d got %+v want %+v before %+v", scope, r.ID, before[r.Slot].Kind, before[r.Slot].Routine, i, e, want[i], before[i])
					}
				}
			}
			var sounds []uint8
			for _, ev := range r.Events {
				if ev.Kind != "sound" {
					t.Fatal("unhandled original snow event", ev)
				}
				sounds = append(sounds, ev.Code)
			}
			if !reflect.DeepEqual(s.PendingSounds, sounds) {
				t.Fatalf("%s record%d sounds %v want %v", scope, r.ID, s.PendingSounds, sounds)
			}
			if s.JointScroll != r.After.World[16] {
				t.Fatal("joint flag changed unexpectedly")
			}
		}
	}
}

func TestSnowTankAndCarrierOriginal(t *testing.T) {
	// Original queue refusal, immediate palette/global writes, extended tank X,
	// aim carry chains, shared work-byte effects, child allocation, audio reset
	// and the delayed exit are compared together before adding these rules.
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := resources.Graphics(4, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	rows := snowRecords(t, "testdata/original-enemies-snow-complex-cases.json.gz")
	if len(rows) != 182 {
		t.Fatal("tank/carrier oracle coverage", len(rows))
	}
	verifySnowComplexRecords(t, rows, resources, tiles)
}

func verifySnowComplexRecords(t *testing.T, rows []snowOracleRecord, resources *LevelResources, tiles GraphicsTileset) {
	t.Helper()
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		s := EnemySystem{Slots: before, JointScroll: r.Before.World[16]}
		c, terrain := snowContext(t, r.Before)
		c.Terrain = &terrain
		b := r.Before.World
		palette := PaletteOutput{FadeTimer: b[11], PauseCycle: b[15], RequestedColors: b[28]}
		header := PaletteState{}
		decoded, _ := hex.DecodeString(r.Before.Palette)
		copy(palette.Desired[:], decoded)
		decoded, _ = hex.DecodeString(r.Before.Indices)
		copy(header.PaletteIndices[:], decoded)
		decoded, _ = hex.DecodeString(r.Before.Supertiles)
		maps := ([128]uint8)(decoded)
		queue := GraphicsQueue{Used: r.Before.Used}
		tank := b[14]
		boss, delayLo, delayHi := b[5], b[24], b[25]
		var events []snowOracleEvent
		soundAt := 0
		flushSounds := func() {
			for _, code := range s.PendingSounds[soundAt:] {
				events = append(events, snowOracleEvent{Kind: "sound", Code: code})
			}
			soundAt = len(s.PendingSounds)
		}
		c.TankScroll = &tank
		c.Scene = EnemySceneContext{Header: &header, Palette: &palette}
		c.Scene.RefreshBackgroundPalette = func(count uint8) (PaletteResult, error) {
			flushSounds()
			events = append(events, snowOracleEvent{Kind: "palette", Count: count, Indices: hex.EncodeToString(header.PaletteIndices[:]), Fade: palette.FadeTimer})
			return palette.Load(count, c.SceneType, header, resources)
		}
		c.OnBossControl = func(control BossControl) error {
			switch control.Kind {
			case BossAudioReset:
				flushSounds()
				events = append(events, snowOracleEvent{Kind: "audioReset"})
				soundAt = 0
			case BossBeginDefeat:
				boss, delayLo = 1, 255
			case BossExitReady:
				delayLo, delayHi = control.Delay, 0
			default:
				t.Fatal("unknown snow Boss control", control)
			}
			return nil
		}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			flushSounds()
			used := queue.Used
			result, err := queue.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &maps, Tileset: tiles})
			carry := uint8(1)
			if result.Accepted {
				carry = 0
			}
			events = append(events, snowOracleEvent{Kind: "terrain", X: req.X, Y: req.Y, Code: req.Code, Used: used, AfterUsed: queue.Used, Carry: carry, Address: result.Address, AttributeLookup: result.AttributeLookup})
			return EnemyTerrainResponse{Accepted: result.Accepted, AddressValid: result.Accepted, Address: result.Address, AttributeLookupWritten: result.AttributeLookupWritten, AttributeLookup: result.AttributeLookup}, err
		}
		var stepErr error
		switch before[r.Slot].Kind {
		case 16:
			stepErr = s.StepIceGenerator(r.Slot, c)
		case 17:
			stepErr = s.StepIceGrenade(r.Slot, c)
		case 19:
			stepErr = s.StepIceJoint(r.Slot, c)
		case 18:
			stepErr = s.StepSnowTank(r.Slot, c)
		case 20:
			stepErr = s.StepSnowCarrier(r.Slot, c)
		case 21:
			stepErr = s.StepMiniUFO(r.Slot, c)
		case 22:
			stepErr = s.StepSnowBomb(r.Slot, c)
		default:
			t.Fatal("unexpected controlled snow kind", before[r.Slot].Kind)
		}
		if stepErr != nil {
			t.Fatalf("snow record%d kind%d state%d: %v", r.ID, before[r.Slot].Kind, before[r.Slot].Routine, stepErr)
		}
		flushSounds()
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("snow record%d kind%d state%d slot%d got %+v want %+v before %+v", r.ID, before[r.Slot].Kind, before[r.Slot].Routine, i, e, want[i], before[i])
				}
			}
		}
		if len(events) != len(r.Events) {
			t.Fatalf("record%d events %+v want %+v", r.ID, events, r.Events)
		}
		for i, event := range events {
			wantEvent := r.Events[i]
			if event.Kind == "terrain" && event.Carry != 0 {
				event.Address, wantEvent.Address = 0, 0
				event.AttributeLookup, wantEvent.AttributeLookup = 0, 0
			}
			if event != wantEvent {
				t.Fatalf("record%d event%d got %+v want %+v", r.ID, i, event, wantEvent)
			}
		}
		if palette.FadeTimer != r.After.World[11] || tank != r.After.World[14] || palette.PauseCycle != r.After.World[15] || s.JointScroll != r.After.World[16] || palette.RequestedColors != r.After.World[28] || boss != r.After.World[5] || delayLo != r.After.World[24] || delayHi != r.After.World[25] {
			t.Fatalf("record%d scene mismatch fade%d tank%d pause%d joint%d req%d boss%d delay%d/%d want%v", r.ID, palette.FadeTimer, tank, palette.PauseCycle, s.JointScroll, palette.RequestedColors, boss, delayLo, delayHi, r.After.World)
		}
		wantIndices, _ := hex.DecodeString(r.After.Indices)
		wantPalette, _ := hex.DecodeString(r.After.Palette)
		if !bytes.Equal(header.PaletteIndices[:], wantIndices) || !bytes.Equal(palette.Desired[:], wantPalette) {
			t.Fatalf("record%d palette differs", r.ID)
		}
		work := b[29]
		if s.GenerationBiasChanged {
			work = s.GenerationBiasValue
		}
		if work != r.After.World[29] {
			t.Fatalf("record%d shared work got%d want%d", r.ID, work, r.After.World[29])
		}
		if queue.Used != r.After.Used {
			t.Fatalf("record%d queue used %d want%d", r.ID, queue.Used, r.After.Used)
		}
		var encoded []uint8
		for _, p := range queue.Packets {
			data, err := p.Encode()
			if err != nil {
				t.Fatal(err)
			}
			encoded = append(encoded, data...)
		}
		beforeQueue, _ := hex.DecodeString(r.Before.Queue)
		afterQueue, _ := hex.DecodeString(r.After.Queue)
		copy(beforeQueue[int(r.Before.Used):], encoded)
		if !bytes.Equal(beforeQueue, afterQueue) {
			t.Fatalf("record%d encoded queue differs", r.ID)
		}
	}
}
