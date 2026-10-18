package campaign

import "fmt"

// Fixed FF99 pointer tables, in opponent order with home/away pairs. Keeping
// ordinary semantic data here makes later gameplay modifications independent
// of the ROM. No source instructions or random generator run in this package.
var rewardTables = [4][14]Item{
	{0, 1, 2, 11, 18, 18, 16, 23, 14, 4, 27, 3, 13, 8},
	{29, 30, 7, 23, 19, 22, 17, 4, 11, 19, 12, 3, 5, 8},
	{7, 28, 11, 31, 0, 6, 3, 15, 5, 31, 27, 26, 30, 14},
	{10, 30, 16, 26, 7, 15, 8, 18, 30, 25, 27, 20, 13, 9},
}

// RewardItem returns BD8E's table lookup, without awarding it or changing
// progress. The match driver decides whether the reward phase runs: the final
// Period28 outcome goes to the ending instead, despite the lookup being valid.
func RewardItem(rewardSet, opponent, postMatchPeriod uint8) (Item, error) {
	if rewardSet >= 4 || opponent < 1 || opponent > 7 || postMatchPeriod < 2 || postMatchPeriod > 28 || postMatchPeriod&1 != 0 {
		return Empty, fmt.Errorf("reward lookup outside original campaign domain")
	}
	index := (opponent-1)*2 + ((postMatchPeriod >> 1) & 1)
	return rewardTables[rewardSet][index], nil
}
