// Command verify-arena-pan exercises physical human inputs from a new match.
// It is an E2E verifier, not a rule implementation or reference-state importer.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/runtimebridge"
)

type report struct {
	Pass                 bool                       `json:"pass"`
	Environment          string                     `json:"environment"`
	Arena                uint16                     `json:"arena"`
	Players              int                        `json:"players"`
	Characters           [2]uint8                   `json:"characters"`
	Ticks                int                        `json:"ticks"`
	ReferenceStateLoaded bool                       `json:"reference_state_loaded"`
	HumanAutoTakeovers   int                        `json:"human_auto_takeovers"`
	PanChildFirstTick    int                        `json:"pan_child_first_tick"`
	FirstPanTick         int                        `json:"first_pan_tick"`
	PanFrames            int                        `json:"pan_frames"`
	MaxAbsPan            int                        `json:"max_abs_pan"`
	FeedbackFrames       int                        `json:"feedback_frames"`
	RestoredPanFrames    int                        `json:"restored_pan_frames"`
	RestoredPixelFrames  int                        `json:"restored_pixel_frames"`
	RestoredStateFrames  int                        `json:"restored_state_frames"`
	PausePanFrames       int                        `json:"pause_pan_frames"`
	SpriteFrames         int                        `json:"sprite_frames"`
	PanSaveTick          int                        `json:"pan_save_tick"`
	ArenaSpriteRequests  int                        `json:"arena_sprite_requests"`
	HUDWindowComplete    bool                       `json:"hud_window_complete"`
	RGBAFramesComplete   bool                       `json:"rgba_frames_complete"`
	RulesMatchRuntime    bool                       `json:"rules_match_runtime"`
	FeedbackStreamGaps   bool                       `json:"feedback_stream_gaps_preserved"`
	PanReturnedToZero    bool                       `json:"pan_returned_to_zero"`
	PanChildCleared      bool                       `json:"pan_child_cleared"`
	DamageGateTick       int                        `json:"damage_gate_tick"`
	DamageGateHealth     int16                      `json:"damage_gate_health"`
	CommandTick          int                        `json:"command_tick"`
	CommandHealth        int16                      `json:"command_health"`
	CommandEnergy        int16                      `json:"command_energy"`
	HeldTraceSHA         string                     `json:"held_trace_sha256"`
	RulesTraceSHA        string                     `json:"rules_trace_sha256"`
	StateTraceSHA        string                     `json:"state_trace_sha256"`
	PixelTraceSHA        string                     `json:"pixel_trace_sha256"`
	FinalStateSHA        string                     `json:"final_state_sha256"`
	Artifacts            map[string]string          `json:"artifacts"`
	FinalPlayers         []runtimebridge.PlayerView `json:"final_players"`
}

type inputFixture struct {
	Schema     int                 `json:"schema"`
	Arena      uint16              `json:"arena"`
	Seed       uint32              `json:"seed"`
	Characters [2]uint8            `json:"characters"`
	Held       [][8]engine.Buttons `json:"held"`
}

type traceFrame struct {
	Tick      int               `json:"tick"`
	Held      [8]engine.Buttons `json:"held"`
	Health    int16             `json:"health"`
	Energy    int16             `json:"energy"`
	Action    uint8             `json:"action"`
	X         [2]int16          `json:"x"`
	Pan       int16             `json:"pan"`
	Paused    bool              `json:"paused"`
	Feedback  bool              `json:"feedback"`
	RulesSHA  string            `json:"rules_sha256"`
	StateSHA  string            `json:"state_sha256"`
	PixelsSHA string            `json:"pixels_sha256"`
}

type controller struct {
	phase, settledAt, commandAt, panAt int
	damageHealth                       int16
	commandHealth, commandEnergy       int16
}

// The controller observes only the public player view. Source7's original
// recorded r6 command is Down+B+A with health<=111 and energy>=200. The other
// explicit human supplies normal directional/B inputs to reach that health.
func (c *controller) next(tick int, view runtimebridge.View) (held [8]engine.Buttons) {
	if c.panAt != 0 {
		if tick == c.panAt+12 || tick == c.panAt+20 {
			held[0] = engine.ButtonStart
		}
		return held
	}
	if tick < 320 || len(view.Players) != 2 {
		return held
	}
	first, second := view.Players[0], view.Players[1]
	if c.phase == 0 {
		if first.Health <= 100 {
			c.phase, c.settledAt, c.damageHealth = 1, tick, first.Health
			return held
		}
		dx := int(first.X) - int(second.X)
		if dx < 0 {
			held[1] = engine.ButtonLeft
		} else if dx > 0 {
			held[1] = engine.ButtonRight
		}
		if dx >= -64 && dx <= 64 && tick%24 == 0 {
			held[1] |= engine.ButtonB
		}
		return held
	}
	if c.phase == 1 {
		if tick-c.settledAt < 96 {
			return held
		}
		if first.Energy < 200 {
			held[0] = engine.ButtonB
			return held
		}
		if first.Action != 0 {
			return held
		}
		c.phase, c.commandAt = 2, tick
		c.commandHealth, c.commandEnergy = first.Health, first.Energy
	}
	if c.phase == 2 && tick-c.commandAt < 4 {
		held[0] = engine.ButtonDown | engine.ButtonB | engine.ButtonA
	}
	return held
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func marshal(value any) ([]byte, error) { return json.Marshal(value) }

func readAsset(name string) ([]byte, error) {
	raw, err := os.ReadFile("games/yuyuhakusho/data/" + name + ".json.gz")
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	return io.ReadAll(z)
}

func inside(path string) (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("verification artifacts must remain inside the current worktree")
	}
	return full, nil
}

// This projection is only for comparison. The live worlds are not modified.
// GraphicCache is the existing renderer-owned four-long cache, not a rule.
func ruleProjection(world engine.ChuWorldState) engine.ChuWorldState {
	objects := make(engine.WorldObjects, len(world.Objects))
	for id, object := range world.Objects {
		object.GraphicCache = [4]uint32{}
		objects[id] = object
	}
	world.Objects = objects
	return world
}

func worldFromSave(raw []byte) (engine.ChuWorldState, error) {
	var saved struct {
		World engine.ChuWorldState `json:"world"`
	}
	err := json.Unmarshal(raw, &saved)
	return saved.World, err
}

func gzipBytes(raw []byte) ([]byte, error) {
	var out bytes.Buffer
	z := gzip.NewWriter(&out)
	if _, err := z.Write(raw); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func run() (r report, failure error) {
	selected := flag.Uint("arena", 2, "recovered nondefault arena offset,even2..14")
	input := flag.String("inputs", "", "optional fixed physical Held fixture inside the worktree")
	output := flag.String("output", ".local/md-yuyuhakusho/arena-pan", "artifact directory inside the worktree")
	limit := flag.Int("max-ticks", 6000, "maximum physical Held ticks")
	flag.Parse()
	r = report{Environment: runtime.GOOS + "/" + runtime.GOARCH, Arena: uint16(*selected), Players: 2, Characters: [2]uint8{7, 9},
		HUDWindowComplete: true, RGBAFramesComplete: true, RulesMatchRuntime: true, FeedbackStreamGaps: true}
	if *selected < 2 || *selected > 14 || *selected&1 != 0 || *limit < 1 || *limit > 20000 {
		return r, errors.New("invalid arena or tick limit")
	}
	out, err := inside(*output)
	if err != nil {
		return r, err
	}
	fixture := inputFixture{Schema: 1, Arena: r.Arena, Seed: 0x12345678, Characters: r.Characters}
	var fixed [][8]engine.Buttons
	if *input != "" {
		path, err := inside(*input)
		if err != nil {
			return r, err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return r, err
		}
		var supplied inputFixture
		if err := json.Unmarshal(raw, &supplied); err != nil {
			return r, err
		}
		if supplied.Schema != 1 || supplied.Arena != fixture.Arena || supplied.Seed != fixture.Seed || supplied.Characters != fixture.Characters || len(supplied.Held) == 0 || len(supplied.Held) > *limit {
			return r, errors.New("invalid fixed physical Held fixture")
		}
		fixed = supplied.Held
	}
	rules, err := readAsset("chu-world-rules")
	if err != nil {
		return r, err
	}
	library, err := engine.DecodeChuWorldRules(rules)
	if err != nil {
		return r, err
	}
	sprites, err := readAsset("world-sprites")
	if err != nil {
		return r, err
	}
	arena, err := readAsset(fmt.Sprintf("arena-assets-%x", r.Arena))
	if err != nil {
		return r, err
	}
	hud, err := readAsset("hud-assets")
	if err != nil {
		return r, err
	}
	var aux []byte
	if r.Arena == 10 {
		aux, err = readAsset("arena-a-aux-assets")
		if err != nil {
			return r, err
		}
	}
	assets, err := runtimebridge.DecodeAssetsForArena(rules, sprites, arena, hud, aux, r.Arena)
	if err != nil {
		return r, err
	}
	options := engine.ChuSessionOptions{Arena: r.Arena, Seed: fixture.Seed, Difficulty: 6, Players: []engine.ChuSessionPlayer{
		{Slot: 0, CharacterID: 7, Color: 0, HumanChannel: 0},
		{Slot: 3, CharacterID: 9, Color: 1, HumanChannel: 1},
	}}
	world, _, err := engine.NewChuSession(options, library)
	if err != nil {
		return r, err
	}
	resource, err := engine.ChuSessionResourceForArena(r.Arena, library)
	if err != nil {
		return r, err
	}
	world, err = engine.CompleteChuSessionResource(world, resource, library)
	if err != nil {
		return r, err
	}
	session, err := runtimebridge.NewSession(options, assets)
	if err != nil {
		return r, err
	}
	if err = session.Ready(); err != nil {
		return r, err
	}
	var inputs controller
	var restored *runtimebridge.Session
	var restoredWorld engine.ChuWorldState
	var panSave, panPixels, finalSave []byte
	var trace bytes.Buffer
	heldHash, rulesHash, stateHash, pixelHash := sha256.New(), sha256.New(), sha256.New(), sha256.New()
	for tick := 1; tick <= *limit; tick++ {
		held := inputs.next(tick, session.View())
		if fixed != nil {
			if tick > len(fixed) {
				break
			}
			held = fixed[tick-1]
		}
		for _, buttons := range held {
			if err := engine.ValidateControllerButtons(buttons); err != nil {
				return r, err
			}
		}
		fixture.Held = append(fixture.Held, held)
		encodedHeld, _ := marshal(held)
		heldHash.Write(encodedHeld)
		previous := world
		var events []engine.ChuWorldEvent
		world, events, _, err = engine.StepChuSessionFrame(world, held, library)
		if err != nil {
			return r, fmt.Errorf("rules tick%d: %w", tick, err)
		}
		frame, err := session.Step(held)
		if err != nil {
			return r, fmt.Errorf("Canvas tick%d: %w", tick, err)
		}
		saved, err := session.Save()
		if err != nil {
			return r, err
		}
		actual, err := worldFromSave(saved)
		if err != nil {
			return r, err
		}
		if !reflect.DeepEqual(ruleProjection(world), ruleProjection(actual)) {
			r.RulesMatchRuntime = false
			return r, fmt.Errorf("fresh rule/runtime worlds diverged at tick%d", tick)
		}
		if len(frame.Pixels) != 320*224*4 {
			r.RGBAFramesComplete = false
			return r, errors.New("incomplete Canvas RGBA frame")
		}
		for i := 3; i < len(frame.Pixels); i += 4 {
			if frame.Pixels[i] != 255 {
				r.RGBAFramesComplete = false
				return r, fmt.Errorf("transparent completed Canvas pixel at tick%d pixel%d", tick, i/4)
			}
		}
		if !frame.View.WindowComplete {
			r.HUDWindowComplete = false
			return r, errors.New("incomplete HUD/window during dynamic pan")
		}
		for i, player := range frame.View.Players {
			if player.Computer || player.InputChannel != uint8(i) || player.CharacterID != r.Characters[i] {
				r.HumanAutoTakeovers++
				return r, errors.New("explicit human controller mapping changed")
			}
		}
		if frame.SpritePixels > 0 {
			r.SpriteFrames++
		}
		r.ArenaSpriteRequests += frame.ArenaSpriteRequests
		for _, event := range events {
			if event.Kind == "callback" && event.Header.Behavior == 0x15094 && r.PanChildFirstTick == 0 {
				r.PanChildFirstTick = tick
			}
		}
		pan := int16(world.Clock.PanOffset)
		feedback := pan != 0 && world.Clock.PauseFlags&128 == 0 && world.Clock.SlowFlag == 0
		if pan != 0 {
			r.PanFrames++
			absolute := int(pan)
			if absolute < 0 {
				absolute = -absolute
			}
			if absolute > r.MaxAbsPan {
				r.MaxAbsPan = absolute
			}
			if r.FirstPanTick == 0 {
				r.FirstPanTick, inputs.panAt = tick, tick
			}
			if frame.View.Paused {
				r.PausePanFrames++
			}
		}
		if feedback {
			if world.ScrollSources != [2]int32{-0xa000, -0x9e00} || len(world.Stream) != 1024 || world.PanRows(0) == ([224]int16{}) || world.PanRows(1) == ([224]int16{}) {
				return r, fmt.Errorf("arena%x did not expose its real feedback stream at tick%d", r.Arena, tick)
			}
			for _, span := range [][2]int{{448, 512}, {960, 1024}} {
				if !bytes.Equal(world.Stream[span[0]:span[1]], previous.Stream[span[0]:span[1]]) {
					r.FeedbackStreamGaps = false
					return r, errors.New("arena feedback changed the Stream gap bytes")
				}
			}
			r.FeedbackFrames++
		}
		if restored != nil {
			var otherEvents []engine.ChuWorldEvent
			restoredWorld, otherEvents, _, err = engine.StepChuSessionFrame(restoredWorld, held, library)
			if err != nil || !reflect.DeepEqual(world, restoredWorld) || !reflect.DeepEqual(events, otherEvents) {
				return r, fmt.Errorf("rules restored during pan diverged at tick%d: %v", tick, err)
			}
			other, err := restored.Step(held)
			if err != nil {
				return r, err
			}
			otherSave, err := restored.Save()
			if err != nil || !reflect.DeepEqual(other.View, frame.View) || !bytes.Equal(other.Pixels, frame.Pixels) || !bytes.Equal(otherSave, saved) {
				return r, fmt.Errorf("pan-restored Canvas/view/save diverged at tick%d: %v", tick, err)
			}
			r.RestoredStateFrames++
			r.RestoredPixelFrames++
			if pan != 0 {
				r.RestoredPanFrames++
			}
		}
		if r.FirstPanTick == tick {
			panSave, panPixels = append([]byte(nil), saved...), append([]byte(nil), frame.Pixels...)
			restored, err = runtimebridge.RestoreSession(panSave, assets)
			if err != nil {
				return r, err
			}
			rawWorld, _ := marshal(world)
			if err = json.Unmarshal(rawWorld, &restoredWorld); err != nil {
				return r, err
			}
			r.PanSaveTick = tick
		}
		ruleRaw, err := marshal(struct {
			Tick   int
			World  engine.ChuWorldState
			Events []engine.ChuWorldEvent
		}{tick, world, events})
		if err != nil {
			return r, err
		}
		rulesHash.Write(ruleRaw)
		stateHash.Write(saved)
		pixelHash.Write(frame.Pixels)
		row := traceFrame{Tick: tick, Held: held, Health: frame.View.Players[0].Health, Energy: frame.View.Players[0].Energy,
			Action: frame.View.Players[0].Action, X: [2]int16{frame.View.Players[0].X, frame.View.Players[1].X}, Pan: pan,
			Paused: frame.View.Paused, Feedback: feedback, RulesSHA: digest(ruleRaw), StateSHA: digest(saved), PixelsSHA: digest(frame.Pixels)}
		encoded, _ := marshal(row)
		trace.Write(encoded)
		trace.WriteByte('\n')
		r.Ticks, finalSave = tick, saved
		if fixed == nil && r.FirstPanTick != 0 && tick >= r.FirstPanTick+360 {
			break
		}
	}
	r.DamageGateTick, r.DamageGateHealth = inputs.settledAt, inputs.damageHealth
	r.CommandTick, r.CommandHealth, r.CommandEnergy = inputs.commandAt, inputs.commandHealth, inputs.commandEnergy
	r.PanReturnedToZero = r.FirstPanTick > 0 && world.Clock.PanOffset == 0 && world.Clock.PanAmplitude == 0
	r.PanChildCleared = true
	for _, object := range world.Objects {
		if object.Header.Behavior == 0x15094 {
			r.PanChildCleared = false
		}
	}
	r.HeldTraceSHA, r.RulesTraceSHA = hex.EncodeToString(heldHash.Sum(nil)), hex.EncodeToString(rulesHash.Sum(nil))
	r.StateTraceSHA, r.PixelTraceSHA, r.FinalStateSHA = hex.EncodeToString(stateHash.Sum(nil)), hex.EncodeToString(pixelHash.Sum(nil)), digest(finalSave)
	r.FinalPlayers = session.View().Players
	r.Pass = r.PanChildFirstTick > 0 && r.FirstPanTick > r.PanChildFirstTick && r.PanFrames > 0 && r.FeedbackFrames > 0 && r.RestoredPanFrames > 0 && r.RestoredPixelFrames >= 30 && r.PausePanFrames > 0 && r.HumanAutoTakeovers == 0 && r.PanReturnedToZero && r.PanChildCleared && r.SpriteFrames > 0
	if !r.Pass {
		if err := os.MkdirAll(out, 0755); err != nil {
			return r, err
		}
		heldRaw, _ := marshal(fixture)
		tracePacked, _ := gzipBytes(trace.Bytes())
		finalPacked, _ := gzipBytes(finalSave)
		for name, data := range map[string][]byte{"failure-held-inputs.json": heldRaw, "failure-trace.jsonl.gz": tracePacked, "failure-final-save.json.gz": finalPacked} {
			if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
				return r, err
			}
		}
		return r, fmt.Errorf("legal human inputs did not complete source7 dynamic pan in arena%x within%d ticks (health=%d energy=%d phase=%d)", r.Arena, r.Ticks, session.View().Players[0].Health, session.View().Players[0].Energy, inputs.phase)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return r, err
	}
	heldRaw, _ := marshal(fixture)
	heldRaw = append(heldRaw, '\n')
	tracePacked, err := gzipBytes(trace.Bytes())
	if err != nil {
		return r, err
	}
	panPacked, err := gzipBytes(panSave)
	if err != nil {
		return r, err
	}
	var pngBytes bytes.Buffer
	canvas := &image.RGBA{Pix: panPixels, Stride: 320 * 4, Rect: image.Rect(0, 0, 320, 224)}
	if err := png.Encode(&pngBytes, canvas); err != nil {
		return r, err
	}
	artifacts := map[string][]byte{"held-inputs.json": heldRaw, "trace.jsonl.gz": tracePacked, "first-pan.png": pngBytes.Bytes(), "pan-save.json.gz": panPacked}
	r.Artifacts = make(map[string]string, len(artifacts))
	for name, data := range artifacts {
		if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
			return r, err
		}
		r.Artifacts[name] = digest(data)
	}
	return r, nil
}

func main() {
	r, err := run()
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(r); encodeErr != nil {
		fmt.Fprintln(os.Stderr, encodeErr)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
