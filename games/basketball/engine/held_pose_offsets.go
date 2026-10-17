package engine

// Original fixed-bank $ECF5 action selectors and $ED65 pointers. The ROM
// SHA-256 is 0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b.
// This is the complete pose-offset table, independent of which player action
// handlers are restored. It does not authorize any unimplemented action.
type heldBallPoseOffset struct {
	X                     int16
	Height, SubtractDepth uint8
}

const (
	heldAtHip            uint8 = 0
	heldInFront          uint8 = 2
	heldRaised           uint8 = 3
	heldOverhead         uint8 = 4
	heldForShot          uint8 = 5
	heldAtFeet           uint8 = 6
	heldForPickup        uint8 = 8
	heldAtChest          uint8 = 9
	heldForDunk          uint8 = 10
	heldWithOverheadPose uint8 = 11
	heldLowFront         uint8 = 12
	heldBehindShoulder   uint8 = 13
	heldBehindFeet       uint8 = 14
)

var originalHeldBallOffsets = [...]heldBallPoseOffset{
	heldAtHip:            {X: -4, Height: 7, SubtractDepth: 1},    // $ED85
	heldInFront:          {X: 6, Height: 8, SubtractDepth: 1},     // $ED8D
	heldRaised:           {X: 0, Height: 32, SubtractDepth: 1},    // $ED91
	heldOverhead:         {X: 0, Height: 30, SubtractDepth: 1},    // $ED95
	heldForShot:          {X: 2, Height: 30, SubtractDepth: 1},    // $ED99
	heldAtFeet:           {X: 0, Height: 0, SubtractDepth: 1},     // $ED9D
	heldForPickup:        {X: 0, Height: 8, SubtractDepth: 1},     // $EDA5
	heldAtChest:          {X: 0, Height: 16, SubtractDepth: 1},    // $EDA9
	heldForDunk:          {X: -4, Height: 18, SubtractDepth: 255}, // $EDAD
	heldWithOverheadPose: {X: 4, Height: 6, SubtractDepth: 1},     // $EDB1
	heldLowFront:         {X: 6, Height: 4, SubtractDepth: 1},     // $EDB5
	heldBehindShoulder:   {X: -9, Height: 22, SubtractDepth: 1},   // $EDB9
	heldBehindFeet:       {X: -12, Height: 0, SubtractDepth: 0},   // $EDBD
}

var originalHeldBallActionOffsets = [...]uint8{
	heldAtHip, heldAtHip, heldAtHip, heldAtHip, heldInFront, heldInFront, heldAtFeet, heldOverhead, heldOverhead, heldForShot, heldForShot, heldForShot, heldForShot, heldInFront, heldRaised, heldInFront, // actions $00..$0F
	heldAtHip, heldAtHip, heldAtHip, heldAtHip, heldInFront, heldInFront, heldAtFeet, heldOverhead, heldOverhead, heldForShot, heldForShot, heldForShot, heldForShot, heldInFront, heldInFront, heldInFront, // actions $10..$1F
	heldAtHip, heldAtHip, heldAtHip, heldAtHip, heldInFront, heldInFront, heldAtFeet, heldOverhead, heldOverhead, heldForShot, heldForShot, heldForShot, heldForShot, heldInFront, heldInFront, heldInFront, // actions $20..$2F
	heldLowFront, heldLowFront, heldLowFront, heldLowFront, heldLowFront, heldLowFront, heldLowFront, heldOverhead, heldOverhead, heldForShot, heldForShot, heldForShot, heldForShot, heldInFront, heldInFront, heldInFront, // actions $30..$3F
	heldInFront, heldInFront, heldInFront, heldInFront, heldInFront, heldInFront, heldAtFeet, heldLowFront, heldOverhead, heldForShot, heldForShot, heldForShot, heldForShot, heldInFront, heldInFront, heldAtFeet, // actions $40..$4F
	heldAtChest, heldAtChest, heldAtChest, heldAtChest, heldAtChest, heldAtChest, heldAtChest, heldForPickup, heldForDunk, heldForDunk, heldLowFront, heldBehindShoulder, heldWithOverheadPose, heldInFront, heldInFront, heldBehindFeet, // actions $50..$5F
	heldAtHip, heldAtHip, heldAtHip, heldAtHip, heldInFront, heldInFront, heldAtFeet, heldOverhead, heldOverhead, heldForShot, // actions $60..$69
}
