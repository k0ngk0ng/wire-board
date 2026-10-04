package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCatanNewWorldPreparedMapPreservesLayout(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for sample := 0; sample < 30; sample++ {
			layout, err := GenerateCatanNewWorldMap(n)
			if err != nil {
				t.Fatal(err)
			}
			before := clone(*layout)
			if err = ValidateCatanNewWorldMap(n, layout); err != nil {
				t.Fatal(n, err)
			}
			s, err := NewCatanNewWorldWithMap(n, CatanOptions{FiveSix: n > 4, Helpers: true}, layout)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.Catan.NewWorldMap(), layout) || !reflect.DeepEqual(*layout, before) {
				t.Fatal("approved layout rerolled or mutated")
			}
			if s.Phase != "catan_world_ports" || s.Catan.Robber != -1 || s.Catan.Seafarers.Pirate != -1 || len(s.Catan.Ports) != 0 {
				t.Fatal("prepared map skipped port setup")
			}
			// Submitted slices must not alias the running board.
			layout.Hexes[0].Resource = (layout.Hexes[0].Resource + 1) % 8
			if s.Catan.Tiles[0].Resource != before.Hexes[0].Resource {
				t.Fatal("caller can mutate running map")
			}
		}
	}
}

func TestCatanNewWorldCustomMapValidation(t *testing.T) {
	layout, err := GenerateCatanNewWorldMap(4)
	if err != nil {
		t.Fatal(err)
	}
	// Gold and desert are valid custom additions to the default three/four map,
	// provided the gold's disc is black and the desert has no disc.
	changed := 0
	for i, h := range layout.Hexes {
		if h.Resource < CatanDesert && h.Number != 6 && h.Number != 8 {
			if changed == 0 {
				layout.Hexes[i].Resource = CatanGold
			} else {
				layout.Hexes[i] = CatanNewWorldHex{Resource: CatanDesert}
				break
			}
			changed++
		}
	}
	if err = ValidateCatanNewWorldMap(4, layout); err != nil {
		t.Fatal("custom gold/desert", err)
	}
	s, err := NewCatanNewWorldWithMap(4, CatanOptions{}, layout)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(layout, s.Catan.NewWorldMap()) {
		t.Fatal("custom layout changed")
	}
	cases := []struct {
		name string
		edit func(*CatanNewWorldMap)
	}{
		{"wrong length", func(m *CatanNewWorldMap) { m.Hexes = m.Hexes[:41] }},
		{"fog", func(m *CatanNewWorldMap) { m.Hexes[0].Resource = CatanFog }},
		{"negative", func(m *CatanNewWorldMap) { m.Hexes[0].Resource = -1 }},
		{"sea disc", func(m *CatanNewWorldMap) { m.Hexes[0] = CatanNewWorldHex{Resource: CatanSea, Number: 5} }},
		{"desert disc", func(m *CatanNewWorldMap) { m.Hexes[0] = CatanNewWorldHex{Resource: CatanDesert, Number: 5} }},
		{"seven", func(m *CatanNewWorldMap) { m.Hexes[0] = CatanNewWorldHex{Resource: 0, Number: 7} }},
		{"missing disc", func(m *CatanNewWorldMap) { m.Hexes[0] = CatanNewWorldHex{Resource: 0} }},
		{"large disc", func(m *CatanNewWorldMap) { m.Hexes[0] = CatanNewWorldHex{Resource: 0, Number: 13} }},
		{"red gold", func(m *CatanNewWorldMap) { m.Hexes[0] = CatanNewWorldHex{Resource: CatanGold, Number: 6} }},
		{"adjacent reds", func(m *CatanNewWorldMap) {
			m.Hexes[0] = CatanNewWorldHex{Resource: 0, Number: 6}
			m.Hexes[1] = CatanNewWorldHex{Resource: 1, Number: 8}
		}},
		{"no production", func(m *CatanNewWorldMap) {
			for i := range m.Hexes {
				m.Hexes[i] = CatanNewWorldHex{Resource: CatanDesert}
			}
		}},
		{"no coast", func(m *CatanNewWorldMap) {
			for i := range m.Hexes {
				m.Hexes[i] = CatanNewWorldHex{Resource: CatanSea}
			}
			m.Hexes[0] = CatanNewWorldHex{Resource: 0, Number: 5}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := clone(*layout)
			tc.edit(&bad)
			before, _ := json.Marshal(bad)
			if ValidateCatanNewWorldMap(4, &bad) == nil {
				t.Fatal("invalid map accepted")
			}
			if _, err := NewCatanNewWorldWithMap(4, CatanOptions{}, &bad); err == nil {
				t.Fatal("invalid map started")
			}
			after, _ := json.Marshal(bad)
			if string(before) != string(after) {
				t.Fatal("validation mutated input")
			}
		})
	}
	if ValidateCatanNewWorldMap(6, layout) == nil || ValidateCatanNewWorldMap(2, layout) == nil || ValidateCatanNewWorldMap(4, nil) == nil {
		t.Fatal("invalid seat count/map accepted")
	}
}

func TestCatanNewWorldCustomComponentLimits(t *testing.T) {
	for _, n := range []int{3, 6} {
		base, err := GenerateCatanNewWorldMap(n)
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"terrain", "number"} {
			layout := clone(*base)
			for i, h := range layout.Hexes {
				if h.Resource >= 5 {
					continue
				}
				if kind == "terrain" && h.Resource != 0 {
					layout.Hexes[i].Resource = 0
					break
				}
				target := 3
				if n > 4 {
					target = 5
				}
				if kind == "number" && h.Number != target {
					layout.Hexes[i].Number = target
					break
				}
			}
			if ValidateCatanNewWorldMap(n, &layout) == nil {
				t.Fatal("component inventory overdraw accepted", n, kind)
			}
		}
	}
}

func TestCatanNewWorldRearrangedCustomMapsComplete(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, helpers := range []bool{false, true} {
			layout, err := GenerateCatanNewWorldMap(n)
			if err != nil {
				t.Fatal(err)
			}
			sea, land := -1, -1
			for i, h := range layout.Hexes {
				if h.Resource == CatanSea {
					sea = i
				}
				if h.Resource < CatanDesert && h.Number != 6 && h.Number != 8 {
					land = i
				}
			}
			layout.Hexes[sea], layout.Hexes[land] = layout.Hexes[land], layout.Hexes[sea]
			s, err := NewCatanNewWorldWithMap(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}, layout)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.Catan.Seafarers.Islands, s.Catan.findIslands()) {
				t.Fatal("edited island topology stale")
			}
			for step := 0; step < 10000 && !s.Finished; step++ {
				actor := s.Turn
				if p := s.CatanPendingActor(); p >= 0 {
					actor = p
				} else if s.Phase == "catan_discard" {
					for i, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = i
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(n, helpers, step, s.Phase, err)
				}
				helperApply(t, s, actor, a)
				fleetSupply(t, s.Catan)
				if step%37 == 0 {
					restored := clone(*s)
					s = &restored
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 12 {
				t.Fatal("custom map failed to finish", n, helpers)
			}
		}
	}
}
