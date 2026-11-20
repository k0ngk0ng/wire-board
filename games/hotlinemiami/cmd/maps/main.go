package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

func main() {
	check := flag.Bool("check", false, "检查原版规则生成结果")
	flag.Parse()
	if err := generate(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(check bool) error {
	source := filepath.Join("games", "hotlinemiami", "data", "maps.json")
	target := filepath.Join("games", "hotlinemiami", "engine", "maps_native.go")
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	var maps []engine.Map
	if err = json.Unmarshal(raw, &maps); err != nil {
		return err
	}
	if len(maps) != 3 {
		return fmt.Errorf("应包含三张原版规则地图")
	}
	buffer := &bytes.Buffer{}
	fmt.Fprintln(buffer, "package engine\n\nvar recoveredMaps = []Map{")
	for _, world := range maps {
		fmt.Fprintf(buffer, "{ID:%q,Name:%q,Width:%d,Height:%d,\n", world.ID, world.Name, world.Width*engine.Unit, world.Height*engine.Unit)
		fmt.Fprintln(buffer, "Walls:[]Rect{")
		for _, wall := range world.Walls {
			fmt.Fprintf(buffer, "{X:%d,Y:%d,W:%d,H:%d,Kind:%q},\n", wall.X*engine.Unit, wall.Y*engine.Unit, wall.W*engine.Unit, wall.H*engine.Unit, wall.Kind)
		}
		fmt.Fprintln(buffer, "},Spawns:[]Point{")
		for _, point := range world.Spawns {
			fmt.Fprintf(buffer, "{X:%d,Y:%d},\n", point.X*engine.Unit, point.Y*engine.Unit)
		}
		fmt.Fprintln(buffer, "},WeaponSpawns:[]WeaponSpawn{")
		for _, point := range world.WeaponSpawns {
			fmt.Fprintf(buffer, "{X:%d,Y:%d,Weapon:%q},\n", point.X*engine.Unit, point.Y*engine.Unit, point.Weapon)
		}
		fmt.Fprintln(buffer, "},Doors:[]DoorDef{")
		for _, door := range world.Doors {
			fmt.Fprintf(buffer, "{ID:%d,X:%d,Y:%d,W:%d,H:%d},\n", door.ID, door.X*engine.Unit, door.Y*engine.Unit, door.W*engine.Unit, door.H*engine.Unit)
		}
		fmt.Fprintln(buffer, "}},")
	}
	fmt.Fprintln(buffer, "}")
	generated, err := format.Source(buffer.Bytes())
	if err != nil {
		return err
	}
	if check {
		current, readErr := os.ReadFile(target)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(current, generated) {
			return fmt.Errorf("地图规则已变化，请运行 go run ./games/hotlinemiami/cmd/maps")
		}
		return nil
	}
	return os.WriteFile(target, generated, 0644)
}
