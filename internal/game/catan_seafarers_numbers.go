package game

import "errors"

const catanSeaNumberNotice = "本站数字配置：五六人新海岸主岛沿逆时针螺旋使用本站28枚固定数列，跳过沙漠；外围岛屿不变，不宣称对应2025实体字母背面"

func (s *State) validateCatanSeaNumberRecipe() error {
	if s.Catan == nil || s.Catan.Seafarers == nil || s.Catan.Seafarers.NumberRecipe == "" {
		return nil
	}
	g, sea := s.Catan, s.Catan.Seafarers
	if sea.NumberRecipe != CatanExtendedNumberRecipe || len(g.Players) < 5 || len(g.Players) > 6 || !g.Options.FiveSix || sea.Scenario != "shores" || len(g.Tiles) != 56 {
		return errors.New("新海岸数字配置版本或人数地图不一致")
	}
	return nil
}
