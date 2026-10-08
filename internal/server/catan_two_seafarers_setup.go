package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) twoCatanSeafarers() bool {
	return r.Kind == "catan" && r.Capacity == 2 && r.CatanTwoRules == game.CatanTwoRules && game.CatanTwoSeafarersScenario(r.CatanTwoScenario)
}
func (r *Room) setCatanTwoSeafarers(request game.CatanSeafarersSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" || r.Capacity != 2 || r.CatanTwoRules != game.CatanTwoRules {
		return errors.New("只能在双人等待房间选择航海家地图")
	}
	setup, err := game.NormalizeCatanTwoSeafarersSetup(request)
	if err != nil {
		return err
	}
	if r.CatanSeafarers != nil && *r.CatanSeafarers == setup && r.CatanTwoScenario == setup.Scenario {
		return nil
	}
	next := *r
	next.CatanTwoScenario = setup.Scenario
	next.CatanSeafarers = &setup
	next.CatanNewWorldMap = nil
	if setup.Scenario == "new_world" {
		next.CatanNewWorldMap, err = game.GenerateCatanNewWorldMap(4)
		if err != nil {
			return err
		}
	}
	if err = next.validateCatanTwoSetup(); err != nil {
		return err
	}
	r.CatanTwoScenario, r.CatanSeafarers, r.CatanNewWorldMap = next.CatanTwoScenario, next.CatanSeafarers, next.CatanNewWorldMap
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
