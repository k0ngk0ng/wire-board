package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCatanCaravansVariantsNatural(t *testing.T) {
	for name, build := range map[string]func(int) (*State, error){"shores": NewCatanCaravansShoresSeafarers, "islands": NewCatanCaravansIslandsSeafarers, "desert": NewCatanCaravansDesertSeafarers, "tribe": NewCatanCaravansTribeSeafarers, "world": NewCatanCaravansWorld} {
		for _, n := range []int{2, 4, 6} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/events%t", name, n, events), func(t *testing.T) {
					s, err := build(n)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.EnableCatanCaravansSeaHelpers(true); err != nil {
						t.Fatal(err)
					}
					if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
						t.Fatal(err)
					}
					if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
						t.Fatal(err)
					}
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					for step := 0; step < 18000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(step, s.Phase, e)
						}
						if step%137 == 0 {
							b := clone(*s)
							s = &b
							if e = s.validateCaravans(); e != nil {
								t.Fatal(e)
							}
						}
					}
					if !s.Finished {
						raw, _ := json.Marshal(s)
						dir := filepath.Join("..", "..", ".local", "catan-long-games")
						if e := os.MkdirAll(dir, 0700); e != nil {
							t.Fatal(e)
						}
						f, e := os.CreateTemp(dir, "merchant-variants-*.json")
						if e != nil {
							t.Fatal(e)
						}
						_, e = f.Write(raw)
						f.Close()
						if e != nil {
							t.Fatal(e)
						}
						t.Fatal("unfinished; state saved", f.Name(), "round", s.Round)
					}
					t.Log("round", s.Round)
				})
			}
		}
	}
}

func TestCatanCaravansVariantsAdmissionAndTargets(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		for _, bits := range []int{1, 2, 3} {
			s, e := NewCatanCaravansIslandsSeafarers(n)
			if e != nil {
				t.Fatal(e)
			}
			base := s.Catan.victoryTarget()
			if bits&1 != 0 {
				if e = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); e != nil {
					t.Fatal(e)
				}
			}
			if bits&2 != 0 {
				if e = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); e != nil {
					t.Fatal(e)
				}
				base++
			}
			if s.Catan.victoryTarget() != base {
				t.Fatal("target adjustment")
			}
			if e = s.validateCaravans(); e != nil {
				t.Fatal(e)
			}
			if n == 2 {
				if e = s.validateCatanTwo(); e != nil {
					t.Fatal(e)
				}
			}
			b := clone(*s)
			if bits&1 != 0 {
				b.Catan.FriendlyRobber.Rules = "bad"
			} else {
				b.Catan.Harbors.Rules = "bad"
			}
			if b.validateCaravans() == nil {
				t.Fatal("corrupt variant accepted")
			}
		}
	}
}
