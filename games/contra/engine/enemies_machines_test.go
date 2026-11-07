package contra

// Failure scenarios precede implementation: animation refusal advances motion;
// scroll/removal skips the remainder of a native state; trigger delays use the
// wrong clock; 8-bit underflow or nearest-player ties change; multi-slot and
// sound side effects disappear; visual writes mutate collision too early.

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type machineOracleState struct {
	Enemies                                       [][]uint8
	Players                                       [][]uint8
	Globals, Timers, Queue, Supertiles, Collision string
	Palette                                       string
	AudioSlots                                    string
}
type machineOracleRecord struct {
	ID, Frame, Slot      int
	Level, Kind, Routine uint8
	Before, After        machineOracleState
	Audio                []struct {
		Play  uint8
		Reset bool
	}
}

func machineRecords(t *testing.T, path string) []machineOracleRecord {
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
		Records  []machineOracleRecord
	}
	if err = json.NewDecoder(z).Decode(&d); err != nil || !d.Original || len(d.Records) == 0 {
		t.Fatal("invalid original machinery oracle", err)
	}
	return d.Records
}

func TestOriginalMachineClaw(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-natural.json.gz")
	if len(rows) != 6845 {
		t.Fatal("claw oracle coverage changed", len(rows))
	}
	counts := map[uint8]int{}
	for _, r := range rows {
		if r.Level != 6 || r.Kind != 16 {
			t.Fatal("unexpected natural type", r.Level, r.Kind)
		}
		verifyMachineStep(t, resources, r)
		counts[r.Routine]++
	}
	if !reflect.DeepEqual(counts, map[uint8]int{1: 9, 2: 5549, 3: 447, 4: 840}) {
		t.Fatal(counts)
	}
	t.Logf("%d original claw calls match all four routines and graphics queue", len(rows))
}

func verifyMachineStep(t *testing.T, resources *LevelResources, r machineOracleRecord) {
	t.Helper()
	g, _ := hex.DecodeString(r.Before.Globals)
	afterG, _ := hex.DecodeString(r.After.Globals)
	windowBytes, _ := hex.DecodeString(r.Before.Collision)
	window := CollisionWindow(windowBytes)
	mapBytes, _ := hex.DecodeString(r.Before.Supertiles)
	maps := ([128]uint8)(mapBytes)
	queue := GraphicsQueue{Used: g[0x21]}
	tiles, err := resources.Graphics(int(r.Level), RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	animations, err := OriginalTileAnimations(r.Level)
	if err != nil && r.Kind >= 16 {
		t.Fatal(err)
	}
	before := enemyPoolFromOracle(r.Before.Enemies)
	s := EnemySystem{Slots: before, BossPartsDestroyed: g[0x85]}
	ready := g[0x84]
	defeated, delayLow, delayHigh, resets := g[0x3b], g[0x2a], g[0x2b], 0
	soundCarryCalls := 0
	c := EnemyStepContext{Level: r.Level, Frame: g[0x1a], FrameScroll: g[0x68], Vertical: g[0x41] != 0, SceneType: g[0x40], Random: g[0x34], WeaponStrength: g[0x2f], GameLoop: g[0x31], AttackEnabled: g[0x8e] != 0, Terrain: &window, Scroll: CollisionScroll{X: g[0xfd], Y: g[0xfc], NameTable: g[0xff]}, Scene: EnemySceneContext{BossScrollComplete: &ready}}
	c.OnBossControl = func(control BossControl) error {
		switch control.Kind {
		case BossAudioReset:
			resets++
		case BossBeginDefeat:
			defeated = 1
			delayLow = 255
		case BossExitReady:
			delayLow = control.Delay
			delayHigh = 0
		default:
			t.Fatal("unknown boss control", control)
		}
		return nil
	}
	c.SoundRequestCarry = func(code uint8, prefix EnemyAudioPrefix) (bool, error) {
		if code != 0x2b {
			t.Fatal("unexpected sound carry request", code)
		}
		if prefix.Reset || len(prefix.Sounds) != 0 {
			t.Fatal("this single-dispatch oracle begins with no prior sounds", prefix)
		}
		soundCarryCalls++
		// Input-side original sound-slot projection: 2B's sole slot4
		// update returns C0, except 87B1's slot1=62 global veto returns C1.
		// No expected enemy-after field is supplied through this callback.
		ids, _ := hex.DecodeString(r.Before.AudioSlots)
		return len(ids) == 6 && ids[1] == 0x62, nil
	}
	var palette PaletteOutput
	var header PaletteState
	if r.Before.Palette != "" {
		b, _ := hex.DecodeString(r.Before.Palette)
		copy(palette.Desired[:], b)
		palette.RequestedColors = g[0x36]
		palette.FadeTimer = g[0x74]
		copy(header.CollisionLimits[:], g[0x49:0x4c])
		copy(header.CycleIndices[:], g[0x4c:0x50])
		copy(header.PaletteIndices[:], g[0x50:0x58])
		c.Scene.Header = &header
		c.Scene.Palette = &palette
		c.Scene.RefreshBackgroundPalette = func(count uint8) (PaletteResult, error) { return palette.Load(count, c.SceneType, header, resources) }
	}
	for i, p := range r.Before.Players {
		c.Players[i] = EnemyTarget{State: p[0], X: p[1], Y: p[3], GameOver: p[41]}
	}
	c.ChangeTiles = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
		v, err := queue.RequestTileAnimation(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &maps, Tileset: tiles}, animations)
		return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, err
	}
	c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
		v, err := queue.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &maps, Tileset: tiles})
		return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, err
	}
	var stepErr error
	if r.Kind == 11 {
		stepErr = s.StepMortarProjectile(r.Slot, c)
	} else if r.Kind == 14 {
		stepErr = s.StepTurretMan(r.Slot, c)
	} else if r.Kind == 15 {
		stepErr = s.StepTurretManBullet(r.Slot, c)
	} else if r.Level == 5 && r.Kind == 20 {
		stepErr = s.StepSpikedDisk(r.Slot, c)
	} else if r.Level == 5 && r.Kind == 19 {
		stepErr = s.StepGiantSoldier(r.Slot, c)
	} else if r.Level == 6 && r.Kind == 19 {
		stepErr = s.StepCartGenerator(r.Slot, c)
	} else if r.Level == 6 && (r.Kind == 20 || r.Kind == 21) {
		stepErr = s.StepCart(r.Slot, c)
	} else if r.Level == 6 && r.Kind == 23 {
		stepErr = s.StepMortarLauncher(r.Slot, c)
	} else if r.Level == 6 && r.Kind == 24 {
		stepErr = s.StepHangarSoldierGenerator(r.Slot, c)
	} else if r.Level == 6 && (r.Kind == 17 || r.Kind == 18) {
		stepErr = s.StepSpikeWall(r.Slot, c)
	} else if r.Level == 6 && r.Kind == 22 {
		stepErr = s.StepArmoredDoor(r.Slot, c)
	} else if r.Level == 5 {
		stepErr = s.StepFlame(r.Slot, c)
	} else {
		stepErr = s.StepClaw(r.Slot, c)
	}
	if stepErr != nil {
		t.Fatalf("call%d: %v", r.ID, stepErr)
	}
	if r.Before.Palette != "" {
		bias := g[6]
		if s.GenerationBiasChanged {
			bias = s.GenerationBiasValue
		}
		if hex.EncodeToString(palette.Desired[:]) != r.After.Palette || palette.RequestedColors != afterG[0x36] || palette.FadeTimer != afterG[0x74] || !bytes.Equal(header.PaletteIndices[:], afterG[0x50:0x58]) || bias != afterG[6] {
			t.Fatalf("giant call%d palette or generation side effect differs bias%d want%d", r.ID, bias, afterG[6])
		}
	}
	want := enemyPoolFromOracle(r.After.Enemies)
	if s.Slots != want {
		for i, e := range s.Slots {
			if e != want[i] {
				t.Fatalf("call%d state%d slot%d got%+v want%+v before%+v", r.ID, r.Routine, i, e, want[i], before[i])
			}
		}
	}
	var sounds []uint8
	wantResets := 0
	wantSoundCarries := 0
	for _, a := range r.Audio {
		if a.Reset {
			wantResets++
		} else {
			sounds = append(sounds, a.Play)
			if r.Level == 5 && r.Kind == 19 && a.Play == 0x2b {
				wantSoundCarries++
			}
		}
	}
	if soundCarryCalls != wantSoundCarries {
		t.Fatalf("call%d sound carry callback count%d want%d", r.ID, soundCarryCalls, wantSoundCarries)
	}
	if !reflect.DeepEqual(r.Before.Players, r.After.Players) || hex.EncodeToString(window[:]) != r.After.Collision || hex.EncodeToString(maps[:]) != r.After.Supertiles || len(s.PendingSounds) != len(sounds) || s.BossPartsDestroyed != afterG[0x85] || ready != afterG[0x84] || resets != wantResets || defeated != afterG[0x3b] || delayLow != afterG[0x2a] || delayHigh != afterG[0x2b] {
		t.Fatal("unmodeled claw side effect", r.ID)
	}
	for i, sound := range s.PendingSounds {
		if sound != sounds[i] {
			t.Fatal("wrong native audio", r.ID, s.PendingSounds, r.Audio)
		}
	}
	beforeQueue, _ := hex.DecodeString(r.Before.Queue)
	afterQueue, _ := hex.DecodeString(r.After.Queue)
	var appended []uint8
	for _, p := range queue.Packets {
		b, err := p.Encode()
		if err != nil {
			t.Fatal(err)
		}
		appended = append(appended, b...)
	}
	copy(beforeQueue[int(g[0x21]):], appended)
	if queue.Used != afterG[0x21] || !bytes.Equal(beforeQueue, afterQueue) {
		t.Fatalf("call%d graphics used%d want%d", r.ID, queue.Used, afterG[0x21])
	}
}

func TestControlledMachineGiantSoldier(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-giant.json.gz")
	if len(rows) != 342 {
		t.Fatal("giant oracle coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
	t.Logf("%d original giant soldier calls match", len(rows))
}

func TestControlledMachineFlameIdleFrames(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-edges.json.gz")
	if len(rows) != 15 {
		t.Fatal("idle flame coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
}

func TestControlledMachineGiantSoundCarry(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-giant-audio.json.gz")
	if len(rows) != 9 {
		t.Fatal("sound carry coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
}

func TestControlledMachineSpawnVariants(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-spawn-variants.json.gz")
	if len(rows) != 18 {
		t.Fatal("spawn attribute coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
}

func TestControlledMachineMortarVertical(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-mortar-vertical.json.gz")
	if len(rows) != 108 {
		t.Fatal("vertical mortar coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
}

func TestControlledMachineWallsAndArmoredDoor(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-walls.json.gz")
	if len(rows) != 207 {
		t.Fatal("wall oracle coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
	t.Logf("%d original spike wall and armored door calls match", len(rows))
}

func TestControlledMachineHangarMechanisms(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-mechanisms.json.gz")
	if len(rows) != 542 {
		t.Fatal("hangar oracle coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
	t.Logf("%d original hangar mechanism calls match", len(rows))
}

func TestControlledMachineCommonActors(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-common.json.gz")
	if len(rows) != 469 {
		t.Fatal("common machine oracle coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
	t.Logf("%d original common mortar/turret/disk calls match", len(rows))
}

func TestControlledMachineFlamesAndClaw(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := machineRecords(t, "testdata/original-enemy-machines-cases.json.gz")
	if len(rows) != 546 {
		t.Fatal("controlled machinery coverage changed", len(rows))
	}
	for _, r := range rows {
		verifyMachineStep(t, resources, r)
	}
	t.Logf("%d original flame/claw controlled calls match", len(rows))
}
