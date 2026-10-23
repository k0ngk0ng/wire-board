// Package maps contains immutable, versioned geographic data shared with the UI.
package maps

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed world.json europe.json
var data embed.FS

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Province struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Label        Point    `json:"label"`
	Neighbors    []string `json:"neighbors"`
	SeaNeighbors []string `json:"seaNeighbors"`
}

type SeaRoute struct {
	From string `json:"from"`
	To   string `json:"to"`
	Path string `json:"path"`
}

type Map struct {
	ID          string              `json:"id"`
	Version     int                 `json:"version"`
	SHA256      string              `json:"sha256"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Width       float64             `json:"width"`
	Height      float64             `json:"height"`
	Provinces   []Province          `json:"provinces"`
	Starts      map[string][]string `json:"starts"`
	SeaRoutes   []SeaRoute          `json:"seaRoutes"`
}

type Summary struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	SHA256        string  `json:"sha256"`
	Version       int     `json:"version"`
	ProvinceCount int     `json:"provinceCount"`
	Width         float64 `json:"width"`
	Height        float64 `json:"height"`
}

// Load returns a separate decoded copy, so one match cannot mutate another map.
func Load(id string) (Map, error) {
	if id != "world" && id != "europe" {
		return Map{}, fmt.Errorf("未知地图：%s", id)
	}
	raw, err := data.ReadFile(id + ".json")
	if err != nil {
		return Map{}, err
	}
	var result Map
	if err := json.Unmarshal(raw, &result); err != nil {
		return Map{}, err
	}
	return result, nil
}

func Catalog() []Summary {
	result := make([]Summary, 0, 2)
	for _, id := range []string{"europe", "world"} {
		m, err := Load(id)
		if err != nil {
			panic(err) // Compiled-in map corruption is a build error, never user input.
		}
		result = append(result, Summary{ID: m.ID, Name: m.Name, Description: m.Description, SHA256: m.SHA256, Version: m.Version, ProvinceCount: len(m.Provinces), Width: m.Width, Height: m.Height})
	}
	return result
}
