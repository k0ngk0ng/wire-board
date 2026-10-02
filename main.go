package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/server"
)

//go:embed all:web/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "data"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	files, err := fs.Sub(assets, "web/dist")
	if err != nil {
		return err
	}
	app, err := server.New(server.Config{DataDir: dir, InviteCode: os.Getenv("INVITE_CODE"), Origin: os.Getenv("PUBLIC_ORIGIN"), AssetsBaseURL: os.Getenv("ASSETS_BASE_URL"), SecureCookie: os.Getenv("COOKIE_SECURE") == "true"}, files)
	if err != nil {
		return err
	}
	defer app.Close()
	srv := &http.Server{Addr: addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16384}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("Wire Board is listening on %s", listener.Addr())
	return serve(ctx, srv, listener)
}

// Shutdown closes the listener immediately, but Serve returning does NOT mean
// in-flight requests have completed. Await Shutdown before closing the database.
func serve(ctx context.Context, srv *http.Server, listener net.Listener) error {
	result := make(chan error, 1)
	go func() { result <- srv.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			_ = srv.Close()
			return err
		}
		return nil
	}
}
