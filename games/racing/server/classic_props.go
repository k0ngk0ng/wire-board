package racing

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
)

// Numeric identities/collision references only; no client visual payload.
//
//go:embed data/prop-index.json data/props/*.json
var classicPropFiles embed.FS

type ClassicPropState struct {
	Catalog string `json:"catalog"`
	Broken  []byte `json:"broken"`
}
type classicPropDefinition struct {
	Index        int       `json:"index"`
	ID           string    `json:"id"`
	RouteBarrier int       `json:"routeBarrier"`
	RuleEdges    []int     `json:"ruleEdges"`
	Position     []float64 `json:"position"`
	Bounds       struct {
		Min []float64 `json:"min"`
		Max []float64 `json:"max"`
	} `json:"bounds"`
}
type classicPropCatalog struct {
	Version     int                     `json:"version"`
	Unit        string                  `json:"unit"`
	Track       string                  `json:"track"`
	Geometry    string                  `json:"geometry"`
	RuleSHA256  string                  `json:"ruleSha256"`
	CatalogHash string                  `json:"catalogHash"`
	Props       []classicPropDefinition `json:"props"`
	byEdge      []int
}
type classicPropRegistry struct {
	byHash  map[string]*classicPropCatalog
	current map[string]*classicPropCatalog
	legacy  map[string]*classicPropCatalog
}

var classicProps = loadClassicProps()

func loadClassicProps() classicPropRegistry {
	registry := classicPropRegistry{byHash: map[string]*classicPropCatalog{}, current: map[string]*classicPropCatalog{}, legacy: map[string]*classicPropCatalog{}}
	var entries []struct {
		Geometry string `json:"geometry"`
		Catalog  string `json:"catalog"`
		SHA256   string `json:"sha256"`
		Current  bool   `json:"current"`
		Legacy   bool   `json:"legacy"`
	}
	data, err := classicPropFiles.ReadFile("data/prop-index.json")
	if err != nil || json.Unmarshal(data, &entries) != nil {
		panic("invalid classic prop index")
	}
	for _, entry := range entries {
		payload, err := classicPropFiles.ReadFile("data/props/" + entry.Catalog + ".json")
		if err != nil {
			panic(err)
		}
		digest := sha256.Sum256(payload)
		var catalog classicPropCatalog
		if hex.EncodeToString(digest[:]) != entry.SHA256 || json.Unmarshal(payload, &catalog) != nil || catalog.CatalogHash != entry.Catalog || catalog.Geometry != entry.Geometry || len(catalog.CatalogHash) != 64 || catalog.Version != 1 || catalog.Unit != "decimetres" || len(catalog.Props) == 0 || len(catalog.Props) > 108 {
			panic("invalid immutable classic prop catalog")
		}
		track, ok := classicTrackByGeometry(catalog.Geometry)
		rules, err := classicRuleFiles.ReadFile("data/rules/" + catalog.Geometry + ".json")
		ruleDigest := sha256.Sum256(rules)
		if !ok || err != nil || track.ID != catalog.Track || catalog.RuleSHA256 != hex.EncodeToString(ruleDigest[:]) {
			panic("classic prop catalog does not match immutable road rules")
		}
		catalog.byEdge = make([]int, len(track.Barriers))
		for i := range catalog.byEdge {
			catalog.byEdge[i] = -1
		}
		for i, prop := range catalog.Props {
			if prop.Index != i || prop.ID == "" || len(prop.RuleEdges) == 0 || len(prop.Position) != 3 || len(prop.Bounds.Min) != 3 || len(prop.Bounds.Max) != 3 {
				panic("invalid classic prop identity")
			}
			for k := 0; k < 3; k++ {
				if !finite(prop.Position[k]) || !finite(prop.Bounds.Min[k]) || !finite(prop.Bounds.Max[k]) || prop.Bounds.Min[k] > prop.Bounds.Max[k] {
					panic("invalid classic prop bounds")
				}
			}
			for _, edge := range prop.RuleEdges {
				if edge < 0 || edge >= len(track.Barriers) || track.Barriers[edge].Kind != "object" || catalog.byEdge[edge] != -1 {
					panic("invalid classic prop collision edge")
				}
				catalog.byEdge[edge] = i
			}
		}
		registry.byHash[catalog.CatalogHash] = &catalog
		if entry.Current {
			if registry.current[catalog.Geometry] != nil {
				panic("duplicate current classic prop catalog")
			}
			registry.current[catalog.Geometry] = &catalog
		}
		if entry.Legacy {
			if registry.legacy[catalog.Geometry] != nil {
				panic("duplicate legacy classic prop catalog")
			}
			registry.legacy[catalog.Geometry] = &catalog
		}
	}
	return registry
}

func newClassicPropState(catalog *classicPropCatalog) *ClassicPropState {
	if catalog == nil {
		return nil
	}
	return &ClassicPropState{Catalog: catalog.CatalogHash, Broken: make([]byte, (len(catalog.Props)+7)/8)}
}
func validateClassicPropState(s *State) error {
	if s.Props == nil {
		// This per-geometry fallback remains pinned to the first catalog when
		// future versions are added. Restore itself retains the original bytes.
		s.Props = newClassicPropState(classicProps.legacy[s.Track.Geometry])
	}
	if s.Props == nil {
		return errors.New("赛车道具版本不可用")
	}
	catalog := classicProps.byHash[s.Props.Catalog]
	if catalog == nil || catalog.Geometry != s.Track.Geometry || len(s.Props.Broken) != (len(catalog.Props)+7)/8 {
		return errors.New("赛车道具状态无效")
	}
	if tail := uint(len(catalog.Props) % 8); tail != 0 && s.Props.Broken[len(s.Props.Broken)-1]>>tail != 0 {
		return errors.New("赛车道具状态超出范围")
	}
	return nil
}
func classicPropBroken(props *ClassicPropState, index int) bool {
	return props != nil && index >= 0 && index/8 < len(props.Broken) && props.Broken[index/8]&(1<<uint(index%8)) != 0
}
func classicBrokenBarrier(props *ClassicPropState, edge int) bool {
	if props == nil {
		return false
	}
	catalog := classicProps.byHash[props.Catalog]
	return catalog != nil && edge >= 0 && edge < len(catalog.byEdge) && classicPropBroken(props, catalog.byEdge[edge])
}

type classicPropCollision struct {
	catalog *classicPropCatalog
	state   *ClassicPropState
	breakAt func(int, float64, float64, float64)
}

func classicCollisionProps(s *State, seat int, at int64) *classicPropCollision {
	if s.Props == nil {
		return nil
	}
	catalog := classicProps.byHash[s.Props.Catalog]
	if catalog == nil {
		return nil
	}
	return &classicPropCollision{catalog: catalog, state: s.Props, breakAt: func(index int, vx, vz, closing float64) {
		if classicPropBroken(s.Props, index) {
			return
		}
		s.Props.Broken[index/8] |= 1 << uint(index%8)
		s.emit("prop", seat, at, clamp(closing/20, 0, 1))
		event := &s.Events[len(s.Events)-1]
		event.Prop = &index
		event.Dir = math.Round(math.Atan2(vz, vx)*1000) / 1000
		// Project-defined reconstruction threshold, not original runtime proof.
		event.High = closing >= 8
		event.X, event.Y = 0, 0
	}}
}
