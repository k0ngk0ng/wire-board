package engine

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// Original fixtures and this acceptance precede ground_actions.go. Natural
// tail-called resets overlap their parent constructors and are not extra turns.
func TestGroundConstructorsAgainstOriginal(t *testing.T) {
	for _, name := range []string{"ground-actions-oracle", "ground-actions-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open("testdata/" + name + ".json.gz")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			z, err := gzip.NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			raw, err := io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			var rows []struct {
				Kind          string
				Before, After GroundActionState
				Direction     uint8
				Held          Buttons
			}
			if err = json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var m struct {
				Pass  bool
				Count int    `json:"complete_calls"`
				SHA   string `json:"fixture_sha256"`
			}
			if err = json.Unmarshal(mraw, &m); err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(raw)
			if !m.Pass || len(rows) == 0 || len(rows) != m.Count || hex.EncodeToString(hash[:]) != m.SHA {
				t.Fatal("unverified constructor fixture")
			}
			transitions := map[string]GroundTransition{"stand": GroundStand, "rise": GroundRise, "crouch": GroundCrouch, "guard": GroundGuard, "turn": GroundTurn, "crouch_turn": GroundCrouchTurn, "walk": GroundWalk}
			for i, r := range rows {
				var got GroundActionState
				switch r.Kind {
				case "reset":
					got = ResetActionState(r.Before)
				case "prepare_jump":
					got = PrepareJump(r.Before, r.Direction)
				default:
					transition, ok := transitions[r.Kind]
					if !ok {
						t.Fatal("unknown original constructor", r.Kind)
					}
					got, err = EnterGroundAction(r.Before, transition, r.Held)
					if err != nil {
						t.Fatal(err)
					}
				}
				if got != r.After {
					t.Fatalf("call %d %s: got %+v want %+v", i, r.Kind, got, r.After)
				}
			}
		})
	}
}

func TestGroundUnknownTransitionDoesNotChangeState(t *testing.T) {
	before := GroundActionState{Pose: 27, ActionKind: 42, Clock: AnimationClock{Completed: 3}, VerticalVelocity: -123456}
	got, err := EnterGroundAction(before, GroundTransition(255), 0)
	if err == nil || got != before {
		t.Fatalf("unsupported transition changed state: %+v %v", got, err)
	}
}
