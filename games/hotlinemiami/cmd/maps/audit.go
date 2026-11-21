package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

type sourceRoom struct {
	Width  int
	Height int
	Offset int
	Floor  []engine.Point
}

type roomReader struct {
	Data   []byte
	Cursor int
}

func (r *roomReader) words(count int) ([]int, error) {
	if count < 0 || count > 1000000 || r.Cursor > len(r.Data)-count*4 {
		return nil, errors.New("原版房间数据越界")
	}
	values := make([]int, count)
	for i := range values {
		values[i] = int(int32(binary.LittleEndian.Uint32(r.Data[r.Cursor+i*4:])))
	}
	r.Cursor += count * 4
	return values, nil
}

func (r *roomReader) skipRecords(width int) error {
	count, err := r.words(1)
	if err != nil {
		return err
	}
	_, err = r.words(count[0] * width)
	return err
}

func readSourceFloors(data []byte) (map[int]sourceRoom, error) {
	r := roomReader{Data: data}
	if err := r.skipRecords(1); err != nil {
		return nil, err
	}
	if err := r.skipRecords(1); err != nil {
		return nil, err
	}
	count, err := r.words(1)
	if err != nil || count[0] < 1 || count[0] > 10000 {
		return nil, errors.New("原版房间数量无效")
	}
	rooms := map[int]sourceRoom{}
	for n := 0; n < count[0]; n++ {
		offset := r.Cursor
		header, readErr := r.words(117)
		if readErr != nil || header[1] < 1 || header[2] < 1 || header[116] < 0 || header[116] > 100000 {
			return nil, errors.New("原版房间头部无效")
		}
		floor := map[engine.Point]bool{}
		for tile := 0; tile < header[116]; tile++ {
			fields, tileErr := r.words(8)
			if tileErr != nil {
				return nil, tileErr
			}
			if fields[5] == 16 && fields[6] == 16 && fields[7] >= 1000 {
				floor[engine.Point{X: fields[0] + 8, Y: fields[1] + 8}] = true
			}
		}
		if err = r.skipRecords(5); err != nil {
			return nil, err
		}
		if err = r.skipRecords(10); err != nil {
			return nil, err
		}
		points := make([]engine.Point, 0, len(floor))
		for point := range floor {
			points = append(points, point)
		}
		sort.Slice(points, func(i, j int) bool {
			if points[i].X != points[j].X {
				return points[i].X < points[j].X
			}
			return points[i].Y < points[j].Y
		})
		if _, duplicate := rooms[header[0]]; duplicate {
			return nil, errors.New("原版房间编号重复")
		}
		rooms[header[0]] = sourceRoom{Width: header[1], Height: header[2], Offset: offset, Floor: points}
	}
	if r.Cursor != len(data) {
		return nil, errors.New("原版房间数据未完整消费")
	}
	return rooms, nil
}

func auditGraph(world engine.Map, floor []engine.Point) map[engine.Point][]engine.Point {
	graph := map[engine.Point][]engine.Point{}
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
			graph[point] = nil
		}
	}
	doors := make([]engine.Door, len(world.Doors))
	for i, door := range world.Doors {
		doors[i] = engine.Door{ID: door.ID, Open: true}
	}
	for point := range graph {
		for _, direction := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
			next := engine.Point{X: point.X + direction[0]*16, Y: point.Y + direction[1]*16}
			if _, ok := graph[next]; !ok {
				continue
			}
			x, y, _ := engine.PredictMotion(world, point.X*engine.Unit, point.Y*engine.Unit, 0, direction[0]*1000, direction[1]*1000, 5, doors, nil)
			x, y, _ = engine.PredictMotion(world, x, y, 0, direction[0]*334, direction[1]*334, 1, doors, nil)
			if x == next.X*engine.Unit && y == next.Y*engine.Unit {
				graph[point] = append(graph[point], next)
			}
		}
	}
	return graph
}

func auditDistances(graph map[engine.Point][]engine.Point, start engine.Point) map[engine.Point]int {
	distance := map[engine.Point]int{}
	if _, exists := graph[start]; !exists {
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

func fixedArena(world engine.Map) engine.Map {
	world.Width *= engine.Unit
	world.Height *= engine.Unit
	for i := range world.Walls {
		world.Walls[i].X *= engine.Unit
		world.Walls[i].Y *= engine.Unit
		world.Walls[i].W *= engine.Unit
		world.Walls[i].H *= engine.Unit
	}
	for i := range world.Doors {
		world.Doors[i].X *= engine.Unit
		world.Doors[i].Y *= engine.Unit
		world.Doors[i].W *= engine.Unit
		world.Doors[i].H *= engine.Unit
	}
	return world
}

func updateMapAudit(sourcePath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	rooms, err := readSourceFloors(data)
	if err != nil {
		return err
	}
	sourceSHA := sha256.Sum256(data)
	sourceDigest := hex.EncodeToString(sourceSHA[:])
	mapPath := filepath.Join("games", "hotlinemiami", "data", "maps.json")
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		return err
	}
	var records []map[string]json.RawMessage
	var worlds []engine.Map
	if json.Unmarshal(raw, &records) != nil || json.Unmarshal(raw, &worlds) != nil || len(worlds) != 9 {
		return errors.New("九张混战规则数据无效")
	}
	proofs := []map[string]any{}
	audits := make([]map[string]any, len(records))
	baseline := 0
	newMaps := map[string]bool{"highball": true, "exposed": true, "hot-heavy": true}
	for i, world := range worlds {
		roomID, roomOffset := 0, 0
		if json.Unmarshal(records[i]["sourceRoomId"], &roomID) != nil || json.Unmarshal(records[i]["sourceRoomOffset"], &roomOffset) != nil {
			return errors.New("缺少原版房间来源")
		}
		room, exists := rooms[roomID]
		if !exists || room.Width != world.Width || room.Height != world.Height || room.Offset != roomOffset {
			return errors.New("原版房间来源或尺寸不匹配")
		}
		floorJSON, _ := json.Marshal(room.Floor)
		floorSHA := sha256.Sum256(floorJSON)
		graph := auditGraph(fixedArena(world), room.Floor)
		if len(world.Spawns) < 4 || len(graph) == 0 {
			return errors.New("地图缺少出生点或原版地板")
		}
		reached := auditDistances(graph, world.Spawns[0])
		minPath := int(^uint(0) >> 1)
		for a, point := range world.Spawns {
			if _, exists := reached[point]; !exists {
				return errors.New("出生点不在相同原版地板可达分量")
			}
			distances := auditDistances(graph, point)
			for b := a + 1; b < len(world.Spawns); b++ {
				minPath = min(minPath, distances[world.Spawns[b]]*16)
			}
		}
		for _, point := range world.WeaponSpawns {
			if _, exists := reached[engine.Point{X: point.X, Y: point.Y}]; !exists {
				return errors.New("武器点不在出生可达分量")
			}
		}
		if minPath <= 0 {
			return errors.New("出生点重复或不可达")
		}
		audits[i] = map[string]any{"gridStep": 16, "radius": 9, "floorCells": len(room.Floor), "walkableFloorCells": len(graph), "reachableFloorCells": len(reached), "reachableAreaPx2": len(reached) * 256, "reachablePermille": len(reached) * 1000 / len(graph), "minSpawnPathPx": minPath, "allSpawnAndWeaponsReachable": true, "sourceFloorSHA256": hex.EncodeToString(floorSHA[:]), "sourceMetadataSHA256": sourceDigest}
		proofs = append(proofs, map[string]any{"id": world.ID, "sourceRoomId": roomID, "floor": room.Floor})
		if !newMaps[world.ID] {
			baseline = max(baseline, len(reached))
		}
	}
	if baseline <= 0 {
		return errors.New("旧地图可达面积基准无效")
	}
	for i, audit := range audits {
		audit["baselineReachableFloorCells"] = baseline
		audit["baselineRatioPermille"] = audit["reachableFloorCells"].(int) * 1000 / baseline
		records[i]["arenaAudit"], err = json.Marshal(audit)
		if err != nil {
			return err
		}
	}
	updated, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	proof, err := json.Marshal(map[string]any{"sourceMetadataSHA256": sourceDigest, "maps": proofs})
	if err != nil {
		return err
	}
	proofPath := filepath.Join("games", "hotlinemiami", "engine", "testdata", "native-floor-points.json")
	if err = os.MkdirAll(filepath.Dir(proofPath), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(proofPath, append(proof, '\n'), 0644); err != nil {
		return err
	}
	return os.WriteFile(mapPath, append(updated, '\n'), 0644)
}
