package engine

import "fmt"

// ContactGeometryContext supplies the original shape selector's live semantic
// sources. No byte-addressed RAM or persistent copy of world state is stored.
type ContactGeometryContext struct {
	PhysicalControllers *[4]ControllerFrame
	Positions           *[13]Position
}

// OriginalContactProfileWithContext also handles Kind14/action3: original
// $8318's group30 pointer resolves to $0080 or $0000 for cursors0..3.
// The four-controller sample is before seat mapping; even unused ports matter.
func OriginalContactProfileWithContext(key ContactProfileKey, ctx ContactGeometryContext) (ContactProfile, error) {
	if key.Kind != 14 || key.Action&127 != 3 {
		return OriginalContactProfile(key)
	}
	var raw [8]uint8
	switch key.Frame {
	case 0, 2:
		if ctx.Positions == nil {
			return ContactProfile{}, fmt.Errorf("kind14 action3 cursor%d requires current world positions", key.Frame)
		}
		raw[0], raw[1] = uint8(ctx.Positions[11].X>>8), uint8(ctx.Positions[12].X>>8)
		for i := 0; i < 6; i++ {
			raw[i+2] = uint8(ctx.Positions[i].X >> 16)
		}
	case 1, 3:
		if ctx.PhysicalControllers == nil {
			return ContactProfile{}, fmt.Errorf("kind14 action3 cursor%d requires physical controller sample", key.Frame)
		}
		for i, frame := range ctx.PhysicalControllers {
			raw[i], raw[i+4] = uint8(frame.Held), uint8(frame.Pressed)
		}
	default:
		return ContactProfile{}, fmt.Errorf("kind14 action3 cursor%d has no reconstructed geometry source", key.Frame)
	}
	box := func(i int, attack bool) ContactBoxSpec {
		minimum := int16(raw[i])
		if attack {
			minimum = int16(int8(raw[i]))
		}
		return ContactBoxSpec{Enabled: raw[i] != 128, MinX: int16(int8(raw[i+3])), MaxX: int16(int8(raw[i+2])), MinHeight: minimum, MaxHeight: int16(raw[i+1])}
	}
	return ContactProfile{Shape: ContactShape{Attack: box(0, true), Body: box(4, false)}, AttackFlags: 0x28, ReceiveFlags: 0x28}, nil
}
