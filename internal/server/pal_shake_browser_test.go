package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func palStoryShake(v pal.View) (int, int) {
	g := v.Raster
	if g == nil || g.Root < 1 || g.Root > len(g.Nodes) {
		return 0, 0
	}
	fill := g.Nodes[g.Root-1]
	if fill[0] != 4 || fill[1] < 1 || fill[1] > len(g.Nodes) || fill[5] != 0 || fill[7] != 320 || fill[9] != 0 {
		return 0, 0
	}
	copy := g.Nodes[fill[1]-1]
	if copy[0] != 8 || copy[1] < 1 || copy[3] != 320 || copy[4] != 200 || copy[5] != 0 || copy[7] != 320 || copy[9] != 0 || copy[11] != 320 {
		return 0, 0
	}
	shift := copy[10] - copy[6]
	level := max(copy[6], copy[10])
	if level < 1 || level >= 200 || copy[8] != 200-level || copy[12] != 200-level || min(copy[6], copy[10]) != 0 || fill[8] != level || (shift < 0 && fill[6] != 200-level) || (shift > 0 && fill[6] != 0) {
		return 0, 0
	}
	return shift, copy[1]
}

func TestPALGenuinePrayerPhysicalShakeAndSavedContinuation(t *testing.T) {
	if os.Getenv("PAL_PRAYER_SHAKE") != "1" {
		t.Skip("explicit accepted genuine prayer physical-shake continuation")
	}
	module, source := palCampaignBrowserModule(t, "earth-beast-arrival")
	sourceSHA := sha256.Sum256(source)
	out := filepath.Join("../..", ".local/pal/verification/prayer-shake-story")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err == nil {
			err = os.WriteFile(filepath.Join(out, name), append(raw, '\n'), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	c := gameapi.Context{Now: time.Now(), Players: 1, Actor: 0, Version: 1}
	match, err := module.Start(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	frames := []map[string]any{}
	directions, resumed := map[int]int{}, map[int]bool{}
	pages, callbacks, arrived := 0, 0, false
	for steps := 0; steps < 6000; steps++ {
		projected, err := module.View(match, 0)
		if err != nil {
			t.Fatal(err)
		}
		v := projected.(pal.View)
		if shift, logical := palStoryShake(v); shift != 0 {
			directions[shift]++
			if directions[shift] <= 4 {
				frames = append(frames, map[string]any{"label": fmt.Sprintf("scene-%d-shift-%d-frame-%d", v.Scene, shift, directions[shift]), "shift": shift, "logicalRoot": logical, "view": v})
			}
			if !resumed[shift] {
				frozen, err := module.SaveControl(c, match, true)
				if err != nil {
					t.Fatal("freeze genuine physical frame", err)
				}
				write(fmt.Sprintf("shift-%d-frozen-match.json", shift), frozen)
				fresh, _ := palCampaignBrowserModule(t, "earth-beast-arrival")
				match, err = fresh.ResumeSaved(c, frozen)
				if err != nil {
					t.Fatal("fresh runtime physical frame resume", err)
				}
				value, err := fresh.View(match, 0)
				if err != nil {
					t.Fatal(err)
				}
				restored := value.(pal.View)
				if !reflect.DeepEqual(v.Raster, restored.Raster) || !reflect.DeepEqual(v.Party, restored.Party) || v.Cash != restored.Cash || v.Scene != restored.Scene || v.X != restored.X || v.Y != restored.Y {
					t.Fatal("genuine saved physical frame changed its pixels, resources or logical position")
				}
				module, resumed[shift] = fresh, true
			}
		}
		if v.Scene == 232 && v.Dialogue != nil && strings.Contains(v.Dialogue.Text, "黑苗人竟然") {
			arrived = true
			break
		}
		var action *pal.Action
		if v.Dialogue != nil {
			action = &pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID}
			pages++
		}
		if action != nil {
			raw, err := json.Marshal(action)
			if err != nil {
				t.Fatal(err)
			}
			match, err = module.Apply(c, match, raw)
			if err != nil {
				t.Fatal("normal original input", err)
			}
		} else {
			if match.SystemDeadline == 0 {
				t.Fatal("genuine prayer lost its original scheduled continuation", v.Scene, v.Phase)
			}
			c.Now = time.UnixMilli(match.SystemDeadline)
			match, err = module.OnTimer(c, match)
			if err != nil {
				t.Fatal("normal original timer", err)
			}
			callbacks++
		}
		c.Version++
	}
	passed := arrived && len(frames) >= 16
	for _, shift := range []int{-5, -4, -2, -1, 1, 2, 4, 5} {
		passed = passed && directions[shift] >= 2 && resumed[shift]
	}
	write("frames.json", map[string]any{"passed": passed, "physicalBrowserAccepted": false, "scope": "accepted same-core actual priestess invitation through original earth-beast arrival, final display shift geometry and fresh-runtime save at each direction; browser pixels verified separately", "entrySHA256": hex.EncodeToString(sourceSHA[:]), "pages": pages, "callbacks": callbacks, "directions": directions, "resumed": resumed, "frames": frames})
	if !passed {
		t.Fatal("genuine prayer physical-shake frames or continuation missing", arrived, directions, resumed)
	}
}

func TestPALGenuineShakeBoardBrowserFixture(t *testing.T) {
	if os.Getenv("PAL_SHAKE_BOARD_BROWSER") != "1" {
		t.Skip("explicit genuine campaign physical-shake platform browser fixture")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/pal/prayer-shake-board-browser")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(out, "stop")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	module, source := palCampaignBrowserModule(t, "earth-beast-arrival")
	if err := os.WriteFile(filepath.Join(out, "entry-state.json"), source, 0600); err != nil {
		t.Fatal(err)
	}
	writeView := func(name string, match gameapi.Match) error {
		value, err := module.View(match, 0)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, name), raw, 0600)
	}
	freeze, resume := module.SaveControl, module.ResumeSaved
	module.SaveControl = func(c gameapi.Context, g gameapi.Match, paused bool) (gameapi.Match, error) {
		next, err := freeze(c, g, paused)
		if err == nil && paused {
			err = writeView("frozen-view.json", next)
		}
		return next, err
	}
	module.ResumeSaved = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
		next, err := resume(c, g)
		if err == nil {
			err = writeView("restored-view.json", next)
		}
		return next, err
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/pal/v1/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/pal-shake-probe.js", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, filepath.Join(out, "probe.js")) })
	mux.Handle("/", s.Handler())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if err := os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for until := time.Now().Add(15 * time.Minute); time.Now().Before(until); {
		if _, err := os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("genuine shake browser fixture timed out")
}
