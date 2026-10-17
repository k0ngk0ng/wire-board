package engine

// Original team/character parameters from fixed C1D8, decoded by
// scripts/basketball/extract_character_grades.py. Runtime needs no ROM.
var originalCharacterGrades = [8][4]CharacterGrades{
	{{Run: 5, Jump: 3, Shot: 6, Recovery: 4, Attack: 3}, {Run: 5, Jump: 3, Shot: 3, Recovery: 4, Attack: 5}, {Run: 5, Jump: 1, Shot: 1, Recovery: 3, Attack: 6}, {Run: 5, Jump: 2, Shot: 0, Recovery: 3, Attack: 2}},
	{{Run: 6, Jump: 6, Shot: 2, Recovery: 4, Attack: 3}, {Run: 2, Jump: 5, Shot: 1, Recovery: 5, Attack: 3}, {Run: 5, Jump: 5, Shot: 1, Recovery: 7, Attack: 0}, {Run: 7, Jump: 6, Shot: 0, Recovery: 1, Attack: 0}},
	{{Run: 2, Jump: 3, Shot: 6, Recovery: 4, Attack: 7}, {Run: 1, Jump: 4, Shot: 3, Recovery: 3, Attack: 5}, {Run: 0, Jump: 1, Shot: 7, Recovery: 1, Attack: 3}, {Run: 4, Jump: 0, Shot: 7, Recovery: 7, Attack: 0}},
	{{Run: 5, Jump: 3, Shot: 6, Recovery: 7, Attack: 2}, {Run: 6, Jump: 1, Shot: 1, Recovery: 6, Attack: 6}, {Run: 7, Jump: 4, Shot: 7, Recovery: 1, Attack: 2}, {Run: 7, Jump: 0, Shot: 7, Recovery: 0, Attack: 3}},
	{{Run: 4, Jump: 5, Shot: 7, Recovery: 6, Attack: 4}, {Run: 6, Jump: 1, Shot: 1, Recovery: 3, Attack: 2}, {Run: 0, Jump: 3, Shot: 6, Recovery: 6, Attack: 3}, {Run: 2, Jump: 2, Shot: 2, Recovery: 5, Attack: 2}},
	{{Run: 4, Jump: 6, Shot: 6, Recovery: 3, Attack: 6}, {Run: 3, Jump: 5, Shot: 5, Recovery: 2, Attack: 2}, {Run: 7, Jump: 3, Shot: 1, Recovery: 5, Attack: 4}, {Run: 6, Jump: 2, Shot: 7, Recovery: 1, Attack: 0}},
	{{Run: 5, Jump: 6, Shot: 7, Recovery: 2, Attack: 1}, {Run: 4, Jump: 3, Shot: 0, Recovery: 3, Attack: 0}, {Run: 3, Jump: 4, Shot: 4, Recovery: 0, Attack: 3}, {Run: 5, Jump: 0, Shot: 6, Recovery: 1, Attack: 1}},
	{{Run: 3, Jump: 2, Shot: 5, Recovery: 4, Attack: 7}, {Run: 3, Jump: 1, Shot: 4, Recovery: 1, Attack: 5}, {Run: 0, Jump: 2, Shot: 3, Recovery: 2, Attack: 6}, {Run: 1, Jump: 0, Shot: 5, Recovery: 6, Attack: 6}},
}
