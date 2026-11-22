package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/internal/game"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) configureRoomSettings(r *Room, capacity int, options json.RawMessage, password *string) error {
	if capacity < len(r.Seats) {
		return errors.New("房间人数不能少于已入座人数，请先移除电脑或等待玩家离开")
	}
	changed := capacity != r.Capacity
	if module, ok := s.modules[r.Kind]; ok {
		if capacity < module.MinPlayers || capacity > module.MaxPlayers {
			return errors.New("人数不符合游戏要求")
		}
		if len(options) == 0 {
			options = r.Options
		}
		configured, err := module.Configure(capacity, options)
		if err != nil {
			return err
		}
		if !json.Valid(configured) || bytes.Equal(bytes.TrimSpace(configured), []byte("null")) {
			return errors.New("无效游戏设置")
		}
		changed = changed || !bytes.Equal(configured, r.Options)
		r.Options = configured
	} else {
		minimum, maximum := 2, 4
		switch r.Kind {
		case "carcassonne":
			maximum = 5
		case "sanguosha":
			minimum, maximum = 4, 8
			if r.SanguoshaOptions.Mode == "3v3" {
				minimum, maximum = 6, 6
			}
		case "rail":
			info, ok := game.RailMapInfo(r.RailMap)
			if !ok {
				return errors.New("未知铁路地图")
			}
			minimum, maximum = info.MinPlayers, info.MaxPlayers
		case "catan":
			maximum = 6
		}
		if capacity < minimum || capacity > maximum {
			return fmt.Errorf("当前规则支持 %d～%d 人", minimum, maximum)
		}
		if changed && r.Kind == "catan" {
			if err := r.resizeCatanRoom(capacity); err != nil {
				return err
			}
		}
	}
	r.Capacity = capacity
	if password != nil {
		if len(*password) > 72 {
			return errors.New("房间密码最多 72 字节")
		}
		r.Password = ""
		if *password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
			if err != nil {
				return errors.New("无法设置房间密码")
			}
			r.Password = string(hash)
		}
	}
	if changed {
		for index := range r.Seats {
			r.Seats[index].Ready = r.Seats[index].Bot
		}
	}
	return nil
}

func (r *Room) resizeCatanRoom(capacity int) error {
	scenario := r.CatanScenario
	if r.CatanTwoRules != "" {
		scenario = r.CatanTwoScenario
	}
	r.Capacity = capacity
	r.CatanOptions.FiveSix = capacity > 4 && !publicCatanFlexibleScenario(scenario)
	var err error
	r.CatanOptions, err = game.NormalizeCatanOptions(r.CatanOptions)
	if err != nil {
		return err
	}
	if capacity == 2 && !publicCatanFlexibleScenario(scenario) {
		r.CatanBaseConfiguration = nil
		r.CatanScenario = ""
		r.CatanSeafarers, r.CatanNewWorldMap = nil, nil
		if scenario == "cities-knights" {
			r.CatanCitiesKnights = nil
		}
		return r.setCatanTwoScenario(scenario)
	}
	r.CatanTwoRules, r.CatanTwoScenario = "", ""
	r.CatanScenario = scenario
	if r.CatanBaseConfiguration != nil {
		setup := *r.CatanBaseConfiguration
		setup.Rules = ""
		if capacity <= 4 {
			setup.Layout = "variable"
		}
		normal, err := game.NormalizeCatanBaseConfiguration(capacity, setup)
		if err != nil {
			return err
		}
		r.CatanBaseConfiguration = &normal
	}
	if publicCatanSeaScenario(scenario) {
		setup, err := game.NormalizeCatanSeafarersSetup(capacity, game.CatanSeafarersSetup{Scenario: scenario})
		if err != nil {
			return err
		}
		r.CatanSeafarers = &setup
	}
	if scenario == "cities-knights" && r.CatanCitiesKnights == nil {
		r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{}
	}
	if r.CatanCitiesKnights != nil {
		setup := *r.CatanCitiesKnights
		setup.Rules = ""
		normal, err := r.normalizeCatanCombinationKnights(setup)
		if err != nil {
			return err
		}
		r.CatanCitiesKnights = &normal
	}
	if r.CatanNewWorldMap != nil {
		world, err := r.generateCatanWorldMap()
		if err != nil {
			return err
		}
		r.CatanNewWorldMap = world
	}
	return r.validateCatanScenario()
}
