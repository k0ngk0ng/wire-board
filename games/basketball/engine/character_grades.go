package engine

import "fmt"

type CharacterGrades struct{ Run, Jump, Shot, Recovery, Attack uint8 }

func OriginalCharacterGrades(team, character uint8) (CharacterGrades, error) {
	if team >= 8 || character >= 4 {
		return CharacterGrades{}, fmt.Errorf("unknown original character %d/%d", team, character)
	}
	return originalCharacterGrades[team][character], nil
}

// ApplyOriginalEquipmentGrades restores AD8F's three slot programs, then the
// single final signed clamp. Clamping each item independently changes stacking.
func ApplyOriginalEquipmentGrades(base CharacterGrades, slots [3]int8) (CharacterGrades, error) {
	values := [5]uint8{base.Run, base.Jump, base.Shot, base.Recovery, base.Attack}
	for _, v := range values {
		if v > 7 {
			return base, fmt.Errorf("invalid base character grade")
		}
	}
	targets := [33]uint8{0, 2, 0, 0, 3, 0, 0, 0, 0, 1, 0, 2, 0, 2, 1, 0, 0, 2, 3, 4, 0, 0, 0, 0, 0, 0, 3, 0, 0, 1, 0, 0, 0}
	amounts := [33]uint8{0, 3, 0, 0, 1, 0, 0, 0, 0, 3, 7, 2, 0, 1, 3, 0, 0, 2, 7, 7, 1, 5, 0, 7, 0, 0, 5, 0, 0, 7, 7, 0, 7}
	for _, slot := range slots {
		if slot < 0 {
			continue
		}
		if slot >= 32 {
			return base, fmt.Errorf("unknown original equipment %d", slot)
		}
		index := uint8(slot) + 1
		switch index {
		case 4:
			values[0]--
		case 10, 32:
			values[1], values[2], values[3], values[4] = 7, 7, 7, 7
		case 18:
			values[4] >>= 1
		case 26:
			values[0]--
			values[1]--
			values[2]--
			values[4]--
		}
		values[targets[index]] += amounts[index]
	}
	for i, v := range values {
		if int8(v) < 0 {
			values[i] = 0
		} else if v >= 8 {
			values[i] = 7
		}
	}
	return CharacterGrades{Run: values[0], Jump: values[1], Shot: values[2], Recovery: values[3], Attack: values[4]}, nil
}
