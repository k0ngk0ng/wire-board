package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Failure inventory was written in SUSPENDED_BALL_REPAIR_PROPOSAL.md before
// this acceptance and implementation. The source fixtures remain unmodified.
type suspendedBallRecoveryCase struct {
	Name          string
	Court         uint8
	BeforeVideo   uint64
	BeforeBall    Ball
	OriginalState WorldActionState
	Sounds        []uint8
}

func suspendedBallCases(t *testing.T) []suspendedBallRecoveryCase {
	t.Helper()
	var rows []suspendedBallRecoveryCase
	readAITeamModeFixture(t, "suspended-ball-recovery", &rows)
	if len(rows) != 6 {
		t.Fatal("missing original trigger/stall sources", len(rows))
	}
	return rows
}

func TestSuspendedBallRecoveryOnlySelectsOriginalRelease(t *testing.T) {
	for _, row := range suspendedBallCases(t) {
		t.Run(row.Name, func(t *testing.T) {
			state := row.OriginalState
			want := state
			want.Ball.Action, want.Ball.Entered = 10, false
			if err := recoverUnsupportedLooseBall(&state, row.Court); err != nil || state != want {
				t.Fatal("repair must only select source release", err, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(want), "state"))
			}
			if err := recoverUnsupportedLooseBall(&state, row.Court); err != nil || state != want {
				t.Fatal("repeated recovery changed selected release", err)
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored WorldActionState
			if err = json.Unmarshal(raw, &restored); err != nil || restored != state {
				t.Fatal("pending release lost on restore", err)
			}
		})
	}
}

func TestSuspendedBallRecoveryRejectsIntentionalOrSupportedStates(t *testing.T) {
	base := suspendedBallCases(t)[0].OriginalState
	cases := []struct {
		name string
		edit func(*WorldActionState)
	}{
		{"player-holder", func(s *WorldActionState) { s.Holder = 0 }},
		{"aux-holder", func(s *WorldActionState) { s.Holder = 4 }},
		{"negative-holder", func(s *WorldActionState) { s.Holder = -1 }},
		{"attached", func(s *WorldActionState) { s.BallLink = 0x84 }},
		{"not-airborne", func(s *WorldActionState) { s.Ball.Airborne = false }},
		{"rising", func(s *WorldActionState) { s.Ball.Body.Velocity.Height = 1 }},
		{"falling", func(s *WorldActionState) { s.Ball.Body.Velocity.Height = -1 }},
		{"gravity", func(s *WorldActionState) { s.Ball.Body.Gravity = 1 }},
		{"negative-gravity", func(s *WorldActionState) { s.Ball.Body.Gravity = -1 }},
		{"request", func(s *WorldActionState) { s.Ball.Request = BallRequest{Pending: true, Action: BallHeld} }},
		{"supported-platform", func(s *WorldActionState) { s.Ball.Body.Position.Depth = 72 * Pixel }},
		{"fraction-above-platform", func(s *WorldActionState) {
			s.Ball.Body.Position.Depth = 72 * Pixel
			s.Ball.Body.Position.Height = 32*Pixel + 255
		}},
		{"ground", func(s *WorldActionState) { s.Ball.Body.Position.Height = 0 }},
		{"fraction-above-ground", func(s *WorldActionState) { s.Ball.Body.Position.Height = 255 }},
	}
	for action := BallAction(0); action < 24; action++ {
		if action == BallRest || action == BallRolling {
			continue
		}
		a := action
		cases = append(cases, struct {
			name string
			edit func(*WorldActionState)
		}{name: "dedicated-action-" + string(rune('A'+a)), edit: func(s *WorldActionState) { s.Ball.Action = a }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := base
			tc.edit(&state)
			want := state
			if err := recoverUnsupportedLooseBall(&state, 0); err != nil || state != want {
				t.Fatal("ineligible ball changed", err)
			}
		})
	}
	for _, court := range []uint8{8, 255} {
		state := base
		if err := recoverUnsupportedLooseBall(&state, court); err == nil || state != base {
			t.Fatal("unknown terrain fabricated or changed state", court, err)
		}
	}
	state := base
	state.Ball.Body.Position.X = -1
	want := state
	if err := recoverUnsupportedLooseBall(&state, 0); err == nil || state != want {
		t.Fatal("out-of-map terrain fabricated", err)
	}
}

func TestSuspendedBallRecoveryNormalNextDispatch(t *testing.T) {
	for _, row := range suspendedBallCases(t) {
		t.Run(row.Name, func(t *testing.T) {
			state := row.OriginalState
			if err := recoverUnsupportedLooseBall(&state, row.Court); err != nil {
				t.Fatal(err)
			}
			// A source-mode call consumes the selected original action, with no
			// recovery hook needed. Its first displacement is -112/256 pixels.
			step := WorldActionStep{State: state}
			ctx := WorldActionContext{CourtID: row.Court}
			court, err := originalWorldMotionCourt(&step, ctx, 6)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = advanceWorldBallStep(&step.State, ctx, court); err != nil {
				t.Fatal(err)
			}
			b := step.State.Ball
			if b.Action != 10 || !b.Entered || b.Body.Gravity != 128 || b.Body.Velocity.Height != -112 || b.Body.Position.Height != state.Ball.Body.Position.Height-112 {
				t.Fatal("not original falling entry", b)
			}
			if len(step.MotionSurfaceWrites) != 1 {
				t.Fatal("fall integrated more than once", step.MotionSurfaceWrites)
			}
		})
	}
}

func TestSuspendedBallRecoveryDispatchPolicy(t *testing.T) {
	for _, row := range suspendedBallCases(t) {
		for _, enabled := range []bool{false, true} {
			state := row.OriginalState
			state.Ball = row.BeforeBall
			result := WorldActionStep{State: state}
			ctx := WorldActionContext{CourtID: row.Court, RecoverUnsupportedBall: enabled}
			court, err := originalWorldMotionCourt(&result, ctx, 6)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = advanceWorldBallStep(&result.State, ctx, court); err != nil {
				t.Fatal(row.Name, enabled, err)
			}
			want := row.OriginalState
			if enabled {
				want.Ball.Action, want.Ball.Entered = 10, false
			}
			if result.State != want {
				t.Fatal(row.Name, enabled, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(want), "state"))
			}
			if len(result.MotionSurfaceWrites) != 1 {
				t.Fatal("recovery added a second motion event", row.Name, enabled, result.MotionSurfaceWrites)
			}
			if result.MotionSurfaceWrites[0].Origin != row.BeforeBall.Body.Position {
				t.Fatal("changed source motion origin")
			}
		}
	}
}
