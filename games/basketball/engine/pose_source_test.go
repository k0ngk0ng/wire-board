package engine

import "testing"

func TestWorldBallPoseSourceWrites(t *testing.T) {
	for _, tc := range []struct {
		name    string
		action  BallAction
		entered bool
		pose    uint8
		want    uint8
	}{
		{"dribble entry", BallDribble, false, 0, 2},
		{"dribble continuing", BallDribble, true, 0, 2},
		{"held entry", BallHeld, false, 13, 2},
		{"held continuing", BallHeld, true, 13, 2},
		{"held overhead clears temporary flag", BallHeld, true, 30, 2},
		{"pass launch keeps passer", BallPass, false, 13, 2},
		{"pass flight retains old source", BallPass, true, 13, 0x85},
		{"free flight retains old source", BallBounce, true, 0, 0x85},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := WorldActionState{PoseSource: 0x85, Holder: 2}
			s.Actors[2].Player = Player{Action: ActionCatchBall, Pose: Pose{Frame: tc.pose}, Body: Body{Position: Position{X: 220 * Pixel, Depth: 40 * Pixel, Height: 40 * Pixel}}}
			s.Actors[3].Player.Body.Position = Position{X: 300 * Pixel, Depth: 40 * Pixel}
			s.Ball = Ball{Action: tc.action, Entered: tc.entered, Airborne: true, Body: Body{Position: Position{X: 210 * Pixel, Depth: 40 * Pixel, Height: 48 * Pixel}}}
			s.LaunchTarget = s.Actors[3].Player.Body.Position
			if err := advanceWorldBall(&s, WorldActionContext{CourtID: 1}); err != nil {
				t.Fatal(err)
			}
			if s.PoseSource != tc.want {
				t.Fatalf("pose source got %#x want %#x", s.PoseSource, tc.want)
			}
		})
	}
}
