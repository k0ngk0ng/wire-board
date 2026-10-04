package game

import "errors"

const CatanExpansionRules = "catan-2025-helpers-2022"

type CatanOptions struct {
	Rules      string `json:"rules,omitempty"`
	Helpers    bool   `json:"helpers,omitempty"`
	AllHelpers bool   `json:"allHelpers,omitempty"`
}

func NormalizeCatanOptions(o CatanOptions) (CatanOptions, error) {
	if o.Rules != "" && o.Rules != CatanExpansionRules {
		return o, errors.New("不支持的卡坦岛规则版本")
	}
	if o.AllHelpers && !o.Helpers {
		return o, errors.New("使用全部助手需要启用 Helpers 扩展")
	}
	if o.Helpers {
		o.Rules = CatanExpansionRules
	} else {
		o.Rules = ""
	}
	return o, nil
}

func NewCatan(n int, options CatanOptions) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	if err != nil {
		return nil, err
	}
	s, err := New("catan", n)
	if err != nil {
		return nil, err
	}
	s.Catan.Options = o
	if o.Helpers {
		pool := []int{}
		for id := n + 1; id <= 12; id++ {
			pool = append(pool, id)
		}
		shuffle(pool)
		if !o.AllHelpers {
			pool = pool[:n]
		}
		s.Catan.HelperDisplay = pool
		s.Log = append(s.Log, "加入 Helpers：每人完成第二组起始建设后领取助手，使用后翻面或交换")
	}
	return s, nil
}
