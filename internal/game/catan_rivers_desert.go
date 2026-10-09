package game

// 2025 Rivers + Seafarers pp. 3–4: left diagram crosses the former
// desert belt; right diagram retains it. Ports retain ordinary positions.
func riverDesertRecipe(n int, layout string) (map[int]seaTerrain, [][]int, []int, int) {
	if n == 3 {
		if layout == "rivers-across" {
			return map[int]seaTerrain{1: {catanSwamp, 0}, 6: {1, 4}, 10: {catanSwamp, 0}, 11: {0, 5}, 12: {4, 6}, 16: {2, 3}, 22: {1, 2}, 26: {0, 10}, 28: {4, 8}},
				[][]int{{28, 22, 16, 10}, {12, 6, 1}}, []int{1, 3}, -1
		}
		return map[int]seaTerrain{0: {4, 4}, 4: {1, 3}, 6: {catanSwamp, 0}, 9: {catanSwamp, 0}, 11: {2, 5}, 12: {3, 6}, 16: {1, 3}, 21: {4, 6}, 22: {2, 12}, 26: {4, 10}, 28: {1, 8}},
			[][]int{{21, 16, 11, 6}, {0, 4, 9}}, []int{4, 1}, 22
	}
	if layout == "rivers-across" {
		return map[int]seaTerrain{1: {catanSwamp, 0}, 12: {catanSwamp, 0}, 13: {1, 8}, 14: {4, 10}, 19: {2, 10}, 20: {3, 11}, 24: {3, 12}, 26: {1, 5}, 31: {0, 3}},
			[][]int{{33, 26, 19, 12}, {14, 7, 1}}, []int{1, 3}, -1
	}
	return map[int]seaTerrain{0: {4, 10}, 5: {1, 11}, 8: {catanSwamp, 0}, 11: {catanSwamp, 0}, 14: {2, 10}, 20: {1, 11}, 21: {3, 9}, 24: {4, 12}, 26: {4, 5}, 33: {3, 4}},
		[][]int{{26, 20, 14, 8}, {0, 5, 11}}, []int{4, 1}, 24
}
