package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalCharacterGradeTables(t *testing.T) {
	data, e := os.ReadFile("testdata/character-grades.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Team, Character uint8
		Grades          CharacterGrades
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 32 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		got, e := OriginalCharacterGrades(r.Team, r.Character)
		if e != nil || got != r.Grades {
			t.Fatal(r, got, e)
		}
	}
	if _, e = OriginalCharacterGrades(8, 0); e == nil {
		t.Fatal("unknown team accepted")
	}
}
func TestOriginalEquipmentGradeProgram(t *testing.T) {
	base := CharacterGrades{Run: 5, Jump: 3, Shot: 6, Recovery: 4, Attack: 3}
	for _, tc := range []struct {
		slots [3]int8
		want  CharacterGrades
	}{
		{[3]int8{-1, -1, -1}, base},
		{[3]int8{0, -1, -1}, CharacterGrades{Run: 5, Jump: 3, Shot: 7, Recovery: 4, Attack: 3}},
		{[3]int8{3, -1, -1}, CharacterGrades{Run: 4, Jump: 3, Shot: 6, Recovery: 5, Attack: 3}},
		{[3]int8{17, -1, -1}, CharacterGrades{Run: 5, Jump: 3, Shot: 6, Recovery: 7, Attack: 1}},
		{[3]int8{25, -1, -1}, CharacterGrades{Run: 4, Jump: 2, Shot: 5, Recovery: 7, Attack: 2}},
		{[3]int8{9, -1, -1}, CharacterGrades{Run: 7, Jump: 7, Shot: 7, Recovery: 7, Attack: 7}},
		{[3]int8{22, 3, 3}, CharacterGrades{Run: 7, Jump: 3, Shot: 6, Recovery: 6, Attack: 3}},
	} {
		got, e := ApplyOriginalEquipmentGrades(base, tc.slots)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	if _, e := ApplyOriginalEquipmentGrades(base, [3]int8{32, -1, -1}); e == nil {
		t.Fatal("unknown equipment accepted")
	}
}
