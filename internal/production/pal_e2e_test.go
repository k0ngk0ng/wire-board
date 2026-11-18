package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type productionPALClient struct {
	t      *testing.T
	base   string
	client *http.Client
}

func (c *productionPALClient) request(method, path string, body any, status int) map[string]any {
	c.t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		c.t.Fatal(err)
	}
	req, err := http.NewRequest(method, c.base+path, bytes.NewReader(data))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer response.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		c.t.Fatal(path, err)
	}
	if response.StatusCode != status {
		c.t.Fatal(method, path, response.StatusCode, status, result)
	}
	return result
}

func (c *productionPALClient) state() map[string]any {
	return c.request("GET", "/api/state", nil, 200)
}

func (c *productionPALClient) room() map[string]any {
	c.t.Helper()
	room, ok := c.state()["room"].(map[string]any)
	if !ok {
		c.t.Fatal("production PAL room is missing")
	}
	return room
}

func (c *productionPALClient) command(kind string, extra map[string]any) {
	c.t.Helper()
	room := c.room()
	body := map[string]any{"type": kind, "version": room["version"], "nonce": fmt.Sprint(time.Now().UnixNano())}
	for key, value := range extra {
		body[key] = value
	}
	c.request("POST", "/api/rooms/"+room["id"].(string), body, 200)
}

func (c *productionPALClient) ready() {
	c.t.Helper()
	gate, ok := c.room()["loading"].(map[string]any)
	if !ok {
		c.t.Fatal("production PAL loading gate is missing")
	}
	for _, kind := range []string{"load_begin", "load_ready"} {
		c.command(kind, map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": "production-pal-client"})
	}
}

func (c *productionPALClient) reading(previous any) map[string]any {
	c.t.Helper()
	for until := time.Now().Add(30 * time.Second); time.Now().Before(until); {
		data := c.room()["game"].(map[string]any)["data"].(map[string]any)
		if dialogue, ok := data["dialogue"].(map[string]any); ok && (previous == nil || dialogue["id"] != previous) {
			return data
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatal("original production PAL reading did not arrive")
	return nil
}

func TestPALProductionCompositionAndProcessRestart(t *testing.T) {
	if os.Getenv("PAL_PRODUCTION_APP") != "1" {
		t.Skip("explicit production PAL composition and actual process save/restart")
	}
	if runtime.CompiledRulesBuildID == "" || runtime.CompiledRuntimeBuildID == "" {
		t.Fatal("actual verified application runtime identities are required")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, ".local/pal/server-packages/a9234deb9d91a84e0d273f0d0ef3e90e81a7fbdf052c41ed4fcb0c91a35566ac")
	if configured := os.Getenv("PAL_PRODUCTION_RULES"); configured != "" {
		folder = configured
		if !filepath.IsAbs(folder) {
			folder = filepath.Join(root, folder)
		}
	}
	scope := os.Getenv("PAL_PRODUCTION_APP_SCOPE")
	if scope != "" && !regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`).MatchString(scope) {
		t.Fatal("invalid original production evidence scope", scope)
	}
	previousFolder := os.Getenv("PAL_PRODUCTION_OLD_RULES")
	if previousFolder != "" {
		if scope == "" {
			t.Fatal("an upgrade requires isolated evidence")
		}
		if !filepath.IsAbs(previousFolder) {
			previousFolder = filepath.Join(root, previousFolder)
		}
	}
	if failed, err := Open(context.Background(), fstest.MapFS{}); err == nil || failed != nil || !strings.Contains(err.Error(), "pal") {
		t.Fatal("missing PAL bundle did not reject the entire production composition", err)
	}
	registry, err := Open(context.Background(), os.DirFS(folder))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { registry.Close(context.Background()) })
	seen := map[string]bool{}
	var pal gameapi.Module
	for _, module := range registry.Modules {
		if seen[module.ID] {
			t.Fatal("duplicate production module", module.ID)
		}
		seen[module.ID] = true
		if module.ID == "pal" {
			pal = module
		}
	}
	if !seen["pal"] || !seen["contra"] || pal.MinPlayers != 1 || pal.MaxPlayers != 1 || !pal.SupportsSave || !pal.SupportsCheckpoint || pal.Controls == nil || pal.Controls.Layout != "game" {
		t.Fatal("production PAL registration lost its single-player/save/controller contract")
	}
	retained := false
	manifestBytes, err := os.ReadFile(filepath.Join(folder, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Retained []string `json:"retained"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, core := range manifest.Retained {
		if core != "7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a" {
			continue
		}
		saved, err := os.ReadFile(filepath.Join(root, ".local/pal/verification/campaign-tournament-state-1.json"))
		if err != nil {
			t.Fatal(err)
		}
		before := bytes.Clone(saved)
		state, err := pal.ResumeSaved(gameapi.Context{Players: 1, Now: time.Unix(1800000000, 0)}, gameapi.Match{Schema: 1, Data: saved})
		if err != nil {
			t.Fatal("production registry did not select the retained legacy core", err)
		}
		projected, err := pal.View(state, 0)
		if err != nil || !bytes.Equal(before, saved) {
			t.Fatal("retained legacy original save changed", err)
		}
		data, _ := json.Marshal(projected)
		var value struct {
			Scene int `json:"scene"`
			Party []struct {
				Role int `json:"role"`
			} `json:"party"`
		}
		if err := json.Unmarshal(data, &value); err != nil || value.Scene != 40 || len(value.Party) != 2 || value.Party[0].Role != 2 || value.Party[1].Role != 0 {
			t.Fatal("retained original manor ending differs", err, value)
		}
		retained = true
	}
	if err := registry.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(context.Background()); err != nil {
		t.Fatal("repeated production registry close", err)
	}
	if _, err := pal.Start(gameapi.Context{Now: time.Now(), Players: 1, Actor: 0, Version: 1}, nil); err == nil {
		t.Fatal("closed production PAL runtime still starts games")
	}
	scratch := t.TempDir()
	binary := filepath.Join(scratch, "wire-board")
	flags := "-X github.com/k0ngk0ng/wire-board/games/contra/runtime.CompiledRulesBuildID=" + runtime.CompiledRulesBuildID + " -X github.com/k0ngk0ng/wire-board/games/contra/runtime.CompiledRuntimeBuildID=" + runtime.CompiledRuntimeBuildID
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags="+flags, "-o", binary, ".")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatal("production application build", err, string(output))
	}
	install := func(from string) {
		t.Helper()
		if err := os.RemoveAll(filepath.Join(scratch, "pal")); err != nil {
			t.Fatal(err)
		}
		if err := fs.WalkDir(os.DirFS(from), ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			path := filepath.Join(scratch, "pal", filepath.FromSlash(name))
			if entry.IsDir() {
				return os.MkdirAll(path, 0755)
			}
			data, err := os.ReadFile(filepath.Join(from, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			return os.WriteFile(path, data, 0600)
		}); err != nil {
			t.Fatal(err)
		}
	}
	var upgradeFrom string
	if previousFolder != "" {
		type rulesManifest struct {
			Current  string   `json:"current"`
			Retained []string `json:"retained"`
		}
		previousData, err := os.ReadFile(filepath.Join(previousFolder, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var origin, target rulesManifest
		if err := json.Unmarshal(previousData, &origin); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(manifestBytes, &target); err != nil {
			t.Fatal(err)
		}
		if origin.Current == target.Current || !slices.Contains(target.Retained, origin.Current) {
			t.Fatal("upgraded rules must retain the old source runner")
		}
		upgradeFrom = origin.Current
		install(previousFolder)
	} else {
		install(folder)
	}
	database := filepath.Join(scratch, "database")
	missing := exec.Command(binary)
	missing.Env = append(os.Environ(), "DATA_DIR="+database, "ADDR=invalid listen address", "PAL_RULES_DIR="+filepath.Join(scratch, "missing"))
	if output, err := missing.CombinedOutput(); err == nil || !strings.Contains(string(output), "initialize pal") {
		t.Fatal("application did not reject missing PAL rules before startup", err, string(output))
	}
	if _, err := os.Stat(database); !os.IsNotExist(err) {
		t.Fatal("missing PAL rules created the application database", err)
	}
	start := func() (string, func()) {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		listener.Close()
		command := exec.Command(binary)
		command.Dir = scratch
		command.Env = append(os.Environ(), "DATA_DIR="+database, "ADDR="+address, "PAL_RULES_DIR=", "INVITE_CODE=production-pal-test")
		log, err := os.CreateTemp(scratch, "application-log-")
		if err != nil {
			t.Fatal(err)
		}
		command.Stdout, command.Stderr = log, log
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		closed := false
		stop := func() {
			if closed {
				return
			}
			closed = true
			command.Process.Signal(os.Interrupt)
			if err := command.Wait(); err != nil {
				t.Error("production process shutdown", err)
			}
			log.Close()
		}
		t.Cleanup(stop)
		base := "http://" + address
		for until := time.Now().Add(20 * time.Second); time.Now().Before(until); {
			response, err := http.Get(base + "/healthz")
			if err == nil {
				response.Body.Close()
				if response.StatusCode == 200 {
					return base, stop
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		data, _ := os.ReadFile(log.Name())
		t.Fatal("production application did not start", string(data))
		return "", stop
	}
	base, stop := start()
	newClient := func(name string) *productionPALClient {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client := &productionPALClient{t: t, base: base, client: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
		client.request("POST", "/api/register", map[string]any{"name": name, "password": "production-pal-test-123", "invite": "production-pal-test"}, 200)
		return client
	}
	owner, stranger := newClient("PALOwner"), newClient("PALStranger")
	controls := owner.request("GET", "/api/controllers?game=pal", nil, 200)
	models, ok := controls["models"].([]any)
	if !ok || len(models) != 1 {
		t.Fatal("missing production PAL controller model", controls)
	}
	model := models[0].(map[string]any)
	if model["gameId"] != "pal" || model["layout"] != "game" {
		t.Fatal("unexpected production PAL controller model", model)
	}
	keys := controls["profiles"].(map[string]any)["game:pal"].(map[string]any)["keyboard"].(map[string]any)
	for action, key := range map[string]string{"up": "ArrowUp", "down": "ArrowDown", "left": "ArrowLeft", "right": "ArrowRight", "confirm": "Enter", "cancel": "Backspace"} {
		if keys[action] != key {
			t.Fatal("missing production PAL keyboard mapping", action, keys[action])
		}
	}
	room := owner.request("POST", "/api/rooms", map[string]any{"kind": "pal", "name": "Production PAL Save", "capacity": 1}, 201)
	id := room["id"].(string)
	owner.command("ready", nil)
	owner.command("start", nil)
	owner.ready()
	before := owner.reading(nil)
	oldPage := owner.room()
	owner.command("save", nil)
	if owner.state()["room"] != nil || len(stranger.state()["saves"].([]any)) != 0 {
		t.Fatal("production save did not release its owner privately")
	}
	stranger.request("POST", "/api/saves/"+id+"/resume", map[string]any{}, 404)
	stop()
	if previousFolder != "" {
		install(folder)
	}
	base, stop = start()
	defer stop()
	owner.base, stranger.base = base, base
	owner.request("POST", "/api/saves/"+id+"/resume", map[string]any{}, 200)
	owner.ready()
	after := owner.reading(nil)
	for _, key := range []string{"dialogue", "party", "cash", "scene", "map", "x", "y"} {
		if !reflect.DeepEqual(before[key], after[key]) {
			t.Fatal("production restart changed original PAL state", key)
		}
	}
	if owner.room()["matchId"] != oldPage["matchId"] || before["epoch"] == after["epoch"] {
		t.Fatal("production PAL match or input boundary was not restored")
	}
	prompt := before["dialogue"].(map[string]any)["id"]
	owner.request("POST", "/api/rooms/"+id, map[string]any{"type": "action", "version": oldPage["version"], "nonce": "old-production-pal-page", "action": map[string]any{"type": "read", "epoch": before["epoch"], "prompt": prompt}}, 409)
	owner.command("action", map[string]any{"action": map[string]any{"type": "read", "epoch": after["epoch"], "prompt": prompt}})
	advanced := owner.reading(prompt)
	owner.command("load_checkpoint", nil)
	owner.ready()
	loaded := owner.reading(nil)
	if !reflect.DeepEqual(loaded["dialogue"], before["dialogue"]) || loaded["epoch"].(float64) <= advanced["epoch"].(float64) {
		t.Fatal("production application manual checkpoint was not restored")
	}
	result := map[string]any{"passed": true, "scope": "actual production registry and compiled application processes; missing PAL rules fail before database, original single-player capability, shutdown, authenticated SQLite save/restart/resume, stranger rejection, load gate, old page rejection and manual checkpoint; not final package/image/client-payload boundary or complete campaign acceptance", "moduleCount": len(seen), "registeredPAL": true, "processRestart": true, "privateSave": true, "manualCheckpoint": true, "retainedOriginalStage4": retained}
	path := ".local/pal/verification/production-application.json"
	if scope != "" {
		path = ".local/pal/verification/production-application-" + scope + ".json"
		result["upgradedFrom"] = upgradeFrom
	}
	report, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(filepath.Join(root, path), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
