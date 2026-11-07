// Render a saved independent Session for development diagnosis. Only its
// submitted picture reaches the renderer; no policy or rule tick is executed.
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	state := flag.String("state", "", "saved Session JSON")
	output := flag.String("out", "", "PNG path inside current workspace")
	flag.Parse()
	if err := run(*state, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(state, output string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, destination)
	if state == "" || output == "" || err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("state and workspace-local output are required")
	}
	raw, err := os.ReadFile(state)
	if err != nil {
		return err
	}
	var session engine.Session
	if err = json.Unmarshal(raw, &session); err != nil {
		return err
	}
	if err = session.Validate(); err != nil {
		return err
	}
	file, err := os.Open("games/contra/engine/testdata/bot-visual-catalog.json.gz")
	if err != nil {
		return err
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer z.Close()
	var source struct {
		Catalog engine.BotVisualCatalogManifest
		RGB     []byte
		Pages   map[uint8][]byte
	}
	if err = json.NewDecoder(z).Decode(&source); err != nil {
		return err
	}
	catalog, err := engine.NewVerifiedBotVisualCatalog(source.Catalog, source.RGB, func(id uint8) ([]byte, error) { return source.Pages[id], nil })
	if err != nil {
		return err
	}
	frame, err := engine.NewBotVisualFrame(session.World.Active.Presentation.Snapshot(), engine.BotPublicHUD{Seats: 1}, catalog, 0, session.Random.Frames)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	picture := &image.RGBA{Pix: frame.Pixels(), Stride: 256 * 4, Rect: image.Rect(0, 0, 256, 240)}
	if err = png.Encode(out, picture); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	fmt.Printf("rendered submitted frame %d revision %d to %s\n", session.Random.Frames, session.Revision, relative)
	return nil
}
