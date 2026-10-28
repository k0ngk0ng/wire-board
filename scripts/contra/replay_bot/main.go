// Replay recorded legal pads and explicitly recorded test reserve grants.
// No policy is executed and no unrecorded state correction is permitted.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

func main() {
	dir := flag.String("run", "", "bot evidence players directory")
	before := flag.Uint64("before-terminal", 0, "capture this many recorded zero-pad frames before a cleared terminal")
	out := flag.String("boundary-out", "", "new workspace directory for a verified assisted terminal boundary")
	flag.Parse()
	if err := run(*dir, *before, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(dir string, before uint64, outputDirectory string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	var prefix []byte
	inputHashes := map[string]string{}
	hash := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	var report struct {
		Seed          uint64
		Players       int
		InfiniteLives bool `json:"infinite_lives"`
	}
	read := func(name string, out any) error {
		raw, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			return e
		}
		inputHashes[name] = hash(raw)
		return json.Unmarshal(raw, out)
	}
	if err := read("result.json", &report); err != nil {
		return err
	}
	var expected engine.Session
	if err := read("final-session.json", &expected); err != nil {
		return err
	}
	if before != 0 && (before > 600 || before >= expected.Random.Frames || !expected.Finalized || expected.World.Campaign.Outcome != engine.CampaignCleared || outputDirectory == "") {
		return fmt.Errorf("capture requires a recorded cleared terminal and 1..600 prior frames")
	}
	var grants []struct {
		Frame         uint64
		Actor         int
		Before, After uint8
	}
	if report.InfiniteLives {
		if err := read("test-life-refills.json", &grants); err != nil {
			return err
		}
	}
	f, err := os.Open(filepath.Join(dir, "pads.bin.gz"))
	if err != nil {
		return err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer z.Close()
	pads, err := io.ReadAll(z)
	if err != nil {
		return err
	}
	if len(pads)%2 != 0 {
		return fmt.Errorf("incomplete pad stream")
	}
	packed, err := os.ReadFile(filepath.Join(dir, "pads.bin.gz"))
	if err != nil {
		return err
	}
	inputHashes["pads.bin.gz"] = hash(packed)
	if before != 0 {
		if uint64(len(pads)/2) != expected.Random.Frames {
			return fmt.Errorf("recorded frame count differs")
		}
		for _, button := range pads[len(pads)-int(before)*2:] {
			if button != 0 {
				return fmt.Errorf("terminal suffix requires recorded physical input; choose a shorter zero-pad suffix")
			}
		}
		for _, grant := range grants {
			if grant.Frame > expected.Random.Frames-before {
				return fmt.Errorf("terminal suffix contains a reserve grant")
			}
		}
	}
	s, err := engine.NewSession(report.Seed, report.Players, expected.Audio.Epoch)
	if err != nil {
		return err
	}
	nextGrant := 0
	apply := func(frame uint64) error {
		for nextGrant < len(grants) && grants[nextGrant].Frame <= frame {
			g := grants[nextGrant]
			if g.Frame != frame || g.Actor < 0 || g.Actor >= report.Players {
				return fmt.Errorf("invalid grant at %d", frame)
			}
			player := &s.World.Active.Combat.Players[g.Actor]
			if player.Lives != g.Before {
				return fmt.Errorf("grant input differs at %d", frame)
			}
			player.Lives = g.After
			if frame == 0 {
				s.PresentedLives[g.Actor] = g.After
			}
			nextGrant++
		}
		return nil
	}
	if err = apply(0); err != nil {
		return err
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		return err
	}
	for i := 0; i < len(pads); i += 2 {
		frame := uint64(i/2 + 1)
		res, e := bundle.Stage(s.World.Active.Combat.World.Level)
		if e != nil {
			return e
		}
		if _, e = s.StepPads(frame, [2]engine.Buttons{engine.Buttons(pads[i]), engine.Buttons(pads[i+1])}, res); e != nil {
			return fmt.Errorf("frame%d: %w", frame, e)
		}
		if e = apply(frame); e != nil {
			return e
		}
		if before != 0 && frame == expected.Random.Frames-before {
			if s.Finalized || s.World.Campaign.Outcome != engine.CampaignOngoing {
				return fmt.Errorf("capture boundary is already terminal")
			}
			prefix, e = json.Marshal(s)
			if e != nil {
				return e
			}
		}
	}
	if nextGrant != len(grants) {
		return fmt.Errorf("unconsumed reserve grants")
	}
	if expected.Finalized {
		if _, err = s.FlushTerminal(); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	want, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	// Older evidence predates the optional persisted battlefield. Preserve its
	// original World/RNG/audio/controls/terminal comparison, and explicitly mark
	// only replay-derived presentation metadata as an addition.
	rulesOnly, expectedRules := *s, expected
	rulesOnly.Review, expectedRules.Review = nil, nil
	rulesBytes, _ := json.Marshal(rulesOnly)
	expectedRulesBytes, _ := json.Marshal(expectedRules)
	rulesMatch := bytes.Equal(rulesBytes, expectedRulesBytes)
	reviewAdded := expected.Review == nil && s.Review != nil
	var restored engine.Session
	if err = json.Unmarshal(raw, &restored); err != nil {
		return err
	}
	var differences []string
	diff(reflect.ValueOf(*s), reflect.ValueOf(restored), "Session", &differences)
	result := map[string]any{"frames": s.Random.Frames, "recordedStateMatches": bytes.Equal(raw, want), "recordedRulesAndEffectsMatch": rulesMatch, "replayDerivedReviewAdded": reviewAdded, "roundTripDifferences": differences}
	if s.World.Campaign.Outcome == engine.CampaignOngoing {
		res, e := bundle.Stage(s.World.Active.Combat.World.Level)
		if e != nil {
			return e
		}
		held := s.Controls.Held
		if _, e = s.StepPads(s.Random.Frames+1, held, res); e != nil {
			return e
		}
		if _, e = restored.StepPads(restored.Random.Frames+1, held, res); e != nil {
			return e
		}
		result["nextStepStructEqual"] = reflect.DeepEqual(*s, restored)
		a, _ := json.Marshal(s)
		b, _ := json.Marshal(restored)
		result["nextStepStoredStateEqual"] = bytes.Equal(a, b)
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(output))
	if !bytes.Equal(raw, want) && !(rulesMatch && reviewAdded) {
		return fmt.Errorf("recorded final state did not reproduce")
	}
	if before != 0 {
		out, e := filepath.Abs(outputDirectory)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, out)
		if e != nil || rel == ".." || len(rel) > 3 && rel[:3] == "../" || filepath.IsAbs(rel) {
			return fmt.Errorf("boundary output must stay in workspace")
		}
		if len(prefix) == 0 {
			return fmt.Errorf("missing live boundary")
		}
		if e = os.Mkdir(out, 0755); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(out, "before-terminal.json"), prefix, 0644); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(out, "expected-final.json"), raw, 0644); e != nil {
			return e
		}
		binding := map[string]any{"schema": 1, "scope": "Declared reserve-refill diagnostic replay; test-only terminal and rating projection, not standard lives or fresh campaign acceptance", "players": report.Players, "seed": report.Seed, "assisted": report.InfiniteLives, "reserveGrants": len(grants), "source": dir, "sourceSHA256": inputHashes, "prefixFrame": expected.Random.Frames - before, "finalFrame": expected.Random.Frames, "tailFrames": before, "tailPadsZero": true, "recordedStateMatches": bytes.Equal(raw, want), "recordedRulesAndEffectsMatch": rulesMatch, "replayDerivedReviewAdded": reviewAdded, "prefixSHA256": hash(prefix), "finalSHA256": hash(raw)}
		data, e := json.MarshalIndent(binding, "", "  ")
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(out, "victory-boundary.json"), append(data, '\n'), 0644); e != nil {
			return e
		}
	}
	return nil
}
func diff(a, b reflect.Value, path string, out *[]string) {
	if a.CanInterface() && b.CanInterface() && reflect.DeepEqual(a.Interface(), b.Interface()) {
		return
	}
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			diff(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name, out)
		}
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			diff(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i), out)
		}
	case reflect.Slice, reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			*out = append(*out, fmt.Sprintf("%s: nil=%v/%v len=%d/%d", path, a.IsNil(), b.IsNil(), a.Len(), b.Len()))
			return
		}
		if a.Kind() == reflect.Slice {
			for i := 0; i < a.Len(); i++ {
				diff(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i), out)
			}
		} else {
			for _, k := range a.MapKeys() {
				v := b.MapIndex(k)
				if !v.IsValid() {
					*out = append(*out, path+": missing map key")
					continue
				}
				diff(a.MapIndex(k), v, path+"["+fmt.Sprint(k)+"]", out)
			}
		}
	case reflect.Pointer:
		if a.IsNil() != b.IsNil() {
			*out = append(*out, path+": nil pointer differs")
		} else if !a.IsNil() {
			diff(a.Elem(), b.Elem(), path, out)
		}
	default:
		*out = append(*out, fmt.Sprintf("%s: %v / %v", path, a, b))
	}
}
