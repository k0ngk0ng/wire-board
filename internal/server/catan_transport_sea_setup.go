package server

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func transportSeaRoomSetup(r *Room) game.CatanTransportSeaSetup {
	scene := "shores"
	if r.CatanScenario == "transport-desert" {
		scene = "desert"
	}
	setup := game.CatanTransportSeaSetup{Scenario: scene}
	if r.CatanTransportSea != nil {
		setup = *r.CatanTransportSea
	}
	setup, _ = game.NormalizeCatanTransportSeaSetup(setup)
	return setup
}

func (r *Room) setCatanTransportSea(request game.CatanTransportSeaSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" || !publicCatanTransportSea(r.CatanScenario) {
		return errors.New("只能在运输海图等待房间选择布局")
	}
	setup, err := game.NormalizeCatanTransportSeaSetup(request)
	if err != nil {
		return err
	}
	expected := transportSeaRoomSetup(r).Scenario
	if setup.Scenario != expected {
		return errors.New("运输海图布局与所选剧本不一致")
	}
	if r.CatanTransportSea != nil && *r.CatanTransportSea == setup {
		return nil
	}
	r.CatanTransportSea = &setup
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

func (r *Room) validateCatanTransportSeaSetup() error {
	if r.CatanTransportSea == nil {
		return nil
	}
	if r.Kind != "catan" || !publicCatanTransportSea(r.CatanScenario) {
		return errors.New("运输海图设置与剧本不匹配")
	}
	setup, err := game.NormalizeCatanTransportSeaSetup(*r.CatanTransportSea)
	scene := "shores"
	if r.CatanScenario == "transport-desert" {
		scene = "desert"
	}
	if err != nil || setup != *r.CatanTransportSea || setup.Scenario != scene {
		return errors.New("运输海图配置无效")
	}
	return nil
}
