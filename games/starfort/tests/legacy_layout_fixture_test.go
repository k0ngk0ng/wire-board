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
	s.World.Layout = ""
	s.SkillRules = 0
	s.SkillRNG = 0
	s.IceWalls = nil
	s.IceWallSerial = 0
	s.World.Map = archive.Maps[int(s.World.Seed%15)]
	s.World.Objectives = []game.Objective{}
	s.Craters = nil
	s.CraterSerial = 0
	return s.Envelope()
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
