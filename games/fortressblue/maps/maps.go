// Package maps embeds the original Fortress 2 BLUE stage catalog.
package maps

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed catalog.json
var raw []byte

//go:embed client.json
var clientRaw []byte

type Spawn struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Team int `json:"team"`
	Seat int `json:"seat"`
}

type Map struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	NameEn     string  `json:"nameEn"`
	Stage      int     `json:"stage"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	OffY       int     `json:"offY"`
	FG         string  `json:"fg"`
	FG2        string  `json:"fg2"`
	BG         string  `json:"bg"`
	Thumb      string  `json:"thumb"`
	Music      string  `json:"music"`
	MaskCols   int     `json:"maskCols"`
	MaskRows   int     `json:"maskRows"`
	Mask       string  `json:"mask"`
	BulletMask string  `json:"bulletMask"`
	Spawns     []Spawn `json:"spawns"`
}

type Catalog struct {
	Version     int   `json:"version"`
	WorldHeight int   `json:"worldHeight"`
	Cell        int   `json:"cell"`
	Maps        []Map `json:"maps"`
}

type ClientMap struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameEn string `json:"nameEn"`
	Stage  int    `json:"stage"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	OffY   int    `json:"offY"`
	FG     string `json:"fg"`
	FG2    string `json:"fg2"`
	BG     string `json:"bg"`
	Thumb  string `json:"thumb"`
	Music  string `json:"music"`
}

type ClientCatalog struct {
	Version     int         `json:"version"`
	WorldHeight int         `json:"worldHeight"`
	Maps        []ClientMap `json:"maps"`
}

var (
	once           sync.Once
	catalog        Catalog
	client         ClientCatalog
	loadErr        error
	maskByID       map[string][]byte
	bulletMaskByID map[string][]byte
	worldH         int
	cellSize       int
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(raw, &catalog); err != nil {
			loadErr = fmt.Errorf("maps: %w", err)
			return
		}
		if err := json.Unmarshal(clientRaw, &client); err != nil {
			loadErr = fmt.Errorf("maps client: %w", err)
			return
		}
		if catalog.Version != 2 || catalog.WorldHeight <= 0 || catalog.Cell <= 0 || len(catalog.Maps) == 0 {
			loadErr = fmt.Errorf("maps: invalid catalog")
			return
		}
		worldH, cellSize = catalog.WorldHeight, catalog.Cell
		maskByID = make(map[string][]byte, len(catalog.Maps))
		bulletMaskByID = make(map[string][]byte, len(catalog.Maps))
		for _, m := range catalog.Maps {
			if m.ID == "" || m.Width <= 0 || m.MaskCols <= 0 || m.MaskRows <= 0 || m.Height != worldH {
				loadErr = fmt.Errorf("maps: invalid map %q", m.ID)
				return
			}
			bits, err := base64.StdEncoding.DecodeString(m.Mask)
			if err != nil || len(bits) != (m.MaskCols*m.MaskRows+7)/8 {
				loadErr = fmt.Errorf("maps: invalid mask %q", m.ID)
				return
			}
			bullet, err := base64.StdEncoding.DecodeString(m.BulletMask)
			if err != nil || len(bullet) != len(bits) {
				loadErr = fmt.Errorf("maps: invalid bullet mask %q", m.ID)
				return
			}
			if len(m.Spawns) != 8 {
				loadErr = fmt.Errorf("maps: missing spawns %q", m.ID)
				return
			}
			maskByID[m.ID] = bits
			bulletMaskByID[m.ID] = bullet
		}
	})
}

func All() (Catalog, error) {
	load()
	return catalog, loadErr
}

func Client() (ClientCatalog, error) {
	load()
	return client, loadErr
}

func ByID(id string) (Map, bool) {
	load()
	for _, m := range catalog.Maps {
		if m.ID == id {
			return m, true
		}
	}
	return Map{}, false
}

func BaseBulletMask(m Map) []byte {
	load()
	bits := bulletMaskByID[m.ID]
	return append([]byte(nil), bits...)
}

func BaseMask(m Map) []byte {
	load()
	bits := maskByID[m.ID]
	out := make([]byte, len(bits))
	copy(out, bits)
	return out
}

func WorldHeight() int { load(); return worldH }
func Cell() int        { load(); return cellSize }
