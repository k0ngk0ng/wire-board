package game

import (
	"errors"
	"fmt"
	"math"
)

// Axial coordinates describe the physical hexes of a scenario. Frame artwork
// is separate: it is not silently converted into extra buildable sea hexes.
type CatanHexSpec struct {
	Q        int
	R        int
	Resource int
	Number   int
}

func (g *Catan) makeScenarioMap(hexes []CatanHexSpec) error {
	if len(hexes) == 0 {
		return errors.New("剧本地图不能为空")
	}
	seen := map[[2]int]bool{}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, h := range hexes {
		key := [2]int{h.Q, h.R}
		if seen[key] || h.Resource < 0 || h.Resource > CatanFog || h.Number < 0 || h.Number > 12 || h.Number == 7 {
			return errors.New("剧本地块坐标、地形或数字不合法")
		}
		seen[key] = true
		x, y := math.Sqrt(3)*(float64(h.Q)+float64(h.R)/2), 1.5*float64(h.R)
		minX = math.Min(minX, x)
		maxX = math.Max(maxX, x)
		minY = math.Min(minY, y)
		maxY = math.Max(maxY, y)
	}
	size := math.Min(580/(maxX-minX+math.Sqrt(3)), 480/(maxY-minY+2))
	tiles := []CatanTile{}
	vertices := []CatanVertex{}
	edges := []CatanEdge{}
	vertexIDs := map[string]int{}
	edgeIDs := map[[2]int]int{}
	for _, h := range hexes {
		x := 340 + (math.Sqrt(3)*(float64(h.Q)+float64(h.R)/2)-(minX+maxX)/2)*size
		y := 290 + (1.5*float64(h.R)-(minY+maxY)/2)*size
		tile := CatanTile{ID: len(tiles), X: x, Y: y, Resource: h.Resource, Number: h.Number, Vertices: []int{}}
		for k := 0; k < 6; k++ {
			angle := (30 + float64(k)*60) * math.Pi / 180
			vx, vy := x+size*math.Cos(angle), y+size*math.Sin(angle)
			key := fmt.Sprintf("%.3f:%.3f", vx, vy)
			id, exists := vertexIDs[key]
			if !exists {
				id = len(vertices)
				vertexIDs[key] = id
				vertices = append(vertices, CatanVertex{ID: id, X: vx, Y: vy, Owner: -1})
			}
			tile.Vertices = append(tile.Vertices, id)
		}
		for k := 0; k < 6; k++ {
			a, b := tile.Vertices[k], tile.Vertices[(k+1)%6]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			id, exists := edgeIDs[key]
			if !exists {
				id = len(edges)
				edgeIDs[key] = id
				edges = append(edges, CatanEdge{ID: id, A: a, B: b, Owner: -1})
			}
			edges[id].Tiles = append(edges[id].Tiles, tile.ID)
		}
		tiles = append(tiles, tile)
	}
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize = tiles, vertices, edges, nil, size
	g.Robber = -1
	for _, t := range tiles {
		if t.Resource == CatanDesert {
			g.Robber = t.ID
			break
		}
	}
	return nil
}
