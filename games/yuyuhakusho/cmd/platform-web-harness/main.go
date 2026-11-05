// Local-only browser fixture around the actual platform module. Authentication
// and WebSockets are deliberately outside this harness's acceptance scope.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	yuyu "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func main() {
	build := flag.String("build", "", "local browser build directory")
	root := flag.String("root", "", "repository root")
	flag.Parse()
	mod := yuyu.Module()
	var mu sync.Mutex
	var match gameapi.Match
	version := 1
	context := func(actor int) gameapi.Context {
		return gameapi.Context{Actor: actor, Players: 2, Now: time.Now(), Version: version, RequestedVersion: version, Automated: []bool{false, true}}
	}
	restart := func(raw []byte) error {
		next, err := mod.Start(context(-1), raw)
		if err == nil {
			match = next
			version++
		}
		return err
	}
	if err := restart([]byte(`{"arena":0,"teamBattle":false,"characters":[3,9]}`)); err != nil {
		log.Fatal(err)
	}
	go func() {
		timer := time.NewTicker(20 * time.Millisecond)
		defer timer.Stop()
		for range timer.C {
			mu.Lock()
			if match.SystemDeadline != 0 && match.SystemDeadline <= time.Now().UnixMilli() {
				next, err := mod.OnTimer(context(-1), match)
				if err == nil {
					match = next
					version++
				} else {
					log.Print(err)
				}
			}
			mu.Unlock()
		}
	}()
	jsonReply := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(value); err != nil {
			log.Print(err)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		view, err := mod.View(match, 0)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		jsonReply(w, map[string]any{"version": version, "serverNow": time.Now().UnixMilli(), "data": map[string]any{"view": view}})
	})
	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		data, err := mod.SyncSnapshot(match, 0)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		jsonReply(w, map[string]any{"roomId": "board-harness", "version": version, "serverInstance": "local-module-harness", "serverNow": time.Now().UnixMilli(), "data": data})
	})
	mux.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 2048))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if err = restart(raw); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		jsonReply(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/act", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 2048))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		delay, _ := strconv.Atoi(r.URL.Query().Get("delay"))
		if delay > 0 && delay <= 2000 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		next, err := mod.Apply(context(0), match, raw)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		match = next
		version++
		jsonReply(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/yuyuhakusho/data/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/yuyuhakusho/data/")
		if name == "" || filepath.Base(name) != name || !(strings.HasSuffix(name, ".json.gz") || name == "runtime-resource-descriptor.json") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(*root, "games/yuyuhakusho/data", name))
	})
	mux.HandleFunc("/yuyuhakusho/v1/runtime/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/yuyuhakusho/v1/runtime/")
		if name != "build.json" && name != "client.wasm" && name != "wasm_exec.js" {
			http.NotFound(w, r)
			return
		}
		if name == "client.wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
		http.ServeFile(w, r, filepath.Join(*build, name))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			name := strings.TrimPrefix(r.URL.Path, "/")
			if filepath.Base(name) != name {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(*build, name))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>魔强统一战 Board 验收</title><link rel="stylesheet" href="/harness.css"><style>*{box-sizing:border-box}body{margin:0;background:#111815;color:#e6e5d7;font-family:system-ui}button{padding:8px;margin:6px;background:#29382d;color:inherit;border:1px solid #656c4c;border-radius:6px}#public-players{display:flex;gap:20px;padding:10px}#root{min-width:0}</style><div id="root"></div><script type="module" src="/harness.js"></script></html>`)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("http://" + listener.Addr().String() + "/")
	if err = http.Serve(listener, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
