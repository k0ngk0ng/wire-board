package engine

import "testing"

func knifeHostageReferenceFixture(t *testing.T, group int) *State {
	t.Helper()
	s, shot := originalHostageDamageFixture(t, group)
	s.World.Hostages[0].X -= 425 * Unit
	s.Hostages[0].X -= 425 * Unit
	p := &s.Players[0]
	p.Weapon, p.Inventory = "knife", []WeaponState{{ID: "knife", Ammo: 1}}
	p.X, p.Y, p.Z = shot.Start.X, shot.Start.Y, 0
	p.Z = shot.Start.Z - Eye(*p).Z
	p.Yaw, p.Pitch = 0, 0
	s.Frame = 100
	return s
}

func knifeHostageReferenceHit(t *testing.T, state *State, secondary bool) Event {
	t.Helper()
	before := len(state.Events)
	state.Players[0].Input = Input{Fire: !secondary, Secondary: secondary}
	StepCombat(state)
	for _, event := range state.Events[before:] {
		if event.Kind == "hostage_hit" {
			return event
		}
	}
	t.Fatal("knife did not damage actual hostage model or hull")
	return Event{}
}

func TestKnifeHostageOriginalModelDamageE2E(t *testing.T) {
	for _, scenario := range []struct {
		group, health, money int
		secondary            bool
	}{
		{1, 0, 13500, false}, {2, 67, 15340, false}, {3, 61, 15220, false}, {6, 94, 15880, false},
		{1, 0, 13500, true}, {2, 0, 13500, true}, {3, 0, 13500, true}, {6, 71, 15420, true},
	} {
		s := knifeHostageReferenceFixture(t, scenario.group)
		hit := knifeHostageReferenceHit(t, s, scenario.secondary)
		if hit.Hitgroup != scenario.group || s.Hostages[0].Health != scenario.health || s.Players[0].Money != scenario.money {
			t.Fatalf("group=%d secondary=%t actualGroup=%d health=%d money=%d", scenario.group, scenario.secondary, hit.Hitgroup, s.Hostages[0].Health, s.Players[0].Money)
		}
	}
}

func TestKnifeHostageDoesNotReceivePlayerBackstabBonusE2E(t *testing.T) {
	for _, yaw := range []int{0, 32768} {
		s := knifeHostageReferenceFixture(t, 6)
		s.World.Hostages[0].Yaw = yaw
		hit := knifeHostageReferenceHit(t, s, true)
		if hit.Hitgroup != 6 && hit.Hitgroup != 7 || s.Hostages[0].Health != 71 || s.Players[0].Money != 15420 {
			t.Fatalf("NPC yaw=%d group=%d health=%d money=%d", yaw, hit.Hitgroup, s.Hostages[0].Health, s.Players[0].Money)
		}
	}
}

func TestKnifeHostageWallAndInactiveTargetsE2E(t *testing.T) {
	for _, scenario := range []string{"wall", "dead", "rescued"} {
		s := knifeHostageReferenceFixture(t, 6)
		if scenario == "wall" {
			s.World.Planes = []Plane{{X: NormalUnit, Dist: s.Players[0].X + 10*Unit}}
			s.World.Nodes = []Node{{Plane: 0, Front: ContentsSolid, Back: ContentsEmpty}}
			s.World.Root = 0
		} else if scenario == "dead" {
			s.Hostages[0].Health = 0
		} else {
			s.Hostages[0].Rescued = true
		}
		before := s.Hostages[0].Health
		s.Players[0].Input.Secondary = true
		StepCombat(s)
		if s.Hostages[0].Health != before || s.Players[0].Money != 16000 {
			t.Fatal("knife damaged a blocked or inactive hostage", scenario)
		}
		for _, event := range s.Events {
			if event.Kind == "hostage_hit" {
				t.Fatal("blocked or inactive hostage emitted damage", scenario)
			}
		}
	}
}

func TestKnifeHostageHullNearestEntityE2E(t *testing.T) {
	for _, hostageFirst := range []bool{false, true} {
		s := knifeReferenceArena()
		xPlayer, xHostage := 33, 52
		if hostageFirst {
			xPlayer, xHostage = 58, 27
		}
		s.Players[1].X, s.Players[1].Y = xPlayer*Unit, -31*Unit
		s.World.Hostages = []Spawn{{X: xHostage * Unit, Y: 25 * Unit}}
		s.Hostages = []Hostage{{ID: 1, X: xHostage * Unit, Y: 25 * Unit, Health: 100, Following: -1}}
		s.Players[0].Money = 16000
		s.Players[0].Input.Secondary = true
		StepCombat(s)
		if hostageFirst {
			if s.Hostages[0].Health != 0 || s.Players[1].Health != 100 || s.Players[0].Money != 13500 {
				t.Fatal("foreground hostage hull did not stop the knife")
			}
		} else if s.Players[1].Health != 35 || s.Hostages[0].Health != 100 || s.Players[0].Money != 16000 {
			t.Fatal("foreground player hull did not stop the knife")
		}
	}
}

func TestKnifeReferenceHullRangeBoundaryE2E(t *testing.T) {
	for _, hostage := range []bool{false, true} {
		for _, offset := range []int{-1, 0, 1} {
			s := knifeReferenceArena()
			x := 64*Unit + offset
			if hostage {
				x = 58*Unit + offset
				s.Players[1].X = 10000 * Unit
				s.World.Hostages = []Spawn{{X: x, Y: 25 * Unit}}
				s.Hostages = []Hostage{{ID: 1, X: x, Y: 25 * Unit, Health: 100, Following: -1}}
			} else {
				s.Players[1].X, s.Players[1].Y = x, 31*Unit
			}
			s.Players[0].Input.Secondary = true
			StepCombat(s)
			health := s.Players[1].Health
			want := 100
			if offset < 0 {
				want = 35
				if hostage {
					want = 0
				}
			}
			if hostage {
				health = s.Hostages[0].Health
			}
			if health != want {
				t.Fatalf("hostage=%t boundaryOffset=%d health=%d want=%d", hostage, offset, health, want)
			}
		}
	}
}
