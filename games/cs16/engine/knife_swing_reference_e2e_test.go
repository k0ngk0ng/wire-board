package engine

import (
	"fmt"
	"testing"
)

func TestKnifeReferencePinnedPrimaryFirstDamageE2E(t *testing.T) {
	s := knifeReferenceArena()
	s.Players[0].Pitch = 4000
	hit := knifeReferenceHit(t, s, false)
	if hit.Hitgroup != 2 || hit.Value != 15 || s.Players[1].Health != 85 {
		t.Fatalf("pinned first slash group=%d damage=%d health=%d", hit.Hitgroup, hit.Value, s.Players[1].Health)
	}
}

func TestKnifeReferencePinnedPrimaryTimingE2E(t *testing.T) {
	for _, setup := range []struct {
		name     string
		cooldown int
	}{
		{"air", 35}, {"hit", 40}, {"wall", 40}, {"stab-hit", 110}, {"stab-air", 100},
	} {
		for _, gap := range []int{setup.cooldown - 1, setup.cooldown, setup.cooldown + 1, 74, 75, 76, 79, 80, 81, 119, 120, 121, 5000} {
			t.Run(fmt.Sprintf("%s-gap-%d", setup.name, gap), func(t *testing.T) {
				s := knifeReferenceArena()
				p, target := &s.Players[0], &s.Players[1]
				p.Pitch, target.Health = 4000, 10000
				world := s.World
				if setup.name == "air" || setup.name == "stab-air" || setup.name == "wall" {
					target.X = 1000 * Unit
				}
				if setup.name == "air" || setup.name == "stab-air" {
					p.Pitch = 0
				}
				if setup.name == "wall" {
					s.World.Planes = append(append([]Plane(nil), s.World.Planes...), Plane{X: NormalUnit, Dist: 10 * Unit})
					s.World.Nodes = append(append([]Node(nil), s.World.Nodes...), Node{Plane: len(s.World.Planes) - 1, Front: ContentsSolid, Back: ContentsEmpty})
					s.World.Root = len(s.World.Nodes) - 1
				}
				secondary := setup.name == "stab-hit" || setup.name == "stab-air"
				p.Input = Input{Fire: !secondary, Secondary: secondary}
				StepCombat(s)
				if p.NextAttack != 100+int64(setup.cooldown) {
					t.Fatal("pinned first attack cooldown changed", p.NextAttack, setup.cooldown)
				}
				s.World = world
				target.X = 25 * Unit
				p.Pitch = 4000
				p.Input, p.Previous = Input{Fire: true}, Input{}
				s.Frame = 100 + int64(gap)
				beforeHealth, beforeEvents := target.Health, len(s.Events)
				StepCombat(s)
				hits, damage, group := 0, beforeHealth-target.Health, 0
				for _, event := range s.Events[beforeEvents:] {
					if event.Kind == "hit" && event.Weapon == "knife" {
						hits++
						group = event.Hitgroup
					}
				}
				if gap < setup.cooldown {
					if hits != 0 || damage != 0 {
						t.Fatal("slash fired before native cooldown", hits, damage)
					}
				} else if hits != 1 || group != 2 || damage != 15 || p.NextAttack != s.Frame+40 {
					t.Fatalf("setup=%s gap=%d hits=%d group=%d damage=%d next=%d", setup.name, gap, hits, group, damage, p.NextAttack)
				}
				t.Logf("setup=%s gapFrames=%d cooldownFrames=%d hits=%d damage=%d", setup.name, gap, setup.cooldown, hits, damage)
			})
		}
	}
}
