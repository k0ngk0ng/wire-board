package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestShutdownWaitsForInFlightRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	entered, release, stopping := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "action saved")
	})}
	defer srv.Close()
	srv.RegisterOnShutdown(func() { close(stopping) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exited := make(chan error, 1)
	go func() { exited <- serve(ctx, srv, listener) }()
	response := make(chan string, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		res, err := client.Post("http://"+listener.Addr().String(), "application/json", nil)
		if err != nil {
			response <- err.Error()
			return
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		response <- string(body)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-stopping:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not begin")
	}
	select {
	case err := <-exited:
		t.Fatalf("exited before action completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	select {
	case body := <-response:
		if body != "action saved" {
			t.Fatal(body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request lost during shutdown")
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not finish")
	}
}
