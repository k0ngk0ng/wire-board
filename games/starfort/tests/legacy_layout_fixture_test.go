package starfort_test

// Historical schema-2 maps were captured before the central-map change. These
// test-only archived starts keep legacy expansion/migration coverage explicit;
// no production Configure field can select the old layout.
import (
	_ "embed"
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
)

//go:embed testdata/legacy-initial-maps.json
var legacyInitialMaps []byte

// Central layouts captured from v1.38 before restoring cooperative expansion.
//
//go:embed testdata/central-initial-maps.json
var centralInitialMaps []byte

func legacyStartMatch(m gameapi.Match) (gameapi.Match, error) {
	var s game.State
	if e := json.Unmarshal(m.Data, &s); e != nil {
		return m, e
	}
	var archive struct {
		Maps map[int]game.Map `json:"maps"`
	}
	if e := json.Unmarshal(legacyInitialMaps, &archive); e != nil {
		return m, e
	}
	s.BalanceRules = 0
	s.RevisionRules = 0
	s.GeologyRules = 0
	s.Geology = nil
	s.GeoRNG = 0
	s.WeatherRules = 0
	s.World.Event = game.WaveEvent{ID: "calm", Label: "晴空守望", Wave: s.Wave}
	s.GrowthRules = 0
	s.World.Objectives = nil
	s.World.TaskOffersWave = 0
	s.World.TaskAttempts = nil
	s.World.Layout = ""
	s.SkillRules = 0
	s.SpecializationRules = 0
	s.SkillRNG = 0
	s.IceWalls = nil
	s.IceWallSerial = 0
	s.World.Map = archive.Maps[int(s.World.Seed%15)]
	s.World.Objectives = []game.Objective{}
	s.Craters = nil
	s.CraterSerial = 0
	return s.Envelope()
}

// Explicit schema-7 starts keep the previous task/growth suite independent of
// the new schema-8 lottery. Later preparation uses the saved production rules.
func growthTestModule() gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, err := start(ctx, raw)
		if err != nil {
			return v, err
		}
		var s game.State
		if err := json.Unmarshal(v.Data, &s); err != nil {
			return v, err
		}
		s.RevisionRules = 0
		s.GeologyRules = 0
		s.Geology = nil
		s.GeoRNG = 0
		s.WeatherRules = 0
		s.World.Event = game.WaveEvent{ID: "calm", Label: "晴空守望", Wave: s.Wave}
		r := s.World.Map.Routes[0]
		x, y := fixturePathPosition(r.Points, fixturePathLength(r.Points)*.75)
		s.World.Objectives = []game.Objective{{ID: "task-1-mine", Kind: "mine", Label: "历史矿场任务", HP: 10, MaxHP: 10, Route: r.ID, Progress: fixturePathLength(r.Points) * .75, Sector: 1, X: x, Y: y, Cost: 100, Reward: 120, Status: "available", Owner: -1}}
		return s.Envelope()
	}
	return m
}
func legacyTestModule() gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, options)
		if e != nil {
			return v, e
		}
		return legacyStartMatch(v)
	}
	return m
}
func newLegacyRulesTable(t *testing.T, n int, options map[string]any) *rulesTable {
	t.Helper()
	return newModuleRulesTable(t, n, options, legacyTestModule())
}

// Archived specialization/cadence cases exercise the previous saved rules.
// Current starts are covered by TestStarfortRebalance* through Module directly.
func preBalanceTestModule() gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, raw)
		if e != nil {
			return v, e
		}
		v, e = legacyStartMatch(v)
		if e != nil {
			return v, e
		}
		var s game.State
		if e = json.Unmarshal(v.Data, &s); e != nil {
			return v, e
		}
		s.SkillRules = 2
		s.SpecializationRules = 1
		return s.Envelope()
	}
	return m
}

// Select the archived schema-11 rules for historical runtime matrices.
// The current default is exercised independently by TestStarfortRevision*.
func preRevisionTestModule() gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, err := start(ctx, raw)
		if err != nil {
			return v, err
		}
		var s game.State
		if err = json.Unmarshal(v.Data, &s); err != nil {
			return v, err
		}
		s.RevisionRules = 0
		return s.Envelope()
	}
	return m
}
