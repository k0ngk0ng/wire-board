package engine_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

type arenaAudit struct {
	GridStep                    int    `json:"gridStep"`
	Radius                      int    `json:"radius"`
	FloorCells                  int    `json:"floorCells"`
	WalkableFloorCells          int    `json:"walkableFloorCells"`
	ReachableFloorCells         int    `json:"reachableFloorCells"`
	ReachableAreaPx2            int    `json:"reachableAreaPx2"`
	ReachablePermille           int    `json:"reachablePermille"`
	MinSpawnPathPx              int    `json:"minSpawnPathPx"`
	BaselineReachableFloorCells int    `json:"baselineReachableFloorCells"`
	BaselineRatioPermille       int    `json:"baselineRatioPermille"`
	AllSpawnAndWeaponsReachable bool   `json:"allSpawnAndWeaponsReachable"`
	SourceFloorSHA256           string `json:"sourceFloorSHA256"`
	SourceMetadataSHA256        string `json:"sourceMetadataSHA256"`
}

type floorEntry struct {
	ID           string         `json:"id"`
	SourceRoomID int            `json:"sourceRoomId"`
	Floor        []engine.Point `json:"floor"`
}

type floorEvidence struct {
	SourceMetadataSHA256 string       `json:"sourceMetadataSHA256"`
	Maps                 []floorEntry `json:"maps"`
}

func TestStaticActorsNeverBecomeArenaObstacles(t *testing.T) {
	raw, err := os.ReadFile("../data/maps.json")
	if err != nil {
		t.Fatal(err)
	}
	var maps []struct {
		ID    string `json:"id"`
		Walls []struct {
			SourceObject int    `json:"sourceObject"`
			SourceSprite string `json:"sourceSprite"`
		} `json:"walls"`
	}
	if err = json.Unmarshal(raw, &maps); err != nil {
		t.Fatal(err)
	}
	actors := map[int]bool{503: true, 416: true, 863: true, 535: true, 536: true, 537: true, 373: true, 377: true, 547: true, 390: true, 417: true, 418: true, 862: true, 699: true}
	recordPlayer := false
	for _, world := range maps {
		for _, wall := range world.Walls {
			if actors[wall.SourceObject] {
				t.Errorf("map %s contains actor obstacle object=%d sprite=%s", world.ID, wall.SourceObject, wall.SourceSprite)
			}
			if world.ID == "push-it" && wall.SourceObject == 506 {
				recordPlayer = true
			}
		}
	}
	if !recordPlayer {
		t.Fatal("the record-player furniture was removed with the DJ")
	}
}

func TestNativeMapFloorAreasAndReachability(t *testing.T) {
	raw, err := os.ReadFile("../data/maps.json")
	if err != nil {
		t.Fatal(err)
	}
	var source []struct {
		ID           string     `json:"id"`
		SourceRoomID int        `json:"sourceRoomId"`
		ArenaAudit   arenaAudit `json:"arenaAudit"`
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/native-floor-points.json")
	if err != nil {
		t.Fatal(err)
	}
	var evidence floorEvidence
	if err = json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence.Maps) != 9 || len(source) != 9 {
		t.Fatal("nine native floor evidence records required")
	}
	proofs := map[string]floorEntry{}
	for _, entry := range evidence.Maps {
		proofs[entry.ID] = entry
	}
	newRooms := map[string][3]int{"highball": {65, 808, 800}, "exposed": {148, 1500, 640}, "hot-heavy": {59, 800, 640}}
	baseline := 0
	for _, entry := range source {
		if _, large := newRooms[entry.ID]; !large {
			baseline = max(baseline, entry.ArenaAudit.ReachableFloorCells)
		}
	}
	for _, entry := range source {
		world := engine.MapByID(entry.ID)
		proof, ok := proofs[entry.ID]
		if !ok || proof.SourceRoomID != entry.SourceRoomID {
			t.Fatalf("native source mismatch %s", entry.ID)
		}
		floorJSON, _ := json.Marshal(proof.Floor)
		digest := sha256.Sum256(floorJSON)
		audit := entry.ArenaAudit
		if audit.SourceFloorSHA256 != hex.EncodeToString(digest[:]) || audit.SourceMetadataSHA256 != evidence.SourceMetadataSHA256 || len(audit.SourceMetadataSHA256) != 64 {
			t.Fatalf("native floor digest mismatch %s", entry.ID)
		}
		if audit.GridStep != 16 || audit.Radius != 9 || audit.FloorCells != len(proof.Floor) {
			t.Fatalf("native metric units mismatch %s", entry.ID)
		}
		graph := floorGraph(world, proof.Floor)
		if len(graph) != audit.WalkableFloorCells {
			t.Fatalf("safe native floor mismatch %s: have %d want %d", entry.ID, len(graph), audit.WalkableFloorCells)
		}
		if len(world.Spawns) < 4 || len(world.WeaponSpawns) < 4 {
			t.Fatalf("missing distributed arena points %s", entry.ID)
		}
		first := engine.Point{X: world.Spawns[0].X / engine.Unit, Y: world.Spawns[0].Y / engine.Unit}
		reached := floorDistances(graph, first)
		minPath := int(^uint(0) >> 1)
		for i, spawn := range world.Spawns {
			point := engine.Point{X: spawn.X / engine.Unit, Y: spawn.Y / engine.Unit}
			if _, ok := reached[point]; !ok {
				t.Fatalf("unreachable spawn %s/%d", entry.ID, i)
			}
			distances := floorDistances(graph, point)
			for j := i + 1; j < len(world.Spawns); j++ {
				other := engine.Point{X: world.Spawns[j].X / engine.Unit, Y: world.Spawns[j].Y / engine.Unit}
				minPath = min(minPath, distances[other]*16)
			}
		}
		for i, drop := range world.WeaponSpawns {
			if _, ok := reached[engine.Point{X: drop.X / engine.Unit, Y: drop.Y / engine.Unit}]; !ok {
				t.Fatalf("unreachable weapon %s/%d", entry.ID, i)
			}
		}
		if len(reached) != audit.ReachableFloorCells || audit.ReachableAreaPx2 != len(reached)*16*16 || audit.MinSpawnPathPx != minPath || !audit.AllSpawnAndWeaponsReachable || minPath <= 0 {
			t.Fatalf("native playable area mismatch %s: have %d cells, minPath=%d audit=%+v", entry.ID, len(reached), minPath, audit)
		}
		if audit.ReachablePermille != len(reached)*1000/len(graph) || audit.BaselineReachableFloorCells != baseline || audit.BaselineRatioPermille != len(reached)*1000/baseline {
			t.Fatalf("native ratios mismatch %s", entry.ID)
		}
		if expected, large := newRooms[entry.ID]; large {
			if entry.SourceRoomID != expected[0] || world.Width != expected[1]*engine.Unit || world.Height != expected[2]*engine.Unit {
				t.Fatalf("large scene was rescaled or replaced %s", entry.ID)
			}
			t.Logf("native map=%s room=%d size=%dx%d connectedCells=%d areaPx2=%d reachablePermille=%d baselineRatioPermille=%d minSpawnPathPx=%d", entry.ID, entry.SourceRoomID, world.Width/engine.Unit, world.Height/engine.Unit, len(reached), audit.ReachableAreaPx2, audit.ReachablePermille, audit.BaselineRatioPermille, minPath)
		}
	}
}

func floorGraph(world engine.Map, floor []engine.Point) map[engine.Point][]engine.Point {
	nodes := map[engine.Point][]engine.Point{}
	for _, point := range floor {
		x, y := point.X*engine.Unit, point.Y*engine.Unit
		if x < engine.Radius || y < engine.Radius || x > world.Width-engine.Radius || y > world.Height-engine.Radius {
			continue
		}
		safe := true
		for _, wall := range world.Walls {
			cx, cy := max(wall.X, min(x, wall.X+wall.W)), max(wall.Y, min(y, wall.Y+wall.H))
			dx, dy := int64(x-cx), int64(y-cy)
			if dx*dx+dy*dy < int64(engine.Radius)*int64(engine.Radius) {
				safe = false
				break
			}
		}
		if safe {
			nodes[point] = nil
		}
	}
	doors := make([]engine.Door, len(world.Doors))
	for i, door := range world.Doors {
		doors[i] = engine.Door{ID: door.ID, Open: true}
	}
	for point := range nodes {
		for _, direction := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
			next := engine.Point{X: point.X + direction[0]*16, Y: point.Y + direction[1]*16}
			if _, ok := nodes[next]; !ok {
				continue
			}
			x, y, _ := engine.PredictMotion(world, point.X*engine.Unit, point.Y*engine.Unit, 0, direction[0]*1000, direction[1]*1000, 5, doors, nil)
			x, y, _ = engine.PredictMotion(world, x, y, 0, direction[0]*334, direction[1]*334, 1, doors, nil)
			if x == next.X*engine.Unit && y == next.Y*engine.Unit {
				nodes[point] = append(nodes[point], next)
			}
		}
	}
	return nodes
}

func floorDistances(graph map[engine.Point][]engine.Point, start engine.Point) map[engine.Point]int {
	distance := map[engine.Point]int{}
	if _, ok := graph[start]; !ok {
		return distance
	}
	distance[start] = 0
	queue := []engine.Point{start}
	for cursor := 0; cursor < len(queue); cursor++ {
		current := queue[cursor]
		for _, next := range graph[current] {
			if _, seen := distance[next]; seen {
				continue
			}
			distance[next] = distance[current] + 1
			queue = append(queue, next)
		}
	}
	return distance
}
