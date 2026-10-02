package main

import (
	"context"
	"embed"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed all:web/dist
var assets embed.FS

func main() {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "data"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	files, e := fs.Sub(assets, "web/dist")
	if e != nil {
		log.Fatal(e)
	}
	app, e := server.New(server.Config{DataDir: dir, InviteCode: os.Getenv("INVITE_CODE"), Origin: os.Getenv("PUBLIC_ORIGIN"), SecureCookie: os.Getenv("COOKIE_SECURE") == "true"}, files)
	if e != nil {
		log.Fatal(e)
	}
	defer app.Close()
	srv := &http.Server{Addr: addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16384}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("Wire Board is listening on %s", addr)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
