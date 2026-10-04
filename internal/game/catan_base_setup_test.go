package game

import (
	"reflect"
	"testing"
)

func TestCatanBaseConfigurationValidationAndIdentity(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := CatanBaseLayouts(n)
		if len(layouts) != (1 + map[bool]int{true: 1}[n > 4]) {
			t.Fatal("unsupported layout catalog", n, layouts)
		}
		for _, layout := range layouts {
			setup, err := NormalizeCatanBaseConfiguration(n, CatanBaseConfiguration{Layout: layout})
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewCatanConfigured(n, CatanOptions{FiveSix: n > 4, Helpers: true}, setup)
			if err != nil {
				t.Fatal(err)
			}
			if s.Catan.BaseSetup.Layout != layout || s.Catan.BaseSetup.Rules != setup.Rules || len(s.Catan.BaseSetup.Colors) != n {
				t.Fatal("actual setup identity")
			}
			if (layout == "fixed") != (s.Phase == "catan_roll") {
				t.Fatal("wrong initial phase", s.Phase)
			}
			if layout == "variable" {
				for i, c := range s.Catan.BaseSetup.Colors {
					if i != c {
						t.Fatal("variable seat colors changed")
					}
				}
			}
			restored := clone(*s)
			if !reflect.DeepEqual(*s, restored) {
				t.Fatal("saved identity")
			}
		}
	}
	for _, c := range []struct {
		n int
		s CatanBaseConfiguration
	}{{2, CatanBaseConfiguration{}}, {7, CatanBaseConfiguration{}}, {4, CatanBaseConfiguration{Layout: "fixed"}}, {5, CatanBaseConfiguration{Layout: "prepared"}}, {6, CatanBaseConfiguration{Rules: "catan-base-2025"}}, {4, CatanBaseConfiguration{Rules: "catan-base-5-6-2025"}}} {
		if _, err := NormalizeCatanBaseConfiguration(c.n, c.s); err == nil {
			t.Fatal("invalid config accepted", c)
		}
	}
	if _, err := NewCatanConfigured(5, CatanOptions{}, CatanBaseConfiguration{}); err == nil {
		t.Fatal("missing extension")
	}
}
