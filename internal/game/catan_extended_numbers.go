package game

// Explicit site recipe, not a claim about the 2025 physical letter backs.
// Keep the existing extended-board sequence, printed face inventory and
// counterclockwise placement. Older saves omit the tag but use this sequence.
const CatanExtendedNumberRecipe = "wire-board-extended-numbers-v1"
const catanExtendedNumberNotice = "本站数字配置：五六人沿逆时针螺旋使用本站固定数列，跳过沼泽或水源；数字数量不变，不宣称对应2025实体字母背面"

func catanExtendedNumberRecipe() (order, numbers []int) {
	return []int{2, 1, 0, 3, 7, 12, 18, 23, 27, 28, 29, 26, 22, 17, 11, 6, 5, 4, 8, 13, 19, 24, 25, 21, 16, 10, 9, 14, 20, 15},
		[]int{2, 5, 4, 6, 3, 9, 8, 11, 11, 10, 6, 3, 8, 4, 8, 10, 11, 12, 10, 5, 4, 9, 5, 9, 12, 3, 2, 6}
}

func validCatanExtendedNumberRecipe(recipe string, players int) bool {
	return recipe == "" || (players > 4 && recipe == CatanExtendedNumberRecipe)
}
