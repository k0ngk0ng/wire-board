// verify-world runs one initial world through logical inputs only. Expected
// snapshots are read-only assertions, never inputs to the game engine.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

type actionConfig struct {
	CourtID                                                                 uint8
	Controls, Characters, RunGrades, JumpGrades, ShotSkills, RecoveryGrades [4]uint8
	Teams                                                                   [2]uint8
	Equipment                                                               uint8
	EnhancementEnabled                                                      bool
	Enhancements                                                            [4]uint8
	ShotEquipment                                                           [4]uint8
	OtherObjectArcFlags                                                     [7]uint8
}
type fixture struct {
	Initial engine.WorldLoopState
	Config  struct {
		PursuitAlias74                                         *engine.PursuitAlias74
		PursuitReserved                                        *engine.PursuitReservedBytes
		Actions                                                actionConfig
		ScoringBonuses                                         [4]bool
		ObjectKinds                                            [13]uint8
		HoopLinks                                              [6]uint8
		AttackGrades, JumpEnhancements                         [4]uint8
		HumanTactics                                           [2]uint8
		Difficulty, BackboardStrength, LayoutID, TrackedObject uint8
	}
	Records []struct {
		InputFrame, ProjectionFrame int
		Input                       engine.WorldLoopInput
		Expected                    engine.WorldLoopState
	}
}

func contextOf(f fixture) (engine.WorldLoopContext, error) {
	v := f.Config
	a := v.Actions
	c := engine.WorldLoopContext{ObjectKinds: v.ObjectKinds, HoopLinks: v.HoopLinks, AttackGrades: v.AttackGrades, JumpEnhancements: v.JumpEnhancements, HumanTactics: v.HumanTactics, Difficulty: v.Difficulty, BackboardStrength: v.BackboardStrength, LayoutID: v.LayoutID, TrackedObject: v.TrackedObject}
	c.PursuitAlias74 = v.PursuitAlias74
	c.PursuitReserved = v.PursuitReserved
	c.Actions = engine.WorldActionContext{CourtID: a.CourtID, Controls: a.Controls, Characters: a.Characters, RecoveryGrades: a.RecoveryGrades, Teams: a.Teams, Equipment: a.Equipment, EnhancementEnabled: a.EnhancementEnabled, Enhancements: a.Enhancements, OtherObjectArcFlags: a.OtherObjectArcFlags}
	c.Actions.ShotEquipment = a.ShotEquipment
	for i := range c.Actions.Profiles {
		profile := engine.KunioProfile()
		var err error
		profile.RunSpeed, err = engine.RunSpeedForGrade(a.RunGrades[i])
		if err != nil {
			return c, err
		}
		profile.JumpVelocity, err = engine.JumpVelocityForGrade(a.JumpGrades[i])
		if err != nil {
			return c, err
		}
		profile.ShotSkill, profile.ScoringBonus = a.ShotSkills[i], v.ScoringBonuses[i]
		c.Actions.Profiles[i] = profile
	}
	return c, nil
}

func difference(got, want reflect.Value, path string) string {
	if reflect.DeepEqual(got.Interface(), want.Interface()) {
		return ""
	}
	switch got.Kind() {
	case reflect.Struct:
		for i := 0; i < got.NumField(); i++ {
			if d := difference(got.Field(i), want.Field(i), path+"."+got.Type().Field(i).Name); d != "" {
				return d
			}
		}
	case reflect.Array:
		for i := 0; i < got.Len(); i++ {
			if d := difference(got.Index(i), want.Index(i), fmt.Sprintf("%s[%d]", path, i)); d != "" {
				return d
			}
		}
	}
	return fmt.Sprintf("%s: got %v want %v", path, got.Interface(), want.Interface())
}

func run() error {
	path := flag.String("fixture", "games/basketball/engine/testdata/world-loop.json.gz", "single initial state, inputs and original expected states")
	flag.Parse()
	raw, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer z.Close()
	var f fixture
	decoder := json.NewDecoder(z)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&f); err != nil {
		return err
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("unexpected data after replay: %v", err)
	}
	if len(f.Records) == 0 {
		return fmt.Errorf("empty world replay")
	}
	ctx, err := contextOf(f)
	if err != nil {
		return err
	}
	state := f.Initial
	stateHash, inputHash := sha256.New(), sha256.New()
	calls, sounds, streams := 0, 0, 0
	for index, row := range f.Records {
		step, err := engine.StepWorldLoop(state, row.Input, ctx)
		if err != nil {
			return fmt.Errorf("update %d original input frame %d: %w", index, row.InputFrame, err)
		}
		if step.State != row.Expected {
			return fmt.Errorf("update %d original projection frame %d: %s", index, row.ProjectionFrame, difference(reflect.ValueOf(step.State), reflect.ValueOf(row.Expected), "state"))
		}
		state = step.State
		calls += len(step.ActionCalls)
		sounds += len(step.SoundIDs)
		if step.StreamColumn {
			streams++
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			return err
		}
		stateHash.Write(encoded)
		encoded, err = json.Marshal(row.Input)
		if err != nil {
			return err
		}
		inputHash.Write(encoded)
	}
	fixtureHash := sha256.Sum256(raw)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"status": "passed", "scope": "continuous core world; video clock and full match flow are separate",
		"updates": len(f.Records), "state_initializations": 1, "action_calls": calls, "sound_requests": sounds, "stream_requests": streams,
		"fixture_sha256": hex.EncodeToString(fixtureHash[:]), "state_sha256": hex.EncodeToString(stateHash.Sum(nil)), "input_sha256": hex.EncodeToString(inputHash.Sum(nil)),
		"final_scores": state.Preparation.Scores, "final_random": state.Preparation.Actions.Random,
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
