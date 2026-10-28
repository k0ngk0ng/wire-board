// Replay a bot attempt using only its held input stream and standard NewSession.
// Strategy code is not called; the final core owners must match independently.
//go:build ignore

package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	contra "github.com/k0ngk0ng/wire-board/games/contra/engine"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: go run scripts/contra/bot_replay.go <playersN attempt directory>")
	}
	root, e := os.Getwd()
	if e != nil {
		return e
	}
	out, e := filepath.Abs(os.Args[1])
	if e != nil {
		return e
	}
	if !strings.HasPrefix(out, root+string(filepath.Separator)) {
		return fmt.Errorf("replay artifacts must stay in workspace")
	}
	raw, e := os.ReadFile(filepath.Join(out, "result.json"))
	if e != nil {
		return e
	}
	var result struct {
		Seed    uint64
		Players int
		Frames  uint64
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		return e
	}
	input, e := os.Open(filepath.Join(out, "pads.bin.gz"))
	if e != nil {
		return e
	}
	defer input.Close()
	z, e := gzip.NewReader(input)
	if e != nil {
		return e
	}
	defer z.Close()
	pads, e := io.ReadAll(z)
	if e != nil {
		return e
	}
	if len(pads) != int(result.Frames)*2 {
		return fmt.Errorf("pad count differs from accepted frame count")
	}
	raw, e = os.ReadFile(filepath.Join(out, "final-session.json"))
	if e != nil {
		return e
	}
	var expected contra.Session
	if e = json.Unmarshal(raw, &expected); e != nil {
		return e
	}
	var resources [8]*contra.LevelResources
	for i := range resources {
		raw, e = os.ReadFile(filepath.Join(root, "games/contra/engine/leveldata", fmt.Sprintf("stage-%d.json", i+1)))
		if e != nil {
			return e
		}
		resources[i], e = contra.DecodeLevelResources(raw)
		if e != nil {
			return e
		}
	}
	s, e := contra.NewSession(result.Seed, result.Players, "bot-standard")
	if e != nil {
		return e
	}
	for frame := uint64(1); frame <= result.Frames; frame++ {
		i := (frame - 1) * 2
		_, e = s.StepPads(frame, [2]contra.Buttons{contra.Buttons(pads[i]), contra.Buttons(pads[i+1])}, resources[min(7, int(s.World.Active.Combat.World.Level))])
		if e != nil {
			return fmt.Errorf("replay frame %d: %w", frame, e)
		}
	}
	audio, _ := json.Marshal(s.Audio)
	wantAudio, _ := json.Marshal(expected.Audio)
	// Audio's empty Published map is omitted when saved; nil and an allocated
	// empty map have the same serialized state. Compare every serialized field.
	checks := map[string]bool{"world": reflect.DeepEqual(s.World, expected.World), "entropy": reflect.DeepEqual(s.Random, expected.Random), "serialized_audio": string(audio) == string(wantAudio), "held_controls": reflect.DeepEqual(s.Controls, expected.Controls)}
	passed := true
	for _, same := range checks {
		passed = passed && same
	}
	hash := sha256.Sum256(pads)
	report := map[string]any{"passed": passed, "frames": result.Frames, "seed": result.Seed, "players": result.Players, "raw_pad_sha256": hex.EncodeToString(hash[:]), "checks": checks, "strategy_called": false, "state_injection": false, "scope": "Existing World, entropy, audio and controls compared; additive outer Session transport metadata is not historical simulation state."}
	raw, _ = json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(filepath.Join(out, "replay-verification.json"), append(raw, '\n'), 0644); e != nil {
		return e
	}
	fmt.Println(string(raw))
	if !passed {
		return fmt.Errorf("independent held-pad replay differs")
	}
	return nil
}
