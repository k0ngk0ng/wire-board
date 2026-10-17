package engine

import "fmt"

// Contact boxes use integer world pixels, separate from rendering coordinates
// and the 1/256-pixel motion accumulator. Original overlap bounds are inclusive.
type ContactBoxSpec struct {
	Enabled              bool
	MinX, MaxX           int16
	MinHeight, MaxHeight int16
}

type ContactShape struct{ Attack, Body ContactBoxSpec }
type ContactProfile struct {
	Shape                     ContactShape
	AttackFlags, ReceiveFlags uint8
}
type ContactProfileKey struct{ Kind, Action, Variant, Frame uint8 }

func OriginalContactProfile(key ContactProfileKey) (ContactProfile, error) {
	key.Action &= 127
	// All eight player kinds share the original action geometry and flags;
	// attack geometry is selected by the character-specific Variant instead.
	if key.Kind < 8 {
		key.Kind = 0
	}
	if key.Action < 96 {
		key.Variant = 0
	}
	if key.Kind == 14 && key.Action == 3 {
		return ContactProfile{}, fmt.Errorf("original kind14 action3 contact geometry requires current semantic context: %+v", key)
	}
	if key.Action < 96 && (key.Kind == 0 || key.Kind == 8) {
		groups, flags := originalPlayerContactGroups[:], originalPlayerContactFlags[:]
		if key.Kind == 8 {
			groups, flags = originalBallContactGroups[:], originalBallContactFlags[:]
		}
		if int(key.Action) < len(groups) {
			frames := originalNormalContactShapes[groups[key.Action]]
			if int(key.Frame) < len(frames) {
				return ContactProfile{Shape: frames[key.Frame], AttackFlags: flags[key.Action], ReceiveFlags: flags[key.Action]}, nil
			}
		}
		return ContactProfile{}, fmt.Errorf("original normal contact profile outside source table: %+v", key)
	}
	if key.Kind == 0 && key.Action >= 96 && key.Action < 106 {
		frames, ok := originalAttackContactShapes[key.Variant]
		if !ok || int(key.Frame) >= len(frames) {
			return ContactProfile{}, fmt.Errorf("original attack contact profile outside restored animation: %+v", key)
		}
		flags := originalAttackContactFlags[key.Action-96]
		return ContactProfile{Shape: frames[key.Frame], AttackFlags: flags, ReceiveFlags: flags}, nil
	}
	profile, ok := originalContactProfiles[key]
	if !ok {
		return ContactProfile{}, fmt.Errorf("original contact profile not recovered: %+v", key)
	}
	return profile, nil
}

type ContactObject struct {
	Position                      Position
	Action                        uint8
	Entered, Mirrored, Suppressed bool
	Owner                         int8
	Profile                       ContactProfile
}

type ContactSearch struct {
	Candidates [7]int8
	Rotation   uint8
}

type contactBounds struct{ minX, maxX, minHeight, maxHeight uint16 }

func boundsForContact(position Position, mirrored bool, spec ContactBoxSpec) contactBounds {
	x, height := int32(position.X>>8), int32(position.Height>>8)
	left, right := x+int32(spec.MinX), x+int32(spec.MaxX)
	if mirrored {
		left, right = x-int32(spec.MaxX), x-int32(spec.MinX)
	}
	return contactBounds{minX: uint16(left), maxX: uint16(right), minHeight: uint16(height + int32(spec.MinHeight)), maxHeight: uint16(height + int32(spec.MaxHeight))}
}

func contactOverlaps(strike, body contactBounds) bool {
	return strike.minHeight&0x8000 == 0 && strike.maxX >= body.minX && body.maxX >= strike.minX && body.maxHeight >= strike.minHeight && strike.maxHeight >= body.minHeight
}

// FindObjectContacts restores the original source loop and rotating player
// priority. It runs before damage/interaction handling; catch eligibility must
// later use the updated actors, not blindly accept every candidate found here.
func FindObjectContacts(objects [7]ContactObject, rotation uint8) ContactSearch {
	result := ContactSearch{Rotation: rotation}
	for i := range result.Candidates {
		result.Candidates[i] = -1
	}
	groups := [7]uint8{0x41, 0x41, 0x82, 0x82, 0xc8, 0xd0, 4}
	for sourceIndex, source := range objects {
		threshold := uint8(1)
		if sourceIndex < 4 {
			threshold = 96
		}
		if source.Suppressed || !source.Entered || source.Action < threshold || source.Profile.AttackFlags&32 == 0 || !source.Profile.Shape.Attack.Enabled {
			continue
		}
		strike := boundsForContact(source.Position, source.Mirrored, source.Profile.Shape.Attack)
		depth := uint8(source.Position.Depth >> 8)
		lower, upper := depth-7, depth+8
		mask := uint8(8)
		if sourceIndex == 6 {
			mask = 16
		}
		for ordinal := 0; ordinal < 7; ordinal++ {
			targetIndex := ordinal
			if ordinal < 4 {
				result.Rotation &= 3
				targetIndex = int(result.Rotation)
				result.Rotation++
				if !objects[targetIndex].Entered {
					continue
				}
			}
			target := objects[targetIndex]
			if sourceIndex >= 4 && source.Owner == int8(targetIndex) {
				continue
			}
			if sourceIndex == 6 && targetIndex >= 4 && source.Profile.AttackFlags&2 == 0 {
				continue
			}
			if groups[sourceIndex]&groups[targetIndex]&31 != 0 || target.Profile.ReceiveFlags&mask == 0 {
				continue
			}
			z := uint8(target.Position.Depth >> 8)
			if z >= upper || z < lower || !target.Profile.Shape.Body.Enabled {
				continue
			}
			body := boundsForContact(target.Position, target.Mirrored, target.Profile.Shape.Body)
			if !contactOverlaps(strike, body) {
				continue
			}
			result.Candidates[sourceIndex] = int8(targetIndex)
			break
		}
	}
	return result
}
