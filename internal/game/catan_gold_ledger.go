package game

import "errors"

// Safe persisted integer bound, not the number of physical coins in a box.
const catanGoldLedgerLimit = 1_000_000_000

func catanGoldShortfall(bank, issued, amount int) (int, error) {
	if amount < 0 || amount > catanGoldLedgerLimit || issued < 0 || issued > catanGoldLedgerLimit || bank < 0 {
		return 0, errors.New("金币记账数量无效")
	}
	missing := max(0, amount-bank)
	if missing > catanGoldLedgerLimit-issued {
		return 0, errors.New("金币记账超出安全范围")
	}
	return missing, nil
}

func (a *catanAttack) ensureGold(amount int) error {
	missing, err := catanGoldShortfall(a.GoldBank, a.GoldIssued, amount)
	if err != nil {
		return err
	}
	a.GoldIssued += missing
	a.GoldBank += missing
	return nil
}

// Site supplement, approved for Rivers: reuse returned coins first and issue
// only the missing amount. Physical coins do not limit earned rewards.
func (r *CatanRivers) ensureGold(amount int) error {
	missing, err := catanGoldShortfall(r.Bank, r.GoldIssued, amount)
	if err != nil {
		return err
	}
	r.GoldIssued += missing
	r.Bank += missing
	return nil
}
