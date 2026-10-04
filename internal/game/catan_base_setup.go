package game

import "fmt"

// Waiting-room selection, separate from the game's randomized seat colors.
type CatanBaseConfiguration struct {
	Layout string `json:"layout"`
	Rules  string `json:"rules"`
}

func CatanBaseLayouts(n int) []string {
	if n < 3 || n > 6 {
		return nil
	}
	if n > 4 {
		return []string{"variable", "fixed"}
	}
	return []string{"variable"}
}

func NormalizeCatanBaseConfiguration(n int, setup CatanBaseConfiguration) (CatanBaseConfiguration, error) {
	if n < 3 || n > 6 {
		return setup, fmt.Errorf("卡坦岛需要3至6位玩家")
	}
	rules := "catan-base-2025"
	if n > 4 {
		rules = "catan-base-5-6-2025"
	}
	if setup.Rules != "" && setup.Rules != rules {
		return setup, fmt.Errorf("布局规则版本与人数扩充不一致")
	}
	if setup.Layout == "" {
		setup.Layout = "variable"
	}
	if setup.Layout != "variable" && (setup.Layout != "fixed" || n < 5) {
		return setup, fmt.Errorf("该人数不支持所选基础地图布局")
	}
	setup.Rules = rules
	return setup, nil
}

func NewCatanConfigured(n int, options CatanOptions, setup CatanBaseConfiguration) (*State, error) {
	setup, err := NormalizeCatanBaseConfiguration(n, setup)
	if err != nil {
		return nil, err
	}
	if setup.Layout == "fixed" {
		return NewCatanFixedFiveSix(n, options)
	}
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	colors := make([]int, n)
	for i := range colors {
		colors[i] = i
	}
	s.Catan.BaseSetup = &CatanBaseSetup{Layout: setup.Layout, Rules: setup.Rules, Colors: colors, NeutralColor: -1}
	return s, nil
}
