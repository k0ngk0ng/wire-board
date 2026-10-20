// Package data embeds original Fortress 2 BLUE tank metadata and media paths.
package data

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed tanks.json
var raw []byte

type Sprite struct {
	URL string `json:"url"`
	W   int    `json:"w"`
	H   int    `json:"h"`
	AX  int    `json:"ax"`
	AY  int    `json:"ay"`
}

type Tank struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	HP      int               `json:"hp"`
	Defense int               `json:"defense"`
	Speed   int               `json:"speed"`
	Primary int               `json:"primary"`
	Special int               `json:"special"`
	Angle   int               `json:"angle"`
	Range   int               `json:"range"`
	Skills  []int             `json:"skills"`
	Skill   string            `json:"skillName"`
	Sprites map[string]Sprite `json:"sprites"`
	Bullet  *Sprite           `json:"bullet"`
}

type Explosion struct {
	URL   string `json:"url"`
	CellW int    `json:"cellW"`
	CellH int    `json:"cellH"`
	Count int    `json:"count"`
}

type Catalog struct {
	Tanks     map[string]Tank   `json:"tanks"`
	Portraits map[string]Sprite `json:"portraits"`
	Explosion Explosion         `json:"explosion"`
	Audio     map[string]string `json:"audio"`
}

var (
	once sync.Once
	cat  Catalog
	err  error
)

func All() (Catalog, error) {
	once.Do(func() {
		if e := json.Unmarshal(raw, &cat); e != nil {
			err = fmt.Errorf("tank data: %w", e)
			return
		}
		if len(cat.Tanks) == 0 {
			err = fmt.Errorf("tank data: empty")
		}
	})
	return cat, err
}

func IDs() []int {
	c, e := All()
	if e != nil {
		return nil
	}
	ids := make([]int, 0, len(c.Tanks))
	for _, t := range c.Tanks {
		ids = append(ids, t.ID)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

func TankByID(id int) (Tank, bool) {
	c, e := All()
	if e != nil {
		return Tank{}, false
	}
	t, ok := c.Tanks[fmt.Sprint(id)]
	return t, ok
}
