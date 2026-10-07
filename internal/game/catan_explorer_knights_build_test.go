package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerCityActionReject(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	before := clone(*s)
	if err := s.catanExplorerCityAction(player, a); err == nil {
		t.Fatal("invalid combined action accepted", a)
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("rejected combined action mutated state")
	}
}

// Controlled midgame: replace the empty initial harbor with a settlement at
// the same legal coastal intersection. This does not model a harbor downgrade.
func explorerCityVillageFixture(t *testing.T, n int) (*State, int) {
	t.Helper()
	s := explorerCityProductionFixture(t, n)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	vertex := -1
	for i, v := range g.Vertices {
		if v.Owner == 0 && v.Harbor {
			vertex = i
			g.Vertices[i].Level, g.Vertices[i].Harbor = 1, false
			break
		}
	}
	if vertex < 0 {
		t.Fatal("missing initial harbor")
	}
	for r, want := range []int{0, 0, 2, 6, 8, 1, 1, 1} {
		delta := want - g.Players[0].Resources[r]
		g.Players[0].Resources[r] += delta
		g.Bank[r] -= delta
	}
	s.catanScores()
	explorerCityRestore(t, s)
	return s, vertex
}

func TestCatanExplorerCityUpgradeAndMedicine(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"city", "harbor"} {
			for _, medicine := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/medicine%t", n, kind, medicine), func(t *testing.T) {
					s, v := explorerCityVillageFixture(t, n)
					a := Action{Type: "catan_city", Vertex: v, Prompt: int(s.Catan.TurnSerial)}
					grain, ore := 2, 3
					if kind == "harbor" {
						a.Type, ore = "catan_explorer_harbor", 2
					}
					if medicine {
						a.Type, a.Card = "catan_progress", 5
						if kind == "harbor" {
							a.Choice = "harbor"
						}
						ckProgressGive(t, s, 0, 5)
						grain--
						ore--
					}
					before := clone(*s.Catan)
					if err := s.catanExplorerCityAction(0, a); err != nil {
						t.Fatal(err)
					}
					g := s.Catan
					if g.Vertices[v].Level != 2 || g.Vertices[v].Harbor != (kind == "harbor") || g.Players[0].Score != before.Players[0].Score+1 {
						t.Fatal("wrong upgrade or score")
					}
					for r, want := range []int{0, 0, 0, grain, ore, 0, 0, 0} {
						if g.Players[0].Resources[r] != before.Players[0].Resources[r]-want || g.Bank[r] != before.Bank[r]+want {
							t.Fatal("wrong conserved price", r)
						}
					}
					if medicine && (slices.Contains(g.CitiesKnights.Players[0].Progress, 5) || len(g.CitiesKnights.ProgressDecks[0]) != len(before.CitiesKnights.ProgressDecks[0])+1) {
						t.Fatal("Medicine was not returned exactly once")
					}
					explorerCityRestore(t, s)
					explorerCityActionReject(t, s, 0, a)
					// Neither type of level-two building may convert to the other.
					for _, action := range []string{"catan_city", "catan_explorer_harbor"} {
						explorerCityActionReject(t, s, 0, Action{Type: action, Vertex: v, Prompt: a.Prompt})
					}
				})
			}
		}
	}
}

func TestCatanExplorerCityUpgradeAtomicGuards(t *testing.T) {
	base, v := explorerCityVillageFixture(t, 3)
	ckProgressGive(t, base, 0, 5)
	for _, name := range []string{"foreign", "stale", "phase", "skill", "choice", "missing_card", "resources", "bad_target", "foreign_target", "fallen_harbor", "fallen_priority", "bad_fallen"} {
		t.Run(name, func(t *testing.T) {
			s := clone(*base)
			a := Action{Type: "catan_progress", Card: 5, Vertex: v, Prompt: int(s.Catan.TurnSerial)}
			p := 0
			switch name {
			case "foreign":
				p = 1
			case "stale":
				a.Prompt = 0
			case "phase":
				s.Phase = "catan_roll"
			case "skill":
				a.Type, a.Skill = "catan_city", "medicine"
			case "choice":
				a.Type, a.Choice = "catan_city", "harbor"
			case "missing_card":
				s.Catan.CitiesKnights.Players[0].Progress = nil
				s.Catan.CitiesKnights.returnProgress([]int{5})
			case "resources":
				s.Catan.Bank[4] += s.Catan.Players[0].Resources[4]
				s.Catan.Players[0].Resources[4] = 0
			case "bad_target":
				a.Vertex = -1
			case "foreign_target":
				s.Catan.Vertices[v].Owner = 1
			case "fallen_harbor":
				s.Catan.CitiesKnights.FallenCities = []int{v}
				a.Choice = "harbor"
			case "fallen_priority":
				for i, b := range s.Catan.Vertices {
					if b.Owner == 0 && s.Catan.cityAt(i) {
						s.Catan.Vertices[i].Level = 1
						s.Catan.CitiesKnights.FallenCities = []int{i}
						break
					}
				}
			case "bad_fallen":
				s.Catan.CitiesKnights.FallenCities = []int{-1}
			}
			explorerCityActionReject(t, &s, p, a)
		})
	}
}

func TestCatanExplorerCityFallenRestoration(t *testing.T) {
	s, v := explorerCityVillageFixture(t, 3)
	g := s.Catan
	g.CitiesKnights.FallenCities = []int{v}
	if g.settlementPiecesLeft(0) != 5 || g.cityPiecesLeft(0) != 2 {
		t.Fatal("fallen city used settlement or harbor inventory")
	}
	ckProgressGive(t, s, 0, 5)
	explorerCityRestore(t, s)
	if err := s.catanExplorerCityAction(0, Action{Type: "catan_progress", Card: 5, Vertex: v, Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.CitiesKnights.FallenCities) != 0 || !s.Catan.cityAt(v) || s.Catan.settlementPiecesLeft(0) != 5 || s.Catan.cityPiecesLeft(0) != 2 {
		t.Fatal("restoration changed physical inventory")
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityWoolEightCardSupply(t *testing.T) {
	s, _ := explorerCityVillageFixture(t, 6)
	g, x := s.Catan, s.Catan.Explorer
	if err := x.Cargo.beginMovement(g, x.Fleet, 0, g.TurnSerial, 0); err != nil {
		t.Fatal(err)
	}
	s.Phase = "catan_explorer_move"
	before := clone(*s)
	if err := x.Fleet.wool(g, 0, g.TurnSerial, 0); err != nil {
		t.Fatal(err)
	}
	if g.Players[0].Resources[2] != before.Catan.Players[0].Resources[2]-1 || g.Bank[2] != before.Catan.Bank[2]+1 || x.Fleet.Turn.Ships[0].Remaining != 6 {
		t.Fatal("wrong wool payment or movement")
	}
	explorerCityRestore(t, s)
	g, x = s.Catan, s.Catan.Explorer
	paid := clone(*s)
	if err := x.Fleet.wool(g, 0, g.TurnSerial, 0); err == nil || !reflect.DeepEqual(*s, paid) {
		t.Fatal("repeat wool payment mutated state")
	}
	for _, size := range []int{0, 2, 5, 7, 9} {
		bad := clone(before)
		bad.Catan.Bank, bad.Catan.Players[0].Resources = make([]int, size), make([]int, size)
		if err := bad.Catan.Explorer.Fleet.wool(bad.Catan, 0, bad.Catan.TurnSerial, 0); err == nil {
			t.Fatal("malformed combined supply accepted", size)
		}
	}
}
