package engine

import "errors"

type ChuThrowSubmissionState struct {
	Actor    GroundActionState
	ObjectID uint16
	X, Y     int32
	Held     uint8
	Buffer   ThrowContactBuffer
}

// SubmitChuThrow recovers the selected branch at 0x17ec2 and its tail-called
// action constructor at 0x18854. The selector owns the button/direction gate.
// The original branch has no capacity clamp. This API requires the caller to
// provide the addressed semantic slot and never invents a fixed buffer limit.
func SubmitChuThrow(state ChuThrowSubmissionState) (ChuThrowSubmissionState, error) {
	index := int(state.Buffer.Offset / 32)
	if state.Buffer.Offset&31 != 0 || index >= len(state.Buffer.Records) {
		return state, errors.New("throw submission storage is unavailable")
	}
	state.Buffer.Records = append([]ThrowContactRecord(nil), state.Buffer.Records...)
	state.Buffer.Records[index] = ThrowContactRecord{
		X: int16(state.X >> 16), Y: int16(state.Y>>16) - 0x28,
		Width: 0x18, Height: 0x28, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID,
		OffsetX: 0x18, OffsetY: 0,
	}
	state.Buffer.Offset += 32
	state.Actor.ActionKind, state.Actor.Pose, state.Actor.EnabledStages = 3, 0x83, 0
	if state.Held&8 != 0 {
		state.Actor.Facing = 0
	} else if state.Held&4 != 0 {
		state.Actor.Facing = 1
	}
	state.Actor = ResetActionState(state.Actor)
	return state, nil
}
