package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing/fstest"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func main() {
	public := flag.String("public", "", "private generated browser directory")
	resources := flag.String("resources", "", "current-stage resource JSON")
	data := flag.String("data", "", "private platform profile DB")
	urlFile := flag.String("url-file", "", "write bound localhost URL")
	flag.Parse()
	raw, err := os.ReadFile(*resources)
	if err != nil {
		log.Fatal(err)
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		log.Fatal(err)
	}
	game, err := runtime.NewBundleDevelopmentHandler(bundle.Manifest(), raw)
	if err != nil {
		log.Fatal(err)
	}
	platform, err := server.New(server.Config{DataDir: *data, InviteCode: "contra-browser"}, fstest.MapFS{"index.html": {Data: []byte("development controller host")}})
	if err != nil {
		log.Fatal(err)
	}
	defer platform.Close()
	mux := http.NewServeMux()
	mux.Handle("/runtime/", http.StripPrefix("/runtime", game))
	mux.Handle("/api/", platform.Handler())
	mux.Handle("/", http.FileServer(http.Dir(*public)))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*urlFile, []byte("http://"+listener.Addr().String()), 0600); err != nil {
		log.Fatal(err)
	}
	stop := make(chan struct{})
	go game.Run(stop)
	httpServer := &http.Server{Handler: mux}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Print(err)
		}
	}()
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	close(stop)
	_ = httpServer.Close()
}
