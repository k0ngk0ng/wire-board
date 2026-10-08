package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) publicCatanEventsAvailable() bool {
	if r.Kind != "catan" || r.CatanFishing && !publicCatanFishingSea(r.CatanScenario) {
		return false
	}
	scenario := r.CatanScenario
	if r.CatanSeafarers != nil {
		scenario = r.CatanSeafarers.Scenario
	}
	if r.CatanNewWorldMap != nil && scenario != "new_world" {
		return false
	}
	switch scenario {
	case "", "fishing", "cities-knights", "rivers", "caravans", "barbarian-attack", "shores", "islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world":
		return true
	}
	return false
}
func (r *Room) validateCatanEvents() error {
	if r.CatanEvents == "" {
		return nil
	}
	if r.CatanEvents != game.CatanEventCatalogue {
		return errors.New("未知事件牌组版本")
	}
	if !r.publicCatanEventsAvailable() {
		return errors.New("该组合尚未接通事件牌；请先关闭事件牌选项")
	}
	return nil
}
func (r *Room) setCatanEvents(enabled bool) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return errors.New("只能在卡坦开局前切换事件牌")
	}
	next := *r
	next.CatanEvents = ""
	if enabled {
		next.CatanEvents = game.CatanEventCatalogue
	}
	if err := next.validateCatanEvents(); err != nil {
		return err
	}
	if r.CatanEvents == next.CatanEvents {
		return nil
	}
	r.CatanEvents = next.CatanEvents
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
