// Command verify-chu-runtime is a repeatable whole-match render acceptance.
// Its assertions and executable harness preceded runtimebridge implementation.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/runtimebridge"
)

type report struct {
	Pass            bool   `json:"pass"`
	Complete        bool   `json:"complete"`
	Environment     string `json:"environment"`
	Players         int    `json:"players"`
	Characters      []int  `json:"characters,omitempty"`
	Team            bool   `json:"team"`
	Ticks           uint64 `json:"ticks"`
	Restored        int    `json:"restored_pixel_frames"`
	SpriteFrames    int    `json:"sprite_frames"`
	MaxDraws        int    `json:"max_draw_requests"`
	UnreadyBlocked  bool   `json:"unready_tick_blocked"`
	InvalidAtomic   bool   `json:"invalid_input_atomic"`
	AssetChecked    bool   `json:"asset_version_checked"`
	ReferenceLoaded bool   `json:"reference_state_loaded"`
	WindowComplete  bool   `json:"hud_window_complete"`
	StateSHA        string `json:"final_state_sha256"`
	PixelsSHA       string `json:"pixel_trace_sha256"`
	StateTraceSHA   string `json:"state_trace_sha256"`
}

func readAsset(name string) ([]byte, error) {
	raw, err := os.ReadFile("games/yuyuhakusho/data/" + name + ".json.gz")
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	return io.ReadAll(z)
}
func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func run() error {
	players := flag.Int("players", 2, "active players,2..4")
	selection := flag.String("characters", "", "comma-separated source IDs,0/1/2/3/6/8/9/10; defaults to all9")
	team := flag.Bool("team", false, "four-player teams")
	out := flag.String("output", ".local/md-yuyuhakusho/runtime-preview", "PNG output inside the worktree")
	flag.Parse()
	if *players < 2 || *players > 4 {
		return errors.New("players must be2..4")
	}
	characters := make([]uint8, *players)
	for i := range characters {
		characters[i] = 9
	}
	if *selection != "" {
		parts := strings.Split(*selection, ",")
		if len(parts) != *players {
			return errors.New("one character ID is required for each player")
		}
		for i, part := range parts {
			id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
			if err != nil || (id != 0 && id != 1 && id != 2 && id != 3 && id != 6 && id != 8 && id != 9 && id != 10) {
				return fmt.Errorf("unsupported character ID %q", part)
			}
			characters[i] = uint8(id)
		}
	}
	rules, err := readAsset("chu-world-rules")
	if err != nil {
		return err
	}
	sprites, err := readAsset("world-sprites")
	if err != nil {
		return err
	}
	arena, err := readAsset("arena-assets-default")
	if err != nil {
		return err
	}
	hud, err := readAsset("hud-assets")
	if err != nil {
		return err
	}
	assets, err := runtimebridge.DecodeAssets(rules, sprites, arena, hud)
	if err != nil {
		return err
	}
	options := engine.ChuSessionOptions{Seed: 0x12345678, Difficulty: 6, TeamBattle: *team}
	slots := []uint8{0, 3}
	if *players == 3 {
		slots = []uint8{0, 1, 3}
	} else if *players == 4 {
		slots = []uint8{0, 1, 2, 3}
	}
	for i, slot := range slots {
		options.Players = append(options.Players, engine.ChuSessionPlayer{Slot: slot, CharacterID: characters[i], Color: uint8(i), Computer: true})
	}
	session, err := runtimebridge.NewSession(options, assets)
	if err != nil {
		return err
	}
	r := report{Environment: runtime.GOOS + "/" + runtime.GOARCH, Players: *players, Team: *team}
	if *selection != "" {
		for _, character := range characters {
			r.Characters = append(r.Characters, int(character))
		}
	}
	before, err := session.Save()
	if err != nil {
		return err
	}
	_, err = session.Step([8]engine.Buttons{})
	after, _ := session.Save()
	r.UnreadyBlocked = err != nil && bytes.Equal(before, after)
	if !r.UnreadyBlocked {
		return errors.New("unready rule state advanced")
	}
	if err = session.Ready(); err != nil {
		return err
	}
	before, _ = session.Save()
	_, err = session.Step([8]engine.Buttons{engine.Buttons(0x1000)})
	after, _ = session.Save()
	r.InvalidAtomic = err != nil && bytes.Equal(before, after)
	if !r.InvalidAtomic {
		return errors.New("invalid input was not atomic")
	}
	var changed map[string]any
	if err = json.Unmarshal(before, &changed); err != nil {
		return err
	}
	changed["asset_digests"] = []string{"different", "different", "different"}
	wrong, _ := json.Marshal(changed)
	_, err = runtimebridge.RestoreSession(wrong, assets)
	r.AssetChecked = err != nil
	if !r.AssetChecked {
		return errors.New("asset version binding was ignored")
	}
	if err = os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	pixels, stateTrace := sha256.New(), sha256.New()
	var restored *runtimebridge.Session
	for tick := 1; tick <= 20000; tick++ {
		frame, stepErr := session.Step([8]engine.Buttons{})
		if stepErr != nil {
			return fmt.Errorf("tick%d: %w", tick, stepErr)
		}
		if len(frame.Pixels) != 320*224*4 {
			return errors.New("incomplete RGBA frame")
		}
		pixels.Write(frame.Pixels)
		saved, saveErr := session.Save()
		if saveErr != nil {
			return saveErr
		}
		stateTrace.Write(saved)
		if frame.SpritePixels > 0 {
			r.SpriteFrames++
		}
		if frame.DrawRequests > r.MaxDraws {
			r.MaxDraws = frame.DrawRequests
		}
		if restored != nil && r.Restored < 30 {
			other, restoreErr := restored.Step([8]engine.Buttons{})
			if restoreErr != nil {
				return restoreErr
			}
			otherSave, _ := restored.Save()
			if !bytes.Equal(frame.Pixels, other.Pixels) || !bytes.Equal(saved, otherSave) || !reflect.DeepEqual(frame.View, other.View) {
				return errors.New("saved independent render diverged")
			}
			r.Restored++
		}
		if tick == 600 {
			restored, err = runtimebridge.RestoreSession(saved, assets)
			if err != nil {
				return err
			}
		}
		if tick == 240 || tick == 600 || tick == 1200 || tick == 2400 {
			file, createErr := os.Create(filepath.Join(*out, fmt.Sprintf("%04d.png", tick)))
			if createErr != nil {
				return createErr
			}
			err = png.Encode(file, &image.RGBA{Pix: frame.Pixels, Stride: 320 * 4, Rect: image.Rect(0, 0, 320, 224)})
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		r.Ticks, r.Complete, r.WindowComplete = frame.View.Tick, frame.View.Completed, frame.View.WindowComplete
		if r.Complete {
			r.StateSHA = digest(saved)
			break
		}
	}
	r.PixelsSHA, r.StateTraceSHA = hex.EncodeToString(pixels.Sum(nil)), hex.EncodeToString(stateTrace.Sum(nil))
	r.Pass = r.Complete && r.WindowComplete && r.Restored == 30 && r.SpriteFrames > 100 && r.MaxDraws > 3 && r.UnreadyBlocked && r.InvalidAtomic && r.AssetChecked
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(r); err != nil {
		return err
	}
	if !r.Pass {
		return errors.New("independent render acceptance failed")
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
