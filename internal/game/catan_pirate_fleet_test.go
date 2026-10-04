package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func fleetFixture(t *testing.T, n, player, strength, defense int) *State {
	t.Helper()
	s := pirateIslandsMapGame(t, n)
	g := s.Catan
	p := g.pirateIslands()
	g.SetupStep = g.SetupLimit()
	s.Turn = 0
	s.Phase = "catan_roll"
	g.Dice = []int{strength, 5}
	found := false
	for at, id := range p.FleetPath {
		if slices.Contains(g.Tiles[id].Vertices, p.Fortresses[player].StartVertex) {
			g.Seafarers.Pirate = p.FleetPath[(at-strength+len(p.FleetPath))%len(p.FleetPath)]
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fixture missing attacked sea")
	}
	for i := range g.Edges {
		if defense == 0 {
			break
		}
		e := &g.Edges[i]
		if e.Owner < 0 || e.Owner == player {
			e.Owner = player
			e.Ship = true
			e.Warship = true
			defense--
		}
	}
	return s
}
func fleetRoll(t *testing.T, s *State) {
	t.Helper()
	if err := s.catanRoll(sum(s.Catan.Dice)); err != nil {
		t.Fatal(err)
	}
	if err := s.catanPirateSeven(); err != nil {
		t.Fatal(err)
	}
}
func fleetSupply(t *testing.T, g *Catan) {
	t.Helper()
	for c := range g.Bank {
		total := g.Bank[c]
		for _, p := range g.Players {
			total += p.Resources[c]
		}
		want := 19
		if len(g.Players) > 4 {
			want = 24
		}
		if total != want || g.Bank[c] < 0 {
			t.Fatal("resource conservation", c, total, want)
		}
	}
}
func fleetGive(g *Catan, player int, hand []int) {
	catanMove(g.Bank, g.Players[player].Resources, hand)
}

func TestCatanPirateFleetAttackPrecedesSevenDiscard(t *testing.T) {
	s := fleetFixture(t, 4, 1, 2, 1)
	g := s.Catan
	fleetGive(g, 1, []int{4, 2, 1, 1, 1})
	g.Vertices[g.pirateIslands().Fortresses[1].StartVertex].Level = 2
	for i := range g.Vertices {
		if g.Vertices[i].Level == 0 {
			g.Vertices[i].Level = 2
			g.Vertices[i].Owner = 1
			break
		}
	}
	fleetRoll(t, s)
	if sum(g.Players[1].Resources) != 6 {
		t.Fatal("fleet loss / seven steal order", g.Players[1].Resources)
	}
	if g.DiscardDue[1] != 0 || s.Phase != "catan_steal" || g.Robber != -1 || g.pirateIslands().SevenPending {
		t.Fatal("incorrect seven sequence", s.Phase, g.DiscardDue)
	}
	fleetSupply(t, g)
	for _, log := range s.Log {
		if strings.Contains(log, "未挡住") && !strings.Contains(log, "随机失去 3 张资源") {
			t.Fatal("loss count", log)
		}
	}
	helperApply(t, s, 0, Action{Type: "catan_skip_steal"})
	if s.Phase != "catan_turn" || sum(s.Catan.Players[1].Resources) != 6 {
		t.Fatal("optional seven theft could not be declined")
	}
}
func TestCatanPirateFleetRewardBeforeProductionRestoreAndTimeout(t *testing.T) {
	for _, auto := range []bool{false, true} {
		s := fleetFixture(t, 4, 1, 1, 2)
		g := s.Catan
		for i := range g.Tiles {
			g.Tiles[i].Number = 0
		}
		for i := range g.Tiles {
			tile := &g.Tiles[i]
			if tile.Resource == 3 && slices.Contains(tile.Vertices, g.pirateIslands().Fortresses[1].StartVertex) {
				tile.Number = 6
				break
			}
		}
		fleetRoll(t, s)
		pirate := g.Seafarers.Pirate
		if s.Phase != "catan_fleet_reward" || s.CatanPendingActor() != 1 || sum(g.Players[1].Resources) != 0 {
			t.Fatal("production ran before reward")
		}
		helperReject(t, s, 0, Action{Type: "catan_fleet_reward", Color: 0})
		helperReject(t, s, 1, Action{Type: "catan_roll"})
		helperReject(t, s, 1, Action{Type: "catan_fleet_reward", Color: 5})
		before := clone(*s)
		s = &before
		if auto {
			s.AutoCatanPending()
		} else {
			helperApply(t, s, 1, Action{Type: "catan_fleet_reward", Color: 4})
		}
		g = s.Catan
		if g.Seafarers.Pirate != pirate || g.pirateIslands().Raid != nil || s.Phase != "catan_turn" || sum(g.Players[1].Resources) != 2 || g.Players[1].Resources[3] != 1 {
			t.Fatal("reward continuation", s.Phase, g.Players[1].Resources)
		}
		if !auto && g.Players[1].Resources[4] != 1 {
			t.Fatal("wrong reward")
		}
		fleetSupply(t, g)
	}
}
func TestCatanPirateFleetVictoryRewardCanTriggerDiscard(t *testing.T) {
	s := fleetFixture(t, 3, 1, 2, 3)
	g := s.Catan
	fleetGive(g, 1, []int{2, 2, 1, 1, 1})
	fleetGive(g, 2, []int{1, 0, 0, 0, 0})
	fleetRoll(t, s)
	helperApply(t, s, 1, Action{Type: "catan_fleet_reward", Color: 4})
	g = s.Catan
	if s.Phase != "catan_discard" || g.DiscardDue[1] != 4 || sum(g.Players[1].Resources) != 8 {
		t.Fatal("reward was excluded from seven hand count")
	}
	helperApply(t, s, 1, Action{Type: "catan_discard", Tokens: []int{2, 2, 0, 0, 0}})
	g = s.Catan
	if s.Phase != "catan_steal" || !reflect.DeepEqual(g.Victims, []int{1, 2}) {
		t.Fatal("seven did not allow any opponent", s.Phase, g.Victims)
	}
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 0})
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: -1})
	helperApply(t, s, 0, Action{Type: "catan_steal", Target: 2})
	if s.Phase != "catan_turn" || s.Catan.Players[0].Resources[0] != 1 {
		t.Fatal("distant steal failed")
	}
	fleetSupply(t, s.Catan)
}
func TestCatanPirateFleetTieEmptyHandEliminatedAndSafeHex(t *testing.T) {
	for _, mode := range []string{"tie", "empty", "eliminated", "safe", "removed"} {
		t.Run(mode, func(t *testing.T) {
			s := fleetFixture(t, 6, 0, 1, 1)
			g := s.Catan
			p := g.pirateIslands()
			for i := range g.Tiles {
				g.Tiles[i].Number = 0
			}
			if mode != "empty" {
				fleetGive(g, 0, []int{2, 1, 1, 1, 1})
			}
			if mode != "tie" {
				for i := range g.Edges {
					g.Edges[i].Warship = false
				}
			}
			if mode == "eliminated" {
				g.Players[0].Eliminated = true
				s.Turn = 1
			}
			if mode == "safe" {
				f := p.Fortresses[0]
				g.Vertices[f.Beachhead].Level = 1
				g.Vertices[f.Beachhead].Owner = 0
				at := slices.Index(p.FleetPath, p.SafeTile)
				g.Seafarers.Pirate = p.FleetPath[(at-1+len(p.FleetPath))%len(p.FleetPath)]
			}
			if mode == "removed" {
				g.Seafarers.Pirate = -1
			}
			before := append([]int{}, g.Players[0].Resources...)
			fleetRoll(t, s)
			if !reflect.DeepEqual(before, g.Players[0].Resources) || p.Raid != nil || s.Phase != "catan_turn" {
				t.Fatal("unexpected attack result", mode)
			}
			if mode == "safe" && g.Seafarers.Pirate != p.SafeTile {
				t.Fatal("safe tile skipped movement")
			}
			fleetSupply(t, g)
		})
	}
}
func TestCatanPirateFleetUsesSmallerDieAndWraps(t *testing.T) {
	for _, dice := range [][]int{{1, 6}, {6, 1}, {3, 3}, {6, 6}} {
		s := pirateIslandsMapGame(t, 3)
		g := s.Catan
		p := g.pirateIslands()
		g.SetupStep = g.SetupLimit()
		g.Dice = dice
		for i := range g.Vertices {
			g.Vertices[i].Level = 0
			g.Vertices[i].Owner = -1
		}
		start := len(p.FleetPath) - 1
		g.Seafarers.Pirate = p.FleetPath[start]
		fleetRoll(t, s)
		if g.Seafarers.Pirate != p.FleetPath[(start+min(dice[0], dice[1]))%len(p.FleetPath)] {
			t.Fatal("wrong fleet movement", dice)
		}
	}
}
func TestCatanPirateFleetRewardPrivacyAndBotHiddenInformation(t *testing.T) {
	s := fleetFixture(t, 4, 1, 1, 2)
	fleetRoll(t, s)
	expected, err := s.catanBot(1)
	if err != nil {
		t.Fatal(err)
	}
	changed := clone(*s)
	changed.Catan.Players[2].Resources = []int{10, 1, 2, 3, 4}
	slices.Reverse(changed.Catan.DevDeck)
	actual, err := changed.catanBot(1)
	if err != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatal("bot used hidden info")
	}
	for _, viewer := range []int{-1, 0, 1, 2, 3} {
		v := s.View(viewer)["catan"].(map[string]any)
		for i, raw := range v["players"].([]any) {
			p := raw.(map[string]any)
			_, hand := p["resources"]
			_, dev := p["dev"]
			if hand != (viewer == i) || dev != (viewer == i) {
				t.Fatal("private hand leak")
			}
		}
	}
}

func TestCatanPirateFleetHelpersResumeOnlyAfterReward(t *testing.T) {
	for _, helper := range []int{3, 5} {
		strength := 1
		if helper == 5 {
			strength = 2
		}
		s := fleetFixture(t, 3, 1, strength, 3)
		g := s.Catan
		for i := range g.Tiles {
			g.Tiles[i].Number = 0
		}
		g.Options.Helpers = true
		g.TurnSerial = 4
		g.Players[2].Helper = &CatanHelperSeat{ID: helper}
		g.HelperDisplay = []int{1, 2, 4}
		fleetRoll(t, s)
		if g.HelperPending != nil || s.Phase != "catan_fleet_reward" {
			t.Fatal("helper ran before raid reward")
		}
		helperApply(t, s, 1, Action{Type: "catan_fleet_reward", Color: 0})
		g = s.Catan
		if s.Phase != "catan_helper" || g.HelperPending.Player != 2 || g.HelperPending.Kind != "resource" {
			t.Fatal("production helper not resumed", helper)
		}
		helperApply(t, s, 2, Action{Type: "catan_helper_choice", Color: 4})
		helperApply(t, s, 2, Action{Type: "catan_helper_choice", Choice: "flip"})
		g = s.Catan
		want := "catan_turn"
		if helper == 5 {
			want = "catan_steal"
		}
		if s.Phase != want || g.pirateIslands().Raid != nil || g.pirateIslands().SevenPending {
			t.Fatal("helper continuation", helper, s.Phase)
		}
		if helper == 5 && !reflect.DeepEqual(g.Victims, []int{1, 2}) {
			t.Fatal("Thorolf gain unavailable for seven steal")
		}
		fleetSupply(t, g)
	}
}
func TestCatanPirateFleetEmptyBankDoesNotBlockProduction(t *testing.T) {
	s := fleetFixture(t, 3, 1, 1, 2)
	g := s.Catan
	for i := range g.Tiles {
		g.Tiles[i].Number = 0
	}
	fleetGive(g, 2, append([]int{}, g.Bank...))
	fleetRoll(t, s)
	if s.Phase != "catan_turn" || g.pirateIslands().Raid != nil || sum(g.Players[1].Resources) != 0 {
		t.Fatal("empty bank blocked roll")
	}
	fleetSupply(t, g)
}
func TestCatanPirateFleetCorruptPathRejectsRollAtomically(t *testing.T) {
	s := pirateIslandsMapGame(t, 3)
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_roll"
	g.Seafarers.Pirate = 10000
	helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
}
func TestCatanPirateFleetMultipleBuildingsOnlyAttackOwnerOnce(t *testing.T) {
	s := fleetFixture(t, 4, 1, 1, 0)
	g := s.Catan
	p := g.pirateIslands()
	for i := range g.Tiles {
		g.Tiles[i].Number = 0
	}
	target := p.FleetPath[(slices.Index(p.FleetPath, g.Seafarers.Pirate)+1)%len(p.FleetPath)]
	count := 0
	for _, id := range g.Tiles[target].Vertices {
		v := &g.Vertices[id]
		if v.Level == 0 && count < 2 {
			v.Owner = 1
			v.Level = 1
			count++
		}
	}
	fleetGive(g, 1, []int{4, 0, 0, 0, 0})
	fleetRoll(t, s)
	if sum(g.Players[1].Resources) != 3 {
		t.Fatal("attacked once per building instead of owner")
	}
	fleetSupply(t, g)
}
